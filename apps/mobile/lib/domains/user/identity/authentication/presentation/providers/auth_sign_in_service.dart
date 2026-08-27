import 'package:labuda/core/core.dart';
import '../../domain/entities/firebase_principal.dart';

/// Auth Sign In Service - Handles sign in operations
///
/// 🔒 DETERMINISTIC FLOW: This service ONLY handles Firebase authentication.
/// Backend sync is handled by AuthController's Firebase auth state listener
/// with mutex protection to prevent duplicate sync calls.
class AuthSignInService {
  final IAuthRepository _authRepository;
  final ILoggerService _logger;

  AuthSignInService({
    required IAuthRepository authRepository,
    required ILoggerService logger,
  }) : _authRepository = authRepository,
       _logger = logger;

  /// Sign in dengan email dan password
  ///
  /// Only handles Firebase auth. Backend sync is handled by
  /// AuthController's Firebase auth state listener.
  Future<Result<FirebasePrincipal>> signInWithEmail({
    required String email,
    required String password,
  }) async {
    try {
      await _logger.info('User attempting sign in', extra: {'email': email});

      final result = await _authRepository.signInWithEmail(
        email: email,
        password: password,
      );

      if (result.isSuccess) {
        final user = result.data!;
        await _logger.info('Sign in successful', extra: {'userId': user.id});
        return Result.success(user);
      } else {
        await _logger.error('Sign in failed', extra: {'error': result.error});
        return Result.error(result.error!);
      }
    } catch (e, stackTrace) {
      await _logger.error(
        'Sign in exception',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Terjadi kesalahan. Coba lagi.');
    }
  }

  /// Sign in dengan Google
  ///
  /// 🔒 DETERMINISTIC: Only creates Firebase identity.
  /// Returns void - AuthUser domain entity comes from backend via /users/me.
  Future<Result<void>> signInWithGoogle() async {
    try {
      await _logger.info('User attempting Google sign in');

      final result = await _authRepository.signInWithGoogle();

      if (result.isSuccess) {
        await _logger.info('Google sign in successful');
        return Result.success(null);
      } else {
        await _logger.error(
          'Google sign in failed',
          extra: {'error': result.error},
        );
        return Result.error(result.error!);
      }
    } catch (e, stackTrace) {
      await _logger.error(
        'Google sign in exception',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Terjadi kesalahan. Coba lagi.');
    }
  }

  /// Sign out user
  Future<Result<void>> signOut() async {
    try {
      await _logger.info('User signing out');

      final result = await _authRepository.signOut();

      if (result.isSuccess) {
        await _logger.info('Sign out successful');
        return Result.success(null);
      } else {
        await _logger.error('Sign out failed', extra: {'error': result.error});
        return Result.error(result.error!);
      }
    } catch (e, stackTrace) {
      await _logger.error(
        'Sign out exception',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error('Terjadi kesalahan. Coba lagi.');
    }
  }
}
