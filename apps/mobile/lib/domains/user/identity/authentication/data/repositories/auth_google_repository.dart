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

  /// Normal Google sign-in, or — when [pendingCredential] is provided — a
  /// D1 LINK into the CURRENT Firebase identity.
  ///
  /// Flow (design scope v2): Google sign-in that hits
  /// `account-exists-with-different-credential` parks the Google credential
  /// with the controller (pending-verification intent). After the email
  /// identity is verified, the controller calls this method again with that
  /// credential: `linkWithCredential` unifies BOTH providers under ONE
  /// Firebase UID (one Firebase user per human), so the backend never sees
  /// two UIDs claiming one email. Link failures surface explicitly — there
  /// is no silent retry and no fallback sign-in.
  Future<Result<void>> signInWithGoogle({AuthCredential? pendingCredential}) async {
    if (pendingCredential != null) {
      return _linkGoogleCredential(pendingCredential);
    }
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

  /// D1: link a Google credential into the CURRENT Firebase identity.
  /// No Google picker is shown — the credential was already consented.
  Future<Result<void>> _linkGoogleCredential(AuthCredential credential) async {
    try {
      final currentUser = _firebaseAuth.currentUser;
      if (currentUser == null) {
        return Result.error(
          'Sesi tidak ditemukan. Masuk dengan email dulu, lalu coba lagi.',
        );
      }
      await currentUser.linkWithCredential(credential);
      return Result.success(null);
    } on FirebaseAuthException catch (e) {
      if (e.code == 'credential-already-in-use') {
        // The Google identity is already bound to another Firebase user.
        // Under D4 this is a canonical anomaly: do NOT switch accounts
        // silently — surface it and let the user decide.
        return Result.error(
          'Akun Google ini sudah terhubung ke akun Labuda lain. '
          'Masuk dengan metode semula.',
        );
      }
      if (e.code == 'requires-recent-login') {
        return Result.error(
          'Sesi terlalu lama. Masuk ulang dengan email, lalu coba tautkan '
          'Google lagi.',
        );
      }
      return Result.error(_mapFirebaseError(e));
    } catch (e) {
      return Result.error('Gagal menautkan akun Google: ${e.toString()}');
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

  /// Map Firebase Auth errors to user-friendly messages
  String _mapFirebaseError(FirebaseAuthException e) {
    switch (e.code) {
      case 'account-exists-with-different-credential':
        return 'Akun ini terdaftar dengan metode masuk berbeda. Masuk dengan '
            'metode semula, lalu tautkan Google setelah verifikasi email.';
      case 'invalid-credential':
        return 'Kredensial Google tidak valid atau sudah kedaluwarsa';
      case 'operation-not-allowed':
        return 'Google Sign-In belum diaktifkan di Firebase Console';
      case 'user-disabled':
        return 'Akun kamu telah dinonaktifkan';
      default:
        return 'Terjadi kesalahan: ${e.message}';
    }
  }
}
