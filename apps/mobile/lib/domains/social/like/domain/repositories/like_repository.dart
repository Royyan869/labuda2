import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';

/// Repository interface for Like operations
abstract class LikeRepository {
  /// Toggle like for a target (like/unlike)
  Future<Result<bool>> toggleLike({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  });

  /// Get like statistics for a target
  Future<Result<LikeStats>> getLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  });

  /// Watch like stats changes in real-time
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  });

  /// Push optimistic stats to active stream (0ms UX).
  /// No-op if no active watcher for [targetId]/[targetType].
  void pushOptimisticLikeStats(LikeStats stats);

  /// Re-fetch authoritative stats from backend and push to stream.
  Future<void> refreshLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  });
}
