/// Domain state untuk Home feature
/// Bebas dari implementation details
class HomeState {
  final List<String> feedItemIds;
  final bool isLoading;
  final String? errorMessage;

  const HomeState({
    this.feedItemIds = const [],
    this.isLoading = false,
    this.errorMessage,
  });

  HomeState copyWith({
    List<String>? feedItemIds,
    bool? isLoading,
    String? errorMessage,
  }) {
    return HomeState(
      feedItemIds: feedItemIds ?? this.feedItemIds,
      isLoading: isLoading ?? this.isLoading,
      errorMessage: errorMessage ?? this.errorMessage,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is HomeState &&
          other.feedItemIds == feedItemIds &&
          other.isLoading == isLoading &&
          other.errorMessage == errorMessage;

  @override
  int get hashCode => Object.hash(feedItemIds, isLoading, errorMessage);
}
