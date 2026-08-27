import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_notifier.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_input_area.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_notifier.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_state.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

class _FakeNegotiationNotifier extends NegotiationNotifier {
  @override
  NegotiationState build() => const NegotiationState();
}

void main() {
  const chatId = '00000000-0000-0000-0000-000000000111';
  const currentUserId = '00000000-0000-0000-0000-000000000222';
  const otherUserId = '00000000-0000-0000-0000-000000000333';

  testWidgets(
    'typing in ChatInputArea does not emit typing-indicator failure logs',
    (tester) async {
      final controller = TextEditingController();
      final logs = <String>[];
      final oldDebugPrint = debugPrint;
      try {
        debugPrint = (String? message, {int? wrapWidth}) {
          if (message != null) logs.add(message);
        };

        await tester.pumpWidget(
          _app(
            chatId: chatId,
            currentUserId: currentUserId,
            otherUserId: otherUserId,
            controller: controller,
          ),
        );

        await tester.enterText(find.byType(TextField), 'hello');
        await tester.pump();

        expect(
          logs.where((m) => m.contains('Failed to send typing indicator')),
          isEmpty,
        );
      } finally {
        debugPrint = oldDebugPrint;
        controller.dispose();
      }
    },
  );

  testWidgets('send-message behavior remains unchanged', (tester) async {
    final controller = TextEditingController();
    String? sent;

    addTearDown(controller.dispose);

    await tester.pumpWidget(
      _app(
        chatId: chatId,
        currentUserId: currentUserId,
        otherUserId: otherUserId,
        controller: controller,
        onSendMessage: (content, {type = MessageType.text}) async {
          sent = content;
        },
      ),
    );

    await tester.enterText(find.byType(TextField), 'message to send');
    await tester.pump();
    await tester.tap(find.byIcon(Icons.send));
    await tester.pump();

    expect(sent, 'message to send');
    expect(controller.text, isEmpty);
  });
}

Widget _app({
  required String chatId,
  required String currentUserId,
  required String otherUserId,
  required TextEditingController controller,
  Future<void> Function(String content, {MessageType type})? onSendMessage,
}) {
  final chat = Chat(
    id: chatId,
    participantIds: [currentUserId, otherUserId],
    participantNames: {currentUserId: 'current', otherUserId: 'other'},
    participantAvatars: const {},
    createdAt: DateTime.utc(2026, 6, 2),
    status: ChatStatus.active,
  );

  return ProviderScope(
    overrides: [
      currentUserIdProvider.overrideWith((ref) => currentUserId),
      chatDetailProvider(chatId).overrideWithValue(ChatDetailState(chat: chat)),
      negotiationNotifierProvider.overrideWith(_FakeNegotiationNotifier.new),
    ],
    child: MaterialApp(
      home: Scaffold(
        body: ChatInputArea(
          chatId: chatId,
          messageController: controller,
          onSendMessage:
              onSendMessage ?? ((content, {type = MessageType.text}) async {}),
          onAttachmentTap: () {},
        ),
      ),
    ),
  );
}
