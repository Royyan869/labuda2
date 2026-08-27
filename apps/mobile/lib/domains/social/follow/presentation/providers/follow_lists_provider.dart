import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/domain/use_cases/providers/use_case_providers.dart';

part 'follow_lists_provider.g.dart';

/// Follow Lists State
class FollowListsState {
  final Map<String, List<FollowableUser>> followersMap;
  final Map<String, List<FollowableUser>> followingMap;
  final bool isLoading;
  final String? error;

  const FollowListsState({
    this.followersMap = const {},
    this.followingMap = const {},
    this.isLoading = false,
    this.error,
  });

  /// Get followers for user
  List<FollowableUser> getFollowers(String userId) {
    return followersMap[userId] ?? [];
  }

  /// Get following for user
  List<FollowableUser> getFollowing(String userId) {
    return followingMap[userId] ?? [];
  }

  FollowListsState copyWith({
    Map<String, List<FollowableUser>>? followersMap,
    Map<String, List<FollowableUser>>? followingMap,
    bool? isLoading,
    String? error,
  }) {
    return FollowListsState(
      followersMap: followersMap ?? this.followersMap,
      followingMap: followingMap ?? this.followingMap,
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
    );
  }
}

/// Follow Lists Notifier for followers and following lists
@riverpod
class FollowListsNotifier extends _$FollowListsNotifier {
  @override
  FollowListsState build() {
    return const FollowListsState();
  }

  GetFollowersUseCase get _getFollowersUseCase =>
      ref.read(getFollowersUseCaseProvider);
  GetFollowingUseCase get _getFollowingUseCase =>
      ref.read(getFollowingUseCaseProvider);

  /// Load followers
  Future<void> loadFollowers({
    required String userId,
    int limit = 20,
    String? lastFollowId,
    bool append = false,
  }) async {
    if (!append) {
      state = state.copyWith(isLoading: true, error: null);
    }

    final params = GetFollowersParams(
      userId: userId,
      limit: limit,
      lastFollowId: lastFollowId,
    );

    final result = await _getFollowersUseCase.execute(params);

    result.fold(
      (failure) {
        state = state.copyWith(isLoading: false, error: failure.message);
      },
      (followers) {
        final updatedFollowersMap = Map<String, List<FollowableUser>>.from(
          state.followersMap,
        );

        updatedFollowersMap[userId] =
            append && updatedFollowersMap.containsKey(userId)
            ? [...updatedFollowersMap[userId]!, ...followers]
            : followers;

        state = state.copyWith(
          followersMap: updatedFollowersMap,
          isLoading: false,
          error: null,
        );
      },
    );
  }

  /// Load following
  Future<void> loadFollowing({
    required String userId,
    int limit = 20,
    String? lastFollowId,
    bool append = false,
  }) async {
    if (!append) {
      state = state.copyWith(isLoading: true, error: null);
    }

    final params = GetFollowingParams(
      userId: userId,
      limit: limit,
      lastFollowId: lastFollowId,
    );

    final result = await _getFollowingUseCase.execute(params);

    result.fold(
      (failure) {
        state = state.copyWith(isLoading: false, error: failure.message);
      },
      (following) {
        final updatedFollowingMap = Map<String, List<FollowableUser>>.from(
          state.followingMap,
        );

        updatedFollowingMap[userId] =
            append && updatedFollowingMap.containsKey(userId)
            ? [...updatedFollowingMap[userId]!, ...following]
            : following;

        state = state.copyWith(
          followingMap: updatedFollowingMap,
          isLoading: false,
          error: null,
        );
      },
    );
  }

  /// Clear error
  void clearError() {
    state = state.copyWith(error: null);
  }
}
