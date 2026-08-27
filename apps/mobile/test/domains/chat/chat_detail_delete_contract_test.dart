import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/chat/chat/presentation/screens/chat_detail_screen.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/block_state_provider.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

void main() {
  const chatId = '00000000-0000-0000-0000-000000001111';
  const currentUserId = '00000000-0000-0000-0000-000000002222';
  const otherUserId = '00000000-0000-0000-0000-000000003333';

  Chat makeChat() => Chat(
    id: chatId,
    participantIds: const [currentUserId, otherUserId],
    participantNames: const {currentUserId: 'me', otherUserId: 'other'},
    participantAvatars: const {},
    participantLifecycles: const {otherUserId: ContentLifecycle.active},
    createdAt: DateTime.utc(2026, 6, 2),
    status: ChatStatus.active,
  );

  Message makeMyMessage() => Message(
    id: 'msg-1',
    chatId: chatId,
    senderId: currentUserId,
    senderName: 'me',
    content: 'hello from me',
    createdAt: DateTime.utc(2026, 6, 2, 10, 30),
  );

  testWidgets('message options do not expose delete action for own message', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          currentUserIdProvider.overrideWith((ref) => currentUserId),
          typingIndicatorEnabledProvider.overrideWith((ref) => false),
          isUserBlockedProvider(otherUserId).overrideWith((ref) => true),
          chatDetailProvider(chatId).overrideWithValue(
            ChatDetailState(chat: makeChat(), messages: [makeMyMessage()]),
          ),
        ],
        child: const MaterialApp(home: ChatDetailScreen(chatId: chatId)),
      ),
    );
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.text('hello from me'), findsOneWidget);

    await tester.longPress(find.text('hello from me'));
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.text('Delete'), findsNothing);
    expect(find.text('Copy'), findsOneWidget);
  });

  testWidgets('chat room header renders @username', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          currentUserIdProvider.overrideWith((ref) => currentUserId),
          typingIndicatorEnabledProvider.overrideWith((ref) => false),
          isUserBlockedProvider(otherUserId).overrideWith((ref) => true),
          chatDetailProvider(chatId).overrideWithValue(
            ChatDetailState(chat: makeChat(), messages: const []),
          ),
        ],
        child: const MaterialApp(home: ChatDetailScreen(chatId: chatId)),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.descendant(of: find.byType(AppBar), matching: find.text('@other')),
      findsOneWidget,
    );
  });
}
