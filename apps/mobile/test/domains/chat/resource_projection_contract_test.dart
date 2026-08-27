import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_dto.dart';
import 'package:labuda/domains/chat/chat/data/mappers/chat_mapper.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_resource_projection.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_card.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

const _roomId = 'room-projection-1';
const _otherUserId = 'user-projection-2';
const _currentUserId = 'user-projection-3';

Map<String, dynamic> _liveProfileProjectionJson() => <String, dynamic>{
  'state': 'LIVE',
  'resource_type': 'profile',
  'resource_id': 'user-resource-1',
  'canonical_url': '/user/user-resource-1',
  'viewer_capabilities': {
    'can_view': true,
    'can_interact': false,
    'blocked_by_tombstone': false,
  },
  'profile': {
    'username': 'alice',
    'avatar_url': null,
    'store_name': 'Toko Alice',
    'is_seller': true,
    'lifecycle': 'active',
  },
};

Map<String, dynamic> _roomJson({required Map<String, dynamic> lastMessage}) {
  return <String, dynamic>{
    'id': _roomId,
    'room_type': 'direct',
    'other_user_id': _otherUserId,
    'other_user': {'username': 'alice', 'lifecycle': 'removed'},
    'created_at': '2026-06-01T00:00:00.000Z',
    'updated_at': '2026-06-01T00:00:00.000Z',
    'last_message': lastMessage,
  };
}

void main() {
  group('resource projection contract', () {
    test('last message preview preserves canonical resource_projection', () {
      final dto = ChatDto.fromJson(
        _roomJson(
          lastMessage: {
            'id': 'msg-room-1',
            'room_id': _roomId,
            'sender_id': _otherUserId,
            'sender_name': 'alice',
            'message_type': 'text',
            'body': 'legacy body should not drive preview',
            'resource_projection': _liveProfileProjectionJson(),
            'created_at': '2026-06-01T01:00:00.000Z',
          },
        ),
      );

      final chat = ChatMapper.toDomain(dto);
      final lastMessage = chat.lastMessage;

      expect(dto.lastMessage, isNotNull);
      expect(dto.lastMessage!.resourceProjection, isNotNull);
      expect(lastMessage, isNotNull);
      expect(lastMessage!.resourceProjection, isNotNull);
      expect(lastMessage.resourceProjection!.compactPreviewText, '@alice');
      expect(
        lastMessage.resourceProjection!.canonicalUrl,
        '/user/user-resource-1',
      );
    });

    testWidgets('ChatCard shows the canonical projection preview', (
      tester,
    ) async {
      final chat = ChatMapper.toDomain(
        ChatDto.fromJson(
          _roomJson(
            lastMessage: {
              'id': 'msg-room-2',
              'room_id': _roomId,
              'sender_id': _otherUserId,
              'sender_name': 'alice',
              'message_type': 'text',
              'body': 'legacy body should not drive preview',
              'resource_projection': _liveProfileProjectionJson(),
              'created_at': '2026-06-01T01:00:00.000Z',
            },
          ),
        ),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            currentUserIdProvider.overrideWith((ref) => _currentUserId),
          ],
          child: MaterialApp(
            home: Scaffold(
              body: ChatCard(chat: chat, onTap: () {}),
            ),
          ),
        ),
      );

      expect(find.text('@alice'), findsOneWidget);
      expect(find.text('legacy body should not drive preview'), findsNothing);
    });

    test('resource_projection parser rejects UNKNOWN_STATE state', () {
      expect(
        () => ChatResourceProjection.fromJson({
          'state': 'UNKNOWN_STATE',
          'resource_type': 'profile',
          'viewer_capabilities': {
            'can_view': true,
            'can_interact': false,
            'blocked_by_tombstone': false,
          },
        }),
        throwsFormatException,
      );
    });
  });
}
