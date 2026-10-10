import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/chat/chat/data/dto/message_dto.dart';
import 'package:hishumi/domains/chat/chat/data/mappers/chat_mapper.dart';
import 'package:hishumi/domains/chat/chat/domain/entities/chat_entities.dart';

/// NEGATIVE + POSITIVE CONTRACT — chat media is a real asset, never a link.
///
/// The killed design embedded the object URL in the message text
/// ("📷 Foto: https://…"), so media arrived as a hyperlink and the renderer's
/// media branches could never fire. The canonical pipeline is
/// register → upload → attach:
///
///   1. POST /chat/rooms/:room_id/media (PENDING, room-scoped asset)
///   2. PUT the bytes to the presigned URL
///   3. send the message with media_asset_ids — attached atomically server-side
///
/// Media is therefore ORTHOGONAL to message_type ("text + media_urls" on the
/// wire), and the bubble type is DERIVED from the projected urls on read.
void main() {
  final libRoot = Directory('lib');

  List<File> dartFiles() => libRoot
      .listSync(recursive: true)
      .whereType<File>()
      .where((f) => f.path.endsWith('.dart'))
      .toList();

  String read(String path) => File(path).readAsStringSync();

  String normalize(String path) => path.replaceAll(r'\', '/');

  group('chat media is never a link', () {
    test('the emoji-link hack stays dead', () {
      final offenders = <String>[];
      for (final file in dartFiles()) {
        final source = file.readAsStringSync();
        for (final banned in const ['📷 Foto: ', '🎬 Video: ']) {
          if (source.contains(banned)) {
            offenders.add('${normalize(file.path)}: $banned');
          }
        }
      }
      expect(
        offenders,
        isEmpty,
        reason:
            'media must be attached as an asset, never embedded in the body. '
            'Offenders: $offenders',
      );
    });

    test('the per-URL sender is gone', () {
      final screen = read(
        'lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart',
      );
      expect(screen.contains('_sendMediaMessage'), isFalse);
    });

    test('the send contract carries asset ids, not urls', () {
      final dto = read('lib/domains/chat/chat/data/dto/message_dto.dart');
      // The send DTO exposes media as ASSET ids…
      expect(dto.contains('mediaAssetIds'), isTrue);
      // …and never re-introduces a raw url field on the SEND side (the read
      // side legitimately parses media_urls from the projection).
      final sendDtoStart = dto.indexOf('class SendMessageDto');
      expect(sendDtoStart, greaterThan(0));
      final sendDto = dto.substring(sendDtoStart);
      expect(
        sendDto.contains("'media_urls'"),
        isFalse,
        reason: 'the backend has no media_urls field on send',
      );
    });

    test('every layer of the send path threads the asset ids', () {
      final files = <String, String>{
        'repository contract':
            'lib/domains/chat/chat/domain/repositories/chat_repository.dart',
        'use case':
            'lib/domains/chat/chat/domain/usecases/send_message_usecase.dart',
        'notifier':
            'lib/domains/chat/chat/presentation/providers/chat_notifier.dart',
        'screen':
            'lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart',
      };
      for (final entry in files.entries) {
        expect(
          read(entry.value).contains('mediaAssetIds'),
          isTrue,
          reason: '${entry.key} must carry mediaAssetIds',
        );
      }
    });

    test('uploads go through the room-scoped register step', () {
      final s3 = read('lib/core/services/s3_service.dart');
      expect(s3.contains('uploadChatMedia'), isTrue);
      expect(
        s3.contains(r'/chat/rooms/$roomId/media'),
        isTrue,
        reason: 'chat media is registered under its room, not a flat folder',
      );
    });
  });

  group('the bubble type is derived from the projected media', () {
    MessageDto dto({List<String>? mediaUrls, String content = ''}) => MessageDto(
      id: 'm1',
      chatRoomId: 'r1',
      senderId: 'u1',
      senderName: 'Budi',
      content: content,
      // The backend stores a media message as text: image|video is derived.
      type: 'text',
      mediaUrls: mediaUrls,
      status: 'sent',
      isRead: false,
      isEdited: false,
      createdAt: DateTime.utc(2026, 9, 29),
      updatedAt: DateTime.utc(2026, 9, 29),
    );

    test('one foto projects an image bubble', () {
      final message = ChatMapper.messageToDomain(
        dto(mediaUrls: const ['https://cdn.labuda.app/images/chat/r/1_u.jpg']),
      );
      expect(message.type, MessageType.image);
      expect(message.mediaUrls, hasLength(1));
    });

    test('one video projects a video bubble', () {
      final message = ChatMapper.messageToDomain(
        dto(mediaUrls: const ['https://cdn.labuda.app/videos/chat/r/1_u.mp4']),
      );
      expect(message.type, MessageType.video);
    });

    test('a caption rides along with the media', () {
      final message = ChatMapper.messageToDomain(
        dto(
          mediaUrls: const ['https://cdn.labuda.app/images/chat/r/1_u.jpg'],
          content: 'ini koi-nya',
        ),
      );
      expect(message.type, MessageType.image);
      expect(message.content, 'ini koi-nya');
    });

    test('a text message without media stays text', () {
      final message = ChatMapper.messageToDomain(dto(content: 'halo'));
      expect(message.type, MessageType.text);
      expect(message.mediaUrls, isEmpty);
    });
  });
}
