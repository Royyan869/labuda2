import 'package:equatable/equatable.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';

/// API DTO for comment response from backend
///
/// CONTRACT ALIGNMENT V1:
/// - Aligned with backend CommentResponse at:
///   backend/internal/social/content/delivery/http/comment_response.go
/// - reference is seller response only, NOT a binding offer
/// - Author info embedded in response for proper UI rendering
/// - Reply max depth = 1 (parent_id for replies)
///
/// SINGLE REFERENCE MODEL:
/// - reference uses ShareReference format (canonical cross-domain reference)
/// - targetType = "for_sale" for seller responses
class CommentDto extends Equatable {
  final String id;
  final String contentId;
  final String authorId;
  // Author info embedded in response
  final String authorUsername;
  final String? authorAvatarUrl;
  final String? body;
  final String type;
  final ShareReference? reference; // Unified reference using ShareReference
  final String? parentId; // Set for replies (max depth = 1)
  final DateTime createdAt;
  final DateTime? updatedAt;
  final DateTime? deletedAt;

  /// E3.1 — Comment author lifecycle wire field, parsed from the nested
  /// canonical CommentAuthorCard at `author.lifecycle` with fallback to
  /// `card.author.lifecycle` for envelope shapes that may layer a
  /// ContentCard wrapper. Always tolerant: null / missing / unknown is
  /// converted to `ContentLifecycle.active` at the mapper layer.
  ///
  /// Backend emits `null` today (publiccard.New leaves Lifecycle reserved-
  /// nil on comments per comment_response.go); this field exists so the
  /// mobile redaction seam is in place before any backend activation. No
  /// build_runner regeneration is needed — CommentDto.fromJson is hand-
  /// written, so the new field is read inline.
  final String? authorLifecycle;

  const CommentDto({
    required this.id,
    required this.contentId,
    required this.authorId,
    required this.authorUsername,
    this.authorAvatarUrl,
    this.body,
    required this.type,
    this.reference,
    this.parentId,
    required this.createdAt,
    this.updatedAt,
    this.deletedAt,
    this.authorLifecycle,
  });

  factory CommentDto.fromJson(Map<String, dynamic> json) {
    return CommentDto(
      id: json['id'] as String,
      // C-IDENT — canonical wire key is `target_id` (backend CommentResponse),
      // mapped to the local contentId field.
      contentId: json['target_id'] as String,
      authorId: json['author_id'] as String,
      authorUsername: json['author_username'] as String? ?? '',
      authorAvatarUrl: json['author_avatar_url'] as String?,
      body: json['body'] as String?,
      type: json['type'] as String,
      reference: json['reference'] != null
          ? ShareReference.fromJson(json['reference'] as Map<String, dynamic>)
          : null,
      parentId: json['parent_id'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: json['updated_at'] != null
          ? DateTime.parse(json['updated_at'] as String)
          : null,
      deletedAt: json['deleted_at'] != null
          ? DateTime.parse(json['deleted_at'] as String)
          : null,
      authorLifecycle: _readAuthorLifecycle(json),
    );
  }

  /// Check if this is a commerce reference comment
  bool get isCommerceReference => type == 'commerce_reference';

  /// Check if this is a normal comment
  bool get isNormal => type == 'normal';

  /// Check if this is a reply (has parent)
  bool get isReply => parentId != null;

  /// Check if this is a top-level comment (can be replied to)
  bool get isTopLevel => parentId == null;

  @override
  List<Object?> get props => [id, contentId, createdAt];
}

/// Extract the comment-author lifecycle from the wire envelope.
///
/// Preference order:
///   1. `author.lifecycle`      — populated by publiccard.NewWithLifecycle
///                               on the comment-author card.
///   2. `card.author.lifecycle` — fallback for envelope shapes that may
///                               wrap a ContentCard-style author block.
///
/// Returns null when both paths are absent. The mapper converts null
/// into `ContentLifecycle.active` via ContentLifecycleParse.fromWire.
String? _readAuthorLifecycle(Map<String, dynamic> json) {
  final author = json['author'];
  if (author is Map<String, dynamic>) {
    final lc = author['lifecycle'];
    if (lc is String && lc.isNotEmpty) return lc;
  }
  final card = json['card'];
  if (card is Map<String, dynamic>) {
    final cardAuthor = card['author'];
    if (cardAuthor is Map<String, dynamic>) {
      final lc = cardAuthor['lifecycle'];
      if (lc is String && lc.isNotEmpty) return lc;
    }
  }
  return null;
}

/// Resource reference request for creating commerce reference comments.
class ResourceReferenceRequest {
  final String resourceType;
  final String resourceId;
  final Map<String, dynamic>? preview;

  const ResourceReferenceRequest({
    required this.resourceType,
    required this.resourceId,
    this.preview,
  });

  Map<String, dynamic> toJson() => {
    'resource_type': resourceType,
    'resource_id': resourceId,
    if (preview != null) 'preview': preview,
  };
}

/// Request DTO to create a commerce reference comment.
class CreateCommerceReferenceCommentDto {
  final ResourceReferenceRequest resourceReference;
  final String? body;

  const CreateCommerceReferenceCommentDto({
    required this.resourceReference,
    this.body,
  });

  Map<String, dynamic> toJson() => {
    'resource_reference': resourceReference.toJson(),
    if (body != null) 'body': body,
  };
}

/// Request DTO to create a normal comment
///
/// CANONICAL COMMENT CAPABILITIES V1:
/// - text: YES (body field)
/// - mention user: supported via mentionedUserIds
/// - commerce reference: use specialized CreateCommerceReferenceCommentDto
class CreateCommentDto {
  final String targetId;
  final String targetType;
  final String content;
  final String? parentId;
  final List<String>? mentionedUserIds;

  const CreateCommentDto({
    required this.targetId,
    required this.targetType,
    required this.content,
    this.parentId,
    this.mentionedUserIds,
  });

  Map<String, dynamic> toJson() => {
    'target_id': targetId,
    'target_type': targetType,
    'body': content,
    if (parentId != null) 'parent_id': parentId,
    if (mentionedUserIds != null) 'mentioned_user_ids': mentionedUserIds,
  };
}

/// Response DTO for paginated comments list
class ListCommentsDto {
  final List<CommentDto> comments;
  final int limit;
  final String? nextCursor;

  const ListCommentsDto({
    required this.comments,
    required this.limit,
    this.nextCursor,
  });

  factory ListCommentsDto.fromJson(Map<String, dynamic> json) {
    return ListCommentsDto(
      comments:
          (json['comments'] as List<dynamic>?)
              ?.map((e) => CommentDto.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
      limit: json['limit'] as int? ?? 20,
      nextCursor: json['next_cursor'] as String?,
    );
  }
}

/// Response DTO for toggle like action.
class ToggleLikeDto {
  final bool liked;

  const ToggleLikeDto({required this.liked});

  factory ToggleLikeDto.fromJson(Map<String, dynamic> json) {
    return ToggleLikeDto(liked: json['liked'] as bool);
  }
}
