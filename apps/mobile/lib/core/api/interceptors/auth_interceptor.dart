import 'dart:async';

import 'package:dio/dio.dart';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';

/// Callback invoked by [AuthInterceptor] when a 401 response is received
/// AND the subsequent Firebase token refresh attempt fails. Wired by the
/// auth domain to [AuthController.handleSessionExpired] so a clean
/// logout / route-to-/welcome can occur instead of leaving the user
/// inside an authenticated shell where every API call returns 401.
typedef SessionExpiredCallback = FutureOr<void> Function();

/// Test-only seam for fetching a Firebase ID token without touching
/// [FirebaseAuth.instance]. Production wiring uses
/// `FirebaseAuth.instance.currentUser?.getIdToken(forceRefresh)`.
typedef IdTokenFetcher = Future<String?> Function(bool forceRefresh);

/// Test-only seam for replaying the original request after a successful
/// token refresh. Production wiring uses a fresh `Dio()` instance via
/// `Dio().fetch(...)` to avoid recursing through this interceptor.
typedef RequestRetrier =
    Future<Response<dynamic>> Function(RequestOptions options);

/// Interceptor that attaches Firebase Auth token to all requests
///
/// Automatically:
/// - Gets the current Firebase ID token (with force refresh if needed)
/// - Attaches it as Bearer token in Authorization header
/// - Handles token refresh when 401 received
/// - On refresh failure, signals session-expired to the auth domain via
///   [onSessionExpired] so the app routes cleanly to /welcome instead of
///   silently 401-looping inside an authenticated shell
/// - Logs all token operations for debugging
class AuthInterceptor extends Interceptor {
  final ILoggerService? _logger;
  // Nullable so that tests injecting a [_tokenFetcher] do not have to
  // boot Firebase. In production [_tokenFetcher] is null and the
  // constructor falls back to [FirebaseAuth.instance], so this field
  // is always non-null on the production code path.
  final FirebaseAuth? _firebaseAuth;
  final IdTokenFetcher? _tokenFetcher;
  final RequestRetrier? _requestRetrier;

  /// Process-wide session-expired sink. The auth domain registers a
  /// listener (typically `AuthController.handleSessionExpired`) during
  /// app bootstrap. Null = no listener attached (e.g. unit tests).
  ///
  /// Made a static mutable field rather than a constructor parameter
  /// because [AuthInterceptor] is instantiated inside [ApiClient] at
  /// HTTP-layer construction time, while the auth domain that owns the
  /// session-expired response sits above it. The static is reset by
  /// tests via [setSessionExpiredCallbackForTest].
  static SessionExpiredCallback? onSessionExpired;

  /// Guard that prevents the same logout signal from firing once per
  /// in-flight 401 burst (e.g. if 10 parallel requests all 401 at the
  /// same time, we only need to signal session expiry once). Reset on
  /// the next successful token attach in [onRequest] — that signals a
  /// fresh re-authenticated session and re-arms the detector.
  static bool _sessionExpirySignaled = false;

  AuthInterceptor({
    ILoggerService? logger,
    FirebaseAuth? firebaseAuth,
    IdTokenFetcher? tokenFetcher,
    RequestRetrier? requestRetrier,
  }) : _logger = logger,
       // Skip FirebaseAuth.instance when a token-fetcher seam is in
       // play (test path). Production always supplies neither, so the
       // fallback grabs the singleton.
       _firebaseAuth =
           firebaseAuth ??
           (tokenFetcher != null ? null : FirebaseAuth.instance),
       _tokenFetcher = tokenFetcher,
       _requestRetrier = requestRetrier;

  /// Resets the process-wide session-expiry guard. Called by the auth
  /// domain when a fresh authenticated session is established, so that
  /// the next 401-refresh-failure burst is detected again.
  static void resetSessionExpiryGuard() {
    _sessionExpirySignaled = false;
  }

  /// Visible only for tests — overrides the static callback in a way
  /// that returns a disposer so the test can restore prior state.
  static void Function() setSessionExpiredCallbackForTest(
    SessionExpiredCallback? callback,
  ) {
    final previous = onSessionExpired;
    final previousGuard = _sessionExpirySignaled;
    onSessionExpired = callback;
    _sessionExpirySignaled = false;
    return () {
      onSessionExpired = previous;
      _sessionExpirySignaled = previousGuard;
    };
  }

  /// Internal: fetch an ID token via the injected seam if any, else
  /// fall back to `FirebaseAuth.instance.currentUser.getIdToken(...)`.
  Future<String?> _getToken(bool forceRefresh) async {
    final fetcher = _tokenFetcher;
    if (fetcher != null) {
      return fetcher(forceRefresh);
    }
    // Production path: _firebaseAuth is guaranteed non-null when
    // _tokenFetcher is null (see constructor).
    final user = _firebaseAuth!.currentUser;
    if (user == null) return null;
    return user.getIdToken(forceRefresh);
  }

  /// Internal: invoke the session-expired callback at most once per
  /// 401 burst. Swallows callback errors so the request error chain is
  /// unaffected.
  Future<void> _signalSessionExpiredOnce(String reason) async {
    if (_sessionExpirySignaled) {
      _logger?.debug(
        'AuthInterceptor: session-expired already signaled, '
        'skipping duplicate (reason=$reason)',
      );
      return;
    }
    _sessionExpirySignaled = true;
    _logger?.warning('AuthInterceptor: signaling session expired — $reason');
    final callback = onSessionExpired;
    if (callback == null) {
      _logger?.warning(
        'AuthInterceptor: no onSessionExpired listener registered — '
        'session-expired signal will not force logout',
      );
      return;
    }
    try {
      await callback();
    } catch (e, stackTrace) {
      _logger?.error(
        'AuthInterceptor: onSessionExpired callback threw — $e',
        stackTrace: stackTrace,
      );
    }
  }

  @override
  Future<void> onRequest(
    RequestOptions options,
    RequestInterceptorHandler handler,
  ) async {
    if (_shouldSkipAuth(options)) {
      _logger?.debug(
        'AuthInterceptor: skipping auth for ${options.method} ${options.path}',
      );
      return handler.next(options);
    }

    // Skip auth for public endpoints
    final isPublic = _isPublicEndpoint(options.path, method: options.method);

    if (isPublic) {
      _logger?.debug(
        'AuthInterceptor: Skipping auth for public endpoint: ${options.path}',
      );
      return handler.next(options);
    }

    // 🔍 COLD START AUDIT: Log each request

    try {
      // Get current Firebase ID token
      // 🔧 FIX: Force refresh=true for critical endpoints to ensure fresh token
      final forceRefresh = _shouldForceRefreshToken(options.path);

      // When using the test seam there is no FirebaseAuth currentUser to
      // pre-check; the fetcher is the only token source. In production,
      // a null currentUser means we go to the backend unauthenticated.
      if (_tokenFetcher == null && _firebaseAuth!.currentUser == null) {
        _logger?.warning(
          'AuthInterceptor: No Firebase user found for ${options.path}',
        );
        // Continue without token - backend will return 401
        return handler.next(options);
      }

      // Get token with optional force refresh
      final token = await _getToken(forceRefresh);

      // Security: Token obtained - NOT logged for security

      if (token != null && token.isNotEmpty) {
        options.headers['Authorization'] = 'Bearer $token';

        // Successful token attach = a live, valid session for this
        // request. Re-arm the session-expiry detector so the next
        // 401-refresh-failure burst signals a logout again. Without
        // this reset, a re-login after a prior session-expired event
        // would never trigger expiry detection again.
        _sessionExpirySignaled = false;

        // 🌍 PROOF: Log the Authorization header being attached
        // Security: Authorization header SET - NOT logged for security
        // Security: Authorization value - NOT logged for security
        // Security: Full headers after auth - NOT logged for security

        _logger?.debug(
          'AuthInterceptor: Token attached for ${options.method} ${options.path} '
          '(token length: ${token.length}, forceRefresh: $forceRefresh)',
        );
      } else {
        // Security: Got null/empty token - NOT logged for security
        _logger?.warning(
          'AuthInterceptor: Got null/empty token from Firebase for ${options.path}',
        );
      }
    } catch (e, stackTrace) {
      _logger?.error(
        'AuthInterceptor: Error getting token for ${options.path} - $e',
        stackTrace: stackTrace,
      );
      // Continue without token - let the request fail naturally
    }

    handler.next(options);
  }

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    if (_shouldSkipAuth(err.requestOptions)) {
      return handler.next(err);
    }

    // 🔧 FIX: Handle 401 with token refresh and retry
    if (err.response?.statusCode == 401) {
      _logger?.warning(
        'AuthInterceptor: Received 401 for ${err.requestOptions.path} - attempting token refresh',
      );

      // Don't attempt refresh or signal session-expired for public browse
      // endpoints. A 401 on a public route means the client sent a malformed
      // or expired token — the backend rejected it as per StrictBrowseAuthMiddleware.
      // We propagate the error as-is so the caller can handle it (e.g., clear
      // the stored token), without triggering a global session-expired logout.
      if (_isPublicEndpoint(
        err.requestOptions.path,
        method: err.requestOptions.method,
      )) {
        _logger?.warning(
          'AuthInterceptor: 401 on public browse endpoint — skipping refresh/session-expiry signal',
        );
        return handler.next(err);
      }

      // SESSION HONESTY (Tier 2): If we have no current user at all,
      // there is nothing to refresh — the session is already gone.
      // Signal session expired so the auth domain forces logout instead
      // of letting subsequent requests 401-loop inside an authenticated
      // shell. Skip when a [_tokenFetcher] override is in play (test
      // mode), since tests drive the path explicitly via the fetcher.
      if (_tokenFetcher == null && _firebaseAuth!.currentUser == null) {
        await _signalSessionExpiredOnce('401 with no Firebase currentUser');
        return handler.next(err);
      }

      try {
        // Force refresh token
        final newToken = await _getToken(true);

        if (newToken != null && newToken.isNotEmpty) {
          _logger?.info('AuthInterceptor: Token refreshed, retrying request');

          // Retry the original request with new token
          final options = err.requestOptions;

          // IDEMPOTENCY SAFETY: Log idempotency key presence for checkout requests
          final idempotencyKey = options.headers['Idempotency-Key'];
          if (idempotencyKey != null) {
            _logger?.info(
              'AuthInterceptor: Preserving Idempotency-Key during auth retry',
            );
          }

          // Update only the Authorization header
          options.headers['Authorization'] = 'Bearer $newToken';

          // Successful refresh re-arms the session-expiry detector.
          _sessionExpirySignaled = false;

          // Build the retry RequestOptions. Production uses a fresh
          // Dio instance via `Dio().fetch(...)` to avoid recursing
          // through this interceptor; tests inject a [_requestRetrier]
          // to verify retry semantics without HTTP I/O.
          // CRITICAL: All headers including Idempotency-Key are preserved via options.headers
          final retryOptions = RequestOptions(
            path: options.path,
            data: options.data,
            queryParameters: options.queryParameters,
            onReceiveProgress: options.onReceiveProgress,
            onSendProgress: options.onSendProgress,
            baseUrl: options.baseUrl,
            method: options.method,
            headers: options.headers, // Preserves Idempotency-Key + new Bearer
            connectTimeout: options.connectTimeout,
            sendTimeout: options.sendTimeout,
            receiveTimeout: options.receiveTimeout,
            responseType: options.responseType,
            validateStatus: options.validateStatus,
            followRedirects: options.followRedirects,
            maxRedirects: options.maxRedirects,
            requestEncoder: options.requestEncoder,
            responseDecoder: options.responseDecoder,
            listFormat: options.listFormat,
            contentType: options.contentType,
          );
          final retrier = _requestRetrier ?? _defaultRetrier;
          final response = await retrier(retryOptions);

          // Retry successful - return the response
          return handler.resolve(response);
        }

        // SESSION HONESTY (Tier 2): refresh "succeeded" but returned a
        // null/empty token — same effect as a refresh failure. The
        // session can no longer be trusted.
        await _signalSessionExpiredOnce(
          'token refresh returned null/empty token',
        );
      } catch (retryError, stackTrace) {
        _logger?.error(
          'AuthInterceptor: Token refresh/retry failed - $retryError',
          stackTrace: stackTrace,
        );

        // SESSION HONESTY (Tier 2): Token refresh or retry threw. We do
        // NOT know whether the retry would have succeeded with a fresh
        // token. The original 401 plus a refresh failure is the
        // canonical "session expired" signal — force a clean logout.
        await _signalSessionExpiredOnce(
          'token refresh / retry threw: ${retryError.runtimeType}',
        );
      }
    }

    handler.next(err);
  }

  /// Default retry path — uses a fresh [Dio] instance so the retry
  /// does NOT go through this interceptor (preventing infinite 401
  /// loops at the Dio layer).
  static Future<Response<dynamic>> _defaultRetrier(
    RequestOptions options,
  ) async {
    final retryDio = Dio();
    return retryDio.fetch(options);
  }

  /// Determine if token should be force refreshed for this endpoint.
  bool _shouldForceRefreshToken(String path) {
    final normalizedPath = _normalizeApiPath(path);
    return normalizedPath == '/auth/firebase/exchange' ||
        normalizedPath == '/users/me';
  }

  bool _shouldSkipAuth(RequestOptions options) {
    return options.extra['skipAuth'] == true;
  }

  /// Check if endpoint doesn't require authentication.
  ///
  /// [method] defaults to 'GET'. Only GET requests can be public browse
  /// endpoints - POST/PUT/PATCH/DELETE are always auth-required.
  ///
  /// IMPORTANT: Uses normalized exact match / prefix checks to avoid false
  /// positives across `/api/v1/...` and bare `/...` request paths.
  bool _isPublicEndpoint(String path, {String method = 'GET'}) {
    final normalizedPath = _normalizeApiPath(path);

    if (method.toUpperCase() != 'GET') return false;

    const exactPaths = ['/health'];
    for (final p in exactPaths) {
      if (normalizedPath == p) return true;
    }

    const browsePrefixes = [
      '/listings',
      '/auctions',
      '/search/listings',
      '/search/auctions',
      '/search/content',
      '/search/users',
      '/likes/stats',
    ];
    for (final p in browsePrefixes) {
      if (normalizedPath.startsWith(p)) return true;
    }

    if (normalizedPath.startsWith('/users/') &&
        !normalizedPath.startsWith('/users/me') &&
        !normalizedPath.startsWith('/users/check-username')) {
      return true;
    }

    final contentsWithId = RegExp(r'^/contents/[^/]+$');
    if (contentsWithId.hasMatch(normalizedPath)) return true;

    return false;
  }

  String _normalizeApiPath(String path) {
    if (path == '/api/v1') {
      return '/';
    }
    if (path.startsWith('/api/v1/')) {
      return path.substring('/api/v1'.length);
    }
    return path;
  }
}
