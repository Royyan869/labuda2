import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:hishumi/domains/social/follow/domain/repositories/i_follow_repository.dart';

class GetFollowingParams {
  final String userId;
  final int limit;
  final String? lastFollowId;

  const GetFollowingParams({
    required this.userId,
    this.limit = 20,
    this.lastFollowId,
  });
}

class GetFollowingUseCase {
  final IFollowRepository _repository;

  const GetFollowingUseCase(this._repository);

  Future<Result<List<FollowableUser>>> execute(GetFollowingParams params) async {
    if (params.limit <= 0 || params.limit > 100) {
      return Result.error('Limit harus antara 1-100');
    }

    final result = await _repository.getFollowing(
      userId: params.userId,
      limit: params.limit,
      lastFollowId: params.lastFollowId,
    );

    return result;
  }
}
