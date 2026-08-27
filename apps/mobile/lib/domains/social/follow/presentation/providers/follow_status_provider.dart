import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/domains/social/follow/data/follow_providers.dart';
import 'package:labuda/domains/social/follow/domain/use_cases/providers/use_case_providers.dart';
import 'package:labuda/domains/social/follow/presentation/providers/follow_stream_provider.dart';
import 'package:labuda/domains/social/follow/presentation/providers/follow_lists_provider.dart';

part 'follow_status_provider.g.dart';

/// Follow Status State
class FollowStatusState {
  final Map<String, bool> followStatusMap; // userId -> isFollowedByCurrentUser
  final bool isFollowProcessing;
  final String? error;

  const FollowStatusState({
    this.followStatusMap = const {},
    this.isFollowProcessing = false,
    this.error,
  });

  /// Check if user is followed by current user
  bool isFollowedByCurrentUser(String userId) {
    return followStatusMap[userId] ?? false;
  }

  FollowStatusState copyWith({
    Map<String, bool>? followStatusMap,
    bool? isFollowProcessing,
    String? error,
  }) {
    return FollowStatusState(
      followStatusMap: followStatusMap ?? this.followStatusMap,
      isFollowProcessing: isFollowProcessing ?? this.isFollowProcessing,
      error: error ?? this.error,
    );
  }
}

/// Follow Status Notifier for checking follow status
@riverpod
class FollowStatusNotifier extends _$FollowStatusNotifier {
  @override
  FollowStatusState build() {
    return const FollowStatusState();
  }

  IFollowRepository get _repository => ref.read(followRepositoryProvider);
  FollowUserUseCase get _followUserUseCase =>
      ref.read(followUserUseCaseProvider);
  UnfollowUserUseCase get _unfollowUserUseCase =>
      ref.read(unfollowUserUseCaseProvider);

  /// Check follow status for a specific user
  Future<void> checkFollowStatus({
    required String followerId,
    required String followingId,
  }) async {
    final result = await _repository.checkFollowStatus(
      followerId: followerId,
      followingId: followingId,
    );

    result.fold(
      (error) {
        state = state.copyWith(error: error);
      },
      (isFollowing) {
        final updatedStatusMap = Map<String, bool>.from(state.followStatusMap);
        updatedStatusMap[followingId] = isFollowing;
        state = state.copyWith(followStatusMap: updatedStatusMap);
      },
    );
  }

  /// Update follow status after action
  void updateFollowStatus(String userId, bool isFollowing) {
    final updatedStatusMap = Map<String, bool>.from(state.followStatusMap);
    updatedStatusMap[userId] = isFollowing;
    state = state.copyWith(followStatusMap: updatedStatusMap);
  }

  /// Follow a user (uses FollowUserUseCase for notification trigger)
  Future<void> followUser({
    required String followerId,
    required String followingId,
  }) async {
    state = state.copyWith(isFollowProcessing: true);

    final result = await _followUserUseCase.execute(
      FollowUserParams(followerId: followerId, followingId: followingId),
    );

    result.fold(
      (failure) {
        state = state.copyWith(
          isFollowProcessing: false,
          error: failure.message,
        );
      },
      (success) {
        if (success) {
          updateFollowStatus(followingId, true);
          // Invalidate ALL stream providers for both users to ensure UI updates
          // Stats for both users
          ref.invalidate(followStatsStreamProvider(followerId));
          ref.invalidate(followStatsStreamProvider(followingId));
          // Followers/following for BOTH users (not just one)
          ref.invalidate(followersStreamProvider(followerId));
          ref.invalidate(followersStreamProvider(followingId));
          ref.invalidate(followingStreamProvider(followerId));
          ref.invalidate(followingStreamProvider(followingId));
          // Invalidate lists provider to refresh the list
          ref.invalidate(followListsProvider);
        }
        state = state.copyWith(isFollowProcessing: false);
      },
    );
  }

  /// Unfollow a user (uses UnfollowUserUseCase for notification trigger)
  Future<void> unfollowUser({
    required String followerId,
    required String followingId,
  }) async {
    state = state.copyWith(isFollowProcessing: true);

    final result = await _unfollowUserUseCase.execute(
      UnfollowUserParams(followerId: followerId, followingId: followingId),
    );

    result.fold(
      (failure) {
        state = state.copyWith(
          isFollowProcessing: false,
          error: failure.message,
        );
      },
      (success) {
        if (success) {
          updateFollowStatus(followingId, false);
          // Invalidate ALL stream providers for both users to ensure UI updates
          // Stats for both users
          ref.invalidate(followStatsStreamProvider(followerId));
          ref.invalidate(followStatsStreamProvider(followingId));
          // Followers/following for BOTH users (not just one)
          ref.invalidate(followersStreamProvider(followerId));
          ref.invalidate(followersStreamProvider(followingId));
          ref.invalidate(followingStreamProvider(followerId));
          ref.invalidate(followingStreamProvider(followingId));
          // Invalidate lists provider to refresh the list
          ref.invalidate(followListsProvider);
        }
        state = state.copyWith(isFollowProcessing: false);
      },
    );
  }

  /// Clear error
  void clearError() {
    state = state.copyWith(error: null);
  }
}
