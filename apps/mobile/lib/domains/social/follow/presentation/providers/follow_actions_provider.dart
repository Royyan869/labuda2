import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/follow/domain/use_cases/providers/use_case_providers.dart';

part 'follow_actions_provider.g.dart';

/// Follow Actions State
class FollowActionsState {
  final bool isFollowProcessing;
  final String? error;

  const FollowActionsState({this.isFollowProcessing = false, this.error});

  FollowActionsState copyWith({bool? isFollowProcessing, String? error}) {
    return FollowActionsState(
      isFollowProcessing: isFollowProcessing ?? this.isFollowProcessing,
      error: error ?? this.error,
    );
  }
}

/// Follow Actions Notifier for follow/unfollow operations
@riverpod
class FollowActionsNotifier extends _$FollowActionsNotifier {
  @override
  FollowActionsState build() {
    return const FollowActionsState();
  }

  FollowUserUseCase get _followUserUseCase =>
      ref.read(followUserUseCaseProvider);
  UnfollowUserUseCase get _unfollowUserUseCase =>
      ref.read(unfollowUserUseCaseProvider);
  IAnalyticsRepository get _analytics =>
      ref.read(coreAnalyticsRepositoryProvider);

  /// Follow user
  Future<bool> followUser({
    required String followerId,
    required String followingId,
  }) async {
    state = state.copyWith(isFollowProcessing: true, error: null);

    final params = FollowUserParams(
      followerId: followerId,
      followingId: followingId,
    );

    final result = await _followUserUseCase.execute(params);

    return result.fold(
      (failure) {
        state = state.copyWith(
          isFollowProcessing: false,
          error: failure.message,
        );
        return false;
      },
      (success) {
        state = state.copyWith(
          isFollowProcessing: false,
          error: success ? null : 'Failed to follow user',
        );

        // Track follow analytics (presentation layer responsibility)
        if (success) {
          _trackFollowInteraction(
            action: 'follow',
            followerId: followerId,
            followingId: followingId,
          );
        }

        return success;
      },
    );
  }

  /// Unfollow user
  Future<bool> unfollowUser({
    required String followerId,
    required String followingId,
  }) async {
    state = state.copyWith(isFollowProcessing: true, error: null);

    final params = UnfollowUserParams(
      followerId: followerId,
      followingId: followingId,
    );

    final result = await _unfollowUserUseCase.execute(params);

    return result.fold(
      (failure) {
        state = state.copyWith(
          isFollowProcessing: false,
          error: failure.message,
        );
        return false;
      },
      (success) {
        state = state.copyWith(
          isFollowProcessing: false,
          error: success ? null : 'Failed to unfollow user',
        );

        // Track unfollow analytics (presentation layer responsibility)
        if (success) {
          _trackFollowInteraction(
            action: 'unfollow',
            followerId: followerId,
            followingId: followingId,
          );
        }

        return success;
      },
    );
  }

  /// Track follow/unfollow analytics
  void _trackFollowInteraction({
    required String action,
    required String followerId,
    required String followingId,
  }) {
    try {
      _analytics.logEvent(
        action,
        parameters: {'follower_id': followerId, 'following_id': followingId},
        userId: followerId,
      );
    } catch (e) {
      // Ignore analytics errors - don't fail follow action
    }
  }

  /// Clear error
  void clearError() {
    state = state.copyWith(error: null);
  }
}
