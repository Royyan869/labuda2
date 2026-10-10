// Dialog authority — Slice #3: chat confirmation family.
//
// The only chat confirmation is the delete-chat decision in
// `chat_list_screen.dart`. It delegates its composition to the canonical
// `AppDialog.confirm`; the business side effect (removeChat) stays in the
// screen. The former block-user stub notice was purged with the Snackbar
// convergence — a non-functional placeholder that could only toast.
//
// Proof:
//  1. POSITIVE (real screen, widget runtime): confirm performs the side effect
//     exactly once; cancel performs none; the destructive action carries the
//     canonical error tone.
//  2. NEGATIVE: the migrated file carries no raw dialog and no local dialog
//     authority; no `Chat*Dialog`/`Chat*Modal` wrapper was introduced.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart' show AppTheme;
import 'package:hishumi/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_notifier.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:hishumi/domains/chat/chat/presentation/screens/chat_list_screen.dart';
import 'package:hishumi/domains/chat/chat/presentation/widgets/chat_card.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart'
    show currentUserIdProvider;

const _chatListScreenPath =
    'lib/domains/chat/chat/presentation/screens/chat_list_screen.dart';

/// Minimal `ChatList` notifier so the real screen renders without the chat
/// repository. Only the members the screen actually calls are exercised.
class _FakeChatList extends ChatList {
  _FakeChatList(this._chats);

  final List<Chat> _chats;
  final List<String> removed = <String>[];

  @override
  ChatListState build() => ChatListState(chats: _chats);

  @override
  Future<void> loadChats(String userId, {bool isRefresh = false}) async {}

  @override
  void removeChat(String chatId) => removed.add(chatId);
}

Chat _chat() => Chat(
  id: 'c1',
  participantIds: const ['me', 'other'],
  participantNames: const {'me': 'Me', 'other': 'Other User'},
  participantAvatars: const <String, String?>{},
  createdAt: DateTime(2026, 1, 1),
);

Widget _app(_FakeChatList fake) => ProviderScope(
  overrides: [
    currentUserIdProvider.overrideWithValue('me'),
    chatListProvider.overrideWith(() => fake),
  ],
  child: MaterialApp(theme: AppTheme.lightTheme, home: const ChatListScreen()),
);

void main() {
  group('chat confirmations — positive (real screen)', () {
    testWidgets('delete chat: confirm removes and reports', (tester) async {
      final fake = _FakeChatList([_chat()]);
      await tester.pumpWidget(_app(fake));
      await tester.pumpAndSettle();

      await tester.longPress(find.byType(ChatCard));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Delete chat'));
      await tester.pumpAndSettle();

      expect(find.text('Delete chat?'), findsOneWidget);
      expect(find.byType(AlertDialog), findsOneWidget);

      final scheme = Theme.of(
        tester.element(find.byType(AlertDialog)),
      ).colorScheme;
      final confirm = tester.widget<ElevatedButton>(
        find.widgetWithText(ElevatedButton, 'Delete'),
      );
      expect(
        confirm.style?.backgroundColor?.resolve(const <WidgetState>{}),
        scheme.error,
        reason: 'delete chat is a destructive confirmation',
      );

      await tester.tap(find.text('Delete'));
      await tester.pumpAndSettle();

      expect(fake.removed, <String>['c1']);
      expect(find.text('Chat dihapus'), findsOneWidget);
      expect(find.byType(AlertDialog), findsNothing);
      await tester.pump(const Duration(seconds: 5));
    });

    testWidgets('delete chat: cancel removes nothing', (tester) async {
      final fake = _FakeChatList([_chat()]);
      await tester.pumpWidget(_app(fake));
      await tester.pumpAndSettle();

      await tester.longPress(find.byType(ChatCard));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Delete chat'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();

      expect(fake.removed, isEmpty);
      expect(find.byType(AlertDialog), findsNothing);
    });
  });

  group('chat confirmations — negative gate', () {
    test('the migrated screen composes no raw dialog', () {
      final source = File(_chatListScreenPath).readAsStringSync();
      expect(source.contains('AlertDialog('), isFalse);
      expect(source.contains('showDialog('), isFalse);
      expect(source.contains('showDialog<bool>('), isFalse);
      expect(source.contains('AppType'), isFalse);
      expect(source.contains('fontSize:'), isFalse);
      expect(source.contains('Colors.'), isFalse);
      // The confirmations consume the authority directly.
      expect(source.contains('AppDialog.confirm('), isTrue);
      expect(source.contains('AppDialogIntent.destructive'), isTrue);
    });

    test('no chat-local dialog wrapper authority was introduced', () {
      final offenders = <String>[];
      for (final file
          in Directory('lib/domains/chat')
              .listSync(recursive: true)
              .whereType<File>()
              .where((f) => f.path.endsWith('.dart'))) {
        final source = file.readAsStringSync();
        for (final name in <String>[
          'class ChatDialog',
          'class ChatDialogs',
          'class ChatModal',
          'class ChatAlert',
          'class ChatConfirm',
        ]) {
          if (source.contains(name)) offenders.add('${file.path}: $name');
        }
      }
      expect(offenders, isEmpty, reason: offenders.join('\n'));
    });
  });
}
