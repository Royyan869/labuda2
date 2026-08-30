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
}
