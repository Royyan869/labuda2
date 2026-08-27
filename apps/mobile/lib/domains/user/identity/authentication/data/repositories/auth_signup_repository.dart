import 'package:firebase_auth/firebase_auth.dart';
import 'package:labuda/core/core.dart';
import '../../domain/entities/firebase_principal.dart';

/// Sign Up Repository - Handles user registration operations
///
/// 🔒 DETERMINISTIC FLOW: Only creates Firebase identity.
/// Does NOT create domain AuthUser - that comes from backend via /users/me.
///
/// MIGRATION NOTES:
/// - Firestore writes removed (auth_users, profiles, reserved_usernames)
/// - User data now synced to Go backend via UserSyncService
/// - Firebase Auth retained for authentication only
class AuthSignUpRepository {
  final FirebaseAuth _firebaseAuth;

  AuthSignUpRepository({FirebaseAuth? firebaseAuth})
    : _firebaseAuth = firebaseAuth ?? FirebaseAuth.instance;

  Future<Result<FirebasePrincipal>> signUpWithEmail({
    required String email,
    required String password,
    required String username,
  }) async {
    try {
      final credential = await _firebaseAuth.createUserWithEmailAndPassword(
        email: email,
        password: password,
      );

      if (credential.user != null) {
        await credential.user!.reload();

        final updatedUser = _firebaseAuth.currentUser!;

        // Send email verification
        await updatedUser.sendEmailVerification();

        // EMAIL REGISTER DIAGNOSTIC: User setup complete

        // 🔒 DETERMINISTIC: Return void - AuthUser comes from backend via /users/me
        return Result.success(FirebasePrincipal.fromFirebaseUser(updatedUser));
      } else {
        return Result.error('Sign up failed: User cannot be created');
      }
    } on FirebaseAuthException catch (e) {
      return Result.error(_mapFirebaseError(e));
    } catch (e) {
      // Handle username conflict errors (from Go backend)
      if (e.toString().contains('username') ||
          e.toString().contains('already exists')) {
        return Result.error(
          'Username already used. Please choose another username.',
        );
      }

      // Handle permission errors
      if (e.toString().contains('permission-denied')) {
        return Result.error(
          'No permission to create account. Please contact support.',
        );
      }

      return Result.error(
        'Error occurred while creating account: ${e.toString()}',
      );
    }
  }

  /// Map Firebase Auth errors to user-friendly English messages
  String _mapFirebaseError(FirebaseAuthException e) {
    switch (e.code) {
      case 'email-already-in-use':
        return 'Email already used';
      case 'weak-password':
        return 'Password too weak';
      case 'invalid-email':
        return 'Invalid email format';
      case 'operation-not-allowed':
        return 'Operation not allowed';
      default:
        return 'Error occurred: ${e.message}';
    }
  }
}
