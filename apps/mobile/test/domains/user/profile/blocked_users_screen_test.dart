import 'dart:convert';
import 'dart:io';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/profile/data/models/blocked_user_model.dart';
import 'package:labuda/domains/user/profile/data/services/blocked_users_service.dart';
import 'package:labuda/domains/user/profile/presentation/providers/blocked_users_provider.dart';
import 'package:labuda/domains/user/profile/presentation/screens/blocked_users_screen.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _NoopApiClient implements ApiClient {
  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _StaticImageHttpClient implements HttpClient {
  _StaticImageHttpClient(this._bytes);

  final List<int> _bytes;
  @override
  bool autoUncompress = true;
  @override
  Duration? connectionTimeout;
  @override
  Duration idleTimeout = const Duration(seconds: 15);
  @override
  int? maxConnectionsPerHost;
  @override
  String? userAgent;
  @override
  bool Function(X509Certificate, String, int)? badCertificateCallback;
  @override
  String Function(Uri)? findProxy;
  @override
  Future<bool> Function(Uri, String, String?)? authenticate;
  @override
  Future<bool> Function(String, int, String, String?)? authenticateProxy;
  @override
  Future<ConnectionTask<Socket>> Function(Uri, String?, int?)?
  connectionFactory;

  @override
  Future<HttpClientRequest> getUrl(Uri url) async {
    return _StaticImageHttpClientRequest(_bytes);
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _StaticImageHttpClientRequest implements HttpClientRequest {
  _StaticImageHttpClientRequest(this._bytes);

  final List<int> _bytes;
  @override
  bool bufferOutput = true;
  @override
  bool followRedirects = true;
  @override
  int maxRedirects = 5;
  @override
  bool persistentConnection = true;
  @override
  int contentLength = -1;
  @override
  Encoding encoding = utf8;

  @override
  HttpHeaders get headers => _NoopHttpHeaders();

  @override
  Future<HttpClientResponse> close() async {
    return _StaticImageHttpClientResponse(_bytes);
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _StaticImageHttpClientResponse extends StreamView<List<int>>
    implements HttpClientResponse {
  _StaticImageHttpClientResponse(this._bytes)
    : super(Stream<List<int>>.value(_bytes));

  final List<int> _bytes;

  @override
  int get statusCode => HttpStatus.ok;

  @override
  int get contentLength => _bytes.length;

  @override
  HttpHeaders get headers => _NoopHttpHeaders();

  @override
  HttpClientResponseCompressionState get compressionState =>
      HttpClientResponseCompressionState.notCompressed;

  @override
  bool get isRedirect => false;

  @override
  bool get persistentConnection => false;

  @override
  String get reasonPhrase => 'OK';

  @override
  List<RedirectInfo> get redirects => const [];

  @override
  List<Cookie> get cookies => const [];

  @override
  X509Certificate? get certificate => null;

  @override
  HttpConnectionInfo? get connectionInfo => null;

  @override
  Future<Socket> detachSocket() {
    throw UnimplementedError('detachSocket is not used in this test');
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _NoopHttpHeaders implements HttpHeaders {
  @override
  void add(String name, Object value, {bool preserveHeaderCase = false}) {}

  @override
  void set(String name, Object value, {bool preserveHeaderCase = false}) {}

  @override
  void remove(String name, Object value) {}

  @override
  String? value(String name) => null;

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeBlockedUsersService extends BlockedUsersService {
  _FakeBlockedUsersService(this._blockedUsers) : super(_NoopApiClient());

  final List<BlockedUserModel> _blockedUsers;
  String? lastCurrentUserId;
  String? lastBlockedUserId;

  @override
  Stream<List<BlockedUserModel>> watchBlockedUsers(String userId) {
    return Stream.value(_blockedUsers);
  }

  @override
  Future<void> unblockUser({
    required String currentUserId,
    required String blockedUserId,
  }) async {
    lastCurrentUserId = currentUserId;
    lastBlockedUserId = blockedUserId;
  }
}

AuthUser _authUser({required String id, required String username}) {
  return AuthUser(
    id: id,
    createdAt: DateTime.utc(2026, 7, 1),
    updatedAt: DateTime.utc(2026, 7, 1),
    email: '$username@example.com',
    username: username,
    avatarUrl: 'https://example.com/$username.png',
    bio: 'Bio for $username',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
    lifecycle: ContentLifecycle.active,
    hasSellerProfile: false,
    sellerSubscriptionStatus: 'none',
    hasMarketAuthority: false,
  );
}

Widget _wrap({
  required AuthState authState,
  required BlockedUsersService service,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      blockedUsersServiceProvider.overrideWithValue(service),
    ],
    child: const MaterialApp(home: BlockedUsersScreen()),
  );
}

void main() {
  testWidgets('canonical username and avatar remain visible', (tester) async {
    final avatarBytes = base64Decode(
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9W1Xzj8AAAAASUVORK5CYII=',
    );
    await HttpOverrides.runZoned(() async {
      final service = _FakeBlockedUsersService([
        BlockedUserModel(
          id: 'blocked-1',
          username: 'alice',
          avatarUrl: 'https://example.com/alice.png',
          blockedAt: DateTime.utc(2026, 7, 1),
        ),
      ]);

      await tester.pumpWidget(
        _wrap(
          authState: AuthState.authenticated(
            _authUser(id: 'current-user', username: 'owner'),
            emailVerified: true,
          ),
          service: service,
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('@alice'), findsOneWidget);
      expect(find.text('Pengguna tidak tersedia'), findsNothing);
      expect(
        find.descendant(
          of: find.byType(ListTile),
          matching: find.byIcon(Icons.person),
        ),
        findsNothing,
      );

      final avatar = tester.widget<CircleAvatar>(find.byType(CircleAvatar));
      expect(avatar.backgroundImage, isA<NetworkImage>());
    }, createHttpClient: (context) => _StaticImageHttpClient(avatarBytes));
  });

  testWidgets('lookup failure keeps row without ID prefix', (tester) async {
    final durableId = '9f4c2d3e-1a2b-4c5d-9e0f-1234567890ab';
    final service = _FakeBlockedUsersService([
      BlockedUserModel(
        id: durableId,
        username: '',
        avatarUrl: null,
        blockedAt: DateTime.utc(2026, 7, 1),
      ),
    ]);

    await tester.pumpWidget(
      _wrap(
        authState: AuthState.authenticated(
          _authUser(id: 'current-user', username: 'owner'),
          emailVerified: true,
        ),
        service: service,
      ),
    );
    await tester.pumpAndSettle();

    // Production renders '@$username' directly — empty username shows '@'
    expect(find.text('@'), findsOneWidget);
    expect(find.textContaining('9f4c2d3e'), findsNothing);
    expect(find.textContaining('1234567890ab'), findsNothing);
    expect(find.text('Unblock'), findsOneWidget);
  });

  testWidgets('blank username renders the raw @-prefixed value', (tester) async {
    final service = _FakeBlockedUsersService([
      BlockedUserModel(
        id: 'blocked-blank',
        username: '   ',
        avatarUrl: null,
        blockedAt: DateTime.utc(2026, 7, 1),
      ),
    ]);

    await tester.pumpWidget(
      _wrap(
        authState: AuthState.authenticated(
          _authUser(id: 'current-user', username: 'owner'),
          emailVerified: true,
        ),
        service: service,
      ),
    );
    await tester.pumpAndSettle();

    // Production renders '@$username' directly — blank username shows '@   '
    expect(find.textContaining('@'), findsOneWidget);
  });

  testWidgets('avatar absence uses the safe fallback icon', (tester) async {
    final service = _FakeBlockedUsersService([
      BlockedUserModel(
        id: 'blocked-no-avatar',
        username: 'alice',
        avatarUrl: null,
        blockedAt: DateTime.utc(2026, 7, 1),
      ),
    ]);

    await tester.pumpWidget(
      _wrap(
        authState: AuthState.authenticated(
          _authUser(id: 'current-user', username: 'owner'),
          emailVerified: true,
        ),
        service: service,
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.descendant(
        of: find.byType(ListTile),
        matching: find.byIcon(Icons.person),
      ),
      findsOneWidget,
    );

    final avatar = tester.widget<CircleAvatar>(find.byType(CircleAvatar));
    expect(avatar.backgroundImage, isNull);
  });

  testWidgets('unblock still sends the durable blocked-user ID', (
    tester,
  ) async {
    final service = _FakeBlockedUsersService([
      BlockedUserModel(
        id: 'durable-user-id',
        username: 'alice',
        avatarUrl: null,
        blockedAt: DateTime.utc(2026, 7, 1),
      ),
    ]);

    await tester.pumpWidget(
      _wrap(
        authState: AuthState.authenticated(
          _authUser(id: 'current-user', username: 'owner'),
          emailVerified: true,
        ),
        service: service,
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.widgetWithText(OutlinedButton, 'Unblock'));
    await tester.pumpAndSettle();

    expect(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextButton, 'Unblock'),
      ),
      findsOneWidget,
    );

    await tester.tap(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextButton, 'Unblock'),
      ),
    );
    await tester.pumpAndSettle();

    expect(service.lastCurrentUserId, 'current-user');
    expect(service.lastBlockedUserId, 'durable-user-id');
  });
}
