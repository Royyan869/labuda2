import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/social/content/data/dto/content_dto.dart';
import 'package:labuda/domains/social/content/data/dto/content_request_models.dart';
import 'package:labuda/domains/social/content/data/mappers/content_mapper.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_occurrence.dart';

Map<String, dynamic> _wireJson(dynamic dto) {
  return jsonDecode(jsonEncode(dto.toJson())) as Map<String, dynamic>;
}

Content _contentWithMedia() {
  return Content(
    id: 'content-1',
    content: 'publish me',
    authorId: 'author-1',
    authorUsername: 'tester',
    status: ContentStatus.active,
    media: <MediaEntity>[
      MediaEntity(
        id: 'media-1',
        originalUrl: 'https://cdn.example.com/content/image-a.jpg',
        type: MediaType.image,
        createdAt: DateTime.utc(2026, 8, 11),
      ),
    ],
    tags: const ['koi'],
    taggedUsers: const [],
    mentionedUserIds: const [],
    settings: const ContentSettings(
      visibility: ContentVisibility.public,
      allowComments: false,
    ),
    engagement: const ContentEngagement(),
    moderationInfo: const ContentModerationInfo(),
    createdAt: DateTime.utc(2026, 8, 11),
    updatedAt: DateTime.utc(2026, 8, 11),
  );
}

void main() {
  test(
    'ContentMapper.toCreateDto serializes publish payload without top-level type',
    () {
      final request = ContentMapper.toCreateDto(_contentWithMedia());
      final wire = _wireJson(request);

      expect(wire['caption'], 'publish me');
      expect(wire['visibility'], 'public');
      expect(wire['allowComments'], isFalse);
      expect(wire.containsKey('type'), isFalse);
      expect(wire.containsKey('resource_occurrence'), isFalse);

      final media = wire['media'] as List<dynamic>;
      expect(media, hasLength(1));
      expect(
        (media.first as Map<String, dynamic>).keys,
        unorderedEquals(['url', 'type']),
      );
      expect((media.first as Map<String, dynamic>)['type'], 'image');
    },
  );

  test(
    'ContentCreateRequest emits optional resource_occurrence and no top-level type',
    () {
      final request = ContentCreateRequest(
        content: 'request publish',
        visibility: ContentVisibility.followersOnly,
        media: const <CreateContentMediaRequestDto>[
          CreateContentMediaRequestDto(
            url: 'https://cdn.example.com/content/video-b.mp4',
            type: 'video',
          ),
        ],
        tags: const ['wanted', 'koi'],
        resourceOccurrence: ContentResourceOccurrence.shareToFeed(
          resourceType: ContentResourceOccurrenceResourceType.content,
          resourceId: '550e8400-e29b-41d4-a716-446655440000',
        ),
        location: const ContentLocation(
          city: 'Jakarta',
          province: 'DKI Jakarta',
          country: 'Indonesia',
        ),
      );

      final wire = request.toJson();

      expect(wire['caption'], 'request publish');
      expect(wire['visibility'], 'followers_only');
      expect(wire.containsKey('type'), isFalse);
      expect(wire['resource_occurrence'], isNotNull);
      expect(
        (wire['media'] as List<dynamic>).first,
        isA<Map<String, dynamic>>(),
      );
      expect((wire['media'] as List<dynamic>).first['type'], 'video');
    },
  );
}
