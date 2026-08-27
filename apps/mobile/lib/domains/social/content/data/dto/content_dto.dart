// Content DTOs
// Data Transfer Objects untuk API communication
//
// CONTRACT ALIGNMENT V1:
// - Removed requestStatus shadow field - use unified status instead
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
class CreateContentMediaRequestDto {
  final String url;
  final String type;

  const CreateContentMediaRequestDto({required this.url, required this.type});

  factory CreateContentMediaRequestDto.fromJson(Map<String, dynamic> json) {
    final url = json['url'];
    final type = json['type'];
    if (url is! String || url.isEmpty) {
      throw const FormatException('create content media requires url');
    }
    if (type is! String || type.isEmpty) {
      throw const FormatException('create content media requires type');
    }
    return CreateContentMediaRequestDto(url: url, type: type);
  }

  Map<String, dynamic> toJson() => <String, dynamic>{'url': url, 'type': type};
}

/// Create content request
@JsonSerializable()
class CreateContentDto {
  @JsonKey(name: 'caption')
  final String content;
  @JsonKey(includeIfNull: false)
  final String? status; // active | deleted (canonical status)
  final String? visibility; // public | followers_only | private
  final List<CreateContentMediaRequestDto>? media;
  final List<String>? tags;
  @JsonKey(name: 'mentioned_user_ids')
  final List<String>? mentionedUserIds;
  final bool? allowComments;
  final ContentLocationDto? location;
  final DateTime? scheduledAt;

  // Request-specific fields (budget for request type)
  final double? budgetMin;
  final double? budgetMax;
  final DateTime? deadline;

  const CreateContentDto({
    required this.content,
    this.status,
    this.visibility,
    this.media,
    this.tags,
    this.mentionedUserIds,
    this.allowComments,
    this.location,
    this.scheduledAt,
    this.budgetMin,
    this.budgetMax,
    this.deadline,
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
  @JsonKey(includeIfNull: false)
  final String? status;
  final String? visibility;
  final List<CreateContentMediaRequestDto>? media;
  final List<String>? tags;
  final List<String>? taggedUsers;
  final bool? allowComments;
  final ContentLocationDto? location;
  final DateTime? scheduledAt;

  // Request-specific fields
  final double? budgetMin;
  final double? budgetMax;
  final DateTime? deadline;

  const UpdateContentDto({
    this.content,
    this.status,
    this.visibility,
    this.media,
    this.tags,
    this.taggedUsers,
    this.allowComments,
    this.location,
    this.scheduledAt,
    this.budgetMin,
    this.budgetMax,
    this.deadline,
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
/// CONTRACT: requestStatus field is deprecated - ignored if present
/// SHARE CONTRACT V1: Includes originalAuthorId and canonical resourceProjection for reposts
/// FIELD ALIGNMENT: Backend uses snake_case JSON keys; mapped via @JsonKey.
/// C7C: engagement always non-null (backend emits; mobile defaults to zero).
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
  @JsonKey(name: 'allow_comments', defaultValue: true)
  final bool allowComments;

  final List<MediaDto> media;
  @JsonKey(defaultValue: <String>[])
  final List<String> tags;
  @JsonKey(name: 'tagged_users', defaultValue: <String>[])
  final List<String> taggedUsers;

  final ContentLocationDto? location;
  // C7C: engagement always present from backend (EngagementResponse).
  // Defensive null-safe: defaults to zero counts if backend ever omits.
  final ContentEngagementDto? engagement;
  @JsonKey(name: 'moderation_info')
  final ContentModerationDto? moderationInfo;

  // Request-specific fields
  @JsonKey(name: 'budget_min')
  final double? budgetMin;
  @JsonKey(name: 'budget_max')
  final double? budgetMax;
  final DateTime? deadline;

  @JsonKey(name: 'published_at')
  final DateTime? publishedAt;
  @JsonKey(name: 'scheduled_at')
  final DateTime? scheduledAt;
  @JsonKey(name: 'created_at')
  final DateTime createdAt;
  @JsonKey(name: 'updated_at')
  final DateTime updatedAt;

  // User-specific flags
  @JsonKey(name: 'is_liked')
  final bool? isLiked;
  @JsonKey(name: 'is_saved')
  final bool? isSaved;

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
    required this.allowComments,
    required this.media,
    required this.tags,
    required this.taggedUsers,
    this.location,
    this.engagement,
    this.moderationInfo,
    this.budgetMin,
    this.budgetMax,
    this.deadline,
    this.publishedAt,
    this.scheduledAt,
    required this.createdAt,
    required this.updatedAt,
    this.isLiked,
    this.isSaved,
    this.originalAuthorId,
    this.resourceProjection,
  });

  /// E6 — Hand-written factory mirroring the E2.1 FeedItemDto pattern. The
  /// generated parser remains authoritative for every existing field; this
  /// wrapper only layers on the lifecycle string extracted from the nested
  /// wire shape (`card.author.lifecycle`).
  factory ContentDto.fromJson(Map<String, dynamic> json) {
    _validateVisibilityWire(json['visibility']);
    final base = _$ContentDtoFromJson(json);
    final extracted = _readContentAuthorLifecycle(json);
    if (extracted == null) return base;
    return ContentDto(
      id: base.id,
      content: base.content,
      authorId: base.authorId,
      authorUsername: base.authorUsername,
      authorAvatar: base.authorAvatar,
      authorCity: base.authorCity,
      authorProvince: base.authorProvince,
      status: base.status,
      lifecycle: base.lifecycle,
      authorLifecycle: extracted,
      visibility: base.visibility,
      allowComments: base.allowComments,
      media: base.media,
      tags: base.tags,
      taggedUsers: base.taggedUsers,
      location: base.location,
      engagement: base.engagement,
      moderationInfo: base.moderationInfo,
      budgetMin: base.budgetMin,
      budgetMax: base.budgetMax,
      deadline: base.deadline,
      publishedAt: base.publishedAt,
      scheduledAt: base.scheduledAt,
      createdAt: base.createdAt,
      updatedAt: base.updatedAt,
      isLiked: base.isLiked,
      isSaved: base.isSaved,
      originalAuthorId: base.originalAuthorId,
      resourceProjection: _readContentResourceProjection(json),
    );
  }
  Map<String, dynamic> toJson() => _$ContentDtoToJson(this);
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
  final card = json['card'];
  if (card is Map<String, dynamic>) {
    final cardAuthor = card['author'];
    if (cardAuthor is Map<String, dynamic>) {
      final lc = cardAuthor['lifecycle'];
      if (lc is String && lc.isNotEmpty) return lc;
    }
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
  final String? country;
  final double? latitude;
  final double? longitude;
  final String? placeName;

  const ContentLocationDto({
    this.city,
    this.province,
    this.country,
    this.latitude,
    this.longitude,
    this.placeName,
  });

  factory ContentLocationDto.fromJson(Map<String, dynamic> json) =>
      _$ContentLocationDtoFromJson(json);
  Map<String, dynamic> toJson() => _$ContentLocationDtoToJson(this);
}

/// Content engagement DTO
@JsonSerializable()
class ContentEngagementDto {
  final int viewCount;
  final int likeCount;
  final int commentCount;
  final int shareCount;
  final int saveCount;
  final int reportCount;

  const ContentEngagementDto({
    required this.viewCount,
    required this.likeCount,
    required this.commentCount,
    required this.shareCount,
    required this.saveCount,
    required this.reportCount,
  });

  factory ContentEngagementDto.fromJson(Map<String, dynamic> json) =>
      _$ContentEngagementDtoFromJson(json);
  Map<String, dynamic> toJson() => _$ContentEngagementDtoToJson(this);
}

/// Content moderation DTO
@JsonSerializable()
class ContentModerationDto {
  final bool isApproved;
  final bool hasBeenModerated;
  final int flagCount;
  final String? moderatorId;
  final DateTime? moderatedAt;
  final String? moderationNote;
  final String? lastAction; // warning | content_removed | user_banned

  const ContentModerationDto({
    required this.isApproved,
    required this.hasBeenModerated,
    required this.flagCount,
    this.moderatorId,
    this.moderatedAt,
    this.moderationNote,
    this.lastAction,
  });

  factory ContentModerationDto.fromJson(Map<String, dynamic> json) =>
      _$ContentModerationDtoFromJson(json);
  Map<String, dynamic> toJson() => _$ContentModerationDtoToJson(this);
}
