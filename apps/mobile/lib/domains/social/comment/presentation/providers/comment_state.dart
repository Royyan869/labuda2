import 'package:labuda/domains/social/comment/domain/entities/comment.dart';

/// Comment State
///
/// CONTRACT ALIGNMENT V1:
/// - Represents the state of comments in the application
/// - Pure state class - no business logic
/// - Comments keyed by target (contentId + targetType) for organization
class CommentState {
  /// All loaded comments across different targets
  final List<Comment> comments;

  /// Loading indicator
  final bool isLoading;

  /// Error message if any
  final String? error;

  /// Current target being viewed
  final String? currentTargetId;

  /// Current target type
  final CommentTargetType? currentTargetType;

  /// C-CURSOR — Pagination state owned by the notifier.
  ///
  /// `nextCursor` is the backend cursor to pass to the NEXT loadMore fetch.
  /// Null means the first page has not been fetched yet. The notifier
  /// overwrites it with the page's `nextCursor` on every load; a null page
  /// cursor flips [hasMore] to false. Callers (e.g. discussion_screen) MUST
  /// NOT pass an explicit page argument; the notifier owns the cursor.
  final String? nextCursor;

  /// false once the comment list for the current target is known to be
  /// exhausted (backend returned a page with no nextCursor). Callers MUST
  /// short-circuit loadMore when this is false so the same page is not
  /// refetched indefinitely on continued scroll.
  final bool hasMore;

  /// Distinct from `isLoading` (initial load). True only while
  /// a pagination loadMore is in flight; lets the UI render an in-list
  /// spinner without rebuilding the entire list as a loading state.
  final bool isLoadingMore;

  const CommentState({
    this.comments = const [],
    this.isLoading = false,
    this.error,
    this.currentTargetId,
    this.currentTargetType,
    this.nextCursor,
    this.hasMore = true,
    this.isLoadingMore = false,
  });

  /// Get comments for specific target (content)
  ///
  /// NOTE: Since Comment entity has contentId field, we filter by contentId
  /// for content-type targets. This matches the backend structure where
  /// comments reference their content directly.
  List<Comment> getCommentsForTarget({
    required String targetId,
    required CommentTargetType targetType,
  }) {
    if (targetType == CommentTargetType.content) {
      // For content targets, filter by contentId field
      return comments
          .where((c) => c.contentId == targetId && c.deletedAt == null)
          .toList();
    }

    // For other target types (future), would need different filtering
    // Currently not supported in V1
    return [];
  }

  /// Check if currently loading
  bool get isLoadingComments => isLoading;

  CommentState copyWith({
    List<Comment>? comments,
    bool? isLoading,
    String? error,
    String? currentTargetId,
    CommentTargetType? currentTargetType,
    String? nextCursor,
    bool? hasMore,
    bool? isLoadingMore,
  }) {
    return CommentState(
      comments: comments ?? this.comments,
      isLoading: isLoading ?? this.isLoading,
      error: error,
      currentTargetId: currentTargetId ?? this.currentTargetId,
      currentTargetType: currentTargetType ?? this.currentTargetType,
      nextCursor: nextCursor ?? this.nextCursor,
      hasMore: hasMore ?? this.hasMore,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
    );
  }
}
