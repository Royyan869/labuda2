import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/comment/presentation/providers/comment_providers.dart';
import 'comment_state.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';
import 'package:labuda/domains/social/comment/domain/repositories/comment_repository.dart';

part 'comment_notifier.g.dart';

/// Comment Notifier
///
/// CONTRACT ALIGNMENT V1:
/// - Application layer - orchestrates comment operations
/// - Comment is social interaction, NOT commerce
/// - Commerce reference is seller response only, NOT a binding offer
/// - Uses canonical Comment entity from comment domain
@riverpod
class CommentNotifier extends _$CommentNotifier {
  bool _isLoading = false;

  @override
  CommentState build() {
    ref.watch(commentRepositoryProvider);
    final notificationTrigger = ref.watch(notificationTriggerProvider);

    _initServices(notificationTrigger);
    return const CommentState();
  }

  void _initServices(INotificationTrigger? notificationTrigger) {
    // Notification service initialized if needed
    // Currently unused in V1 - notifications require user info which is NOT
    // embedded in the canonical Comment entity
  }

  CommentRepository get _repository => ref.read(commentRepositoryProvider);

  /// Load comments for a specific target.
  ///
  /// Canonical target: CommentTargetType.content
  ///
  /// C-CURSOR / C-ORDER PAGINATION CONTRACT:
  ///   - Consumes the canonical cursor endpoint GET /contents/:id/comments,
  ///     ordered created_at ASC (oldest-first).
  ///   - Callers MUST NOT pass an explicit `page` argument; the notifier owns
  ///     the cursor via `state.nextCursor`. The `page` parameter is retained
  ///     only for backward source-compat and is IGNORED.
  ///   - On `loadMore: false` (initial load / refresh / retry) the notifier
  ///     fetches from the start (cursor null), replaces this target's rows and
  ///     stores the page's nextCursor. On `loadMore: true` it fetches
  ///     `state.nextCursor`, dedupes by Comment.id, appends only fresh rows,
  ///     and advances nextCursor.
  ///   - `state.hasMore` is `true` only while the backend returns a non-null
  ///     nextCursor; a null nextCursor means the list is exhausted and further
  ///     loadMore calls short-circuit without an HTTP call.
  Future<void> loadComments({
    required String targetId,
    required CommentTargetType targetType,
    int page = 1,
    int limit = 20,
    bool loadMore = false,
  }) async {
    if (_isLoading) {
      return;
    }

    // Exhaustion guard: continued scroll past a known-terminal page must not
    // refire the request. Only applies in loadMore mode — fresh loads always
    // re-fetch from the start.
    if (loadMore && !state.hasMore) {
      return;
    }

    // Cursor is owned by state. Fresh loads always start at the beginning.
    final cursor = loadMore ? state.nextCursor : null;

    _isLoading = true;

    // Loading flag scoping:
    //   - fresh load with empty state → isLoading=true (full-screen spinner)
    //   - loadMore                    → isLoadingMore=true (inline spinner)
    //   - fresh load with existing state (refresh) → neither, just await
    if (!loadMore && state.comments.isEmpty) {
      state = state.copyWith(
        isLoading: true,
        error: null,
        currentTargetId: targetId,
        currentTargetType: targetType,
      );
    } else if (loadMore) {
      state = state.copyWith(isLoadingMore: true);
    }

    try {
      final result = await _repository.getComments(
        targetId: targetId,
        targetType: targetType,
        cursor: cursor,
        limit: limit,
      );

      if (result.isSuccess) {
        final pageResult = result.data!;
        final newComments = pageResult.comments;
        final nextCursor = pageResult.nextCursor;

        if (loadMore) {
          // Dedupe by id against the union of all currently-loaded comments
          // (across targets — cheap because Set lookup is O(1)). A server
          // replay of the same page must not produce duplicate rows.
          final existingIds = state.comments.map((c) => c.id).toSet();
          final freshRows = newComments
              .where((c) => !existingIds.contains(c.id))
              .toList();

          state = state.copyWith(
            comments: [...state.comments, ...freshRows],
            isLoading: false,
            isLoadingMore: false,
            nextCursor: nextCursor,
            // Single exhaustion authority: the backend returns no cursor when
            // the list is fully drained.
            hasMore: nextCursor != null,
          );
        } else {
          // Fresh load / refresh: replace this target's comments, keep any
          // cross-target rows in state. Server order is preserved (ASC).
          final otherComments = state.comments
              .where((c) => c.contentId != targetId)
              .toList();

          state = state.copyWith(
            comments: [...otherComments, ...newComments],
            isLoading: false,
            isLoadingMore: false,
            currentTargetId: targetId,
            currentTargetType: targetType,
            nextCursor: nextCursor,
            hasMore: nextCursor != null,
          );
        }
      } else {
        state = state.copyWith(
          isLoading: false,
          isLoadingMore: false,
          error: result.error ?? 'Failed to load comments',
        );
      }
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        isLoadingMore: false,
        error: e.toString(),
      );
    } finally {
      _isLoading = false;
    }
  }

  /// Create a new comment
  Future<Result<Comment>> createComment({
    required String targetId,
    required CommentTargetType targetType,
    required String content,
    String? parentId,
    List<String> mentionedUserIds = const [],
    String? targetOwnerId,
    String? currentUserId,
    String? currentUserName,
  }) async {
    // Use usecase for validation
    final validateCommentContent = ref.read(
      validateCommentContentUseCaseProvider,
    );
    final validationResult = await validateCommentContent(
      content: content,
    );

    if (validationResult.isError) {
      return Result.error(validationResult.error ?? 'Validation failed');
    }

    final result = await _repository.createComment(
      targetId: targetId,
      targetType: targetType,
      content: content,
      parentId: parentId,
      mentionedUserIds: mentionedUserIds,
    );

    if (result.isSuccess) {
      final newComment = result.data!;

      // C-ORDER — append at the tail to preserve backend ASC (oldest-first)
      // ordering; the comment is the newest row and belongs at the end.
      state = state.copyWith(comments: [...state.comments, newComment]);

      // NOTE: Notification logic would require user info which is NOT embedded
      // in the canonical Comment entity. This is a V1 limitation.
      // Future enhancement: Fetch user info separately or embed in responses.

      return Result.success(newComment);
    }

    return result;
  }

  /// Create a commerce reference comment (seller response).
  Future<Result<Comment>> createCommerceReferenceComment({
    required String contentId,
    required String resourceType,
    required String resourceId,
    String? body,
  }) async {
    final result = await _repository.createCommerceReferenceComment(
      contentId: contentId,
      resourceType: resourceType,
      resourceId: resourceId,
      body: body,
    );

    if (result.isSuccess) {
      final newComment = result.data!;

      // C-ORDER — append at the tail (ASC), same as normal comment create.
      state = state.copyWith(comments: [...state.comments, newComment]);

      return Result.success(newComment);
    }

    return result;
  }

  /// Delete a comment (soft delete)
  Future<Result<bool>> deleteComment(String commentId) async {
    final result = await _repository.deleteComment(commentId);

    if (result.isSuccess) {
      // Remove from state
      final updatedComments = state.comments
          .where((c) => c.id != commentId)
          .toList();
      state = state.copyWith(comments: updatedComments);
    }

    return result;
  }

  /// Clear error state
  void clearError() {
    state = state.copyWith(error: null);
  }

  /// Reset state for new target
  void resetForTarget(String targetId, CommentTargetType targetType) {
    if (state.currentTargetId != targetId ||
        state.currentTargetType != targetType) {
      state = const CommentState();
    }
  }
}

