import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:intl/intl.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart'
    as chat_state;
import 'package:labuda/domains/chat/chat/presentation/screens/chat_detail_screen.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_last_seen_formatter.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_notifier.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_state.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/profile/profile_feature.dart'
    show userDataProvider;
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/block_state_provider.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart'
    show currentUserIdProvider;

const _chatId = '00000000-0000-0000-0000-000000009999';
const _currentUserId = '00000000-0000-0000-0000-000000008888';
const _otherUserId = '00000000-0000-0000-0000-000000007777';
const _thirdUserId = '00000000-0000-0000-0000-000000007776';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.utc(2026, 7, 30, 8);
    final user = AuthUser(
      id: _currentUserId,
      createdAt: now,
      updatedAt: now,
      email: 'me@example.com',
      username: 'me',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: false,
      hasMarketAuthority: false,
      sellerSubscriptionStatus: 'none',
      roles: const [UserRole.user],
      provider: AuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );
    return AuthState.authenticated(user, emailVerified: true);
  }
}

class _FakeNegotiationNotifier extends NegotiationNotifier {
  @override
  NegotiationState build() => const NegotiationState();
}

AuthUser _verifiedPeerUser({required String id, required String username}) {
  final now = DateTime.utc(2026, 7, 30, 8);
  return AuthUser(
    id: id,
    createdAt: now,
    updatedAt: now,
    email: '$username@example.com',
    username: username,
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    hasSellerProfile: false,
    hasMarketAuthority: false,
    sellerSubscriptionStatus: 'none',
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    lifecycle: ContentLifecycle.active,
  );
}

Chat _directChat({
  required String otherUserId,
  required String otherUsername,
  ContentLifecycle lifecycle = ContentLifecycle.active,
  List<String>? participantIds,
  Map<String, ContentLifecycle>? participantLifecycles,
}) {
  return Chat(
    id: _chatId,
    participantIds: participantIds ?? const [_currentUserId, _otherUserId],
    participantNames: {
      _currentUserId: 'me',
      otherUserId: otherUsername,
      if (participantIds != null && participantIds.length > 2)
        _thirdUserId: 'groupmate',
    },
    participantAvatars: const {},
    participantLifecycles:
        participantLifecycles ??
        <String, ContentLifecycle>{otherUserId: lifecycle},
    createdAt: DateTime.utc(2026, 7, 30),
    status: ChatStatus.active,
  );
}

ProviderScope _buildScope({
  required Chat chat,
  required chat_state.PresenceState presence,
  required String currentUserId,
  required String peerUserId,
  required String peerUsername,
  required Map<String, bool> typingUsers,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(_FakeAuthController.new),
      currentUserIdProvider.overrideWithValue(currentUserId),
      typingIndicatorEnabledProvider.overrideWithValue(false),
      isUserBlockedProvider(peerUserId).overrideWith((ref) => false),
      negotiationNotifierProvider.overrideWith(_FakeNegotiationNotifier.new),
      chatDetailProvider(_chatId).overrideWithValue(
        chat_state.ChatDetailState(chat: chat, typingUsers: typingUsers),
      ),
      presenceProvider.overrideWithValue(presence),
      userDataProvider.overrideWith((ref, userId) async {
        return _verifiedPeerUser(id: userId, username: peerUsername);
      }),
    ],
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('en', 'US'),
      home: ChatDetailScreen(chatId: _chatId),
    ),
  );
}

Future<void> _pumpChatDetail(
  WidgetTester tester, {
  required Chat chat,
  required chat_state.PresenceState presence,
  required String peerUserId,
  required String peerUsername,
  required Map<String, bool> typingUsers,
}) async {
  await tester.pumpWidget(
    _buildScope(
      chat: chat,
      presence: presence,
      currentUserId: _currentUserId,
      peerUserId: peerUserId,
      peerUsername: peerUsername,
      typingUsers: typingUsers,
    ),
  );
  await tester.pump(const Duration(milliseconds: 300));
}

void main() {
  setUpAll(() async {
    await initializeDateFormatting('id_ID', null);
    await initializeDateFormatting('en_US', null);
  });

  group('formatChatLastSeen', () {
    test('offline today timestamp renders localized today format', () {
      final locale = 'id_ID';
      final now = DateTime(2026, 7, 30, 12, 0);
      final seen = DateTime(2026, 7, 30, 7, 15);

      final formatted = formatChatLastSeen(
        lastSeen: seen,
        now: now,
        localeName: locale,
      );

      expect(formatted, startsWith('Hari ini, '));
      expect(formatted, contains(DateFormat.jm(locale).format(seen)));
    });

    test('yesterday timestamp renders localized yesterday format', () {
      final locale = 'id_ID';
      final now = DateTime(2026, 7, 30, 12, 0);
      final seen = DateTime(2026, 7, 29, 18, 45);

      final formatted = formatChatLastSeen(
        lastSeen: seen,
        now: now,
        localeName: locale,
      );

      expect(formatted, startsWith('Kemarin, '));
      expect(formatted, contains(DateFormat.jm(locale).format(seen)));
    });

    test('earlier current-year timestamp renders localized date/time', () {
      final locale = 'id_ID';
      final now = DateTime(2026, 7, 30, 12, 0);
      final seen = DateTime(2026, 4, 17, 19, 30);
      final expectedDate = DateFormat.MMMd(locale).format(seen);

      final formatted = formatChatLastSeen(
        lastSeen: seen,
        now: now,
        localeName: locale,
      );

      expect(formatted, startsWith('$expectedDate, '));
      expect(formatted, isNot(contains('2026')));
    });

    test('previous-year timestamp includes the year', () {
      final locale = 'id_ID';
      final now = DateTime(2026, 7, 30, 12, 0);
      final seen = DateTime(2025, 12, 31, 22, 5);
      final expectedDate = DateFormat.yMMMd(locale).format(seen);

      final formatted = formatChatLastSeen(
        lastSeen: seen,
        now: now,
        localeName: locale,
      );

      expect(formatted, startsWith('$expectedDate, '));
      expect(formatted, contains('2025'));
    });

    test('timestamps are converted to device local time', () {
      final locale = 'id_ID';
      final now = DateTime(2026, 7, 30, 12, 0);
      final seenUtc = DateTime.parse('2026-07-30T01:30:00.000Z');
      final localSeen = seenUtc.toLocal();

      final formatted = formatChatLastSeen(
        lastSeen: seenUtc,
        now: now,
        localeName: locale,
      );

      expect(formatted, contains(DateFormat.jm(locale).format(localSeen)));
    });
  });

  group('ChatDetailScreen presence', () {
    testWidgets('typing state renders before Presence state', (tester) async {
      final chat = _directChat(
        otherUserId: _otherUserId,
        otherUsername: 'typing-peer',
      );
      final presence = chat_state.PresenceState(
        onlineUsers: const {_otherUserId: true},
        lastSeen: const {_otherUserId: null},
      );

      await _pumpChatDetail(
        tester,
        chat: chat,
        presence: presence,
        peerUserId: _otherUserId,
        peerUsername: 'typing-peer',
        typingUsers: const {_otherUserId: true},
      );

      expect(find.text('Typing...'), findsOneWidget);
      expect(find.text('Online'), findsNothing);
      expect(find.textContaining('Last seen'), findsNothing);
    });

    testWidgets(
      'online renders when not typing and canonical state is online',
      (tester) async {
        final chat = _directChat(
          otherUserId: _otherUserId,
          otherUsername: 'online-peer',
        );
        final presence = chat_state.PresenceState(
          onlineUsers: const {_otherUserId: true},
          lastSeen: const {_otherUserId: null},
        );

        await _pumpChatDetail(
          tester,
          chat: chat,
          presence: presence,
          peerUserId: _otherUserId,
          peerUsername: 'online-peer',
          typingUsers: const {},
        );

        expect(find.text('Online'), findsOneWidget);
        expect(find.text('Typing...'), findsNothing);
        expect(find.textContaining('Last seen'), findsNothing);
      },
    );

    testWidgets('null lastSeenAt renders no status', (tester) async {
      final chat = _directChat(
        otherUserId: _otherUserId,
        otherUsername: 'offline-peer',
      );
      final presence = chat_state.PresenceState(
        onlineUsers: const {_otherUserId: false},
        lastSeen: const {_otherUserId: null},
      );

      await _pumpChatDetail(
        tester,
        chat: chat,
        presence: presence,
        peerUserId: _otherUserId,
        peerUsername: 'offline-peer',
        typingUsers: const {},
      );

      expect(find.text('Typing...'), findsNothing);
      expect(find.text('Online'), findsNothing);
      expect(find.textContaining('Last seen'), findsNothing);
    });

    testWidgets('hidden/coarsened state renders no status', (tester) async {
      final chat = _directChat(
        otherUserId: _otherUserId,
        otherUsername: 'hidden-peer',
        lifecycle: ContentLifecycle.unavailable,
      );
      final presence = chat_state.PresenceState(
        onlineUsers: const {_otherUserId: false},
        lastSeen: {_otherUserId: DateTime.utc(2026, 7, 29, 18, 0)},
      );

      await _pumpChatDetail(
        tester,
        chat: chat,
        presence: presence,
        peerUserId: _otherUserId,
        peerUsername: 'hidden-peer',
        typingUsers: const {},
      );

      expect(find.text('Typing...'), findsNothing);
      expect(find.text('Online'), findsNothing);
      expect(find.textContaining('Last seen'), findsNothing);
    });

    testWidgets('support chat renders no personal Presence', (tester) async {
      final chat = Chat(
        id: _chatId,
        type: ChatType.support,
        participantIds: const [_currentUserId, _otherUserId],
        participantNames: const {_currentUserId: 'me', _otherUserId: 'support'},
        participantAvatars: const {},
        participantLifecycles: const {_otherUserId: ContentLifecycle.active},
        assignedAdminName: 'Ops Agent',
        createdAt: DateTime.utc(2026, 7, 30),
        status: ChatStatus.active,
      );
      final presence = chat_state.PresenceState(
        onlineUsers: const {_otherUserId: true},
        lastSeen: {_otherUserId: DateTime.utc(2026, 7, 29, 18, 0)},
      );

      await _pumpChatDetail(
        tester,
        chat: chat,
        presence: presence,
        peerUserId: _otherUserId,
        peerUsername: 'support',
        typingUsers: const {},
      );

      expect(find.text('Support'), findsOneWidget);
      expect(find.text('Agent: Ops Agent'), findsOneWidget);
      expect(find.text('Typing...'), findsNothing);
      expect(find.text('Online'), findsNothing);
      expect(find.textContaining('Last seen'), findsNothing);
    });

    testWidgets('group chat renders no personal last seen', (tester) async {
      final peerUserId = _otherUserId;
      final chat = _directChat(
        otherUserId: peerUserId,
        otherUsername: 'group-peer',
        participantIds: const [_currentUserId, _otherUserId, _thirdUserId],
        participantLifecycles: const {
          _otherUserId: ContentLifecycle.active,
          _thirdUserId: ContentLifecycle.active,
        },
      );
      final presence = chat_state.PresenceState(
        onlineUsers: const {_otherUserId: true},
        lastSeen: {_otherUserId: DateTime.utc(2026, 7, 29, 18, 0)},
      );

      await _pumpChatDetail(
        tester,
        chat: chat,
        presence: presence,
        peerUserId: peerUserId,
        peerUsername: 'group-peer',
        typingUsers: const {},
      );

      expect(find.text('Chat'), findsOneWidget);
      expect(find.text('Typing...'), findsNothing);
      expect(find.text('Online'), findsNothing);
      expect(find.textContaining('Last seen'), findsNothing);
    });

    testWidgets(
      'long username + verification badge + status produces no overflow',
      (tester) async {
        final longUsername =
            'very-long-username-for-chat-header-overflow-proof';
        final chat = _directChat(
          otherUserId: _otherUserId,
          otherUsername: longUsername,
        );
        final presence = chat_state.PresenceState(
          onlineUsers: const {_otherUserId: true},
          lastSeen: const {_otherUserId: null},
        );

        await tester.pumpWidget(
          MediaQuery(
            data: const MediaQueryData(size: Size(320, 640)),
            child: _buildScope(
              chat: chat,
              presence: presence,
              currentUserId: _currentUserId,
              peerUserId: _otherUserId,
              peerUsername: longUsername,
              typingUsers: const {},
            ),
          ),
        );
        await tester.pump(const Duration(milliseconds: 300));

        expect(tester.takeException(), isNull);
        expect(find.byIcon(Icons.verified), findsOneWidget);
        expect(find.text('Online'), findsOneWidget);
        expect(find.textContaining(longUsername), findsOneWidget);
      },
    );
  });
}
