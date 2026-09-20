import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_resource_projection.dart';

/// Chat resource-projection INGESTION contract:
/// HTTP `resource_projection` → MessageDto → domain Message.
///
/// The projection is a server-resolved, viewer-aware DISPLAY representation for
/// the resource the message references. Chat never derives it locally.
void main() {
  Map<String, dynamic> liveProfileProjection(String profileId) => {
    'state': 'LIVE',
    'resource_type': 'profile',
    'resource_id': profileId,
    'canonical_url': '/user/$profileId',
    'viewer_capabilities': {
      'can_view': true,
      'can_interact': false,
      'blocked_by_tombstone': false,
    },
    'profile': {
      'username': 'alice',
      'is_seller': false,
      'lifecycle': 'active',
    },
  };

  Map<String, dynamic> messageJson({Map<String, dynamic>? projection}) => {
    'id': 'm1',
    'room_id': 'r1',
    'sender_id': 'u1',
    'message_type': 'text',
    'body': 'sharing a profile',
    'created_at': '2026-01-01T00:00:00Z',
    if (projection != null) 'resource_projection': projection,
  };

  test('MessageDto parses a LIVE resource_projection', () {
    const profileId = '11111111-1111-1111-1111-111111111111';
    final dto = MessageDto.fromJson(
      messageJson(projection: liveProfileProjection(profileId)),
    );

    expect(dto.resourceProjection, isA<ChatLiveResourceProjection>());
    final live = dto.resourceProjection! as ChatLiveResourceProjection;
    expect(live.resourceId, profileId);
    expect(live.resourceType, ChatResourceType.profile);
    expect(live.canonicalUrl, '/user/$profileId');
  });

  test('MessageDto parses a TOMBSTONE resource_projection', () {
    final dto = MessageDto.fromJson(
      messageJson(
        projection: {
          'state': 'TOMBSTONE',
          'resource_type': 'for_sale',
          'viewer_capabilities': {
            'can_view': false,
            'can_interact': false,
            'blocked_by_tombstone': true,
          },
        },
      ),
    );

    expect(dto.resourceProjection, isA<ChatTombstoneResourceProjection>());
    expect(dto.resourceProjection!.isTombstone, isTrue);
  });

  test('absent resource_projection yields null', () {
    final dto = MessageDto.fromJson(messageJson());
    expect(dto.resourceProjection, isNull);
  });

  test('malformed resource_projection is dropped, message still parses', () {
    final dto = MessageDto.fromJson(
      messageJson(
        projection: {
          'state': 'NOT_A_STATE',
          'resource_type': 'profile',
          'viewer_capabilities': {
            'can_view': true,
            'can_interact': false,
            'blocked_by_tombstone': false,
          },
        },
      ),
    );

    expect(dto.resourceProjection, isNull);
    expect(dto.content, 'sharing a profile');
  });

  // The DTO → domain Message carry is a single field passthrough
  // (ChatMapper.messageToDomain assigns `resourceProjection: dto.resourceProjection`),
  // covered by the widget render test which builds a domain Message directly.
}
