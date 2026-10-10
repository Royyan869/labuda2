import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:hishumi/domains/social/follow/domain/repositories/i_follow_repository.dart';

class GetFollowersParams {
  final String userId;
  final int limit;
  final String? lastFollowId;

  const GetFollowersParams({
    required this.userId,
    this.limit = 20,
    this.lastFollowId,
  });
}

class GetFollowersUseCase {
  final IFollowRepository _repository;

  const GetFollowersUseCase(this._repository);

  Future<Result<List<FollowableUser>>> execute(GetFollowersParams params) async {
    if (params.limit <= 0 || params.limit > 100) {
      return Result.error('Limit harus antara 1-100');
    }

    final result = await _repository.getFollowers(
      userId: params.userId,
      limit: params.limit,
      lastFollowId: params.lastFollowId,
    );

    return result;
  }
}
