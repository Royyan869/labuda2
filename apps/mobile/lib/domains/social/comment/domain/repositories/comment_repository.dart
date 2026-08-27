import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';

/// Comment Repository Interface
///
/// CONTRACT ALIGNMENT V1:
/// - Pure domain interface - no implementation details exposed
/// - Defines the contract for comment operations
/// - Comment is social interaction, NOT commerce
///
/// NOTE: Some methods are deprecated as they are not supported by the
/// Content module comment API in V1.
abstract class CommentRepository {
  /// Get comments for a specific target with cursor-based pagination.
  ///
  /// C-CURSOR: consumes GET /contents/:id/comments (created_at ASC). Pass the
  /// [cursor] returned by the previous page; null starts at the first page.
  /// The backend list has no `total` — exhaustion is signalled by a null
  /// nextCursor on the returned [CommentPage].
  ///
  /// V1 canonical target: 'content' (posts and requests)
  Future<Result<CommentPage>> getComments({
    required String targetId,
    required CommentTargetType targetType,
    String? cursor,
    int limit = 20,
  });

  /// Create a new comment
  ///
  /// NOTE: mediaUrls and attachment are NOT supported by Content module API.
  /// These parameters exist for future compatibility but are currently ignored.
  ///
  /// C-IPC: an idempotency key is generated per logical creation attempt and
  /// preserved across any transport retry of that same attempt; never
  /// regenerated for a retry inside the same logical call.
  Future<Result<Comment>> createComment({
    required String targetId,
    required CommentTargetType targetType,
    required String content,
    List<String> mediaUrls = const [],
    CommentAttachment? attachment,
    String? parentId,
    List<String> mentionedUserIds = const [],
  });

  /// Create a commerce reference comment.
  ///
  /// CONTRACT: This creates a seller response attached to exactly one
  /// commerce resource identity. The backend enforces ownership and
  /// market authority.
  Future<Result<Comment>> createCommerceReferenceComment({
    required String contentId,
    required String resourceType,
    required String resourceId,
    String? body,
  });

  /// Delete a comment (soft delete)
  Future<Result<bool>> deleteComment(String commentId);

  /// Validate comment content (anti-circumvention, length check)
  Future<Result<bool>> validateContent(String content);
}

/// Type alias for legacy CommentAttachment
///
/// DEPRECATED: NOT used in canonical V1 comment flow.
typedef CommentAttachment = dynamic;
