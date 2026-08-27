import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';

abstract interface class IFollowRepository {
  Future<Result<bool>> followUser({
    required String followerId,
    required String followingId,
  });

  Future<Result<bool>> unfollowUser({
    required String followerId,
    required String followingId,
  });

  Future<Result<bool>> blockUser({
    required String userId,
    required String targetUserId,
  });

  Future<Result<bool>> unblockUser({
    required String userId,
    required String targetUserId,
  });

  Future<Result<bool>> muteUser({
    required String userId,
    required String targetUserId,
  });

  Future<Result<bool>> unmuteUser({
    required String userId,
    required String targetUserId,
  });

  Future<Result<List<FollowableUser>>> getFollowers({
    required String userId,
    int limit = 20,
    String? lastFollowId,
  });

  Future<Result<List<FollowableUser>>> getFollowing({
    required String userId,
    int limit = 20,
    String? lastFollowId,
  });

  Future<Result<FollowStats>> getFollowStats({
    required String userId,
    String? currentUserId,
  });

  Future<Result<List<FollowableUser>>> searchUsers({
    required String query,
    String? currentUserId,
    UserType? filterByType,
    int limit = 20,
  });

  Future<Result<bool>> checkFollowStatus({
    required String followerId,
    required String followingId,
  });

  Stream<List<FollowableUser>> watchFollowers(String userId);

  Stream<List<FollowableUser>> watchFollowing(String userId);

  Stream<FollowStats> watchFollowStats(String userId);

  Stream<List<FollowActivity>> watchFollowActivities(String userId);
}
