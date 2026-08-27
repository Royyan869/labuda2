import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/domain/use_cases/providers/use_case_providers.dart';

part 'follow_search_provider.g.dart';

/// Follow Search State
class FollowSearchState {
  final List<FollowableUser> searchResults;
  final List<FollowableUser> suggestedUsers;
  final String searchQuery;
  final UserType? searchFilter;
  final bool isSearching;
  final String? error;

  const FollowSearchState({
    this.searchResults = const [],
    this.suggestedUsers = const [],
    this.searchQuery = '',
    this.searchFilter,
    this.isSearching = false,
    this.error,
  });

  FollowSearchState copyWith({
    List<FollowableUser>? searchResults,
    List<FollowableUser>? suggestedUsers,
    String? searchQuery,
    UserType? searchFilter,
    bool? isSearching,
    String? error,
  }) {
    return FollowSearchState(
      searchResults: searchResults ?? this.searchResults,
      suggestedUsers: suggestedUsers ?? this.suggestedUsers,
      searchQuery: searchQuery ?? this.searchQuery,
      searchFilter: searchFilter ?? this.searchFilter,
      isSearching: isSearching ?? this.isSearching,
      error: error ?? this.error,
    );
  }
}

/// Follow Search Notifier for user search functionality
@riverpod
class FollowSearchNotifier extends _$FollowSearchNotifier {
  @override
  FollowSearchState build() {
    return const FollowSearchState();
  }

  SearchUsersUseCase get _searchUsersUseCase =>
      ref.read(searchUsersUseCaseProvider);

  /// Search users
  Future<void> searchUsers({
    required String query,
    String? currentUserId,
    UserType? filterByType,
    int limit = 20,
  }) async {
    state = state.copyWith(
      isSearching: true,
      searchQuery: query,
      searchFilter: filterByType,
      error: null,
    );

    final params = SearchUsersParams(
      query: query,
      currentUserId: currentUserId,
      filterByType: filterByType,
      limit: limit,
    );

    final result = await _searchUsersUseCase.execute(params);

    result.fold(
      (failure) {
        state = state.copyWith(isSearching: false, error: failure.message);
      },
      (users) {
        state = state.copyWith(
          searchResults: users,
          isSearching: false,
          error: null,
        );
      },
    );
  }

  /// Clear search results
  void clearSearch() {
    state = state.copyWith(
      searchResults: [],
      searchQuery: '',
      searchFilter: null,
    );
  }

  /// Set suggested users
  void setSuggestedUsers(List<FollowableUser> users) {
    state = state.copyWith(suggestedUsers: users);
  }

  /// Clear error
  void clearError() {
    state = state.copyWith(error: null);
  }
}
