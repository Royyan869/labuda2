import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/domain/use_cases/providers/use_case_providers.dart';

part 'follow_stats_provider.g.dart';

/// Follow Stats State
class FollowStatsState {
  final Map<String, FollowStats> followStats;
  final bool isLoading;
  final String? error;

  const FollowStatsState({
    this.followStats = const {},
    this.isLoading = false,
    this.error,
  });

  /// Get follow stats for user
  FollowStats? getFollowStats(String userId) {
    return followStats[userId];
  }

  FollowStatsState copyWith({
    Map<String, FollowStats>? followStats,
    bool? isLoading,
    String? error,
  }) {
    return FollowStatsState(
      followStats: followStats ?? this.followStats,
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
    );
  }
}

/// Follow Stats Notifier for follow statistics
@riverpod
class FollowStatsNotifier extends _$FollowStatsNotifier {
  @override
  FollowStatsState build() {
    return const FollowStatsState();
  }

  GetFollowStatsUseCase get _getFollowStatsUseCase =>
      ref.read(getFollowStatsUseCaseProvider);

  /// Load follow stats
  Future<void> loadFollowStats({
    required String userId,
    String? currentUserId,
  }) async {
    state = state.copyWith(isLoading: true, error: null);

    final params = GetFollowStatsParams(
      userId: userId,
      currentUserId: currentUserId,
    );

    final result = await _getFollowStatsUseCase.execute(params);

    result.fold(
      (failure) {
        state = state.copyWith(isLoading: false, error: failure.message);
      },
      (stats) {
        final updatedStats = Map<String, FollowStats>.from(state.followStats);
        updatedStats[userId] = stats;

        state = state.copyWith(
          followStats: updatedStats,
          isLoading: false,
          error: null,
        );
      },
    );
  }

  /// Update stats after follow action
  void updateStatsAfterFollow(String userId, bool isFollow) {
    final updatedStats = Map<String, FollowStats>.from(state.followStats);
    if (updatedStats.containsKey(userId)) {
      final currentStats = updatedStats[userId]!;
      updatedStats[userId] = FollowStats(
        userId: currentStats.userId,
        followersCount: currentStats.followersCount + (isFollow ? 1 : -1),
        followingCount: currentStats.followingCount,
        mutualFollowsCount: currentStats.mutualFollowsCount,
        topFollowers: currentStats.topFollowers,
        suggestedUsers: currentStats.suggestedUsers,
        lastUpdated: DateTime.now(),
      );

      state = state.copyWith(followStats: updatedStats);
    }
  }

  /// Clear error
  void clearError() {
    state = state.copyWith(error: null);
  }
}
