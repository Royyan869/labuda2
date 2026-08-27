import 'package:labuda/features/home/domain/domain.dart'; // R3.1: Import FeedItem from home domain

/// State untuk Feed Notifier
class FeedState {
  final List<FeedItem> items;
  final bool isLoading;
  final bool isLoadingMore;
  final String? errorMessage;
  final bool hasReachedMax;

  const FeedState({
    this.items = const [],
    this.isLoading = false,
    this.isLoadingMore = false,
    this.errorMessage,
    this.hasReachedMax = false,
  });

  FeedState copyWith({
    List<FeedItem>? items,
    bool? isLoading,
    bool? isLoadingMore,
    String? errorMessage,
    bool? hasReachedMax,
  }) {
    return FeedState(
      items: items ?? this.items,
      isLoading: isLoading ?? this.isLoading,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      errorMessage: errorMessage,
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
          other.errorMessage == errorMessage &&
          other.hasReachedMax == hasReachedMax;

  @override
  int get hashCode =>
      Object.hash(items, isLoading, isLoadingMore, errorMessage, hasReachedMax);
}
