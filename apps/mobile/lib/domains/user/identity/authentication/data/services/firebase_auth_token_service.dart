import 'package:firebase_auth/firebase_auth.dart';
import 'package:labuda/core/core.dart';

/// Firebase Auth Token Service - Handles token operations
class FirebaseAuthTokenService {
  final FirebaseAuth _firebaseAuth;

  FirebaseAuthTokenService({FirebaseAuth? firebaseAuth})
    : _firebaseAuth = firebaseAuth ?? FirebaseAuth.instance;

  Future<Result<String>> getIdToken() async {
    try {
      final user = _firebaseAuth.currentUser;
      if (user != null) {
        final token = await user.getIdToken();
        if (token != null) {
          return Result.success(token);
        } else {
          return Result.error('Failed to get ID token');
        }
      } else {
        return Result.error('No user signed in');
      }
    } catch (e) {
      return Result.error('Failed to get ID token: ${e.toString()}');
    }
  }

  Future<Result<void>> refreshToken() async {
    try {
      final user = _firebaseAuth.currentUser;
      if (user != null) {
        await user.getIdToken(true); // Force refresh
        return Result.success(null);
      } else {
        return Result.error('No user signed in');
      }
    } catch (e) {
      return Result.error('Failed to refresh token: ${e.toString()}');
    }
  }

  Future<Result<void>> deleteAccount() async {
    try {
      final user = _firebaseAuth.currentUser;
      if (user != null) {
        await user.delete();
        return Result.success(null);
      } else {
        return Result.error('No user signed in');
      }
    } catch (e) {
      return Result.error('Failed to delete account: ${e.toString()}');
    }
  }
}
