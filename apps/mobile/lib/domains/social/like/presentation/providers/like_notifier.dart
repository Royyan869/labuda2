import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'like_state.dart';
import '../../data/remote/like_api_datasource.dart';
import '../../data/like_repository_impl.dart';

// =============================================================================
// DEPENDENCY PROVIDERS - imported from core_providers
// =============================================================================

/// Provider untuk LikeApiDatasource
final likeApiDatasourceProvider = Provider<LikeApiDatasource>((ref) {
  return LikeApiDatasource(
    ref.watch(apiClientProvider),
    logger: ref.watch(loggerServiceProvider),
  );
});

/// Provider untuk LikeRepository
final likeRepositoryProvider = Provider<LikeRepository>((ref) {
  return LikeRepositoryImpl(
    ref.watch(likeApiDatasourceProvider),
    logger: ref.watch(loggerServiceProvider),
  );
});

// =============================================================================
// NOTIFIER
// =============================================================================

/// Notifier for Like feature
///
/// Handles all like operations using Riverpod + Repository pattern.
/// Logic from UseCases moved here - no separate UseCase classes.
class LikeNotifier extends Notifier<LikeState> {
  late final LikeRepository _repository;

  @override
  LikeState build() {
    _repository = ref.watch(likeRepositoryProvider);
    return const LikeInitial();
  }

  // ===========================================
  // TOGGLE LIKE
  // ===========================================

  /// Toggle like for a target
  Future<Result<bool>> toggleLike({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
    String? likerName,
    String? targetOwnerId,
  }) async {
    // Input validation (was in UseCase, now here)
    if (targetId.isEmpty) {
      return Result.error('Target ID cannot be empty');
    }

    if (userId.isEmpty) {
      return Result.error('User ID cannot be empty');
    }

    // Pass the repository Result through verbatim so the error code
    // (e.g. EMAIL_VERIFICATION_REQUIRED) reaches the call site for the
    // blocked-action gate. Notification side-effects move out of the
    // notifier; the like itself is the only mutation that needs an ack.
    return _repository.toggleLike(
      targetId: targetId,
      targetType: targetType,
      userId: userId,
    );
  }

  // ===========================================
  // GET LIKE STATS
  // ===========================================

  /// Get like statistics for a target
  Future<Result<LikeStats>> getLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async {
    // Input validation
    if (targetId.isEmpty) {
      return Result.error('Target ID cannot be empty');
    }

    if (currentUserId.isEmpty) {
      return Result.error('Current user ID cannot be empty');
    }

    return await _repository.getLikeStats(
      targetId: targetId,
      targetType: targetType,
      currentUserId: currentUserId,
    );
  }

  /// Watch like stats in real-time
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) {
    return _repository.watchLikeStats(
      targetId: targetId,
      targetType: targetType,
      currentUserId: currentUserId,
    );
  }
}

// =============================================================================
// NOTIFIER PROVIDER
// =============================================================================

/// Provider for LikeNotifier
final likeNotifierProvider = NotifierProvider<LikeNotifier, LikeState>(
  LikeNotifier.new,
);

// =============================================================================
// ADDITIONAL PROVIDERS
// =============================================================================

/// Parameters for like stats
class LikeStatsParams {
  final String targetId;
  final LikeTargetType targetType;
  final String currentUserId;

  const LikeStatsParams({
    required this.targetId,
    required this.targetType,
    required this.currentUserId,
  });

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is LikeStatsParams &&
        other.targetId == targetId &&
        other.targetType == targetType &&
        other.currentUserId == currentUserId;
  }

  @override
  int get hashCode => Object.hash(targetId, targetType, currentUserId);
}

/// Provider for specific like stats (real-time stream)
final likeStatsProvider = StreamProvider.family<LikeStats, LikeStatsParams>((
  ref,
  params,
) {
  final repository = ref.watch(likeRepositoryProvider);
  return repository.watchLikeStats(
    targetId: params.targetId,
    targetType: params.targetType,
    currentUserId: params.currentUserId,
  );
});
