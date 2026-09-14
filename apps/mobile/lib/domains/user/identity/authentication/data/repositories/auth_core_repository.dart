import 'package:firebase_auth/firebase_auth.dart';
import 'package:labuda/core/core.dart';

import '../../domain/entities/firebase_principal.dart';

/// Core Authentication Repository - Handles basic auth operations.
///
/// Firebase Auth remains identity-only. Backend account hydration is handled
/// separately by the controller.
class AuthCoreRepository {
  final FirebaseAuth _firebaseAuth;

  AuthCoreRepository({
    FirebaseAuth? firebaseAuth,
  }) : _firebaseAuth = firebaseAuth ?? FirebaseAuth.instance;

  Future<Result<FirebasePrincipal>> signInWithEmail({
    required String email,
    required String password,
  }) async {
    try {
      final credential = await _firebaseAuth.signInWithEmailAndPassword(
        email: email,
        password: password,
      );

      if (credential.user != null) {
        return Result.success(
          FirebasePrincipal.fromFirebaseUser(credential.user!),
        );
      }
      return Result.error('Sign in failed: User not found');
    } on FirebaseAuthException catch (e) {
      return Result.error(_mapFirebaseError(e));
    } catch (e) {
      return Result.error('Error occurred: ${e.toString()}');
    }
  }

  /// Firebase-only sign-out. Local Labuda credential termination is owned
  /// exclusively by [AuthController.signOut]/[signOutAll] via
  /// `clearLabudaCredential()`. This method must NOT call `clear()` or
  /// `clearSecure()` (broad storage wipe).
  Future<Result<void>> signOut() async {
    try {
      await _firebaseAuth.signOut();
      return Result.success(null);
    } catch (e) {
      return Result.error('Sign out failed: ${e.toString()}');
    }
  }

  /// Map Firebase Auth errors to user-friendly English messages.
  String _mapFirebaseError(FirebaseAuthException e) {
    switch (e.code) {
      case 'user-not-found':
        return 'Email not registered';
      case 'wrong-password':
        return 'Wrong password';
      case 'email-already-in-use':
        return 'Email already used';
      case 'weak-password':
        return 'Password too weak';
      case 'invalid-email':
        return 'Invalid email format';
      case 'requires-recent-login':
        return 'Please sign in again to make this change';
      case 'user-disabled':
        return 'Account disabled';
      case 'too-many-requests':
        return 'Too many attempts, try again later';
      case 'operation-not-allowed':
        return 'Operation not allowed';
      default:
        return 'Error occurred: ${e.message}';
    }
  }
}
