import 'package:flutter/foundation.dart';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:google_sign_in/google_sign_in.dart';
import 'package:labuda/core/core.dart';

/// Google Authentication Repository - Handles Google sign-in operations
///
/// 🔒 DETERMINISTIC FLOW: Only creates Firebase identity.
/// Does NOT create domain AuthUser - that comes from backend via /users/me.
///
/// MIGRATION NOTES:
/// - Firestore writes removed (auth_users, profiles, reserved_usernames)
/// - User data now synced to Go backend via UserSyncService
/// - Firebase Auth retained for authentication only
class AuthGoogleRepository {
  final FirebaseAuth _firebaseAuth;
  final GoogleSignIn? _googleSignIn;

  AuthGoogleRepository({FirebaseAuth? firebaseAuth, GoogleSignIn? googleSignIn})
    : _firebaseAuth = firebaseAuth ?? FirebaseAuth.instance,
      _googleSignIn =
          googleSignIn ??
          GoogleSignIn(
            scopes: ['email', 'openid'],
            signInOption: SignInOption.standard,
          );

  Future<Result<void>> signInWithGoogle() async {
    try {
      if (_googleSignIn == null) {
        return Result.error('Google Sign In not available');
      }

      // PENTING: Sign out dulu untuk memaksa pilihan akun muncul
      await _googleSignIn.signOut();

      GoogleSignInAccount? googleUser;

      if (kIsWeb) {
        // WEB: Langsung gunakan signIn() untuk menampilkan popup pilih akun
        try {
          googleUser = await _googleSignIn.signIn();
        } catch (e) {
          // If sign-in fails on web, provide helpful message
          if (e.toString().contains('popup')) {
            return Result.error(
              'Google Sign-In popup blocked. Allow popups or use email/password.',
            );
          }
          rethrow;
        }
      } else {
        // MOBILE/DESKTOP: Use regular sign-in dengan opsi pilih akun
        googleUser = await _googleSignIn.signIn();
      }

      if (googleUser == null) {
        return Result.error('Google Sign In cancelled');
      }

      final GoogleSignInAuthentication googleAuth =
          await googleUser.authentication;

      final credential = GoogleAuthProvider.credential(
        accessToken: googleAuth.accessToken,
        idToken: googleAuth.idToken,
      );

      final userCredential = await _firebaseAuth.signInWithCredential(
        credential,
      );

      if (userCredential.user != null) {
        // 🔒 DETERMINISTIC: Return void - AuthUser comes from backend via /users/me
        return Result.success(null);
      } else {
        return Result.error('Google Sign In failed: User not found');
      }
    } on FirebaseAuthException catch (e) {
      return Result.error(_mapFirebaseError(e));
    } catch (e) {
      return Result.error(_mapGoogleSignInError(e));
    }
  }

  /// Map Google Sign-In errors to user-friendly messages
  /// Error code 10: Usually means configuration issue
  String _mapGoogleSignInError(dynamic e) {
    final errorStr = e.toString();

    // Google Sign-In ApiException error codes
    if (errorStr.contains('ApiException: 10') || errorStr.contains('10:')) {
      return 'Gagal masuk dengan Google. Silakan coba lagi atau gunakan email/password.';
    }

    if (errorStr.contains('ApiException: 4') || errorStr.contains('4:')) {
      return 'Sign In dibatalkan';
    }

    if (errorStr.contains('ApiException: 7') || errorStr.contains('7:')) {
      return 'Update Google Play Services di HP Anda.';
    }

    if (errorStr.contains('network') || errorStr.contains('timeout')) {
      return 'Tidak ada koneksi internet.';
    }

    return 'Gagal masuk dengan Google. Coba lagi nanti.';
  }

  Future<Result<void>> signOutGoogle() async {
    try {
      if (_googleSignIn != null) {
        await _googleSignIn.signOut();
      }
      return Result.success(null);
    } catch (e) {
      return Result.error('Google Sign Out failed: ${e.toString()}');
    }
  }

  /// Map Firebase Auth errors to user-friendly English messages
  String _mapFirebaseError(FirebaseAuthException e) {
    switch (e.code) {
      case 'account-exists-with-different-credential':
        return 'Account already registered with different login method';
      case 'invalid-credential':
        return 'Invalid Google credentials';
      case 'operation-not-allowed':
        return 'Google Sign-In not enabled in Firebase Console';
      case 'user-disabled':
        return 'Your account has been disabled';
      default:
        return 'Error occurred: ${e.message}';
    }
  }
}
