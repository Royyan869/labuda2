// C1B2 — New-chat canonical user-search: runtime behavioral proof.
//
// Validates:
//   1) Production provider execution (success / blank / whitespace / empty / error)
//   2) Same-container principal-switch race safety
//   3) Same-container logout safety
//   4) Production NewChatUserListWidget pump (identity label, avatar wiring, metadata removal)
//   5) Production tap handler (success, principal-switch, self-chat, error)
//   6) Production NewChatScreen states (auth, blank, loading, error, empty, populated, query-race)

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/providers/core_providers.dart'
    show loggerServiceProvider, webSocketServiceProvider;
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
// presence_provider import removed — PresenceSubscriptionRegistry purged
import 'package:labuda/core/websocket/websocket_service.dart';
import 'package:labuda/domains/chat/chat/chat.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/new_chat_user_search_provider.dart'
    show newChatUserSearchProvider;
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/data/remote/search_api_service.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';
import 'package:labuda/features/search/search/presentation/providers/providers.dart'
    show searchApiServiceProvider;
import 'package:labuda/shared/shared.dart';

// =============================================================================
// Fake / recording dependencies
// =============================================================================

class _FakeApiClient extends Fake implements ApiClient {}

class _NoOpAvatarCacheService extends AvatarCacheService {
  _NoOpAvatarCacheService() : super(datasource: _NoOpUserApiDatasource());

  @override
  Future<String?> getUserAvatarUrl(String userId) async => null;
}

class _NoOpUserApiDatasource extends Fake implements UserApiDatasource {}

class _NoopLogger implements ILoggerService {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

// _NoOpPresenceRegistry removed — PresenceSubscriptionRegistry purged.
// Current canonical presence is PresenceManager / userOnlineStatusProvider (lib\core\src\providers\presence_provider.dart).
// Widget NewChatUserListWidget does not depend on presence, so no fake needed.

/// Recording fake [SearchApiService].
class _RecordingSearchApiService extends SearchApiService {
  int callCount = 0;
  String? lastQuery;
  int? lastLimit;
  int? lastOffset;
  UserSearchResponseDto? cannedResponse;
  Object? cannedError;
  Completer<UserSearchResponseDto>? _pendingCompleter;

  _RecordingSearchApiService() : super(_FakeApiClient());

  @override
  Future<UserSearchResponseDto> searchUsers({
    required String query,
    int limit = 20,
    int offset = 0,
  }) async {
    callCount++;
    lastQuery = query;
    lastLimit = limit;
    lastOffset = offset;

    if (_pendingCompleter != null) {
      return _pendingCompleter!.future;
    }

    if (cannedError != null) {
      return Future.error(cannedError!);
    }

    return cannedResponse ??
        UserSearchResponseDto(
          query: query,
          users: [],
          total: 0,
          limit: limit,
          offset: offset,
        );
  }

  void delayNextWith(Completer<UserSearchResponseDto> completer) {
    _pendingCompleter = completer;
  }
}

UserSearchResultDto _userDto({
  required String id,
  required String username,
  String? avatarUrl,
}) {
  return UserSearchResultDto(id: id, username: username, avatarUrl: avatarUrl);
}

/// Canonical row input: the lightweight UserSearch projection from the
/// search domain (no location / farm / verification metadata exists here).
UserSearch _userSearch({
  required String userId,
  String username = 'user',
  String? avatarUrl,
}) {
  return UserSearch(userId: userId, username: username, avatarUrl: avatarUrl);
}

UserSearchResponseDto _responseDto({
  required String query,
  required List<UserSearchResultDto> users,
}) {
  return UserSearchResponseDto(
    query: query,
    users: users,
    total: users.length,
    limit: 20,
    offset: 0,
  );
}

/// Recording fake [ChatRepository].
class _RecordingChatRepository extends Fake implements ChatRepository {
  int getOrCreateChatCallCount = 0;
  List<List<String>> participantIdsCalls = [];
  Chat? nextChat;
  Object? nextError;
  String? nextErrorMessage;

  @override
  Future<Result<Chat>> getOrCreateChat({
    required List<String> participantIds,
  }) async {
    getOrCreateChatCallCount++;
    participantIdsCalls.add(List.of(participantIds));
    if (nextError != null) {
      return Result<Chat>.error(nextError.toString());
    }
    if (nextErrorMessage != null) {
      return Result<Chat>.error(nextErrorMessage!);
    }
    final chat =
        nextChat ??
        Chat(
          id: 'room-$getOrCreateChatCallCount',
          type: ChatType.private,
          participantIds: participantIds,
          participantNames: {},
          participantAvatars: {},
          createdAt: DateTime.now(),
        );
    return Result<Chat>.success(chat);
  }
}

/// Minimal fake AuthController with immutable state.
class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);
  final AuthState _state;

  @override
  AuthState build() => _state;
}

/// Mutable fake AuthController for same-tree logout tests.
/// Allows changing the auth state within one ProviderContainer
/// so the screen rebuilds without replacing the widget tree.
class _MutableFakeAuthController extends AuthController {
  _MutableFakeAuthController(AuthState initial) : _current = initial;

  AuthState _current;

  @override
  AuthState build() => _current;

  /// Mutate auth state in-place.  Triggers rebuild of all watchers.
  void setAuthState(AuthState s) {
    _current = s;
    // Force rebuild via Riverpod's Notifier contract.
    ref.invalidateSelf();
  }
}

// =============================================================================
// Helpers
// =============================================================================

/// Mutable principal notifier for same-container race tests.
class _MutablePrincipal extends Notifier<String> {
  @override
  String build() => 'principal-A';
  void setPrincipal(String v) => state = v;
}

final _testPrincipalProvider = NotifierProvider<_MutablePrincipal, String>(
  _MutablePrincipal.new,
);

AuthUser _authUser(String id, {bool emailVerified = true}) {
  return AuthUser(
    id: id,
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: '$id@test.com',
    username: id,
    isEmailVerified: emailVerified,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
  );
}

/// Provider override list shared by container and widget helpers.
/// Build provider overrides for container / widget tests.
/// Returns `dynamic` to avoid Riverpod version variance with `Override` type.
dynamic _ovr({
  required _RecordingSearchApiService recordingService,
  required String currentUserId,
  bool isEmailVerified = true,
  _RecordingChatRepository? chatRepo,
  bool useMutablePrincipal = false,
}) {
  final authUser = _authUser(currentUserId, emailVerified: isEmailVerified);
  return [
    searchApiServiceProvider.overrideWith((ref) => recordingService),
    isEmailVerifiedProvider.overrideWith((ref) => isEmailVerified),
    authControllerProvider.overrideWith(
      () => _FakeAuthController(
        AuthState.authenticated(authUser, emailVerified: isEmailVerified),
      ),
    ),
    loggerServiceProvider.overrideWithValue(_NoopLogger()),
    webSocketServiceProvider.overrideWithValue(
      WebSocketService(baseUrl: 'ws://localhost'),
    ),
    avatarCacheServiceProvider.overrideWith((ref) => _NoOpAvatarCacheService()),
    // presenceSubscriptionRegistryProvider removed — purged (now presenceManagerProvider)
    if (useMutablePrincipal)
      currentUserIdProvider.overrideWith(
        (ref) => ref.watch(_testPrincipalProvider),
      )
    else
      currentUserIdProvider.overrideWith((ref) => currentUserId),
    if (chatRepo != null)
      chatRepositoryProvider.overrideWith((ref) => chatRepo),
  ];
}

ProviderContainer _providerContainer({
  required _RecordingSearchApiService recordingService,
  String currentUserId = 'principal-A',
  bool isEmailVerified = true,
  _RecordingChatRepository? chatRepo,
  bool useMutablePrincipal = false,
}) {
  final o = _ovr(
    recordingService: recordingService,
    currentUserId: currentUserId,
    isEmailVerified: isEmailVerified,
    chatRepo: chatRepo,
    useMutablePrincipal: useMutablePrincipal,
  );
  return ProviderContainer(overrides: o);
}

/// Wrap a widget for pump tests. Places [child] inside a MaterialApp + Scaffold
/// so Row/Expanded widgets have bounded layout width.
Widget _wrapWidget({
  required Widget child,
  required _RecordingSearchApiService recordingService,
  String currentUserId = 'principal-A',
  bool isEmailVerified = true,
  _RecordingChatRepository? chatRepo,
}) {
  return ProviderScope(
    overrides: _ovr(
      recordingService: recordingService,
      currentUserId: currentUserId,
      isEmailVerified: isEmailVerified,
      chatRepo: chatRepo,
    ),
    child: MaterialApp(home: Scaffold(body: child)),
  );
}

Widget _wrapScreen({
  required _RecordingSearchApiService recordingService,
  String currentUserId = 'principal-A',
  bool isEmailVerified = true,
  _RecordingChatRepository? chatRepo,
}) {
  final goRouter = GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(path: '/', builder: (_, _) => const NewChatScreen()),
      GoRoute(path: '/chat/:chatId', builder: (_, _) => const SizedBox()),
    ],
  );

  return ProviderScope(
    overrides: _ovr(
      recordingService: recordingService,
      currentUserId: currentUserId,
      isEmailVerified: isEmailVerified,
      chatRepo: chatRepo,
    ),
    child: MaterialApp.router(routerConfig: goRouter),
  );
}

// =============================================================================
// Tests
// =============================================================================

const _canonicalRowUserId = '00000000-0000-0000-0000-0000000000a1';
const _canonicalAvatarRowUserId = '00000000-0000-0000-0000-0000000000a2';

void main() {
  // ===========================================================================
  // 1) PRODUCTION PROVIDER RUNTIME
  // ===========================================================================

  group('C1B2 — Provider runtime', () {
    group('success', () {
      test('valid query invokes service exactly once', () async {
        final svc = _RecordingSearchApiService();
        svc.cannedResponse = _responseDto(query: 'alice', users: []);
        final c = _providerContainer(recordingService: svc);

        await c.read(newChatUserSearchProvider('alice').future);

        expect(svc.callCount, 1);
        expect(svc.lastQuery, 'alice');
        expect(svc.lastLimit, 20);
        expect(svc.lastOffset, 0);
        c.dispose();
      });

      test('username and userId survive DTO→UserSearch', () async {
        final svc = _RecordingSearchApiService();
        svc.cannedResponse = _responseDto(
          query: 'alice',
          users: [_userDto(id: 'user-1', username: 'alice_wonder')],
        );
        final c = _providerContainer(recordingService: svc);

        final users = await c.read(newChatUserSearchProvider('alice').future);

        expect(users.length, 1);
        expect(users.first.userId, 'user-1');
        expect(users.first.username, 'alice_wonder');
        c.dispose();
      });

      test('avatar URL survives', () async {
        final svc = _RecordingSearchApiService();
        svc.cannedResponse = _responseDto(
          query: 'bob',
          users: [
            _userDto(
              id: 'user-2',
              username: 'bob_builder',
              avatarUrl: 'https://img.example/bob.png',
            ),
          ],
        );
        final c = _providerContainer(recordingService: svc);

        final users = await c.read(newChatUserSearchProvider('bob').future);

        expect(users.first.avatarUrl, 'https://img.example/bob.png');
        c.dispose();
      });

      test('current principal excluded', () async {
        final svc = _RecordingSearchApiService();
        svc.cannedResponse = _responseDto(
          query: 'test',
          users: [
            _userDto(id: 'principal-A', username: 'me'),
            _userDto(id: 'user-3', username: 'other'),
          ],
        );
        final c = _providerContainer(
          recordingService: svc,
          currentUserId: 'principal-A',
        );

        final users = await c.read(newChatUserSearchProvider('test').future);

        expect(users.length, 1);
        expect(users.first.userId, 'user-3');
        c.dispose();
      });

      test('other users remain', () async {
        final svc = _RecordingSearchApiService();
        svc.cannedResponse = _responseDto(
          query: 'test',
          users: [
            _userDto(id: 'principal-A', username: 'me'),
            _userDto(id: 'other-1', username: 'o1'),
            _userDto(id: 'other-2', username: 'o2'),
          ],
        );
        final c = _providerContainer(
          recordingService: svc,
          currentUserId: 'principal-A',
        );

        final users = await c.read(newChatUserSearchProvider('test').future);

        expect(users.length, 2);
        expect(users.map((u) => u.userId), containsAll(['other-1', 'other-2']));
        c.dispose();
      });

      test('provider reaches AsyncData', () async {
        final svc = _RecordingSearchApiService();
        svc.cannedResponse = _responseDto(
          query: 'eve',
          users: [_userDto(id: 'eve-id', username: 'eve')],
        );
        final c = _providerContainer(recordingService: svc);

        final result = await c.read(newChatUserSearchProvider('eve').future);

        expect(result, isA<List<UserSearch>>());
        expect(result.length, 1);
        c.dispose();
      });
    });

    group('blank / whitespace', () {
      test('empty string → [], zero calls', () async {
        final svc = _RecordingSearchApiService();
        final c = _providerContainer(recordingService: svc);
        final r = await c.read(newChatUserSearchProvider('').future);
        expect(r, isEmpty);
        expect(svc.callCount, 0);
        c.dispose();
      });

      test('whitespace-only → [], zero calls', () async {
        final svc = _RecordingSearchApiService();
        final c = _providerContainer(recordingService: svc);
        final r = await c.read(newChatUserSearchProvider('   ').future);
        expect(r, isEmpty);
        expect(svc.callCount, 0);
        c.dispose();
      });

      test('surrounding whitespace trimmed', () async {
        final svc = _RecordingSearchApiService();
        svc.cannedResponse = _responseDto(
          query: 'alice',
          users: [_userDto(id: 'u1', username: 'alice')],
        );
        final c = _providerContainer(recordingService: svc);
        await c.read(newChatUserSearchProvider(' alice ').future);
        expect(svc.lastQuery, 'alice');
        c.dispose();
      });
    });

    group('empty / error', () {
      test('service returns none → []', () async {
        final svc = _RecordingSearchApiService();
        svc.cannedResponse = _responseDto(query: 'nobody', users: []);
        final c = _providerContainer(recordingService: svc);
        final r = await c.read(newChatUserSearchProvider('nobody').future);
        expect(r, isEmpty);
        c.dispose();
      });

      test('service throws → provider reaches AsyncError', () async {
        final svc = _RecordingSearchApiService();
        svc.cannedError = Exception('Backend unavailable');
        final c = _providerContainer(recordingService: svc);

        final states = <AsyncValue<List<UserSearch>>>[];
        final sub = c.listen(
          newChatUserSearchProvider('fail'),
          (prev, next) => states.add(next),
        );
        c.read(newChatUserSearchProvider('fail'));
        await Future.delayed(const Duration(milliseconds: 50));

        expect(
          states.any((s) => s.hasError),
          isTrue,
          reason: 'Expected AsyncError',
        );
        sub.close();
        c.dispose();
      });

      test('error not converted to empty list', () async {
        final svc = _RecordingSearchApiService();
        svc.cannedError = Exception('Server error');
        final c = _providerContainer(recordingService: svc);

        final states = <AsyncValue<List<UserSearch>>>[];
        final sub = c.listen(
          newChatUserSearchProvider('err'),
          (prev, next) => states.add(next),
        );
        c.read(newChatUserSearchProvider('err'));
        await Future.delayed(const Duration(milliseconds: 50));

        final reachedEmpty = states.any(
          (s) => s is AsyncData<List<UserSearch>> && s.value.isEmpty,
        );
        expect(
          reachedEmpty,
          isFalse,
          reason: 'Error must not become empty list',
        );
        sub.close();
        c.dispose();
      });
    });

    // -----------------------------------------------------------------------
    // D) SAME-CONTAINER PRINCIPAL-SWITCH RACE
    // -----------------------------------------------------------------------
    group('principal-switch race', () {
      test('same-container: stale A does not replace B', () async {
        final svc = _RecordingSearchApiService();

        // Use mutable principal via StateProvider.
        final c = _providerContainer(
          recordingService: svc,
          currentUserId: 'principal-A',
          useMutablePrincipal: true,
        );
        // Start principal at A.
        c.read(_testPrincipalProvider.notifier).state = 'principal-A';

        final completerA = Completer<UserSearchResponseDto>();
        svc.delayNextWith(completerA);

        // Watch provider — starts request A.
        final states = <AsyncValue<List<UserSearch>>>[];
        final sub = c.listen(
          newChatUserSearchProvider('race'),
          (prev, next) => states.add(next),
        );
        c.read(newChatUserSearchProvider('race'));
        expect(svc.callCount, 1); // request A started

        // Mutate principal to B in same container.
        c.read(_testPrincipalProvider.notifier).state = 'principal-B';

        // Provider rebuilds → starts request B.
        final completerB = Completer<UserSearchResponseDto>();
        svc.delayNextWith(completerB);
        // Trigger rebuild.
        c.read(newChatUserSearchProvider('race'));
        expect(svc.callCount, 2); // request B started

        // Complete B first.
        completerB.complete(
          _responseDto(
            query: 'race',
            users: [
              _userDto(id: 'principal-B', username: 'B-self'),
              _userDto(id: 'target-B', username: 'target_b'),
            ],
          ),
        );
        await Future.delayed(const Duration(milliseconds: 50));

        // B state: contains target-B, excludes principal-B.
        final bVal = c.read(newChatUserSearchProvider('race'));
        expect(bVal, isA<AsyncData<List<UserSearch>>>());
        final bData = (bVal as AsyncData<List<UserSearch>>).value;
        expect(bData.any((u) => u.userId == 'target-B'), isTrue);
        expect(bData.any((u) => u.userId == 'principal-B'), isFalse);

        // Now complete stale A.
        completerA.complete(
          _responseDto(
            query: 'race',
            users: [
              _userDto(id: 'principal-A', username: 'A-self'),
              _userDto(id: 'target-A', username: 'target_a'),
            ],
          ),
        );
        await Future.delayed(const Duration(milliseconds: 50));

        // Critical: final state must still be B, not A.
        final finalVal = c.read(newChatUserSearchProvider('race'));
        expect(finalVal, isA<AsyncData<List<UserSearch>>>());
        final finalData = (finalVal as AsyncData<List<UserSearch>>).value;
        expect(finalData.any((u) => u.userId == 'target-B'), isTrue);
        expect(
          finalData.any((u) => u.userId == 'target-A'),
          isFalse,
          reason: 'Stale A leaked into same-container B state',
        );
        expect(
          finalData.any((u) => u.userId == 'principal-A'),
          isFalse,
          reason: 'Stale principal-A appeared as context',
        );

        sub.close();
        c.dispose();
      });

      test(
        'same-container: B-excluded principal stays excluded after A',
        () async {
          final svc = _RecordingSearchApiService();
          final c = _providerContainer(
            recordingService: svc,
            currentUserId: 'principal-A',
            useMutablePrincipal: true,
          );
          c.read(_testPrincipalProvider.notifier).state = 'principal-A';

          final cA = Completer<UserSearchResponseDto>();
          svc.delayNextWith(cA);
          c.read(newChatUserSearchProvider('race'));

          // Switch to B.
          c.read(_testPrincipalProvider.notifier).state = 'principal-B';
          final cB = Completer<UserSearchResponseDto>();
          svc.delayNextWith(cB);
          c.read(newChatUserSearchProvider('race'));

          // Complete B.
          cB.complete(
            _responseDto(
              query: 'race',
              users: [
                _userDto(id: 'principal-B', username: 'B-self'),
                _userDto(id: 'target-B', username: 'target_b'),
              ],
            ),
          );
          await Future.delayed(const Duration(milliseconds: 50));

          // Complete A.
          cA.complete(
            _responseDto(
              query: 'race',
              users: [
                _userDto(id: 'principal-A', username: 'A-self'),
                _userDto(id: 'target-A', username: 'target_a'),
              ],
            ),
          );
          await Future.delayed(const Duration(milliseconds: 50));

          final finalVal = c.read(newChatUserSearchProvider('race'));
          expect(finalVal, isA<AsyncData<List<UserSearch>>>());
          final finalData = (finalVal as AsyncData<List<UserSearch>>).value;
          // B-self was excluded using principal-B.
          expect(finalData.any((u) => u.userId == 'principal-B'), isFalse);
          // A-self must NOT appear — the current principal for this instance is B.
          expect(finalData.any((u) => u.userId == 'principal-A'), isFalse);
          expect(finalData.length, 1);
          expect(finalData.first.userId, 'target-B');

          c.dispose();
        },
      );
    });

    // -----------------------------------------------------------------------
    // E) SAME-CONTAINER LOGOUT (PROVIDER LEVEL)
    // -----------------------------------------------------------------------
    // Provider semantics on logout (empty principal):
    //   - The provider rebuilds when currentUserId becomes ''.
    //   - Self-exclusion (user.userId != currentUserId) no longer filters
    //     because '' ≠ any real user ID.  Public search results are returned.
    //   - Returning public results after logout is CORRECT provider behaviour:
    //     the search endpoint has no caller-identity, so it returns all
    //     active users.  The PRESENTATION LAYER (NewChatScreen auth gate)
    //     is the authority that makes these results unreachable.
    //   - Tests below verify the provider-layer contract; screen-layer
    //     logout visibility is proven in the Screen section.
    group('logout', () {
      test('logout: provider rebuilds, returns public results', () async {
        final svc = _RecordingSearchApiService();
        svc.cannedResponse = _responseDto(
          query: 'alice',
          users: [_userDto(id: 'user-1', username: 'alice')],
        );
        final c = _providerContainer(
          recordingService: svc,
          useMutablePrincipal: true,
          currentUserId: 'principal-A',
        );
        c.read(_testPrincipalProvider.notifier).state = 'principal-A';

        // As A: self-exclusion filters principal-A if present.
        var val = await c.read(newChatUserSearchProvider('alice').future);
        expect(val.length, 1);

        // Logout: principal becomes ''.
        c.read(_testPrincipalProvider.notifier).state = '';
        svc.cannedResponse = _responseDto(
          query: 'alice',
          users: [_userDto(id: 'user-1', username: 'alice')],
        );
        val = await c.read(newChatUserSearchProvider('alice').future);

        // Provider returns public results — self-exclusion is inactive
        // because '' matches no real user ID.

        // NOTE: This is correct provider behaviour.  The screen auth gate
        // is the authority that hides these results from a logged-out user.

        expect(val.length, 1);
        expect(val.first.userId, 'user-1');
        c.dispose();
      });

      test('logout: no stale-principal exclusion', () async {
        final svc = _RecordingSearchApiService();
        svc.cannedResponse = _responseDto(
          query: 'test',
          users: [
            _userDto(id: '', username: 'empty-id'),
            _userDto(id: 'user-x', username: 'real'),
          ],
        );
        final c = _providerContainer(
          recordingService: svc,
          useMutablePrincipal: true,
          currentUserId: '',
        );
        c.read(_testPrincipalProvider.notifier).state = '';

        final val = await c.read(newChatUserSearchProvider('test').future);

        // user with id '' IS excluded (user.userId != '' is false).
        // user-x survives as public result.
        expect(val.length, 1);
        expect(val.first.userId, 'user-x');
        c.dispose();
      });
    });
  });

  // ===========================================================================
  // 2) NEWCHATUSERLISTWIDGET RUNTIME
  // ===========================================================================

  group('C1B2 — Row widget', () {
    group('user projection, no avatar', () {
      testWidgets('HybridAvatar binds stable user id', (tester) async {
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            child: NewChatUserListWidget(
              user: _userSearch(userId: _canonicalRowUserId, username: 'john_doe'),
            ),
          ),
        );
        await tester.pump();
        final a = tester.widget<HybridAvatar>(find.byType(HybridAvatar));
        // Owner decision 2026-09-24: no initials fallback anywhere — avatar
        // is photo-or-person-icon, resolved via userId + AvatarCacheService.
        expect(a.userId, _canonicalRowUserId);
        expect(a.savedAvatarUrl, isNull);
      });

      testWidgets('savedAvatarUrl null when search result has none', (tester) async {
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            child: NewChatUserListWidget(
              user: _userSearch(userId: _canonicalRowUserId, username: 'john_doe'),
            ),
          ),
        );
        await tester.pump();
        final a = tester.widget<HybridAvatar>(find.byType(HybridAvatar));
        expect(a.savedAvatarUrl, isNull);
      });

      testWidgets('HybridAvatar renders via userId only (initials fallback removed)', (tester) async {
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            child: NewChatUserListWidget(
              user: _userSearch(userId: _canonicalRowUserId, username: 'john_doe'),
            ),
          ),
        );
        await tester.pump();
        final a = tester.widget<HybridAvatar>(find.byType(HybridAvatar));
        // farmName no longer derives initials — the avatar is resolved from
        // userId via AvatarCacheService; savedAvatarUrl stays null here.
        expect(a.userId, _canonicalRowUserId);
        expect(a.savedAvatarUrl, isNull);
      });

      testWidgets('no location text when location null', (tester) async {
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            child: NewChatUserListWidget(
              user: _userSearch(userId: _canonicalRowUserId, username: 'john_doe'),
            ),
          ),
        );
        await tester.pump();
        expect(find.text('Jakarta'), findsNothing);
        // location is null, so no location text
        final texts = tester.widgetList<Text>(find.byType(Text)).map((t) => t.data ?? '');
        expect(texts.any((t) => t.contains('Location')), isFalse);
      });

      testWidgets('search projection renders handle, never location', (tester) async {
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            child: NewChatUserListWidget(
              user: _userSearch(userId: _canonicalRowUserId, username: 'john_doe'),
            ),
          ),
        );
        await tester.pump();
        expect(find.text('Jakarta'), findsNothing);
        expect(find.text('@john_doe'), findsOneWidget);
      });

      testWidgets('no verification icon; chevron shown', (tester) async {
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            child: NewChatUserListWidget(
              user: _userSearch(userId: _canonicalRowUserId, username: 'john_doe'),
            ),
          ),
        );
        await tester.pump();
        expect(find.byIcon(Icons.verified), findsNothing);
        expect(find.byIcon(Icons.chevron_right), findsOneWidget);
      });
    });

    group('user projection with avatarUrl', () {
      testWidgets('search-result avatarUrl wired as instant fallback', (tester) async {
        const url = 'https://img.example/avatar.png';
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            child: NewChatUserListWidget(
              user: _userSearch(userId: _canonicalAvatarRowUserId, username: 'cool_bob', avatarUrl: url),
            ),
          ),
        );
        await tester.pump();
        final a = tester.widget<HybridAvatar>(find.byType(HybridAvatar));
        // The UserSearch projection's avatarUrl is passed straight to the
        // canonical resolver as instant fallback (Owner decision 2026-09-24:
        // photo or icon, no initials).
        expect(a.userId, _canonicalAvatarRowUserId);
        expect(a.savedAvatarUrl, url);
      });
    });

    group('empty username', () {
      testWidgets('no initials state when username empty', (tester) async {
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            child: NewChatUserListWidget(
              user: _userSearch(userId: _canonicalRowUserId, username: ''),
            ),
          ),
        );
        await tester.pump();
        final a = tester.widget<HybridAvatar>(find.byType(HybridAvatar));
        // Initials no longer exist on the widget contract.
        expect(a.userId, _canonicalRowUserId);
        expect(find.byType(HybridAvatar), findsOneWidget);
      });

      testWidgets('empty username → canonical "User" label', (tester) async {
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            child: NewChatUserListWidget(
              user: _userSearch(userId: _canonicalRowUserId, username: ''),
            ),
          ),
        );
        await tester.pump();
        // Canonical label: blank username → 'User' fallback label.
        expect(find.text('@john_doe'), findsNothing);
        expect(find.text('User'), findsOneWidget);
      });
    });

    group('metadata removal', () {
      testWidgets('no farm/store text as separate identity', (tester) async {
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            child: NewChatUserListWidget(
              user: _userSearch(userId: _canonicalRowUserId, username: 'alice'),
            ),
          ),
        );
        await tester.pump();
        // No standalone farm/store language
        expect(find.text('Farm'), findsNothing);
        expect(find.text('Store'), findsNothing);
      });

      testWidgets('identity label is handle, no email/phone', (tester) async {
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            child: NewChatUserListWidget(
              user: _userSearch(userId: _canonicalRowUserId, username: 'alice'),
            ),
          ),
        );
        await tester.pump();
        // Canonical label = @username handle; email/phone never render.
        expect(find.text('@alice'), findsOneWidget);
      });

      testWidgets('no userId text exposed', (tester) async {
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            child: NewChatUserListWidget(
              user: _userSearch(userId: 'user-x-12345', username: 'alice'),
            ),
          ),
        );
        await tester.pump();
        expect(find.text('user-x-12345'), findsNothing);
      });
    });
  });

  // ===========================================================================
  // 3) TAP HANDLER
  // ===========================================================================

  group('C1B2 — Tap handler', () {
    group('success', () {
      testWidgets('repo receives [principal, target]', (tester) async {
        final repo = _RecordingChatRepository();
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            chatRepo: repo,
            currentUserId: 'principal-X',
            child: NewChatUserListWidget(
              user: _userSearch(userId: 'target-Y', username: 'tu'),
            ),
          ),
        );
        await tester.pump();
        await tester.tap(find.byType(InkWell));
        await tester.pump();
        await tester.pump(const Duration(seconds: 1));

        expect(repo.getOrCreateChatCallCount, 1);
        expect(repo.participantIdsCalls.first, ['principal-X', 'target-Y']);
      });

      testWidgets('username not used as ID', (tester) async {
        final repo = _RecordingChatRepository();
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            chatRepo: repo,
            currentUserId: 'principal-X',
            child: NewChatUserListWidget(
              user: _userSearch(userId: 'real-id-123', username: 'cool'),
            ),
          ),
        );
        await tester.pump();
        await tester.tap(find.byType(InkWell));
        await tester.pump();
        await tester.pump(const Duration(seconds: 1));

        expect(repo.participantIdsCalls.first, ['principal-X', 'real-id-123']);
      });
    });

    group('principal switch before tap', () {
      testWidgets('tap reads live principal', (tester) async {
        final repo = _RecordingChatRepository();
        final svc = _RecordingSearchApiService();

        // Build with principal A, then switch to B.
        String livePrincipal = 'principal-A';

        final goRouter = GoRouter(
          initialLocation: '/',
          routes: [
            GoRoute(path: '/', builder: (_, _) => const SizedBox.shrink()),
          ],
        );

        // First build with A.
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              currentUserIdProvider.overrideWith((ref) => livePrincipal),
              isEmailVerifiedProvider.overrideWith((ref) => true),
              chatRepositoryProvider.overrideWith((ref) => repo),
              searchApiServiceProvider.overrideWith((ref) => svc),
              authControllerProvider.overrideWith(
                () => _FakeAuthController(
                  AuthState.authenticated(
                    _authUser(livePrincipal),
                    emailVerified: true,
                  ),
                ),
              ),
              avatarCacheServiceProvider.overrideWith(
                (ref) => _NoOpAvatarCacheService(),
              ),
            ],
            child: MaterialApp.router(routerConfig: goRouter),
          ),
        );
        await tester.pump();

        // Pump the row with Builder so we can read live principal.
        livePrincipal = 'principal-B';
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              currentUserIdProvider.overrideWith((ref) => livePrincipal),
              isEmailVerifiedProvider.overrideWith((ref) => true),
              chatRepositoryProvider.overrideWith((ref) => repo),
              searchApiServiceProvider.overrideWith((ref) => svc),
              authControllerProvider.overrideWith(
                () => _FakeAuthController(
                  AuthState.authenticated(
                    _authUser(livePrincipal),
                    emailVerified: true,
                  ),
                ),
              ),
              avatarCacheServiceProvider.overrideWith(
                (ref) => _NoOpAvatarCacheService(),
              ),
            ],
            child: MaterialApp.router(routerConfig: goRouter),
          ),
        );
        // Pump the actual row.
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              currentUserIdProvider.overrideWith((ref) => livePrincipal),
              isEmailVerifiedProvider.overrideWith((ref) => true),
              chatRepositoryProvider.overrideWith((ref) => repo),
              searchApiServiceProvider.overrideWith((ref) => svc),
              authControllerProvider.overrideWith(
                () => _FakeAuthController(
                  AuthState.authenticated(
                    _authUser(livePrincipal),
                    emailVerified: true,
                  ),
                ),
              ),
              avatarCacheServiceProvider.overrideWith(
                (ref) => _NoOpAvatarCacheService(),
              ),
            ],
            child: MaterialApp(
              home: Scaffold(
                body: NewChatUserListWidget(
              user: _userSearch(userId: 'target-Y', username: 'target_user'),
            ),
              ),
            ),
          ),
        );
        await tester.pump();

        await tester.tap(find.byType(InkWell));
        await tester.pump();
        await tester.pump(const Duration(seconds: 1));

        expect(repo.getOrCreateChatCallCount, 1);
        expect(repo.participantIdsCalls.first, ['principal-B', 'target-Y']);
      });
    });

    group('self-chat', () {
      testWidgets('self profile is blocked at row level via live principal', (tester) async {
        final repo = _RecordingChatRepository();
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            chatRepo: repo,
            currentUserId: 'same-user',
            child: NewChatUserListWidget(
              user: _userSearch(userId: 'same-user', username: 'me'),
            ),
          ),
        );
        await tester.pump();
        await tester.tap(find.byType(InkWell));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 100));
        // Canonical: the row blocks self-chat using the LIVE principal read
        // at tap time (defense in depth — the provider also excludes self).
        expect(repo.getOrCreateChatCallCount, 0);
        expect(repo.participantIdsCalls, isEmpty);
      });
    });

    group('repo failure', () {
      testWidgets('loading dismissed, no unhandled exception', (tester) async {
        final repo = _RecordingChatRepository();
        repo.nextError = Exception('fail');
        await tester.pumpWidget(
          _wrapWidget(
            recordingService: _RecordingSearchApiService(),
            chatRepo: repo,
            currentUserId: 'principal-X',
            child: NewChatUserListWidget(
              user: _userSearch(userId: 'target-Z', username: 'tu'),
            ),
          ),
        );
        await tester.pump();
        await tester.tap(find.byType(InkWell));
        await tester.pump();
        await tester.pump(const Duration(seconds: 1));

        // Loading dialog must be dismissed after error.
        // The AppSnackBar might not be captured in widget tree easily,
        // but loading indicator must be gone.
        expect(find.byType(CircularProgressIndicator), findsNothing);
        // The test not crashing proves exception was handled.
      });
    });
  });

  // ===========================================================================
  // 4) NEWCHATSCREEN RUNTIME
  // ===========================================================================

  group('C1B2 — Screen', () {
    testWidgets('blank query → prompt', (tester) async {
      await tester.pumpWidget(
        _wrapScreen(recordingService: _RecordingSearchApiService()),
      );
      await tester.pump();
      expect(find.text('Search user to start a chat'), findsOneWidget);
      expect(find.text('Type a name or username'), findsOneWidget);
      expect(find.byIcon(Icons.search), findsWidgets);
    });

    testWidgets('typing → loading', (tester) async {
      final svc = _RecordingSearchApiService();
      final c = Completer<UserSearchResponseDto>();
      svc.delayNextWith(c);
      await tester.pumpWidget(_wrapScreen(recordingService: svc));
      await tester.pump();
      await tester.enterText(find.byType(TextField), 'alice');
      await tester.pump();
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      c.complete(_responseDto(query: 'alice', users: []));
    });

    testWidgets('error → "Failed to search users"', (tester) async {
      final svc = _RecordingSearchApiService();
      svc.cannedError = Exception('boom');
      await tester.pumpWidget(_wrapScreen(recordingService: svc));
      await tester.pump();
      await tester.enterText(find.byType(TextField), 'err');
      await tester.pumpAndSettle(const Duration(milliseconds: 500));
      expect(find.text('Failed to search users'), findsOneWidget);
    });

    testWidgets('empty result → "User not found"', (tester) async {
      await tester.pumpWidget(
        _wrapScreen(recordingService: _RecordingSearchApiService()),
      );
      await tester.pump();
      await tester.enterText(find.byType(TextField), 'nobody');
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 200));
      expect(find.text('User not found'), findsOneWidget);
    });

    testWidgets('populated → rows with HybridAvatar', (tester) async {
      final svc = _RecordingSearchApiService();
      svc.cannedResponse = _responseDto(
        query: 'alice',
        users: [_userDto(id: 'u1', username: 'alice_wonder')],
      );
      await tester.pumpWidget(_wrapScreen(recordingService: svc));
      await tester.pump();
      await tester.enterText(find.byType(TextField), 'alice');
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 200));
      expect(find.byType(NewChatUserListWidget), findsOneWidget);
      expect(find.byType(HybridAvatar), findsOneWidget);
    });

    testWidgets('rows receive UserSearch', (tester) async {
      final svc = _RecordingSearchApiService();
      svc.cannedResponse = _responseDto(
        query: 'bob',
        users: [
          _userDto(
            id: 'bob-id',
            username: 'bob_builder',
            avatarUrl: 'https://img.example/bob.png',
          ),
        ],
      );
      await tester.pumpWidget(_wrapScreen(recordingService: svc));
      await tester.pump();
      await tester.enterText(find.byType(TextField), 'bob');
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 200));
      final row = tester.widget<NewChatUserListWidget>(find.byType(NewChatUserListWidget));
      expect(row.user, isA<UserSearch>());
      expect(row.user.userId, 'bob-id');
      expect(row.user.username, 'bob_builder');
      expect(row.user.avatarUrl, 'https://img.example/bob.png');
    });

    testWidgets('no duplicate self-exclusion', (tester) async {
      final svc = _RecordingSearchApiService();
      svc.cannedResponse = _responseDto(
        query: 'me',
        users: [
          _userDto(id: 'principal-A', username: 'me_user'),
          _userDto(id: 'other-1', username: 'other'),
        ],
      );
      await tester.pumpWidget(_wrapScreen(recordingService: svc, currentUserId: 'principal-A'));
      await tester.pump();
      await tester.enterText(find.byType(TextField), 'me');
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 200));
      expect(find.byType(NewChatUserListWidget), findsOneWidget);
      // Only other-1 should be shown — self-exclusion happens once, in the
      // provider (live principal), not again in the screen.
      final row = tester.widget<NewChatUserListWidget>(find.byType(NewChatUserListWidget));
      expect(row.user.userId, 'other-1');
    });

    testWidgets('query switch — stale A does not replace B', (tester) async {
      final svc = _RecordingSearchApiService();
      final cA = Completer<UserSearchResponseDto>();
      svc.delayNextWith(cA);
      await tester.pumpWidget(_wrapScreen(recordingService: svc));
      await tester.pump();
      await tester.enterText(find.byType(TextField), 'alice');
      await tester.pump();
      expect(svc.callCount, 1);
      expect(svc.lastQuery, 'alice');
      final cB = Completer<UserSearchResponseDto>();
      svc.delayNextWith(cB);
      await tester.enterText(find.byType(TextField), '');
      await tester.pump();
      await tester.enterText(find.byType(TextField), 'bob');
      await tester.pump();
      expect(svc.callCount, 2);
      expect(svc.lastQuery, 'bob');
      cB.complete(_responseDto(query: 'bob', users: [_userDto(id: 'bob-id', username: 'bob_builder')]));
      await tester.pump(const Duration(milliseconds: 200));
      expect(find.byType(NewChatUserListWidget), findsOneWidget);
      cA.complete(_responseDto(query: 'alice', users: [_userDto(id: 'alice-id', username: 'alice_wonder')]));
      await tester.pump(const Duration(milliseconds: 200));
      // Still bob, not alice
      final row = tester.widget<NewChatUserListWidget>(find.byType(NewChatUserListWidget));
      expect(row.user.userId, 'bob-id');
    });

    testWidgets('unauthenticated: login prompt, no results leaked', (tester) async {
      final svc = _RecordingSearchApiService();
      svc.cannedResponse = _responseDto(
        query: 'alice',
        users: [_userDto(id: 'u1', username: 'alice_wonder')],
      );
      final chatRepo = _RecordingChatRepository();
      final goRouter = GoRouter(initialLocation: '/', routes: [GoRoute(path: '/', builder: (_, __) => const NewChatScreen())]);
      await tester.pumpWidget(ProviderScope(overrides: [
        searchApiServiceProvider.overrideWith((ref) => svc),
        currentUserIdProvider.overrideWith((ref) => ''),
        isEmailVerifiedProvider.overrideWith((ref) => false),
        authControllerProvider.overrideWith(() => _FakeAuthController(const AuthStateUnauthenticated())),
        avatarCacheServiceProvider.overrideWith((ref) => _NoOpAvatarCacheService()),
        chatRepositoryProvider.overrideWith((ref) => chatRepo),
      ], child: MaterialApp.router(routerConfig: goRouter)));
      await tester.pump();
      expect(find.text('Please log in first'), findsOneWidget);
      expect(find.byType(TextField), findsNothing);
      expect(find.byType(NewChatUserListWidget), findsNothing);
      expect(find.byType(CircularProgressIndicator), findsNothing);
      expect(chatRepo.getOrCreateChatCallCount, 0);
    });

    testWidgets('same-tree: authenticated → logout removes results, shows gate', (tester) async {
      final svc = _RecordingSearchApiService();
      svc.cannedResponse = _responseDto(
        query: 'target',
        users: [_userDto(id: 'target-user', username: 'target_user')],
      );
      final chatRepo = _RecordingChatRepository();
      final authCtrl = _MutableFakeAuthController(AuthState.authenticated(_authUser('principal-A'), emailVerified: true));
      final container = ProviderContainer(overrides: [
        searchApiServiceProvider.overrideWith((ref) => svc),
        currentUserIdProvider.overrideWith((ref) => 'principal-A'),
        isEmailVerifiedProvider.overrideWith((ref) => true),
        authControllerProvider.overrideWith(() => authCtrl),
        avatarCacheServiceProvider.overrideWith((ref) => _NoOpAvatarCacheService()),
        chatRepositoryProvider.overrideWith((ref) => chatRepo),
      ]);
      await tester.pumpWidget(UncontrolledProviderScope(container: container, child: const MaterialApp(home: NewChatScreen())));
      await tester.pump();
      await tester.enterText(find.byType(TextField), 'target');
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 200));
      expect(find.byType(NewChatUserListWidget), findsOneWidget);
      authCtrl.setAuthState(const AuthStateUnauthenticated());
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 200));
      expect(find.text('Please log in first'), findsOneWidget);
      expect(find.byType(NewChatUserListWidget), findsNothing);
      expect(find.byType(HybridAvatar), findsNothing);
      expect(find.byType(TextField), findsNothing);
      expect(find.byType(CircularProgressIndicator), findsNothing);
      expect(chatRepo.getOrCreateChatCallCount, 0);
      container.dispose();
    });
  });
}
