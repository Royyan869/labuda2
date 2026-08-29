// Content Mapper
// Konversi antara DTO (Data Layer) dan Entity (Domain Layer)

import 'package:labuda/domains/social/content/data/dto/content_dto.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/content/domain/repositories/content_repository.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Mapper untuk Content Domain Contract Alignment V1
///
/// CONTRACT ALIGNMENT:
/// - Status mapping aligned with backend canonical status (active, deleted)
/// - Author identity (username, avatar) sourced from card.author by the DTO
///   hand-written fromJson factory; the mapper reads the already-extracted values.
class ContentMapper {
  // ==========================================================================
  // DTO → Entity (Response to Domain)
  // ==========================================================================

  /// Map ContentDto to Content entity
  ///
  /// CONTRACT: Backend returns canonical status (active, deleted)
  /// SHARE CONTRACT V1: Maps originalAuthorId and resourceProjection for reposts
  static Content toEntity(ContentDto dto) {
    return Content(
      id: dto.id,
      content: dto.content,
      authorId: dto.authorId,
      authorUsername: dto.authorUsername,
      authorAvatarUrl: dto.authorAvatar,
      authorCity: dto.authorCity,
      authorProvince: dto.authorProvince,
      status: _mapContentStatus(dto.status),
      // D1 — canonical governance lifecycle parsed tolerantly. Backend
      // emits this as a separate top-level field aligned with feed /
      // search. Null / missing / unknown → active (legacy payloads stay
      // backward compatible).
      lifecycle: ContentLifecycleParse.fromWire(dto.lifecycle),
      // E6 — canonical author identity lifecycle parsed from the wire's
      // nested `card.author.lifecycle` slot (extracted by ContentDto's
      // hand-written factory). Null / missing / unknown → active.
      authorLifecycle: ContentLifecycleParse.fromWire(dto.authorLifecycle),
      media: dto.media.map(_mapMediaEntity).toList(),
      tags: dto.tags,
      taggedUsers: dto.taggedUsers,
      mentionedUserIds: [], // Not provided by API, extract from content text
      settings: ContentSettings(
        visibility: _mapVisibility(dto.visibility),
      ),
      // C7C: engagement nullable from DTO — default to zero counts.
      engagement: ContentEngagement(
        viewCount: dto.engagement?.viewCount ?? 0,
        likeCount: dto.engagement?.likeCount ?? 0,
        commentCount: dto.engagement?.commentCount ?? 0,
        shareCount: dto.engagement?.shareCount ?? 0,
        saveCount: dto.engagement?.saveCount ?? 0,
        reportCount: dto.engagement?.reportCount ?? 0,
      ),
      location: dto.location != null
          ? ContentLocation(
              city: dto.location!.city,
              province: dto.location!.province,
              country: dto.location!.country ?? 'Indonesia',
              latitude: dto.location!.latitude,
              longitude: dto.location!.longitude,
              placeName: dto.location!.placeName,
            )
          : null,
      moderationInfo: dto.moderationInfo != null
          ? ContentModerationInfo(
              isApproved: dto.moderationInfo!.isApproved,
              hasBeenModerated: dto.moderationInfo!.hasBeenModerated,
              flagCount: dto.moderationInfo!.flagCount,
              violationCount: 0, // Not provided by API
              detectedViolations: [], // Not provided by API
              moderatorId: dto.moderationInfo!.moderatorId,
              moderatedAt: dto.moderationInfo!.moderatedAt,
              moderationNote: dto.moderationInfo!.moderationNote,
              lastAction: dto.moderationInfo!.lastAction,
            )
          : const ContentModerationInfo(),
      publishedAt: dto.publishedAt,
      scheduledAt: dto.scheduledAt,
      createdAt: dto.createdAt,
      updatedAt: dto.updatedAt,
      // SHARE CONTRACT V1: Map share fields
      originalAuthorId: dto.originalAuthorId,
      resourceProjection: dto.resourceProjection,
    );
  }

  /// Map `List<ContentDto>` to `List<Content>`
  static List<Content> toEntityList(List<ContentDto> dtos) {
    return dtos.map(toEntity).toList();
  }

  // ==========================================================================
  // Entity → DTO (Domain to Request)
  // ==========================================================================

  /// Map Content entity to CreateContentDto
  static CreateContentDto toCreateDto(Content entity) {
    return CreateContentDto(
      content: entity.content,
      status: _mapContentStatusToString(entity.status),
      visibility: _mapVisibilityToString(entity.settings.visibility),
      media: entity.media.isNotEmpty
          ? entity.media.map(_mapMediaToDto).toList()
          : null,
      tags: entity.tags.isNotEmpty ? entity.tags : null,
      mentionedUserIds: entity.taggedUsers.isNotEmpty ? entity.taggedUsers : null,
      location: entity.location != null
          ? ContentLocationDto(
              city: entity.location!.city,
              province: entity.location!.province,
              country: entity.location!.country,
              latitude: entity.location!.latitude,
              longitude: entity.location!.longitude,
              placeName: entity.location!.placeName,
            )
          : null,
      scheduledAt: entity.scheduledAt,
    );
  }

  /// Map Content entity to UpdateContentDto
  static UpdateContentDto toUpdateDto(Content entity) {
    return UpdateContentDto(
      content: entity.content,
      status: _mapContentStatusToString(entity.status),
      visibility: _mapVisibilityToString(entity.settings.visibility),
      media: entity.media.isNotEmpty
          ? entity.media.map(_mapMediaToDto).toList()
          : null,
      tags: entity.tags.isNotEmpty ? entity.tags : null,
      taggedUsers: entity.taggedUsers.isNotEmpty ? entity.taggedUsers : null,
      location: entity.location != null
          ? ContentLocationDto(
              city: entity.location!.city,
              province: entity.location!.province,
              country: entity.location!.country,
              latitude: entity.location!.latitude,
              longitude: entity.location!.longitude,
              placeName: entity.location!.placeName,
            )
          : null,
      scheduledAt: entity.scheduledAt,
    );
  }

  // ==========================================================================
  // Enum Mappers - Canonical Contract
  // ==========================================================================

  /// Map backend status string to ContentStatus enum
  ///
  /// Tolerant of both raw and public-lifecycle vocabulary from the backend:
  /// - "active"               -> ContentStatus.active
  /// - "deleted"              -> ContentStatus.deleted (raw)
  /// - "removed"              -> ContentStatus.deleted (coarsened)
  /// - anything else          -> ContentStatus.active (safe default; matches
  ///                              the tolerant-parse contract used by
  ///                              ContentLifecycleParse.fromWire on the
  ///                              governance lifecycle field above).
  static ContentStatus _mapContentStatus(String status) {
    switch (status.toLowerCase()) {
      case 'active':
        return ContentStatus.active;
      case 'deleted':
      case 'removed':
        return ContentStatus.deleted;
      default:
        return ContentStatus.active;
    }
  }

  /// Map ContentStatus enum to backend status string
  ///
  /// CONTRACT: Only send canonical status values to backend
  static String _mapContentStatusToString(ContentStatus status) {
    switch (status) {
      case ContentStatus.active:
        return 'active';
      case ContentStatus.deleted:
        return 'deleted';
    }
  }

  static ContentVisibility _mapVisibility(String visibility) {
    switch (visibility) {
      case 'public':
        return ContentVisibility.public;
      case 'followers_only':
        return ContentVisibility.followersOnly;
      case 'private':
        return ContentVisibility.private;
      default:
        throw FormatException('Invalid visibility: $visibility');
    }
  }

  static String _mapVisibilityToString(ContentVisibility visibility) {
    switch (visibility) {
      case ContentVisibility.public:
        return 'public';
      case ContentVisibility.followersOnly:
        return 'followers_only';
      case ContentVisibility.private:
        return 'private';
    }
  }

  // ==========================================================================
  // Value Object Mappers
  // ==========================================================================

  static MediaEntity _mapMediaEntity(MediaDto dto) {
    final dimensions = (dto.width != null && dto.height != null)
        ? MediaDimensions(width: dto.width!, height: dto.height!)
        : null;

    final variants = <String, String>{};
    if (dto.thumbnailUrl != null) {
      variants['thumbnail'] = dto.thumbnailUrl!;
    }

    return MediaEntity(
      id: _generateMediaId(dto.url),
      originalUrl: dto.url,
      type: dto.type == 'video' ? MediaType.video : MediaType.image,
      blurhash: dto.blurhash,
      dimensions: dimensions,
      createdAt: DateTime.now(),
      variants: variants,
    );
  }

  static CreateContentMediaRequestDto _mapMediaToDto(MediaEntity entity) {
    return CreateContentMediaRequestDto(
      url: entity.originalUrl,
      type: entity.type == MediaType.video ? 'video' : 'image',
    );
  }

  static String _generateMediaId(String url) {
    final uri = Uri.parse(url);
    final pathSegments = uri.pathSegments;
    return pathSegments.isNotEmpty
        ? pathSegments.last.split('.').first
        : url.hashCode.toString();
  }

  // ==========================================================================
  // Search & Feed Result Mappers
  // ==========================================================================

  /// Map ContentSearchResultDto to domain
  static ContentSearchResult toSearchResult(ContentSearchResultDto dto) {
    return ContentSearchResult(
      contents: toEntityList(dto.contents),
      total: dto.total,
      limit: dto.limit,
      offset: dto.offset,
      query: dto.query,
    );
  }

  // toFeedResult removed (BATCH C2): Content domain no longer provides feed mapping.
  // For social timeline, use Home/Feed domain's FeedItem and FeedMapper instead.
}
