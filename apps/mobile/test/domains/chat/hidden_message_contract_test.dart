import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';
import 'package:labuda/domains/chat/chat/data/mappers/chat_mapper.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/message_bubble.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

void main() {
  group('CHAT_DOMAIN_PASS_15 hidden contract', () {
    test('DTO parses is_hidden=true and defaults false when missing', () {
      final hiddenDto = MessageDto.fromJson(_messageJson(isHidden: true));
      final visibleDto = MessageDto.fromJson(_messageJson());

      expect(hiddenDto.isHidden, isTrue);
      expect(visibleDto.isHidden, isFalse);
    });

    test('mapper carries hidden state into domain entity', () {
      final dto = MessageDto.fromJson(_messageJson(isHidden: true));
      final message = ChatMapper.messageToDomain(dto);

      expect(message.isHidden, isTrue);
    });

    testWidgets(
      'hidden message renders tombstone and suppresses body+attachment',
      (tester) async {
        final message = Message(
          id: 'm-hidden',
          chatId: 'c1',
          senderId: 'u1',
          senderName: 'Sender',
          senderUsername: 'sender',
          content: 'ini body yang tidak boleh tampil',
          isHidden: true,
          objectReference: ShareReference.forSale(
            forSaleId: 'listing-1',
            title: 'Produk Rahasia',
          ),
          createdAt: DateTime.parse('2026-06-01T00:00:00.000Z'),
        );

        await tester.pumpWidget(_testApp(message));

        expect(find.text('Pesan disembunyikan moderator'), findsOneWidget);
        expect(find.text('ini body yang tidak boleh tampil'), findsNothing);
        expect(find.text('Produk Rahasia'), findsNothing);
      },
    );

    testWidgets('visible message still renders body and attachment normally', (
      tester,
    ) async {
      final message = Message(
        id: 'm-visible',
        chatId: 'c1',
        senderId: 'u1',
        senderName: 'Sender',
        senderUsername: 'sender',
        content: 'body visible',
        isHidden: false,
        objectReference: ShareReference.forSale(
          forSaleId: 'listing-1',
          title: 'Produk Tampil',
        ),
        createdAt: DateTime.parse('2026-06-01T00:00:00.000Z'),
      );

      await tester.pumpWidget(_testApp(message, showAvatar: true));
      await tester.pumpAndSettle();

      expect(find.text('Pesan disembunyikan moderator'), findsNothing);
      expect(find.text('body visible'), findsOneWidget);
      expect(find.text('Produk Tampil'), findsOneWidget);
    });

    testWidgets(
      'visible message renders @username and no display_name fallback',
      (tester) async {
        final message = Message(
          id: 'm-identity',
          chatId: 'c1',
          senderId: 'u1',
          senderName: 'Display Name',
          senderUsername: 'alice',
          content: 'body identity',
          isHidden: false,
          createdAt: DateTime.parse('2026-06-01T00:00:00.000Z'),
        );

        await tester.pumpWidget(_testApp(message, showAvatar: true));
        await tester.pumpAndSettle();

        expect(find.text('@alice'), findsOneWidget);
        expect(find.text('Display Name'), findsNothing);
      },
    );

    testWidgets('degraded sender still redacts identity', (tester) async {
      final message = Message(
        id: 'm-redacted',
        chatId: 'c1',
        senderId: 'u1',
        senderName: 'Display Name',
        senderUsername: 'alice',
        content: 'body identity',
        isHidden: false,
        senderLifecycle: ContentLifecycle.removed,
        createdAt: DateTime.parse('2026-06-01T00:00:00.000Z'),
      );

      await tester.pumpWidget(_testApp(message, showAvatar: true));
      await tester.pumpAndSettle();

      expect(find.text('@alice'), findsNothing);
      expect(find.text('Pengguna dihapus'), findsOneWidget);
    });
  });
}

Map<String, dynamic> _messageJson({bool? isHidden}) => {
  'id': 'm1',
  'chat_room_id': 'c1',
  'sender_id': 'u1',
  'sender_name': 'Sender',
  'sender': {'username': 'sender'},
  'content': 'hello',
  'type': 'text',
  'status': 'sent',
  'is_read': false,
  'is_edited': false,
  'created_at': '2026-06-01T00:00:00.000Z',
  'updated_at': '2026-06-01T00:00:00.000Z',
  if (isHidden != null) 'is_hidden': isHidden,
};

Widget _testApp(Message message, {bool showAvatar = false}) {
  return ProviderScope(
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('id'),
      home: Scaffold(
        body: MessageBubble(
          message: message,
          isFromUser: false,
          showAvatar: showAvatar,
        ),
      ),
    ),
  );
}
