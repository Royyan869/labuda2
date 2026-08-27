import 'package:labuda/core/core.dart';

/// Auth Sign Up Service - Handles sign up operations
///
/// 🔒 DETERMINISTIC FLOW: This service ONLY handles Firebase authentication.
/// Does NOT return domain AuthUser - that comes from backend via /users/me.
/// Backend sync is handled by AuthController's Firebase auth state listener
/// with mutex protection to prevent duplicate sync calls.
class AuthSignUpService {
  final IAuthRepository _authRepository;
  final ILoggerService _logger;

  AuthSignUpService({
    required IAuthRepository authRepository,
    required ILoggerService logger,
  }) : _authRepository = authRepository,
       _logger = logger;

  /// Sign up dengan email dan password
  ///
  /// 🔒 DETERMINISTIC: Only creates Firebase identity.
  /// Returns void - AuthUser domain entity comes from backend via /users/me.
  Future<Result<void>> signUpWithEmail({
    required String email,
    required String password,
    required String username,
  }) async {
    try {
      await _logger.info(
        'User attempting sign up',
        extra: {'email': email, 'username': username},
      );

      final result = await _authRepository.signUpWithEmail(
        email: email,
        password: password,
        username: username,
      );

      if (result.isError) {
        await _logger.error(
          'Firebase sign up failed',
          extra: {'error': result.error},
        );
        return Result.error(result.error!);
      }

      await _logger.info('Firebase sign up successful');
      return Result.success(null);
    } catch (e, stackTrace) {
      await _logger.error(
        'Sign up exception',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Terjadi kesalahan. Coba lagi.');
    }
  }
}
