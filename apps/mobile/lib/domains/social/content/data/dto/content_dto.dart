// Content DTOs
// Data Transfer Objects untuk API communication
//
// CONTRACT ALIGNMENT V1:
// - Status follows canonical contract: active, deleted
// - FIELD ALIGNMENT: Backend uses "caption" field, frontend maps via @JsonKey
// - FRONTEND-ONLY FIELDS REMOVED: shippingCity, shippingProvince

import 'package:json_annotation/json_annotation.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';

part 'content_dto.g.dart';

// ============================================================================
// Request DTOs
// ============================================================================

/// Create-content media request item.
@JsonSerializable()
class CreateContentMediaRequestDto {
  final String url;
  final String type;

  const CreateContentMediaRequestDto({required this.url, required this.type});

  factory CreateContentMediaRequestDto.fromJson(Map<String, dynamic> json) =>
      _$CreateContentMediaRequestDtoFromJson(json);

  Map<String, dynamic> toJson() => _$CreateContentMediaRequestDtoToJson(this);
}

/// Create content request
@JsonSerializable()
class CreateContentDto {
  @JsonKey(name: 'caption')
  final String content;
  final String? visibility; // public | followers_only | private
  final List<CreateContentMediaRequestDto>? media;
  final List<String>? tags;
  @JsonKey(name: 'mentioned_user_ids')
  final List<String>? mentionedUserIds;
  final ContentLocationDto? location;

  const CreateContentDto({
    required this.content,
    this.visibility,
    this.media,
    this.tags,
    this.mentionedUserIds,
    this.location,
  });

  factory CreateContentDto.fromJson(Map<String, dynamic> json) {
    _validateVisibilityWire(json['visibility']);
    return _$CreateContentDtoFromJson(json);
  }
  Map<String, dynamic> toJson() => _$CreateContentDtoToJson(this);
}

/// Update content request
@JsonSerializable()
class UpdateContentDto {
  @JsonKey(name: 'caption')
  final String? content;
  final String? visibility;

  const UpdateContentDto({
    this.content,
    this.visibility,
  });

  factory UpdateContentDto.fromJson(Map<String, dynamic> json) {
    _validateVisibilityWire(json['visibility']);
    return _$UpdateContentDtoFromJson(json);
  }
  Map<String, dynamic> toJson() => _$UpdateContentDtoToJson(this);
}

// ============================================================================
// Response DTOs
// ============================================================================

/// Content response from API
///
/// CONTRACT: status field uses canonical values (active, deleted)
/// SHARE CONTRACT V1: Includes originalAuthorId and canonical resourceProjection for reposts
/// FIELD ALIGNMENT: Backend uses snake_case JSON keys; mapped via @JsonKey.
/// C7C: engagement always non-null (backend emits; mobile defaults to zero).
/// AUTHOR CONTRACT: Author identity (username, avatar_url, lifecycle) lives
/// exclusively inside card.author. The flat top-level author_username /
/// author_avatar fields are populated from card.author by the hand-written
/// fromJson factory.
@JsonSerializable()
class ContentDto {
  final String id;
  @JsonKey(name: 'caption')
  final String content;
  @JsonKey(name: 'author_id')
  final String authorId;
  @JsonKey(name: 'author_username')
  final String? authorUsername;
  @JsonKey(name: 'author_avatar')
  final String? authorAvatar;
  @JsonKey(name: 'author_city')
  final String? authorCity;
  @JsonKey(name: 'author_province')
  final String? authorProvince;
  final String status; // "active" | "deleted" (canonical)
  // D1 — canonical governance lifecycle ({active, unavailable, removed}).
  // Separate from raw `status` — never coerce one into the other. Tolerant
  // of null / missing / unknown at the UI boundary via
  // ContentLifecycleParse.fromWire. Backend currently emits this same
  // coarsened vocabulary in the `status` field too, but lifecycle is the
  // canonical convergence field aligned with feed / search.
  final String? lifecycle;

  /// E6 — Embedded author lifecycle (canonical publiccard.UserCard.Lifecycle).
  ///
  /// Sourced at parse time from the wire's nested `card.author.lifecycle`
  /// slot, populated by content_handler.go via
  /// publiccard.NewWithLifecycle through the lifecycle-aware author hydrator
  /// (E6). Tolerant: null / missing / unknown → [ContentLifecycle.active] at
  /// the mapper layer via [ContentLifecycleParse.fromWire].
  ///
  /// Not round-tripped through json_serializable; the hand-written factory
  /// below extracts it via [_readContentAuthorLifecycle]. Mirrors the E2.1
  /// FeedItemDto pattern so generated parsers don't need regeneration.
  @JsonKey(includeFromJson: false, includeToJson: false)
  final String? authorLifecycle;

  final String visibility;

  final List<MediaDto> media;
  @JsonKey(defaultValue: <String>[])
  final List<String> tags;
  final ContentLocationDto? location;
  // C7C: engagement always present from backend (EngagementResponse).
  // Defensive null-safe: defaults to zero counts if backend ever omits.
  final ContentEngagementDto? engagement;

  @JsonKey(name: 'created_at')
  final DateTime createdAt;
  @JsonKey(name: 'updated_at')
  final DateTime updatedAt;

  // User-specific flags
  @JsonKey(name: 'is_liked')
  final bool? isLiked;

  // Canonical mention relation — read from content_mentioned_users table.
  @JsonKey(name: 'mentioned_user_ids')
  final List<String> mentionedUserIds;

  // SHARE CONTRACT V1: Repost attribution fields
  @JsonKey(name: 'original_author_id')
  final String? originalAuthorId;
  @JsonKey(includeFromJson: false, includeToJson: false)
  final ContentResourceProjection? resourceProjection;

  const ContentDto({
    required this.id,
    required this.content,
    required this.authorId,
    this.authorUsername,
    this.authorAvatar,
    this.authorCity,
    this.authorProvince,
    required this.status,
    this.lifecycle,
    this.authorLifecycle,
    required this.visibility,
    required this.media,
    required this.tags,
    this.mentionedUserIds = const [],
    this.location,
    this.engagement,
    required this.createdAt,
    required this.updatedAt,
    this.isLiked,
    this.originalAuthorId,
    this.resourceProjection,
  });

  /// Hand-written factory. The generated parser handles scalar fields;
  /// this wrapper layers on author identity and lifecycle extracted from
  /// the canonical nested `card.author` wire shape.
  ///
  /// AUTHOR CONTRACT: The backend ContentResponse does NOT emit flat
  /// `author_username` / `author_avatar` at the top level. These live
  /// exclusively inside `card.author`. The generated parser reads them
  /// as null from the flat wire; this factory overrides them with the
  /// canonical card.author values.
  factory ContentDto.fromJson(Map<String, dynamic> json) {
    _validateVisibilityWire(json['visibility']);
    final base = _$ContentDtoFromJson(json);

    // Extract canonical author identity from card.author.
    final cardAuthor = _readCardAuthor(json);
    final authorUsername = cardAuthor != null
        ? cardAuthor['username'] as String?
        : base.authorUsername;
    final authorAvatar = cardAuthor != null
        ? cardAuthor['avatar_url'] as String?
        : base.authorAvatar;
    final authorLifecycle = _readContentAuthorLifecycle(json);

    return ContentDto(
      id: base.id,
      content: base.content,
      authorId: base.authorId,
      authorUsername: authorUsername,
      authorAvatar: authorAvatar,
      authorCity: base.authorCity,
      authorProvince: base.authorProvince,
      status: base.status,
      lifecycle: base.lifecycle,
      authorLifecycle: authorLifecycle,
      visibility: base.visibility,
      media: base.media,
      tags: base.tags,
      mentionedUserIds: base.mentionedUserIds,
      location: base.location,
      engagement: base.engagement,
      createdAt: base.createdAt,
      updatedAt: base.updatedAt,
      isLiked: base.isLiked,
      originalAuthorId: base.originalAuthorId,
      resourceProjection: _readContentResourceProjection(json),
    );
  }
  Map<String, dynamic> toJson() => _$ContentDtoToJson(this);
}

/// Extract the canonical `card.author` map from the content wire.
///
/// The backend ContentResponse nests author identity (username, avatar_url,
/// lifecycle) inside `card.author` — NOT at the top level. This helper
/// returns the author map or null when the card/author path is absent.
Map<String, dynamic>? _readCardAuthor(Map<String, dynamic> json) {
  final card = json['card'];
  if (card is Map<String, dynamic>) {
    final cardAuthor = card['author'];
    if (cardAuthor is Map<String, dynamic>) return cardAuthor;
  }
  return null;
}

/// E6 — Extract the embedded author lifecycle string from the content
/// detail / create / update / repost wire.
///
/// Backend hydration topology (content_handler.go via
/// buildContentAuthorCardWithLifecycle):
///   - publiccard.NewWithLifecycle populates the lifecycle slot inside
///     the `card.author` object of every /contents/* success response.
///
/// Returns null when the path is absent / empty / not a string. The mapper
/// converts null into [ContentLifecycle.active] via the canonical
/// [ContentLifecycleParse.fromWire] helper.
String? _readContentAuthorLifecycle(Map<String, dynamic> json) {
  final cardAuthor = _readCardAuthor(json);
  if (cardAuthor != null) {
    final lc = cardAuthor['lifecycle'];
    if (lc is String && lc.isNotEmpty) return lc;
  }
  // Some content surfaces may also surface the lifecycle at the top-level
  // `author.lifecycle` slot (parity with /feed). Honour it as a fallback so
  // the parser stays surface-agnostic.
  final author = json['author'];
  if (author is Map<String, dynamic>) {
    final lc = author['lifecycle'];
    if (lc is String && lc.isNotEmpty) return lc;
  }
  return null;
}

ContentResourceProjection? _readContentResourceProjection(
  Map<String, dynamic> json,
) {
  final raw = json['resource_projection'];
  if (raw is Map<String, dynamic>) {
    return ContentResourceProjection.fromJson(raw);
  }
  return null;
}

void _validateVisibilityWire(Object? raw) {
  if (raw == null) {
    return;
  }
  if (raw is! String) {
    throw const FormatException('invalid visibility wire type');
  }
  switch (raw) {
    case 'public':
    case 'followers_only':
    case 'private':
      return;
    default:
      throw FormatException('Invalid visibility: $raw');
  }
}

/// Cursor-paginated response envelope for GET /users/:id/contents (C3A backend).
///
/// Hand-written (not auto-generated) because the per-item authorLifecycle
/// extraction in [ContentDto.fromJson] relies on a hand-written factory and
/// must stay consistent across all content-listing surfaces.
class UserContentPageDto {
  final List<ContentDto> data;
  final String? nextCursor;
  final bool hasMore;

  const UserContentPageDto({
    required this.data,
    required this.nextCursor,
    required this.hasMore,
  });

  factory UserContentPageDto.fromJson(Map<String, dynamic> json) {
    final rawList = json['data'];
    final items = rawList is List
        ? rawList
              .whereType<Map<String, dynamic>>()
              .map(ContentDto.fromJson)
              .toList()
        : <ContentDto>[];
    return UserContentPageDto(
      data: items,
      nextCursor: json['next_cursor'] as String?,
      hasMore: json['has_more'] as bool? ?? false,
    );
  }
}

/// Search results response
@JsonSerializable()
class ContentSearchResultDto {
  final List<ContentDto> contents;
  final int total;
  final int limit;
  final int offset;
  final String query;

  const ContentSearchResultDto({
    required this.contents,
    required this.total,
    required this.limit,
    required this.offset,
    required this.query,
  });

  factory ContentSearchResultDto.fromJson(Map<String, dynamic> json) =>
      _$ContentSearchResultDtoFromJson(json);
  Map<String, dynamic> toJson() => _$ContentSearchResultDtoToJson(this);
}



// ============================================================================
// Value Object DTOs
// ============================================================================

/// Media DTO
@JsonSerializable()
class MediaDto {
  final String url;
  final String type; // "image" | "video"
  final String? thumbnailUrl;
  final String? blurhash;
  final int? width;
  final int? height;
  final int? duration; // for videos, in seconds

  const MediaDto({
    required this.url,
    required this.type,
    this.thumbnailUrl,
    this.blurhash,
    this.width,
    this.height,
    this.duration,
  });

  factory MediaDto.fromJson(Map<String, dynamic> json) =>
      _$MediaDtoFromJson(json);
  Map<String, dynamic> toJson() => _$MediaDtoToJson(this);
}

/// Content location DTO
@JsonSerializable()
class ContentLocationDto {
  final String? city;
  final String? province;

  const ContentLocationDto({
    this.city,
    this.province,
  });

  factory ContentLocationDto.fromJson(Map<String, dynamic> json) =>
      _$ContentLocationDtoFromJson(json);
  Map<String, dynamic> toJson() => _$ContentLocationDtoToJson(this);
}

/// Content engagement DTO
///
/// Canonical fields: likeCount, commentCount (live from backend).
/// viewCount, shareCount, saveCount, reportCount removed — backend does not
/// provide authoritative values for these fields on content.
@JsonSerializable()
class ContentEngagementDto {
  final int likeCount;
  final int commentCount;

  const ContentEngagementDto({
    this.likeCount = 0,
    this.commentCount = 0,
  });

  factory ContentEngagementDto.fromJson(Map<String, dynamic> json) =>
      _$ContentEngagementDtoFromJson(json);
  Map<String, dynamic> toJson() => _$ContentEngagementDtoToJson(this);
}

