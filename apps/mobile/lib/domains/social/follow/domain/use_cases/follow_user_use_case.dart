import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/social/follow/domain/repositories/i_follow_repository.dart';

class FollowUserParams {
  final String followerId;
  final String followingId;

  const FollowUserParams({required this.followerId, required this.followingId});
}

class FollowUserUseCase {
  final IFollowRepository _repository;

  const FollowUserUseCase(this._repository);

  Future<Result<bool>> execute(FollowUserParams params) async {
    if (params.followerId == params.followingId) {
      return Result.error('Tidak dapat follow diri sendiri');
    }

    final result = await _repository.followUser(
      followerId: params.followerId,
      followingId: params.followingId,
    );

    // BATCH N2: Notification trigger removed - follow notifications are backend-only.
    // Backend emits user.followed events via outbox pattern, handled by notification_worker.
    // This eliminates the dual-path violation where both backend and Flutter could trigger notifications.
    return result;
  }
}
