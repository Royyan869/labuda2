import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/social/content/data/dto/content_dto.dart';
import 'package:labuda/domains/social/content/data/mappers/content_mapper.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';

Map<String, dynamic> _contentJson({
  required String visibility,
  String id = 'content-1',
  Map<String, dynamic>? resourceProjection,
}) {
  return <String, dynamic>{
    'id': id,
    'caption': 'hello visibility',
    'author_id': 'author-1',
    'author_username': 'author',
    'author_city': null,
    'author_province': null,
    'status': 'active',
    'lifecycle': 'active',
    'visibility': visibility,
    'media': <Map<String, dynamic>>[],
    'tags': <String>[],
    'location': null,
    'engagement': <String, dynamic>{
      'likeCount': 0,
      'commentCount': 0,
    },
    'moderation_info': <String, dynamic>{
      'isApproved': true,
      'hasBeenModerated': false,
      'flagCount': 0,
      'moderatorId': null,
      'moderatedAt': null,
      'moderationNote': null,
      'lastAction': null,
    },
    'published_at': null,
    'created_at': '2026-07-23T00:00:00.000Z',
    'updated_at': '2026-07-23T00:00:00.000Z',
    'is_liked': null,
    'is_saved': null,
    'original_author_id': null,
    if (resourceProjection != null) 'resource_projection': resourceProjection,
    'card': <String, dynamic>{
      'id': id,
      'author': <String, dynamic>{
        'id': 'author-1',
        'username': 'author',
        'avatar_url': null,
        'lifecycle': 'active',
      },
    },
  };
}

Content _buildEntity(ContentVisibility visibility) {
  return Content(
    id: 'content-1',
    content: 'hello visibility',
    authorId: 'author-1',
    authorUsername: 'author',
    status: ContentStatus.active,
    media: const [],
    tags: const [],
    settings: ContentSettings(visibility: visibility),
    engagement: const ContentEngagement(),
    createdAt: DateTime.utc(2026, 7, 23),
    updatedAt: DateTime.utc(2026, 7, 23),
  );
}

Map<String, dynamic> _fixedPriceSaleProjection({
  required String resourceId,
  required String title,
  required String thumbnailUrl,
}) {
  return <String, dynamic>{
    'state': 'LIVE',
    'resource_type': 'fixed_price_sale',
    'resource_id': resourceId,
    'fixed_price_sale': <String, dynamic>{
      'title': title,
      'media': <Map<String, dynamic>>[],
      'thumbnail_url': thumbnailUrl,
      'price': 1500000,
      'status': 'active',
      'quantity_available': 3,
      'can_interact': true,
      'seller': <String, dynamic>{
        'user': <String, dynamic>{'id': 'seller-1', 'username': 'seller'},
      },
    },
  };
}

void main() {
  test('ContentVisibility serializes canonical values', () {
    final cases = <({ContentVisibility visibility, String wire})>[
      (visibility: ContentVisibility.public, wire: 'public'),
      (visibility: ContentVisibility.followersOnly, wire: 'followers_only'),
      (visibility: ContentVisibility.private, wire: 'private'),
    ];

    for (final tc in cases) {
      final createJson = CreateContentDto(
        content: 'hello',
        visibility: _visibilityToWire(tc.visibility),
      ).toJson();
      expect(createJson['visibility'], tc.wire);
      expect(createJson.containsKey('share_reference'), isFalse);
      expect(createJson.containsKey('shareReference'), isFalse);
      expect(createJson.containsKey('status'), isFalse);

      final updateJson = UpdateContentDto(
        content: 'hello',
        visibility: _visibilityToWire(tc.visibility),
      ).toJson();
      expect(updateJson['visibility'], tc.wire);
      expect(updateJson.containsKey('share_reference'), isFalse);
      expect(updateJson.containsKey('shareReference'), isFalse);
      expect(updateJson.containsKey('status'), isFalse);

      final dto = ContentDto.fromJson(_contentJson(visibility: tc.wire));
      expect(dto.visibility, tc.wire);
      expect(dto.toJson()['visibility'], tc.wire);
    }
  });

  test('unknown visibility values fail closed instead of becoming public', () {
    expect(
      () => CreateContentDto.fromJson(_contentJson(visibility: 'unknown')),
      throwsFormatException,
    );
    expect(
      () => ContentDto.fromJson(_contentJson(visibility: 'unknown')),
      throwsFormatException,
    );
  });

  test('ContentDto parses resource_projection and mapper preserves it', () {
    final dto = ContentDto.fromJson(
      _contentJson(
        visibility: 'public',
        resourceProjection: _fixedPriceSaleProjection(
          resourceId: 'sale-1',
          title: 'Produk Dijual',
          thumbnailUrl: 'https://example.com/sale.jpg',
        ),
      ),
    );

    expect(dto.resourceProjection, isNotNull);
    expect(
      dto.resourceProjection!.resourceType,
      ContentResourceProjectionType.fixedPriceSale,
    );
    expect(dto.resourceProjection!.resourceId, 'sale-1');
    expect(dto.resourceProjection!.titleText, 'Produk Dijual');

    final entity = ContentMapper.toEntity(dto);
    expect(entity.resourceProjection, isNotNull);
    expect(
      entity.resourceProjection!.resourceType,
      ContentResourceProjectionType.fixedPriceSale,
    );
    expect(entity.resourceProjection!.resourceId, 'sale-1');
  });

  test('malformed resource_projection fails closed in parser', () {
    expect(
      () => ContentDto.fromJson(
        _contentJson(
          visibility: 'public',
          resourceProjection: <String, dynamic>{
            'state': 'LIVE',
            'resource_type': 'fixed_price_sale',
            'resource_id': 'sale-1',
            'fixed_price_sale': <String, dynamic>{
              'title': 'Produk Dijual',
              // Missing required canonical payload fields.
            },
          },
        ),
      ),
      throwsFormatException,
    );
  });

  test('ContentMapper preserves typed visibility for create and update', () {
    final cases = [
      ContentVisibility.public,
      ContentVisibility.followersOnly,
      ContentVisibility.private,
    ];

    for (final visibility in cases) {
      final entity = _buildEntity(visibility);

      final createRequest = ContentMapper.toCreateDto(entity);
      expect(createRequest.visibility, _visibilityToWire(visibility));

      final updateRequest = ContentMapper.toUpdateDto(entity);
      expect(updateRequest.visibility, _visibilityToWire(visibility));
    }
  });
}

String _visibilityToWire(ContentVisibility visibility) {
  switch (visibility) {
    case ContentVisibility.public:
      return 'public';
    case ContentVisibility.followersOnly:
      return 'followers_only';
    case ContentVisibility.private:
      return 'private';
  }
}
