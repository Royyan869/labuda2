import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/social/content/data/dto/content_dto.dart';

Map<String, dynamic> _contentJson() {
  return <String, dynamic>{
    'id': 'content-1',
    'caption': 'mixed media content',
    'author_id': 'author-1',
    'author_username': 'alice',
    'status': 'active',
    'lifecycle': 'active',
    'visibility': 'public',
    'allow_comments': true,
    'media': <Map<String, dynamic>>[
      <String, dynamic>{
        'url': 'https://cdn.example.com/content/image-a.jpg',
        'type': 'image',
        'thumbnailUrl': 'https://cdn.example.com/content/image-a-thumb.jpg',
        'blurhash': 'L5H2EC=PM+yV0g-mq.wG9c010J}I',
        'width': 1200,
        'height': 800,
      },
      <String, dynamic>{
        'url': 'https://cdn.example.com/content/video-b.mp4',
        'type': 'video',
        'thumbnailUrl': 'https://cdn.example.com/content/video-b-poster.jpg',
        'blurhash': 'LEHV6nWB2yk8pyo0adR*.7kCMdnj',
        'width': 1920,
        'height': 1080,
        'duration': 42,
      },
    ],
    'tags': <String>['koi', 'mixed'],
    'tagged_users': <String>[],
    'location': null,
    'engagement': null,
    'moderation_info': null,
    'published_at': null,
    'created_at': '2026-07-28T00:00:00.000Z',
    'updated_at': '2026-07-28T00:00:00.000Z',
    'original_author_id': null,
  };
}

void main() {
  test('ContentDto parses and serializes without top-level type authority', () {
    final dto = ContentDto.fromJson(_contentJson());
    final wire = jsonDecode(jsonEncode(dto.toJson())) as Map<String, dynamic>;

    expect(dto.content, 'mixed media content');
    expect(dto.media, hasLength(2));
    expect(dto.media[0].type, 'image');
    expect(dto.media[1].type, 'video');
    expect(wire.containsKey('type'), isFalse);
    final media = wire['media'] as List<dynamic>;
    expect(media.first, isA<Map<String, dynamic>>());
    expect((media.first as Map<String, dynamic>)['type'], 'image');

    final dtoSource = File(
      'lib/domains/social/content/data/dto/content_dto.dart',
    ).readAsStringSync();
    final mapperSource = File(
      'lib/domains/social/content/data/mappers/content_mapper.dart',
    ).readAsStringSync();

    expect(dtoSource, isNot(contains('ContentType')));
    expect(mapperSource, isNot(contains('ContentType')));
  });

  test(
    'ContentDto round-trips through JSON without requiring top-level type',
    () {
      final dto = ContentDto.fromJson(_contentJson());
      final encoded = jsonEncode(dto.toJson());
      final decoded = jsonDecode(encoded) as Map<String, dynamic>;

      expect(decoded.containsKey('type'), isFalse);
      expect(decoded['caption'], 'mixed media content');
      expect(decoded['visibility'], 'public');
      expect(decoded['media'], hasLength(2));
    },
  );
}
