import 'package:dartz/dartz.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';

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

  Future<Either<Failure, List<FollowableUser>>> execute(
    GetFollowersParams params,
  ) async {
    if (params.limit <= 0 || params.limit > 100) {
      return const Left(ValidationFailure(message: 'Limit harus antara 1-100'));
    }

    final result = await _repository.getFollowers(
      userId: params.userId,
      limit: params.limit,
      lastFollowId: params.lastFollowId,
    );

    return result.fold(
      (error) => Left(UnknownFailure(message: error)),
      (data) => Right(data),
    );
  }
}
