import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
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

  testWidgets('chat detail renders newest message at the bottom', (
    tester,
  ) async {
    final newest = makeMessage(
      id: 'msg_newest',
      content: 'newest message',
      createdAt: DateTime.utc(2026, 6, 2, 10, 0),
    );
    final older = makeMessage(
      id: 'msg_older',
      content: 'older message',
      createdAt: DateTime.utc(2026, 6, 2, 9, 59),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          typingIndicatorEnabledProvider.overrideWith((ref) => false),
          isUserBlockedProvider(_otherUserId).overrideWith((ref) => false),
          authControllerProvider.overrideWith(_FakeAuthController.new),
          negotiationNotifierProvider.overrideWith(
            _FakeNegotiationNotifier.new,
          ),
          chatDetailProvider(_chatId).overrideWithValue(
            ChatDetailState(chat: makeChat(), messages: [newest, older]),
          ),
        ],
        child: const MaterialApp(home: ChatDetailScreen(chatId: _chatId)),
      ),
    );
    await tester.pump(const Duration(milliseconds: 300));

    final listView = tester.widget<ListView>(find.byType(ListView));
    expect(listView.reverse, isTrue);
    expect(find.text('newest message'), findsOneWidget);
    expect(find.text('older message'), findsOneWidget);

    final newerTop = tester.getTopLeft(find.text('newest message'));
    final olderTop = tester.getTopLeft(find.text('older message'));
    expect(olderTop.dy, lessThan(newerTop.dy));
  });
}
