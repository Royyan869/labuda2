import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';

class GetFollowStatsParams {
  final String userId;
  final String? currentUserId;

  const GetFollowStatsParams({required this.userId, this.currentUserId});
}

class GetFollowStatsUseCase {
  final IFollowRepository _repository;

  const GetFollowStatsUseCase(this._repository);

  Future<Result<FollowStats>> execute(GetFollowStatsParams params) async {
    final result = await _repository.getFollowStats(
      userId: params.userId,
      currentUserId: params.currentUserId,
    );

    return result;
  }
}
