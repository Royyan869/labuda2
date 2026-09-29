import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';

class UnfollowUserParams {
  final String followerId;
  final String followingId;

  const UnfollowUserParams({
    required this.followerId,
    required this.followingId,
  });
}

class UnfollowUserUseCase {
  final IFollowRepository _repository;

  const UnfollowUserUseCase(this._repository);

  Future<Result<bool>> execute(UnfollowUserParams params) async {
    if (params.followerId == params.followingId) {
      return Result.error('Tidak dapat unfollow diri sendiri');
    }

    final result = await _repository.unfollowUser(
      followerId: params.followerId,
      followingId: params.followingId,
    );

    return result;
  }
}
