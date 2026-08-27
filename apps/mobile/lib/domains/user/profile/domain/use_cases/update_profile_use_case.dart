import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_profile_repository.dart';

/// Use case untuk update profil user
///
/// Mengikuti GUIDELINES.md:
/// - Business logic validation
/// - Single responsibility principle
/// - `Result<T>` pattern untuk error handling
class UpdateProfileUseCase {
  final IProfileRepository _repository;
  final IValidationService _validation;

  UpdateProfileUseCase(this._repository, this._validation);

  /// Update profile dengan validasi business logic
  Future<Result<ProfileEntity>> call(ProfileEntity profile) async {
    // Business logic validation
    final validationResult = await _validateProfile(profile);
    if (validationResult.isError) {
      return validationResult;
    }

    try {
      return await _repository.updateProfile(profile);
    } catch (e) {
      return Result.error('Failed to update profile: ${e.toString()}');
    }
  }

  Future<Result<ProfileEntity>> _validateProfile(ProfileEntity profile) async {
    // Location validation
    if (profile.location != null && profile.location!.length > 100) {
      return Result.error('Location max 100 characters');
    }

    if (profile.coverPhotoUrl != null) {
      final coverUrlValidation = await _validation.validateUrl(
        profile.coverPhotoUrl!,
      );
      if (coverUrlValidation.isError || !coverUrlValidation.data!) {
        return Result.error('Invalid cover photo URL');
      }
    }

    // Farm info validation for sellers
    // TODO: UserType check now requires AuthUser.role - need to pass UserRole from caller
    // For now, validate farm info if it exists (regardless of user type)
    if (profile.farmInfo != null) {
      final farmValidation = await _validateFarmInfo(profile.farmInfo!);
      if (farmValidation.isError) {
        return Result.error(farmValidation.error!);
      }
    }

    return Result.success(profile);
  }

  /// Validate farm info for seller profiles
  Future<Result<void>> _validateFarmInfo(FarmInfo farmInfo) async {
    if (farmInfo.farmName.isEmpty) {
      return Result.error('Farm name cannot be empty');
    }

    if (farmInfo.farmName.length > 100) {
      return Result.error('Farm name max 100 characters');
    }

    // ✅ businessEmail and businessPhone removed (use AuthUser instead)

    if (farmInfo.farmWebsite != null) {
      final websiteValidation = await _validation.validateUrl(
        farmInfo.farmWebsite!,
      );
      if (websiteValidation.isError || !websiteValidation.data!) {
        return Result.error('Invalid farm website URL');
      }
    }

    return Result.success(null);
  }
}
