import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';

/// Comment Repository Interface
///
/// Pure domain interface - no implementation details exposed.
/// Comment is social interaction, NOT commerce.
abstract class CommentRepository {
  /// Get comments for a specific target with cursor-based pagination.
  ///
  /// C-CURSOR: consumes GET /contents/:id/comments (created_at ASC). Pass the
  /// [cursor] returned by the previous page; null starts at the first page.
  /// The backend list has no `total` — exhaustion is signalled by a null
  /// nextCursor on the returned [CommentPage].
  Future<Result<CommentPage>> getComments({
    required String targetId,
    required CommentTargetType targetType,
    String? cursor,
    int limit = 20,
  });

  /// Create a new comment.
  ///
  /// C-IPC: an idempotency key is generated per logical creation attempt and
  /// preserved across any transport retry of that same attempt; never
  /// regenerated for a retry inside the same logical call.
  Future<Result<Comment>> createComment({
    required String targetId,
    required CommentTargetType targetType,
    required String content,
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
