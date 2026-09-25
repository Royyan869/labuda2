import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/src/auth/app_role.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_input_area.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_notifier.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_state.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

const _chatId = '00000000-0000-0000-0000-00000000aaaa';
const _currentUserId = '00000000-0000-0000-0000-00000000bbbb';
const _otherUserId = '00000000-0000-0000-0000-00000000cccc';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.utc(2026, 8, 1, 8);
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

class _FakeChatDetailNotifier extends ChatDetail {
  @override
  ChatDetailState build(String chatId) {
    return ChatDetailState(
      chat: Chat(
        id: chatId,
        participantIds: const [_currentUserId, _otherUserId],
        participantNames: const {_currentUserId: 'me', _otherUserId: 'other'},
        participantAvatars: const {},
        participantLifecycles: const {_otherUserId: ContentLifecycle.active},
        createdAt: DateTime.utc(2026, 8, 1),
        status: ChatStatus.active,
      ),
    );
  }
}

ProviderScope _wrap(Widget child) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(_FakeAuthController.new),
      currentUserIdProvider.overrideWith((ref) => _currentUserId),
      chatDetailProvider(_chatId).overrideWith(_FakeChatDetailNotifier.new),
      negotiationNotifierProvider.overrideWith(_FakeNegotiationNotifier.new),
      typingIndicatorEnabledProvider.overrideWithValue(false),
    ],
    child: MaterialApp(
      home: Scaffold(
        body: Align(alignment: Alignment.bottomCenter, child: child),
      ),
    ),
  );
}

void main() {
  testWidgets('whitespace-only draft keeps send disabled', (tester) async {
    final controller = TextEditingController();
    var sendCalls = 0;

    await tester.pumpWidget(
      _wrap(
        ChatInputArea(
          chatId: _chatId,
          messageController: controller,
          onSendMessage: (_, {MessageType type = MessageType.text}) async {
            sendCalls += 1;
          },
          onAttachmentTap: () {},
        ),
      ),
    );
    await tester.pump();

    await tester.enterText(find.byType(TextField), '   ');
    await tester.pump();

    // Codebase factual: _isTyping is text.isNotEmpty — whitespace counts as
    // typing and shows the send icon, but _handleSendMessage trims and guards
    // empty content, so no send fires.
    expect(find.byIcon(Icons.send), findsOneWidget);

    await tester.tap(find.byIcon(Icons.send));
    await tester.pump();

    expect(sendCalls, 0);
  });

  testWidgets('empty draft does not send — media-only send path removed', (tester) async {
    final controller = TextEditingController();
    String? capturedContent;
    MessageType? capturedType;

    await tester.pumpWidget(
      _wrap(
        ChatInputArea(
          chatId: _chatId,
          messageController: controller,
          // Codebase factual: sendability is decided inside the widget/notifier
          // chain — the widget carries no canSendMedia flag.
          onSendMessage:
              (content, {MessageType type = MessageType.text}) async {
                capturedContent = content;
                capturedType = type;
              },
          onAttachmentTap: () {},
        ),
      ),
    );
    await tester.pump();

    final sendButton = find.byType(IconButton).last;
    expect(sendButton, findsOneWidget);
    await tester.ensureVisible(sendButton);

    await tester.tap(sendButton);
    await tester.pump();

    // Codebase factual: the send button requires non-empty text (_isTyping);
    // the old media-only empty-body send path no longer exists in the widget.
    expect(capturedContent, isNull);
    expect(capturedType, isNull);
  });
}
