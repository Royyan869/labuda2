import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_dto.dart';
import 'package:labuda/domains/chat/chat/data/mappers/chat_mapper.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_card.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

void main() {
  const roomId = '00000000-0000-0000-0000-000000000001';
  const otherUserId = '00000000-0000-0000-0000-000000000002';
  const currentUserId = '00000000-0000-0000-0000-000000000003';

  Map<String, dynamic> roomJson({
    Map<String, dynamic>? lastMessage,
    int? unreadCount,
  }) {
    final json = <String, dynamic>{
      'id': roomId,
      'room_type': 'direct',
      'other_user_id': otherUserId,
      'other_user': {'username': 'alice', 'lifecycle': 'active'},
      'created_at': '2026-06-01T00:00:00.000Z',
      'updated_at': '2026-06-01T00:00:00.000Z',
    };
    if (lastMessage != null) {
      json['last_message'] = lastMessage;
    }
    if (unreadCount != null) {
      json['unread_count'] = unreadCount;
    }
    return json;
  }

  group('room list preview contract', () {
    test('DTO parses last_message backend shape', () {
      final dto = ChatDto.fromJson(
        roomJson(
          lastMessage: {
            'id': 'm1',
            'room_id': roomId,
            'sender_id': otherUserId,
            'message_type': 'text',
            'body': 'hello from backend',
            'created_at': '2026-06-01T01:00:00.000Z',
          },
        ),
      );

      expect(dto.lastMessage, isNotNull);
      expect(dto.lastMessage!.content, 'hello from backend');
      expect(dto.lastMessage!.type, 'text');
      expect(dto.lastMessage!.isHidden, isFalse);
    });

    test('DTO parses unread_count', () {
      final dto = ChatDto.fromJson(roomJson(unreadCount: 7));
      final chat = ChatMapper.toDomain(dto);
      expect(chat.getUnreadCount(currentUserId), 7);
    });

    testWidgets('ChatCard shows tombstone preview for hidden last message', (
      tester,
    ) async {
      final dto = ChatDto.fromJson(
        roomJson(
          lastMessage: {
            'id': 'm-hidden',
            'room_id': roomId,
            'sender_id': otherUserId,
            'message_type': 'text',
            'is_hidden': true,
            'created_at': '2026-06-01T01:00:00.000Z',
          },
        ),
      );
      final chat = ChatMapper.toDomain(dto);

      await tester.pumpWidget(_app(chat, currentUserId));
      expect(find.text('Pesan disembunyikan moderator'), findsOneWidget);
    });

    testWidgets('ChatCard shows normal preview for normal last message', (
      tester,
    ) async {
      final dto = ChatDto.fromJson(
        roomJson(
          lastMessage: {
            'id': 'm-visible',
            'room_id': roomId,
            'sender_id': otherUserId,
            'message_type': 'text',
            'body': 'preview body',
            'created_at': '2026-06-01T01:00:00.000Z',
          },
        ),
      );
      final chat = ChatMapper.toDomain(dto);

      await tester.pumpWidget(_app(chat, currentUserId));
      expect(find.text('@alice'), findsOneWidget);
      expect(find.text('preview body'), findsOneWidget);
    });

    testWidgets('ChatCard unread badge uses backend unread_count', (
      tester,
    ) async {
      final dto = ChatDto.fromJson(
        roomJson(
          unreadCount: 5,
          lastMessage: {
            'id': 'm-visible',
            'room_id': roomId,
            'sender_id': otherUserId,
            'message_type': 'text',
            'body': 'preview body',
            'created_at': '2026-06-01T01:00:00.000Z',
          },
        ),
      );
      final chat = ChatMapper.toDomain(dto);

      await tester.pumpWidget(_app(chat, currentUserId));
      expect(find.text('5'), findsOneWidget);
    });
  });
}

Widget _app(Chat chat, String currentUserId) {
  return ProviderScope(
    overrides: [currentUserIdProvider.overrideWith((ref) => currentUserId)],
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('id'),
      home: Scaffold(
        body: ChatCard(chat: chat, onTap: () {}),
      ),
    ),
  );
}
