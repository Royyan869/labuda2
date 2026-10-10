import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/social/content/data/mappers/content_mapper.dart';
import 'package:hishumi/domains/social/content/domain/entities/content.dart';

Map<String, dynamic> _wireJson(dynamic dto) {
  return jsonDecode(jsonEncode(dto.toJson())) as Map<String, dynamic>;
}

Content _contentWithMedia({String? blurhash}) {
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
        blurhash: blurhash,
        createdAt: DateTime.utc(2026, 8, 11),
      ),
    ],
    tags: const ['koi'],
    settings: const ContentSettings(
      visibility: ContentVisibility.public,
    ),
    engagement: const ContentEngagement(),
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
      expect(wire.containsKey('allow_comments'), isFalse);
      expect(wire.containsKey('type'), isFalse);
      expect(wire.containsKey('resource_occurrence'), isFalse);

      final media = wire['media'] as List<dynamic>;
      expect(media, hasLength(1));
      // The CODEBASE owns this shape, not this test: `_mapMediaToDto` forwards
      // url + type + blurhash, and the generated `toJson` always writes the
      // blurhash KEY — null when the picker produced none. This expectation
      // used to pin the pair `['url', 'type']` and failed the day the mapper
      // started carrying blurhash through; the test follows the wire now, and
      // the case below pins what the codebase actually delivers.
      final item = media.first as Map<String, dynamic>;
      expect(item.keys, unorderedEquals(['url', 'type', 'blurhash']));
      expect(item['type'], 'image');
      expect(item['blurhash'], isNull);
    },
  );

  test(
    'ContentMapper.toCreateDto carries a media blurhash through to the wire',
    () {
      final request = ContentMapper.toCreateDto(
        _contentWithMedia(blurhash: 'L6PZfSi_,ayE'),
      );
      final wire = _wireJson(request);
      final media = wire['media'] as List<dynamic>;

      expect(
        (media.first as Map<String, dynamic>)['blurhash'],
        'L6PZfSi_,ayE',
      );
    },
  );
}
