import 'package:dartz/dartz.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';

class SearchUsersParams {
  final String query;
  final String? currentUserId;
  final UserType? filterByType;
  final int limit;

  const SearchUsersParams({
    required this.query,
    this.currentUserId,
    this.filterByType,
    this.limit = 20,
  });
}

class SearchUsersUseCase {
  final IFollowRepository _repository;

  const SearchUsersUseCase(this._repository);

  Future<Either<Failure, List<FollowableUser>>> execute(
    SearchUsersParams params,
  ) async {
    if (params.query.trim().isEmpty) {
      return const Left(
        ValidationFailure(message: 'Query pencarian tidak boleh kosong'),
      );
    }

    if (params.query.trim().length < 2) {
      return const Left(
        ValidationFailure(message: 'Query pencarian minimal 2 karakter'),
      );
    }

    if (params.limit <= 0 || params.limit > 50) {
      return const Left(
        ValidationFailure(message: 'Limit harus antara 1-50 untuk pencarian'),
      );
    }

    final result = await _repository.searchUsers(
      query: params.query.trim(),
      currentUserId: params.currentUserId,
      filterByType: params.filterByType,
      limit: params.limit,
    );

    return result.fold(
      (error) => Left(UnknownFailure(message: error)),
      (data) => Right(data),
    );
  }
}
