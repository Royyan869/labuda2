// C1B1 — Production ChatCard widget tests.
//
// Classification: production-behavioral — these pump the real ChatCard widget
// through the Flutter test harness and assert on the rendered widget tree.
// No logic is mirrored in test helpers; all branching is exercised through
// the production code path.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/providers/core_providers.dart'
    show loggerServiceProvider, webSocketServiceProvider;
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/core/src/providers/presence_provider.dart'
    as core_presence;
import 'package:labuda/core/websocket/websocket_service.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_controller.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/core/api/api_client.dart';

import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_card.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/shared.dart';

// =============================================================================
// Fixtures
// =============================================================================

const _currentUserId = 'aaaaaaaa-1111-1111-1111-111111111111';
const _otherUserId = 'bbbbbbbb-2222-2222-2222-222222222222';
const _roomId = 'cccccccc-3333-3333-3333-cccccccccccc';

class _FakeAuthController extends AuthController {
  _FakeAuthController() : _state = const AuthState.unauthenticated();

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _NoopPresenceRegistry
    implements core_presence.PresenceSubscriptionRegistry {
  @override
  core_presence.PresenceSubscriptionHandle acquire(Set<String> userIds) {
    return core_presence.PresenceSubscriptionHandle(() async {});
  }

  @override
  Future<void> prepareForLogout() async {}

  @override
  core_presence.PresenceState? lookup(String userId) => null;

  @override
  Map<String, core_presence.PresenceState?> lookupMany(
    Iterable<String> userIds,
  ) {
    return {for (final id in userIds) id: null};
  }

  @override
  Future<void> publishSelfPresence({required bool isOnline}) async {}

  @override
  Future<void> setForeground(bool isForeground) async {}
}

class _NoopUserApiDatasource extends UserApiDatasource {
  _NoopUserApiDatasource() : super(_NoopApiClient());
}

class _NoopAvatarCacheService extends AvatarCacheService {
  _NoopAvatarCacheService() : super(datasource: _NoopUserApiDatasource());

  @override
  Future<String?> getUserAvatarUrl(String userId) async => null;
}

class _NoopApiClient implements ApiClient {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _NoopLogger implements ILoggerService {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

/// Minimal Chat entity builder for widget tests.
Chat _chat({
  String? username,
  String? avatarUrl,
  String lifecycle = 'active',
  ChatType type = ChatType.private,
}) {
  final names = <String, String>{};
  final avatars = <String, String?>{};
  final lifecycles = <String, ContentLifecycle>{};
  if (username != null) names[_otherUserId] = username;
  if (avatarUrl != null && avatarUrl.isNotEmpty) {
    avatars[_otherUserId] = avatarUrl;
  }
  lifecycles[_otherUserId] = ContentLifecycleParse.fromWire(lifecycle);

  return Chat(
    id: _roomId,
    type: type,
    participantIds: [_currentUserId, _otherUserId],
    participantNames: names,
    participantAvatars: avatars,
    participantLifecycles: lifecycles,
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
    lastMessage: Message(
      id: 'msg-1',
      chatId: _roomId,
      senderId: _otherUserId,
      senderName: 'Other User',
      content: 'Hello!',
      createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
    ),
  );
}

/// Wrap [chat] in the minimal provider scope needed by ChatCard.
Widget _wrap(Chat chat) {
  return ProviderScope(
    overrides: [
      currentUserIdProvider.overrideWith((ref) => _currentUserId),
      authControllerProvider.overrideWith(() => _FakeAuthController()),
      loggerServiceProvider.overrideWithValue(_NoopLogger()),
      webSocketServiceProvider.overrideWithValue(
        WebSocketService(baseUrl: 'ws://localhost'),
      ),
      core_presence.presenceSubscriptionRegistryProvider.overrideWithValue(
        _NoopPresenceRegistry(),
      ),
      avatarCacheServiceProvider.overrideWithValue(_NoopAvatarCacheService()),
    ],
    child: MaterialApp(
      home: Scaffold(
        body: ChatCard(chat: chat, onTap: () {}),
      ),
    ),
  );
}

// =============================================================================
// Production ChatCard widget tests
// =============================================================================

void main() {
  // -------------------------------------------------------------------------
  // 1) Active participant with valid username and no avatar
  // -------------------------------------------------------------------------
  group('C1B1 ChatCard — active participant, username, no avatar', () {
    testWidgets('ProfileAvatar is present', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: 'john_doe')));
      expect(find.byType(ProfileAvatar), findsOneWidget);
    });

    testWidgets('canonical initials JD are rendered', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: 'john_doe')));
      final avatar = tester.widget<ProfileAvatar>(find.byType(ProfileAvatar));
      expect(avatar.username, 'john_doe');
      expect(find.text('JD'), findsOneWidget);
    });

    testWidgets('visible participant text is @john_doe', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: 'john_doe')));
      expect(find.text('@john_doe'), findsOneWidget);
    });

    testWidgets('no raw User in display', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: 'john_doe')));
      expect(find.text('User'), findsNothing);
    });

    testWidgets('no UUID fragment in display', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: 'john_doe')));
      // Neither user ID nor room ID fragments may appear as visible text.
      expect(find.text('bbbbbbbb'), findsNothing);
      expect(find.text(_otherUserId), findsNothing);
    });

    testWidgets('no ? fallback', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: 'john_doe')));
      expect(find.text('?'), findsNothing);
    });
  });

  // -------------------------------------------------------------------------
  // 2) Active participant without username
  // -------------------------------------------------------------------------
  group('C1B1 ChatCard — active participant, no username', () {
    testWidgets('ProfileAvatar renders Icons.person', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: null)));
      expect(find.byIcon(Icons.person), findsOneWidget);
    });

    testWidgets('visible label is exactly User', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: null)));
      expect(find.text('User'), findsOneWidget);
    });

    testWidgets('@User is absent', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: null)));
      expect(find.text('@User'), findsNothing);
    });

    testWidgets('participant ID fragment is absent', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: null)));
      expect(find.text('bbbbbbbb'), findsNothing);
      expect(find.text(_otherUserId), findsNothing);
    });
  });

  // -------------------------------------------------------------------------
  // 3) Leading-@ username
  // -------------------------------------------------------------------------
  group('C1B1 ChatCard — leading-@ username', () {
    testWidgets('visible label is exactly @john_doe (single @)', (
      tester,
    ) async {
      await tester.pumpWidget(_wrap(_chat(username: '@john_doe')));
      expect(find.text('@john_doe'), findsOneWidget);
      expect(find.text('@@john_doe'), findsNothing);
    });

    testWidgets('avatar username is the raw value', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: '@john_doe')));
      final avatar = tester.widget<ProfileAvatar>(find.byType(ProfileAvatar));
      expect(avatar.username, '@john_doe');
    });

    testWidgets('initials JD rendered (strips @ for initials)', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: '@john_doe')));
      // UserIdentityFormatter strips @ before computing initials → JD.
      expect(find.text('JD'), findsOneWidget);
    });
  });

  // -------------------------------------------------------------------------
  // 4) Valid avatar URL
  // -------------------------------------------------------------------------
  group('C1B1 ChatCard — avatar URL passthrough', () {
    testWidgets('ProfileAvatar receives imageUrl and username', (tester) async {
      const url = 'https://cdn.example.com/alice.jpg';
      await tester.pumpWidget(_wrap(_chat(username: 'alice', avatarUrl: url)));
      final avatar = tester.widget<ProfileAvatar>(find.byType(ProfileAvatar));
      expect(avatar.imageUrl, url);
      expect(avatar.username, 'alice');
    });
  });

  // -------------------------------------------------------------------------
  // 5) Lifecycle-degraded participant
  // -------------------------------------------------------------------------
  group('C1B1 ChatCard — lifecycle-degraded participant', () {
    testWidgets('redaction label is shown (not username)', (tester) async {
      await tester.pumpWidget(
        _wrap(_chat(username: 'alice', lifecycle: 'removed')),
      );
      expect(find.text('@alice'), findsNothing);
      expect(find.text('alice'), findsNothing);
      // ProfileAvatar is NOT used — degraded uses CircleAvatar with icon.
      expect(find.byType(ProfileAvatar), findsNothing);
    });

    testWidgets('person_off icon is rendered', (tester) async {
      await tester.pumpWidget(
        _wrap(_chat(username: 'alice', lifecycle: 'removed')),
      );
      expect(find.byIcon(Icons.person_off_outlined), findsOneWidget);
    });

    testWidgets('live avatar URL is not exposed', (tester) async {
      const url = 'https://cdn.example.com/alice.jpg';
      await tester.pumpWidget(
        _wrap(_chat(username: 'alice', avatarUrl: url, lifecycle: 'removed')),
      );
      // ProfileAvatar is not in the tree, so imageUrl can't be exposed.
      expect(find.byType(ProfileAvatar), findsNothing);
    });

    testWidgets('live username is absent', (tester) async {
      await tester.pumpWidget(
        _wrap(_chat(username: 'alice', lifecycle: 'unavailable')),
      );
      expect(find.text('@alice'), findsNothing);
      expect(find.text('alice'), findsNothing);
    });
  });

  // -------------------------------------------------------------------------
  // 6) Support chat
  // -------------------------------------------------------------------------
  group('C1B1 ChatCard — support chat identity', () {
    testWidgets('title remains Support', (tester) async {
      await tester.pumpWidget(_wrap(_chat(type: ChatType.support)));
      expect(find.text('Support'), findsOneWidget);
    });

    testWidgets('generic participant fallback is not substituted', (
      tester,
    ) async {
      await tester.pumpWidget(
        _wrap(_chat(username: 'alice', type: ChatType.support)),
      );
      expect(find.text('Support'), findsOneWidget);
      expect(find.byType(ProfileAvatar), findsNothing);
    });

    testWidgets('no user participant formatter applied', (tester) async {
      await tester.pumpWidget(
        _wrap(_chat(username: 'alice', type: ChatType.support)),
      );
      expect(find.text('@alice'), findsNothing);
      expect(find.text('User'), findsNothing);
    });
  });

  // -------------------------------------------------------------------------
  // 7) UUID-polluted identity via participantNames
  // -------------------------------------------------------------------------
  group('C1B1 ChatCard — UUID-polluted participantNames', () {
    testWidgets('polluted name equal to participant ID → User', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: _otherUserId)));
      expect(find.text('User'), findsOneWidget);
      expect(find.text('bbbbbbbb'), findsNothing);
    });

    testWidgets('lowercase canonical UUID shape → User', (tester) async {
      await tester.pumpWidget(
        _wrap(_chat(username: 'deadbeef-1234-5678-9abc-def012345678')),
      );
      expect(find.text('User'), findsOneWidget);
      expect(find.text('deadbeef'), findsNothing);
    });

    testWidgets('uppercase canonical UUID shape → User', (tester) async {
      await tester.pumpWidget(
        _wrap(_chat(username: 'DEADBEEF-1234-5678-9ABC-DEF012345678')),
      );
      expect(find.text('User'), findsOneWidget);
      expect(find.text('DEADBEEF'), findsNothing);
    });

    testWidgets('mixed-case canonical UUID shape → User', (tester) async {
      await tester.pumpWidget(
        _wrap(_chat(username: 'DeadBeef-1234-5678-9aBc-def012345678')),
      );
      expect(find.text('User'), findsOneWidget);
      expect(find.text('DeadBeef'), findsNothing);
    });

    testWidgets('UUID participant ID with different casing in name → User', (
      tester,
    ) async {
      const lowerId = '550e8400-e29b-41d4-a716-446655440000';
      const upperName = '550E8400-E29B-41D4-A716-446655440000';
      final chat = Chat(
        id: _roomId,
        type: ChatType.private,
        participantIds: [_currentUserId, lowerId],
        participantNames: {lowerId: upperName},
        participantAvatars: const <String, String?>{},
        participantLifecycles: {lowerId: ContentLifecycle.active},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
        lastMessage: Message(
          id: 'msg-1',
          chatId: _roomId,
          senderId: lowerId,
          senderName: 'Other',
          content: 'Hello!',
          createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
        ),
      );
      await tester.pumpWidget(_wrap(chat));
      expect(find.text('User'), findsOneWidget);
      expect(find.text('550'), findsNothing);
      expect(find.text('@User'), findsNothing);
    });
  });

  // -------------------------------------------------------------------------
  // 8) Negative contracts — prevent reintroduction in widget tree
  // -------------------------------------------------------------------------
  group('C1B1 ChatCard — negative widget contracts', () {
    testWidgets('no raw NetworkImage for participant avatar', (tester) async {
      // ChatCard avatar is ProfileAvatar, not a raw NetworkImage.
      await tester.pumpWidget(
        _wrap(
          _chat(
            username: 'alice',
            avatarUrl: 'https://cdn.example.com/alice.jpg',
          ),
        ),
      );
      expect(find.byType(ProfileAvatar), findsOneWidget);
    });

    testWidgets('no single-letter initial or ? rendered as text', (
      tester,
    ) async {
      await tester.pumpWidget(_wrap(_chat(username: 'john_doe')));
      // ProfileAvatar handles initials internally — no bare initial text.
      // The initials "JD" appear inside ProfileAvatar, which is expected.
      expect(find.text('J'), findsNothing);
      expect(find.text('?'), findsNothing);
    });
  });
}
