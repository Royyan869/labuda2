import 'package:equatable/equatable.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Comment Entity - canonical domain entity for comments system.
///
/// Comment is a social interaction object, NOT a commerce object.
/// reference represents seller response/recommendation ONLY.
/// Does NOT represent binding offers, negotiations, or transactions.
/// Reply max depth = 1 (top-level comments can be replied, replies cannot).
///
/// SINGLE REFERENCE MODEL:
/// - reference uses ShareReference (canonical cross-domain reference)
/// - targetType = "for_sale" / "auction" / "content"
///
/// Pure domain entity - no Flutter/Firebase dependencies.
class Comment extends Equatable {
  final String id;
  final String authorId;
  final String contentId;
  // Author info embedded in response for proper UI rendering
  final String authorUsername;
  final String? authorAvatarUrl;
  final String? body;
  final String type;
  final ShareReference? reference; // Unified reference using ShareReference
  final String? parentId; // Set for replies (max depth = 1)
  final DateTime createdAt;
  final DateTime? updatedAt;
  final DateTime? deletedAt;

  /// E3.1 — Canonical governance lifecycle for the comment AUTHOR identity
  /// (PublicCard.UserCard.Lifecycle on the comment-author seam).
  ///
  /// Independent of [deletedAt] (which is the COMMENT's own soft-delete
  /// state, not the author's). Defaults to active for null / unknown /
  /// missing wire values so legacy backend payloads keep rendering
  /// today's behavior.
  ///
  /// Mobile preparation only — backend currently emits `null` for this
  /// field. The mobile redaction seam is wired ahead of any backend
  /// activation per E3.1 doctrine.
  final ContentLifecycle authorLifecycle;

  const Comment({
    required this.id,
    required this.authorId,
    required this.contentId,
    required this.authorUsername,
    this.authorAvatarUrl,
    this.body,
    required this.type,
    this.reference,
    this.parentId,
    required this.createdAt,
    this.updatedAt,
    this.deletedAt,
    this.authorLifecycle = ContentLifecycle.active,
  });

  /// Check if this is a commerce reference comment (seller response)
  bool get isCommerceReference => type == 'commerce_reference';

  /// Check if this comment has a reference attached
  bool get hasReference => reference != null;

  /// Check if this is a normal comment
  bool get isNormal => type == 'normal';

  /// Check if this comment has been deleted (soft delete)
  bool get isDeleted => deletedAt != null;

  /// Check if this is a reply (has parent)
  bool get isReply => parentId != null;

  /// Check if this is a top-level comment (can be replied to)
  bool get isTopLevel => parentId == null;

  @override
  List<Object?> get props => [
    id,
    authorId,
    contentId,
    authorUsername,
    authorAvatarUrl,
    body,
    type,
    reference,
    parentId,
    createdAt,
    updatedAt,
    deletedAt,
    authorLifecycle,
  ];

  Comment copyWith({
    String? id,
    String? authorId,
    String? contentId,
    String? authorUsername,
    String? authorAvatarUrl,
    String? body,
    String? type,
    ShareReference? reference,
    String? parentId,
    DateTime? createdAt,
    DateTime? updatedAt,
    DateTime? deletedAt,
    ContentLifecycle? authorLifecycle,
  }) {
    return Comment(
      id: id ?? this.id,
      authorId: authorId ?? this.authorId,
      contentId: contentId ?? this.contentId,
      authorUsername: authorUsername ?? this.authorUsername,
      authorAvatarUrl: authorAvatarUrl ?? this.authorAvatarUrl,
      body: body ?? this.body,
      type: type ?? this.type,
      reference: reference ?? this.reference,
      parentId: parentId ?? this.parentId,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      deletedAt: deletedAt ?? this.deletedAt,
      authorLifecycle: authorLifecycle ?? this.authorLifecycle,
    );
  }
}

/// Cursor page of comments returned by the canonical list endpoint.
///
/// C-IDENT / C-CURSOR:
/// - [comments] in backend ASC (oldest-first) order.
/// - [nextCursor] is the opaque cursor to pass to the next page fetch; null
///   means the list is exhausted (the backend list carries no `total`).
class CommentPage extends Equatable {
  final List<Comment> comments;
  final String? nextCursor;

  const CommentPage({required this.comments, this.nextCursor});

  @override
  List<Object?> get props => [comments, nextCursor];
}

/// Comment type enum matching backend CommentType
///
/// Aligned with backend/internal/domain/content/entity/comment.go
enum CommentType {
  /// Normal text comment
  normal('normal'),

  /// Commerce reference comment (seller response with resource attachment)
  commerceReference('commerce_reference');

  const CommentType(this.value);
  final String value;

  static CommentType fromString(String value) {
    return CommentType.values.firstWhere(
      (type) => type.value == value,
      orElse: () => CommentType.normal,
    );
  }
}

/// Comment target type enum matching backend CommentTargetType.
///
/// Only 'content' target is canonical for V1.
enum CommentTargetType {
  /// Content — the only canonical target for V1.
  content('content');

  const CommentTargetType(this.value);
  final String value;

  static CommentTargetType fromString(String value) {
    return CommentTargetType.values.firstWhere(
      (type) => type.value == value,
      orElse: () => CommentTargetType.content,
    );
  }
}
