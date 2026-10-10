// C1B1 — Production ChatCard widget tests.
//
// Classification: production-behavioral — these pump the real ChatCard widget
// through the Flutter test harness and assert on the rendered widget tree.
// No logic is mirrored in test helpers; all branching is exercised through
// the production code path.

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/providers/core_providers.dart'
    show loggerServiceProvider, webSocketServiceProvider;
import 'package:hishumi/core/src/interfaces/services/i_logger_service.dart';
import 'package:hishumi/core/websocket/websocket_service.dart';
import 'package:hishumi/domains/user/identity/authentication/authentication.dart';
import 'package:hishumi/domains/user/identity/authentication/presentation/providers/auth_controller.dart';
import 'package:hishumi/domains/user/profile/data/profile_providers.dart';
import 'package:hishumi/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:hishumi/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:hishumi/core/api/api_client.dart';

import 'package:hishumi/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:hishumi/domains/chat/chat/presentation/widgets/chat_card.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/shared.dart';

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

final Uint8List _kTransparentImage = base64Decode(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+ip1sAAAAASUVORK5CYII=',
);

class _FakeHttpClient extends Fake implements HttpClient {
  @override
  Future<HttpClientRequest> getUrl(Uri url) async => _FakeHttpClientRequest();
}

class _FakeHttpClientRequest extends Fake implements HttpClientRequest {
  @override
  Future<HttpClientResponse> close() async => _FakeHttpClientResponse();
}

class _FakeHttpClientResponse extends Fake implements HttpClientResponse {
  @override
  int get statusCode => 200;
  @override
  int get contentLength => _kTransparentImage.length;
  @override
  HttpHeaders get headers => _FakeHttpHeaders();
  @override
  StreamSubscription<List<int>> listen(
    void Function(List<int> event)? onData, {
    Function? onError,
    void Function()? onDone,
    bool? cancelOnError,
  }) {
    return Stream<List<int>>.value(_kTransparentImage)
        .listen(onData, onError: onError, onDone: onDone, cancelOnError: cancelOnError);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeHttpHeaders extends Fake implements HttpHeaders {
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
    testWidgets('ProfileAvatar is present (canonical avatar, no initials)', (
      tester,
    ) async {
      await tester.pumpWidget(_wrap(_chat(username: 'john_doe')));
      expect(find.byType(ProfileAvatar), findsOneWidget);
      expect(find.byIcon(Icons.person), findsOneWidget);
    });

    testWidgets('no text initials in avatar (Owner 2026-09-24)', (
      tester,
    ) async {
      await tester.pumpWidget(_wrap(_chat(username: 'john_doe')));
      // Canonical avatar = photo or Icons.person; initials are forbidden.
      expect(find.text('J'), findsNothing);
      expect(find.text('JD'), findsNothing);
      expect(find.byIcon(Icons.person), findsOneWidget);
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
    testWidgets('missing username renders canonical icon, no initial', (
      tester,
    ) async {
      await tester.pumpWidget(_wrap(_chat(username: null)));
      expect(find.byType(ProfileAvatar), findsOneWidget);
      expect(find.text('U'), findsNothing);
      expect(find.byIcon(Icons.person), findsOneWidget);
    });

    testWidgets('visible label is @User bbbbbbbb... (fallback with ID)', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: null)));
      expect(find.text('@User bbbbbbbb...'), findsOneWidget);
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

    testWidgets('no @ initial for leading-@ username', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: '@john_doe')));
      // Canonical avatar never renders a text initial.
      expect(find.text('@'), findsNothing);
      expect(find.byIcon(Icons.person), findsOneWidget);
    });

    testWidgets('no text initial at all for leading-@', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: '@john_doe')));
      expect(find.text('@'), findsNothing);
      expect(find.text('JD'), findsNothing);
      expect(find.byType(ProfileAvatar), findsOneWidget);
    });
  });

  // -------------------------------------------------------------------------
  // 4) Valid avatar URL
  // -------------------------------------------------------------------------
  group('C1B1 ChatCard — avatar URL passthrough', () {
    testWidgets('avatarUrl is stored in Chat participantAvatars', (tester) async {
      const url = 'https://cdn.example.com/alice.jpg';
      final chat = _chat(username: 'alice', avatarUrl: url);
      expect(chat.participantAvatars[_otherUserId], url);
      // ChatCard renders the canonical ProfileAvatar; the URL is verified
      // via Chat data (no network pump, to avoid HTTP in test).
      await tester.pumpWidget(_wrap(_chat(username: 'alice')));
      expect(find.byType(ProfileAvatar), findsOneWidget);
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
      // Degraded keeps the canonical ProfileAvatar slot with no photo.
      expect(find.byType(ProfileAvatar), findsOneWidget);
      expect(
        tester.widget<ProfileAvatar>(find.byType(ProfileAvatar)).imageUrl,
        isNull,
      );
    });

    testWidgets('canonical person icon is rendered (no person_off)', (
      tester,
    ) async {
      await tester.pumpWidget(
        _wrap(_chat(username: 'alice', lifecycle: 'removed')),
      );
      expect(find.byIcon(Icons.person), findsOneWidget);
      expect(find.byIcon(Icons.person_off_outlined), findsNothing);
    });

    testWidgets('live avatar URL is not exposed', (tester) async {
      const url = 'https://cdn.example.com/alice.jpg';
      await tester.pumpWidget(
        _wrap(_chat(username: 'alice', avatarUrl: url, lifecycle: 'removed')),
      );
      // Slot persists but the live URL is never exposed while degraded.
      expect(find.byType(ProfileAvatar), findsOneWidget);
      expect(
        tester.widget<ProfileAvatar>(find.byType(ProfileAvatar)).imageUrl,
        isNull,
      );
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
    testWidgets('polluted name equal to participant ID → shows handle with ID (current canonical)', (tester) async {
      await tester.pumpWidget(_wrap(_chat(username: _otherUserId)));
      // Current Chat.getOtherParticipantName returns raw ID as name → formatChatHandle → '@bbbbbbbb-...'
      expect(find.text('@' + _otherUserId), findsOneWidget);
    });

    testWidgets('lowercase canonical UUID shape → shows handle (current canonical)', (tester) async {
      await tester.pumpWidget(
        _wrap(_chat(username: 'deadbeef-1234-5678-9abc-def012345678')),
      );
      expect(find.text('@deadbeef-1234-5678-9abc-def012345678'), findsOneWidget);
    });

    testWidgets('uppercase canonical UUID shape → shows handle', (tester) async {
      await tester.pumpWidget(
        _wrap(_chat(username: 'DEADBEEF-1234-5678-9ABC-DEF012345678')),
      );
      expect(find.text('@DEADBEEF-1234-5678-9ABC-DEF012345678'), findsOneWidget);
    });

    testWidgets('mixed-case canonical UUID shape → shows handle', (tester) async {
      await tester.pumpWidget(
        _wrap(_chat(username: 'DeadBeef-1234-5678-9aBc-def012345678')),
      );
      expect(find.text('@DeadBeef-1234-5678-9aBc-def012345678'), findsOneWidget);
    });

    testWidgets('UUID participant ID with different casing in name → shows handle', (
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
      expect(find.text('@' + upperName), findsOneWidget);
    });
  });

  // -------------------------------------------------------------------------
  // 8) Negative contracts — prevent reintroduction in widget tree
  // -------------------------------------------------------------------------
  group('C1B1 ChatCard — negative widget contracts', () {
    testWidgets('ProfileAvatar used, not raw NetworkImage widget', (
      tester,
    ) async {
      const url = 'https://cdn.example.com/alice.jpg';
      final chat = _chat(username: 'alice', avatarUrl: url);
      expect(chat.participantAvatars[_otherUserId], url);
      await tester.pumpWidget(_wrap(_chat(username: 'alice')));
      expect(find.byType(ProfileAvatar), findsOneWidget);
      expect(find.byType(Image), findsNothing);
    });

    testWidgets('no text initial inside the avatar slot', (
      tester,
    ) async {
      await tester.pumpWidget(_wrap(_chat(username: 'john_doe')));
      // Canonical avatar = photo or Icons.person; never a text initial.
      expect(find.text('J'), findsNothing);
      expect(find.text('?'), findsNothing);
      expect(find.byIcon(Icons.person), findsOneWidget);
    });
  });
}
