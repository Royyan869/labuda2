// CHAT DAY-PILL — TODAY/YESTERDAY PRESENTATION BOUNDARY GATE.
//
// Bounded slice: the chat detail date separator is the only remaining
// production Today/Yesterday timestamp presentation outside the CLOSED
// Notification family.
//
// Semantic boundary under test:
// - 'Today' / 'Yesterday' are presentation-specific day-pill labels owned by
//   this chat surface. They are NOT a relative-time engine and must NOT flow
//   through TimeFormatService.
// - Older days fall back to the surface's long-standing `d/M/yyyy` calendar
//   label (un-padded by contract; do NOT "fix" it to AppFormatters here).
// - The pill is a centered intrinsic-width Container: no Row flex risk.
//
// Pumps the REAL ChatDetailScreen with messages spanning three day buckets
// and asserts the exact rendered pills plus zero layout exceptions.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:labuda/core/src/auth/app_role.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/chat/chat/presentation/screens/chat_detail_screen.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_notifier.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_state.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/block_state_provider.dart';

const _chatId = '00000000-0000-0000-0000-000000009999';
const _currentUserId = '00000000-0000-0000-0000-000000008888';
const _otherUserId = '00000000-0000-0000-0000-000000007777';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.parse('2026-06-02T00:00:00.000Z');
    final user = AuthUser(
      id: _currentUserId,
      createdAt: now,
      updatedAt: now,
      email: 'me@example.com',
      username: 'me',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: false,
      sellerSubscriptionStatus: 'none',
      hasMarketAuthority: false,
      roles: [UserRole.user],
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

void main() {
  // Message footers render through AppFormatters (intl); the day pills
  // under test do not, but the real screen needs locale data present.
  setUpAll(() async {
    await initializeDateFormatting();
  });

  Chat makeChat() => Chat(
    id: _chatId,
    participantIds: const [_currentUserId, _otherUserId],
    participantNames: const {_currentUserId: 'me', _otherUserId: 'other'},
    participantAvatars: const {},
    participantLifecycles: const {_otherUserId: ContentLifecycle.active},
    createdAt: DateTime.utc(2026, 6, 2),
    status: ChatStatus.active,
  );

  Message makeMessage({
    required String id,
    required String content,
    required DateTime createdAt,
  }) {
    return Message(
      id: id,
      chatId: _chatId,
      senderId: _otherUserId,
      senderName: 'other',
      content: content,
      createdAt: createdAt,
      status: MessageStatus.sent,
      mentionedUserIds: const [],
      deletedBy: const [],
    );
  }

  testWidgets('day pills render Today, Yesterday, and calendar fallback', (
    tester,
  ) async {
    // Duration-bucket fixtures (inDays counts 24h periods, so these buckets
    // are stable regardless of wall-clock time or timezone).
    final now = DateTime.now();
    final todayAt = now.subtract(const Duration(minutes: 30));
    final yesterdayAt = now.subtract(const Duration(hours: 26));
    final oldAt = now.subtract(const Duration(days: 10));
    final olderAt = now.subtract(const Duration(days: 11));
    final oldLabel =
        '${oldAt.day}/${oldAt.month}/${oldAt.year}';
    final olderLabel =
        '${olderAt.day}/${olderAt.month}/${olderAt.year}';

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          isUserBlockedProvider(_otherUserId).overrideWith((ref) => false),
          authControllerProvider.overrideWith(_FakeAuthController.new),
          negotiationNotifierProvider.overrideWith(
            _FakeNegotiationNotifier.new,
          ),
          chatDetailProvider(_chatId).overrideWithValue(
            ChatDetailState(
              chat: makeChat(),
              messages: [
                makeMessage(
                  id: 'msg_today',
                  content: 'today message',
                  createdAt: todayAt,
                ),
                makeMessage(
                  id: 'msg_yesterday',
                  content: 'yesterday message',
                  createdAt: yesterdayAt,
                ),
                makeMessage(
                  id: 'msg_old',
                  content: 'old message',
                  createdAt: oldAt,
                ),
                makeMessage(
                  id: 'msg_older',
                  content: 'older message',
                  createdAt: olderAt,
                ),
              ],
            ),
          ),
        ],
        child: const MaterialApp(home: ChatDetailScreen(chatId: _chatId)),
      ),
    );
    await tester.pump(const Duration(milliseconds: 300));

    expect(
      tester.takeException(),
      isNull,
      reason: 'date-pill layout must not throw',
    );

    // Semantic boundary: presentation-local labels, not the canonical
    // relative engine ('baru saja' must never appear as a day pill).
    expect(find.text('Today'), findsOneWidget);
    expect(find.text('Yesterday'), findsOneWidget);
    expect(find.text(oldLabel), findsOneWidget);
    // The oldest message has no older neighbour, so it renders no pill:
    // headers separate day groups, they do not label every message.
    expect(find.text(olderLabel), findsNothing);
    expect(find.text('baru saja'), findsNothing);

    // All messages still render beneath their pills.
    expect(find.text('today message'), findsOneWidget);
    expect(find.text('yesterday message'), findsOneWidget);
    expect(find.text('old message'), findsOneWidget);
    expect(find.text('older message'), findsOneWidget);
  });
}
