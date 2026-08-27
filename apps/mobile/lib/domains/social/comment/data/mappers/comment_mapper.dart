import 'package:labuda/domains/social/comment/data/dto/comment_dto.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Mapper for Comment DTO <-> Entity conversions
///
/// CONTRACT ALIGNMENT V1:
/// - Maps between API DTO and domain entity
/// - offerId field removed (legacy offer system)
/// - commerce reference uses ShareReference and is NOT a binding offer
/// - Author info is embedded in the response for proper UI rendering
/// - Reply max depth = 1 (parent_id for replies)
///
/// **SOCIAL FIX 1 - SINGLE REFERENCE MODEL:**
/// - Uses ShareReference for unified cross-domain references
class CommentMapper {
  /// Convert API DTO to Domain Entity
  static Comment toEntity(CommentDto dto) {
    return Comment(
      id: dto.id,
      authorId: dto.authorId,
      contentId: dto.contentId,
      authorUsername: dto.authorUsername,
      authorAvatarUrl: dto.authorAvatarUrl,
      body: dto.body,
      type: dto.type,
      reference:
          dto.reference, // Direct assignment - ShareReference is the same shape
      parentId: dto.parentId,
      createdAt: dto.createdAt,
      updatedAt: dto.updatedAt,
      deletedAt: dto.deletedAt,
      // E3.1 — Tolerant parse of the comment-author lifecycle. Null /
      // unknown / missing all fall back to ContentLifecycle.active, so
      // legacy backends (where authorLifecycle is always null) continue
      // to render exactly as today.
      authorLifecycle: ContentLifecycleParse.fromWire(dto.authorLifecycle),
    );
  }

  /// Convert list of API DTOs to Domain Entities
  static List<Comment> toEntityList(List<CommentDto> dtos) {
    return dtos.map((dto) => toEntity(dto)).toList();
  }

  /// Convert Domain Entity to API DTO (for requests that need entity data)
  static CommentDto toDto(Comment entity) {
    return CommentDto(
      id: entity.id,
      contentId: entity.contentId,
      authorId: entity.authorId,
      authorUsername: entity.authorUsername,
      authorAvatarUrl: entity.authorAvatarUrl,
      body: entity.body,
      type: entity.type,
      reference: entity
          .reference, // Direct assignment - ShareReference is the same shape
      parentId: entity.parentId,
      createdAt: entity.createdAt,
      updatedAt: entity.updatedAt,
      deletedAt: entity.deletedAt,
    );
  }

  /// Map generic attachment to DTO (legacy/future compatibility).
  ///
  /// NOTE: Currently NOT used. Commerce references use the specialized endpoint.
  /// This method is deprecated and should not be called.
  @Deprecated('Use CreateCommerceReferenceCommentDto for commerce references')
  static CommentAttachmentDto? mapAttachmentToDto(dynamic attachment) {
    // This is a no-op stub for API compatibility
    return null;
  }
}
