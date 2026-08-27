import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_profile_repository.dart';

/// Use case untuk mengambil profil user
///
/// Mengikuti GUIDELINES.md:
/// - Single responsibility principle
/// - Business logic validation
/// - `Result<T>` pattern untuk error handling
class GetProfileUseCase {
  final IProfileRepository _repository;

  GetProfileUseCase(this._repository);

  /// Get profile by user ID
  Future<Result<ProfileEntity?>> call(String userId) async {
    // Input validation
    if (userId.isEmpty) {
      return Result.error('User ID cannot be empty');
    }

    if (userId.length < 3) {
      return Result.error('Invalid User ID');
    }

    try {
      return await _repository.getProfile(userId);
    } catch (e) {
      return Result.error('Failed to fetch profile: ${e.toString()}');
    }
  }

  /// Get profile dengan fallback ke create jika tidak ada
  Future<Result<ProfileEntity>> getOrCreateProfile(
    String userId,
    ProfileEntity defaultProfile,
  ) async {
    final result = await call(userId);

    if (result.isError) {
      return Result.error(result.error!);
    }

    // Jika profil ada, return
    if (result.data != null) {
      return Result.success(result.data!);
    }

    // Jika tidak ada, buat profil baru
    return await _repository.createProfile(defaultProfile);
  }
}
