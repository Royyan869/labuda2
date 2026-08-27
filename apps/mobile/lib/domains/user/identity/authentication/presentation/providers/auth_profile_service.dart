import 'package:labuda/core/core.dart';
import '../../domain/entities/auth_user.dart' as domain;
import '../../domain/entities/user_profile_patch.dart';

/// Auth Profile Service - Handles profile operations
class AuthProfileService {
  final IAuthRepository _authRepository;
  final ILoggerService _logger;

  AuthProfileService({
    required IAuthRepository authRepository,
    required ILoggerService logger,
  }) : _authRepository = authRepository,
       _logger = logger;

  /// Change email for current user
  Future<Result<void>> changeEmail({
    required String newEmail,
    required String currentPassword,
  }) async {
    try {
      await _logger.info(
        'User attempting email change',
        extra: {'newEmail': newEmail},
      );

      final result = await _authRepository.changeEmail(
        newEmail: newEmail,
        currentPassword: currentPassword,
      );

      if (result.isSuccess) {
        await _logger.info('Email change successful');
        return Result.success(null);
      } else {
        await _logger.error(
          'Email change failed',
          extra: {'error': result.error},
        );
        return Result.error(result.error!);
      }
    } catch (e, stackTrace) {
      await _logger.error(
        'Email change exception',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Terjadi kesalahan. Coba lagi.');
    }
  }

  /// Change password for current user
  Future<Result<void>> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async {
    try {
      await _logger.info('User attempting password change');

      final result = await _authRepository.changePassword(
        currentPassword: currentPassword,
        newPassword: newPassword,
      );

      if (result.isSuccess) {
        await _logger.info('Password change successful');
        return Result.success(null);
      } else {
        await _logger.error(
          'Password change failed',
          extra: {'error': result.error},
        );
        return Result.error(result.error!);
      }
    } catch (e, stackTrace) {
      await _logger.error(
        'Password change exception',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Terjadi kesalahan. Coba lagi.');
    }
  }

  /// Send email verification
  Future<Result<void>> sendEmailVerification() async {
    try {
      await _logger.info('Sending email verification');

      final result = await _authRepository.sendEmailVerification();

      if (result.isSuccess) {
        await _logger.info('Email verification sent successfully');
        return Result.success(null);
      } else {
        await _logger.error(
          'Send email verification failed',
          extra: {'error': result.error},
        );
        return Result.error(result.error!);
      }
    } catch (e, stackTrace) {
      await _logger.error(
        'Send email verification exception',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Terjadi kesalahan. Coba lagi.');
    }
  }

  /// Update user profile
  Future<Result<UserProfilePatch>> updateProfile({
    String? photoUrl,
    String? username,
    String? bio,
    String? phoneNumber,
    String? location,
    DateTime? phoneVerifiedAt,
    DateTime? dateOfBirth,
  }) async {
    try {
      await _logger.info('Updating user profile');

      final result = await _authRepository.updateProfile(
        photoUrl: photoUrl,
        username: username,
        bio: bio,
        phoneNumber: phoneNumber,
        location: location,
        phoneVerifiedAt: phoneVerifiedAt,
        dateOfBirth: dateOfBirth,
      );

      if (result.isSuccess && result.data != null) {
        await _logger.info('Profile updated successfully');
        return Result.success(result.data!);
      } else {
        await _logger.error(
          'Profile update failed',
          extra: {'error': result.error},
        );
        return Result.error(result.error!);
      }
    } catch (e, stackTrace) {
      await _logger.error(
        'Profile update exception',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Terjadi kesalahan. Coba lagi.');
    }
  }

  /// Complete profile after restricted Firebase exchange.
  Future<Result<domain.AuthUser>> completeProfile({
    required String username,
  }) async {
    try {
      await _logger.info('Completing user profile');

      final result = await _authRepository.completeProfile(username: username);

      if (result.isSuccess && result.data != null) {
        await _logger.info('Profile completion successful');
        return Result.success(result.data!);
      } else {
        await _logger.error(
          'Profile completion failed',
          extra: {'error': result.error},
        );
        return Result.error(
          result.error ?? 'Failed to complete profile',
          code: result.errorCode,
          statusCode: result.statusCode,
          details: result.errorDetails,
        );
      }
    } catch (e, stackTrace) {
      await _logger.error(
        'Profile completion exception',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Terjadi kesalahan. Coba lagi.');
    }
  }
}
