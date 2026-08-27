import 'package:firebase_auth/firebase_auth.dart';
import 'package:labuda/core/core.dart';
import '../../domain/entities/firebase_principal.dart';

/// Firebase Auth Password Service - Handles password and email operations
class FirebaseAuthPasswordService {
  final FirebaseAuth _firebaseAuth;

  FirebaseAuthPasswordService({FirebaseAuth? firebaseAuth})
    : _firebaseAuth = firebaseAuth ?? FirebaseAuth.instance;

  Future<Result<void>> sendPasswordResetEmail(String email) async {
    try {
      await _firebaseAuth.sendPasswordResetEmail(email: email);
      return Result.success(null);
    } on FirebaseAuthException catch (e) {
      return Result.error(_getFirebaseErrorMessage(e));
    } catch (e) {
      return Result.error(
        'Failed to send password reset email: ${e.toString()}',
      );
    }
  }

  Future<Result<void>> sendEmailVerification() async {
    try {
      final user = _firebaseAuth.currentUser;
      if (user != null && !user.emailVerified) {
        await user.sendEmailVerification();
        return Result.success(null);
      } else if (user == null) {
        return Result.error('No user signed in');
      } else {
        return Result.error('Email is already verified');
      }
    } catch (e) {
      return Result.error('Failed to send verification email: ${e.toString()}');
    }
  }

  Future<Result<FirebasePrincipal>> verifyEmail(String code) async {
    try {
      await _firebaseAuth.checkActionCode(code);
      await _firebaseAuth.applyActionCode(code);

      // Reload user to get updated email verification status
      await _firebaseAuth.currentUser?.reload();
      final user = _firebaseAuth.currentUser;

      if (user != null) {
        return Result.success(FirebasePrincipal.fromFirebaseUser(user));
      } else {
        return Result.error('Email verification failed: No user data');
      }
    } catch (e) {
      return Result.error('Email verification failed: ${e.toString()}');
    }
  }

  Future<Result<void>> updatePassword(
    String currentPassword,
    String newPassword,
  ) async {
    try {
      final user = _firebaseAuth.currentUser;
      if (user == null) {
        return Result.error('No user signed in');
      }

      // Re-authenticate user with current password
      final credential = EmailAuthProvider.credential(
        email: user.email!,
        password: currentPassword,
      );

      await user.reauthenticateWithCredential(credential);

      // Update password
      await user.updatePassword(newPassword);

      return Result.success(null);
    } on FirebaseAuthException catch (e) {
      return Result.error(_getFirebaseErrorMessage(e));
    } catch (e) {
      return Result.error('Failed to update password: ${e.toString()}');
    }
  }

  String _getFirebaseErrorMessage(FirebaseAuthException e) {
    switch (e.code) {
      case 'user-not-found':
        return 'Akun dengan email ini tidak ditemukan';
      case 'invalid-email':
        return 'Format email tidak valid';
      case 'weak-password':
        return 'Password terlalu lemah';
      case 'wrong-password':
        return 'Password salah';
      case 'requires-recent-login':
        return 'Silakan login ulang untuk melakukan perubahan ini';
      default:
        return 'Error: ${e.message ?? e.code}';
    }
  }
}
