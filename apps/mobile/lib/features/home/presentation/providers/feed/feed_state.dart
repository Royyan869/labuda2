import 'package:hishumi/features/home/domain/domain.dart'; // R3.1: Import FeedItem from home domain

/// State untuk Feed Notifier
///
/// LOADING SEMANTICS (owner-locked Loading Foundation):
/// - [isLoading] + empty [items] = first load. The page may render
///   [LoadingIndicator] as the main content state.
/// - [isRefreshing] + non-empty [items] = refresh. Last-known-good items stay
///   visible with an update indicator; the page must never swap to full-page
///   loading or full-page error while items exist.
/// - [errorMessage] = first-load failure only (items empty → PageErrorState).
/// - [refreshError] = refresh failure with old data preserved (items stay +
///   inline refresh-failure indication, never PageErrorState).
class FeedState {
  final List<FeedItem> items;
  final bool isLoading;
  final bool isLoadingMore;
  final bool isRefreshing;
  final String? errorMessage;
  final String? refreshError;
  final bool hasReachedMax;

  const FeedState({
    this.items = const [],
    this.isLoading = false,
    this.isLoadingMore = false,
    this.isRefreshing = false,
    this.errorMessage,
    this.refreshError,
    this.hasReachedMax = false,
  });

  FeedState copyWith({
    List<FeedItem>? items,
    bool? isLoading,
    bool? isLoadingMore,
    bool? isRefreshing,
    String? errorMessage,
    String? refreshError,
    bool clearRefreshError = false,
    bool? hasReachedMax,
  }) {
    return FeedState(
      items: items ?? this.items,
      isLoading: isLoading ?? this.isLoading,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      isRefreshing: isRefreshing ?? this.isRefreshing,
      errorMessage: errorMessage,
      refreshError: clearRefreshError
          ? null
          : refreshError ?? this.refreshError,
      hasReachedMax: hasReachedMax ?? this.hasReachedMax,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is FeedState &&
          other.items == items &&
          other.isLoading == isLoading &&
          other.isLoadingMore == isLoadingMore &&
          other.isRefreshing == isRefreshing &&
          other.errorMessage == errorMessage &&
          other.refreshError == refreshError &&
          other.hasReachedMax == hasReachedMax;

  @override
  int get hashCode => Object.hash(
    items,
    isLoading,
    isLoadingMore,
    isRefreshing,
    errorMessage,
    refreshError,
    hasReachedMax,
  );
}
