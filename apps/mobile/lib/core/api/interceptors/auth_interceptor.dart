// ignore_for_file: unused_element, unused_field, unnecessary_non_null_assertion

import 'dart:async';

import 'package:dio/dio.dart';
import 'package:labuda/core/src/interfaces/services/i_local_storage_service.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';

typedef SessionExpiredCallback = FutureOr<void> Function();
typedef LabudaTokenFetcher = Future<String?> Function();
typedef IdTokenFetcher = Future<String?> Function(bool forceRefresh);
typedef RequestRetrier = Future<Response<dynamic>> Function(RequestOptions options);

/// Labuda refresh executor for tests — returns true on success, false on failure.
/// Production uses _dio.post with skipAuth.
typedef LabudaRefreshExecutor = Future<bool> Function(String refreshToken);

class AuthInterceptor extends Interceptor {
  final ILoggerService? _logger;
  final ILocalStorageService? _localStorage;
  final LabudaTokenFetcher? _labudaFetcher;
  final LabudaTokenFetcher? _refreshFetcher;
  final LabudaRefreshExecutor? _refreshExecutor;
  final RequestRetrier? _requestRetrier;
  Dio? _dio;

  static SessionExpiredCallback? onSessionExpired;
  static bool _sessionExpirySignaled = false;

  // Single-flight for concurrent 401s
  Future<bool>? _ongoingRefresh;

  AuthInterceptor({
    ILoggerService? logger,
    ILocalStorageService? localStorage,
    LabudaTokenFetcher? labudaTokenFetcher,
    LabudaTokenFetcher? refreshTokenFetcher,
    LabudaRefreshExecutor? refreshExecutor,
    @Deprecated('Use labudaTokenFetcher — Firebase path is removed (Phase 3B)')
    IdTokenFetcher? tokenFetcher,
    RequestRetrier? requestRetrier,
    Dio? dio,
  })  : _logger = logger,
        _localStorage = localStorage,
        _labudaFetcher = labudaTokenFetcher ??
            (tokenFetcher != null ? () => tokenFetcher(false) : null),
        _refreshFetcher = refreshTokenFetcher,
        _refreshExecutor = refreshExecutor,
        _requestRetrier = requestRetrier,
        _dio = dio;

  void attachDio(Dio dio) {
    _dio = dio;
  }

  static void resetSessionExpiryGuard() {
    _sessionExpirySignaled = false;
  }

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

  Future<String?> _getLabudaToken() async {
    final fetcher = _labudaFetcher;
    if (fetcher != null) return fetcher();
    final storage = _localStorage;
    if (storage != null) {
      try {
        final result = await storage.readLabudaAccessToken();
        if (result.isSuccess) {
          final token = result.data;
          if (token != null && token.trim().isNotEmpty) return token.trim();
        }
        if (result.isError) {
          _logger?.warning('AuthInterceptor: readLabudaAccessToken error — ${result.error}');
        }
      } catch (e, st) {
        _logger?.error('AuthInterceptor: readLabudaAccessToken threw — $e', stackTrace: st);
      }
      return null;
    }
    return null;
  }

  Future<String?> _getRefreshToken() async {
    final fetcher = _refreshFetcher;
    if (fetcher != null) return fetcher();
    final storage = _localStorage;
    if (storage != null) {
      try {
        final result = await storage.readLabudaRefreshToken();
        if (result.isSuccess) {
          final token = result.data;
          if (token != null && token.trim().isNotEmpty) return token.trim();
        }
        if (result.isError) {
          _logger?.warning('AuthInterceptor: readLabudaRefreshToken error — ${result.error}');
        }
      } catch (e, st) {
        _logger?.error('AuthInterceptor: readLabudaRefreshToken threw — $e', stackTrace: st);
      }
      return null;
    }
    return null;
  }

  Future<void> _signalSessionExpiredOnce(String reason) async {
    if (_sessionExpirySignaled) {
      _logger?.debug('AuthInterceptor: session-expired already signaled, skipping duplicate (reason=$reason)');
      return;
    }
    _sessionExpirySignaled = true;
    _logger?.warning('AuthInterceptor: signaling session expired — $reason');
    final cb = onSessionExpired;
    if (cb == null) {
      _logger?.warning('AuthInterceptor: no onSessionExpired listener — signal will not force logout');
      return;
    }
    try {
      await cb();
    } catch (e, st) {
      _logger?.error('AuthInterceptor: onSessionExpired threw — $e', stackTrace: st);
    }
  }

  @override
  Future<void> onRequest(RequestOptions options, RequestInterceptorHandler handler) async {
    if (_shouldSkipAuth(options)) {
      _logger?.debug('AuthInterceptor: skipping auth for ${options.method} ${options.path} (skipAuth)');
      return handler.next(options);
    }
    final isPublic = _isPublicEndpoint(options.path, method: options.method);
    if (isPublic) {
      _logger?.debug('AuthInterceptor: Skipping auth for public endpoint: ${options.path}');
      return handler.next(options);
    }
    try {
      final token = await _getLabudaToken();
      if (token != null && token.isNotEmpty) {
        options.headers['Authorization'] = 'Bearer $token';
        _sessionExpirySignaled = false;
        _logger?.debug('AuthInterceptor: Labuda token attached for ${options.method} ${options.path} (len=${token.length})');
      } else {
        _logger?.debug('AuthInterceptor: No Labuda access token for ${options.path} — sending without Authorization (no Firebase fallback)');
      }
    } catch (e, st) {
      _logger?.error('AuthInterceptor: Error reading Labuda token for ${options.path} - $e', stackTrace: st);
    }
    handler.next(options);
  }

  @override
  Future<void> onError(DioException err, ErrorInterceptorHandler handler) async {
    if (_shouldSkipAuth(err.requestOptions)) return handler.next(err);
    if (err.response?.statusCode != 401) return handler.next(err);

    final reqPath = err.requestOptions.path;
    final reqMethod = err.requestOptions.method;

    _logger?.warning('AuthInterceptor: Received 401 for $reqPath');

    if (_isPublicEndpoint(reqPath, method: reqMethod)) {
      _logger?.warning('AuthInterceptor: 401 on public browse endpoint — skipping refresh');
      return handler.next(err);
    }

    // Do not refresh the refresh endpoint itself
    if (_normalizeApiPath(reqPath) == '/auth/refresh') {
      _logger?.warning('AuthInterceptor: 401 on /auth/refresh — not retrying, propagating');
      return handler.next(err);
    }

    // Single-shot per-request marker
    if (err.requestOptions.extra['labuda_retry'] == true) {
      _logger?.warning('AuthInterceptor: Request already retried once — propagating 401');
      return handler.next(err);
    }

    // Perform single-flight refresh
    final refreshed = await _sharedRefresh();
    if (!refreshed) {
      _logger?.warning('AuthInterceptor: Refresh failed — propagating original 401 (no Firebase fallback)');
      return handler.next(err);
    }

    // Refresh succeeded — retry original request once with new token
    try {
      final newToken = await _getLabudaToken();
      if (newToken == null || newToken.isEmpty) {
        _logger?.warning('AuthInterceptor: No Labuda token after refresh — propagating 401');
        return handler.next(err);
      }
      final opts = err.requestOptions;
      opts.headers['Authorization'] = 'Bearer $newToken';
      opts.extra['labuda_retry'] = true;

      // Use requestRetrier seam if provided, else dio.fetch via attached Dio
      Response<dynamic> resp;
      final dio2 = _dio;
      if (_requestRetrier != null) {
        resp = await _requestRetrier!(opts);
      } else if (dio2 != null) {
        // Build new RequestOptions to preserve all metadata but with new headers/extra
        final retryOpts = RequestOptions(
          path: opts.path,
          baseUrl: opts.baseUrl,
          method: opts.method,
          headers: Map<String, dynamic>.from(opts.headers),
          data: opts.data,
          queryParameters: Map<String, dynamic>.from(opts.queryParameters),
          extra: Map<String, dynamic>.from(opts.extra),
          connectTimeout: opts.connectTimeout,
          sendTimeout: opts.sendTimeout,
          receiveTimeout: opts.receiveTimeout,
          responseType: opts.responseType,
          validateStatus: opts.validateStatus,
          followRedirects: opts.followRedirects,
          maxRedirects: opts.maxRedirects,
          requestEncoder: opts.requestEncoder,
          responseDecoder: opts.responseDecoder,
          listFormat: opts.listFormat,
          contentType: opts.contentType,
          onReceiveProgress: opts.onReceiveProgress,
          onSendProgress: opts.onSendProgress,
        );
        resp = await dio2.fetch(retryOpts);
      } else {
        // No dio available — cannot retry, propagate
        return handler.next(err);
      }
      return handler.resolve(resp);
    } catch (e, st) {
      _logger?.error('AuthInterceptor: Retry after refresh threw — $e', stackTrace: st);
      return handler.next(err);
    }
  }

  Future<bool> _sharedRefresh() async {
    // If a refresh is already in-flight, join it
    if (_ongoingRefresh != null) {
      _logger?.debug('AuthInterceptor: Joining ongoing refresh');
      return _ongoingRefresh!;
    }
    final completer = Completer<bool>();
    _ongoingRefresh = completer.future;
    bool result = false;
    try {
      result = await _doRefresh();
      completer.complete(result);
    } catch (e, st) {
      _logger?.error('AuthInterceptor: _doRefresh threw — $e', stackTrace: st);
      if (!completer.isCompleted) completer.complete(false);
      result = false;
    } finally {
      // Clear single-flight after completion — next 401 will start new refresh
      _ongoingRefresh = null;
    }
    return result;
  }

  Future<bool> _doRefresh() async {
    // If a test-provided refreshExecutor is present, delegate to it (counts refresh calls in tests)
    // Canonical race invariant applies to both paths: current stored refresh must still equal
    // the token we started with before committing any credential result.
    if (_refreshExecutor != null) {
      final token = await _getRefreshToken();
      if (token == null || token.isEmpty) {
        _logger?.warning('AuthInterceptor: No refresh token available — cannot refresh');
        return false;
      }
      bool executorResult = false;
      try {
        executorResult = await _refreshExecutor!(token);
      } catch (e, st) {
        _logger?.error('AuthInterceptor: refreshExecutor threw — $e', stackTrace: st);
        return false;
      }
      if (!executorResult) return false;
      // Race guard even for executor path: if logout/rotation cleared the token while
      // the executor was in flight, the result must be treated as stale.
      // This covers both executor-owns-save and interceptor-owns-save contracts:
      // if mismatch, we revoke any credential the executor may have persisted.
      try {
        final curRes = await _getRefreshToken();
        if (curRes != token) {
          _logger?.warning('AuthInterceptor: Refresh token mismatch after executor (logout/rotation), treating as stale');
          // If executor already persisted new credential after logout, remove it.
          // Best-effort: clear via storage if available; fetcher-only path will
          // naturally see stale on next read, but we still ensure local storage cleared.
          final storage = _localStorage;
          if (storage != null) {
            try {
              final curStorage = await storage.readLabudaRefreshToken();
              if (curStorage.data != token) {
                await storage.clearLabudaCredential();
              }
            } catch (_) {}
          }
          return false;
        }
      } catch (_) {}
      return true;
    }

    final refreshToken = await _getRefreshToken();
    if (refreshToken == null || refreshToken.isEmpty) {
      _logger?.warning('AuthInterceptor: No Labuda refresh token — cannot refresh (no Firebase fallback)');
      return false;
    }

    final dio = _dio;
    if (dio == null) {
      _logger?.warning('AuthInterceptor: No Dio attached — cannot execute refresh');
      return false;
    }

    try {
      _logger?.info('AuthInterceptor: Executing Labuda refresh');
      final resp = await dio.post(
        '/auth/refresh',
        data: {'refresh_token': refreshToken},
        options: Options(extra: {'skipAuth': true}),
      );
      final data = resp.data;
      // Backend wraps in {success, data: {access_token, refresh_token, ...}} or direct
      Map<String, dynamic>? payload;
      if (data is Map<String, dynamic>) {
        if (data.containsKey('success')) {
          if (data['success'] != true) {
            _logger?.warning('AuthInterceptor: Refresh envelope success=false — ${data['error']}');
            return false;
          }
          final d = data['data'];
          if (d is Map<String, dynamic>) payload = d;
        } else {
          payload = data;
        }
      }
      if (payload == null) {
        _logger?.warning('AuthInterceptor: Refresh response payload null');
        return false;
      }
      final access = payload['access_token']?.toString().trim();
      final refresh = payload['refresh_token']?.toString().trim();
      if (access == null || access.isEmpty || refresh == null || refresh.isEmpty) {
        _logger?.warning('AuthInterceptor: Refresh payload missing tokens — access=${access?.length} refresh=${refresh?.length}');
        return false;
      }
      final storage = _localStorage;
      if (storage == null) {
        _logger?.warning('AuthInterceptor: No storage to persist rotated credential');
        return false;
      }
      // Race guard: logout may have cleared/rotated the refresh token while this refresh was in flight
      try {
        final curRes = await storage.readLabudaRefreshToken();
        final cur = curRes.data?.trim();
        if (cur != refreshToken) {
          _logger?.warning('AuthInterceptor: Refresh token mismatch (logout/rotation), aborting credential save');
          return false;
        }
      } catch (_) {}
      final saveRes = await storage.saveLabudaCredential(access, refresh);
      if (saveRes.isError) {
        _logger?.warning('AuthInterceptor: saveLabudaCredential failed — ${saveRes.error}');
        return false;
      }
      _logger?.info('AuthInterceptor: Labuda credential rotated (new access len=${access.length})');
      return true;
    } catch (e, st) {
      _logger?.error('AuthInterceptor: Refresh request threw — $e', stackTrace: st);
      return false;
    }
  }

  static Future<Response<dynamic>> _defaultRetrier(RequestOptions options) async {
    final retryDio = Dio();
    return retryDio.fetch(options);
  }

  bool _shouldForceRefreshToken(String path) {
    final n = _normalizeApiPath(path);
    return n == '/auth/firebase/exchange' || n == '/users/me';
  }

  bool _shouldSkipAuth(RequestOptions options) => options.extra['skipAuth'] == true;

  bool _isPublicEndpoint(String path, {String method = 'GET'}) {
    final n = _normalizeApiPath(path);
    if (method.toUpperCase() != 'GET') return false;
    const exactPaths = ['/health'];
    for (final p in exactPaths) {
      if (n == p) return true;
    }
    const browsePrefixes = ['/listings', '/auctions', '/search/listings', '/search/auctions', '/search/content', '/search/users', '/likes/stats'];
    for (final p in browsePrefixes) {
      if (n.startsWith(p)) return true;
    }
    if (n.startsWith('/users/') && !n.startsWith('/users/me') && !n.startsWith('/users/check-username')) return true;
    final contentsWithId = RegExp(r'^/contents/[^/]+$');
    if (contentsWithId.hasMatch(n)) return true;
    return false;
  }

  static String classifyAuthSemantics(String path, {Map<String, dynamic>? extra, String method = 'GET'}) {
    final normalized = AuthInterceptor._normalizeStatic(path);
    if (extra?['skipAuth'] == true) {
      if (normalized == '/auth/firebase/exchange') return 'firebase-boundary';
      if (normalized == '/auth/complete-profile') return 'restricted-completion';
      if (normalized == '/auth/refresh') return 'refresh-isolated';
      return 'special-skipAuth';
    }
    const exactPaths = ['/health'];
    if (method.toUpperCase() == 'GET') {
      for (final p in exactPaths) {
        if (normalized == p) return 'public';
      }
      const browsePrefixes = ['/listings', '/auctions', '/search/listings', '/search/auctions', '/search/content', '/search/users', '/likes/stats'];
      for (final p in browsePrefixes) {
        if (normalized.startsWith(p)) return 'public';
      }
      if (normalized.startsWith('/users/') && !normalized.startsWith('/users/me') && !normalized.startsWith('/users/check-username')) return 'public';
      final contentsWithId = RegExp(r'^/contents/[^/]+$');
      if (contentsWithId.hasMatch(normalized)) return 'public';
    }
    return 'normal-labuda';
  }

  static String _normalizeStatic(String path) {
    if (path == '/api/v1') return '/';
    if (path.startsWith('/api/v1/')) return path.substring('/api/v1'.length);
    return path;
  }

  String _normalizeApiPath(String path) => _normalizeStatic(path);
}
