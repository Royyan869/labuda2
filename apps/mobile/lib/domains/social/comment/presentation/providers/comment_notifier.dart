import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/comment/presentation/providers/comment_providers.dart';
import 'comment_state.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';
import 'package:labuda/domains/social/comment/domain/repositories/comment_repository.dart';

part 'comment_notifier.g.dart';

/// The exact request behind a pending comment row, replayed on retry.
class _PendingCommentPayload {
  final String targetId;
  final CommentTargetType targetType;
  final String content;
  final String? parentId;
  final List<String> mentionedUserIds;
  final List<String> mediaUrls;
  final String? resourceType;
  final String? resourceId;

  const _PendingCommentPayload({
    required this.targetId,
    required this.targetType,
    required this.content,
    this.parentId,
    this.mentionedUserIds = const [],
    this.mediaUrls = const [],
    this.resourceType,
    this.resourceId,
  });
}

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

  /// Payloads of in-flight/failed comments, keyed by the pending row id, so
  /// "Coba lagi" replays the exact same request.
  final Map<String, _PendingCommentPayload> _pendingCommentPayloads = {};
  int _pendingCommentSeq = 0;

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
    List<String> mediaUrls = const [],
    String? targetOwnerId,
    String? currentUserId,
    String? currentUserName,
  }) async {
    // Media can stand alone: allow empty body when media present
    if (content.trim().isEmpty && mediaUrls.isEmpty) {
      final validateCommentContent = ref.read(
        validateCommentContentUseCaseProvider,
      );
      final validationResult = await validateCommentContent(content: content);
      if (validationResult.isError) {
        return Result.error(validationResult.error ?? 'Validation failed');
      }
    }

    // The row exists BEFORE the request does: a list that looks unchanged after
    // Send is what made the first tap feel ignored. It is dropped the moment the
    // server row arrives, and kept (marked failed) when the send fails.
    final pendingId = _beginPendingComment(
      targetId: targetId,
      targetType: targetType,
      content: content,
      parentId: parentId,
      mentionedUserIds: mentionedUserIds,
      mediaUrls: mediaUrls,
    );

    final result = await _repository.createComment(
      targetId: targetId,
      targetType: targetType,
      content: content,
      parentId: parentId,
      mentionedUserIds: mentionedUserIds,
      mediaUrls: mediaUrls,
    );

    if (result.isSuccess) {
      final newComment = result.data!;

      // C-ORDER — append at the tail to preserve backend ASC (oldest-first)
      // ordering; the comment is the newest row and belongs at the end.
      state = state.copyWith(
        pendingComments: _withoutPendingComment(pendingId),
        comments: [...state.comments, newComment],
      );

      // NOTE: Notification logic would require user info which is NOT embedded
      // in the canonical Comment entity. This is a V1 limitation.
      // Future enhancement: Fetch user info separately or embed in responses.

      return Result.success(newComment);
    }

    _markPendingCommentFailed(pendingId);
    return result;
  }

  /// Retries a failed comment from its own pending row.
  ///
  /// The row is dropped here and re-created by the canonical create path, so a
  /// retry never leaves a duplicate behind.
  Future<Result<Comment>> retryPendingComment(String pendingId) async {
    final payload = _pendingCommentPayloads.remove(pendingId);
    if (payload == null) {
      return Result.error('Tidak ada komentar untuk dikirim ulang');
    }

    state = state.copyWith(
      pendingComments: _withoutPendingComment(pendingId),
    );

    if (payload.resourceType != null && payload.resourceId != null) {
      return createCommerceReferenceComment(
        contentId: payload.targetId,
        resourceType: payload.resourceType!,
        resourceId: payload.resourceId!,
        body: payload.content.isEmpty ? null : payload.content,
      );
    }

    return createComment(
      targetId: payload.targetId,
      targetType: payload.targetType,
      content: payload.content,
      parentId: payload.parentId,
      mentionedUserIds: payload.mentionedUserIds,
      mediaUrls: payload.mediaUrls,
    );
  }

  /// Shows the comment the user is sending, before the server confirms it.
  String _beginPendingComment({
    required String targetId,
    required CommentTargetType targetType,
    required String content,
    String? parentId,
    List<String> mentionedUserIds = const [],
    List<String> mediaUrls = const [],
    String? resourceType,
    String? resourceId,
    String? pendingLabel,
  }) {
    final pendingId =
        'pending_${DateTime.now().microsecondsSinceEpoch}_${_pendingCommentSeq++}';

    _pendingCommentPayloads[pendingId] = _PendingCommentPayload(
      targetId: targetId,
      targetType: targetType,
      content: content,
      parentId: parentId,
      mentionedUserIds: mentionedUserIds,
      mediaUrls: mediaUrls,
      resourceType: resourceType,
      resourceId: resourceId,
    );

    state = state.copyWith(
      pendingComments: [
        ...state.pendingComments,
        PendingComment(
          id: pendingId,
          contentId: targetId,
          // A media-only comment still says something while it is in flight.
          content: content.isEmpty ? (pendingLabel ?? 'Lampiran') : content,
          parentId: parentId,
          createdAt: DateTime.now(),
        ),
      ],
    );

    return pendingId;
  }

  List<PendingComment> _withoutPendingComment(String pendingId) => state
      .pendingComments
      .where((row) => row.id != pendingId)
      .toList();

  /// Keeps a failed comment on screen as an actionable row instead of dropping it
  /// behind a transient message.
  void _markPendingCommentFailed(String pendingId) {
    if (!_pendingCommentPayloads.containsKey(pendingId)) return;

    state = state.copyWith(
      pendingComments: state.pendingComments
          .map(
            (row) => row.id == pendingId ? row.copyWith(failed: true) : row,
          )
          .toList(),
    );
  }

  /// Create a commerce reference comment (seller response).
  Future<Result<Comment>> createCommerceReferenceComment({
    required String contentId,
    required String resourceType,
    required String resourceId,
    String? body,
  }) async {
    // Same provisional row as a normal comment: the seller's attach flow must not
    // look inert either.
    final pendingId = _beginPendingComment(
      targetId: contentId,
      targetType: CommentTargetType.content,
      content: body ?? '',
      resourceType: resourceType,
      resourceId: resourceId,
      pendingLabel: 'Lampiran produk',
    );

    final result = await _repository.createCommerceReferenceComment(
      contentId: contentId,
      resourceType: resourceType,
      resourceId: resourceId,
      body: body,
    );

    if (result.isSuccess) {
      final newComment = result.data!;

      // C-ORDER — append at the tail (ASC), same as normal comment create.
      state = state.copyWith(
        pendingComments: _withoutPendingComment(pendingId),
        comments: [...state.comments, newComment],
      );

      return Result.success(newComment);
    }

    _markPendingCommentFailed(pendingId);
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

  /// Update a comment body (author only)
  Future<Result<Comment>> updateComment({
    required String commentId,
    required String body,
  }) async {
    final trimmed = body.trim();
    if (trimmed.isEmpty) {
      return Result.error('Body cannot be empty');
    }
    if (trimmed.length > 2000) {
      return Result.error('Body too long');
    }
    final result = await _repository.updateComment(
      commentId: commentId,
      body: trimmed,
    );
    if (result.isSuccess) {
      final updated = result.data!;
      final updatedComments = state.comments
          .map((c) => c.id == commentId ? updated : c)
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
