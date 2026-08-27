import 'dart:async';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:labuda/core/core.dart';
import '../../domain/entities/firebase_principal.dart';

/// Firebase Auth Core Service - Handles basic auth operations
class FirebaseAuthCoreService {
  final FirebaseAuth _firebaseAuth;
  final StreamController<FirebasePrincipal?> _authStateController;

  FirebaseAuthCoreService({FirebaseAuth? firebaseAuth})
    : _firebaseAuth = firebaseAuth ?? FirebaseAuth.instance,
      _authStateController = StreamController<FirebasePrincipal?>.broadcast() {
    // Listen to Firebase auth state changes
    _firebaseAuth.authStateChanges().listen((User? user) {
      if (user != null) {
        _authStateController.add(FirebasePrincipal.fromFirebaseUser(user));
      } else {
        _authStateController.add(null);
      }
    });
  }

  Future<Result<FirebasePrincipal>> signInWithEmail(
    String email,
    String password,
  ) async {
    try {
      final UserCredential credential = await _firebaseAuth
          .signInWithEmailAndPassword(email: email, password: password);

      if (credential.user != null) {
        return Result.success(
          FirebasePrincipal.fromFirebaseUser(credential.user!),
        );
      } else {
        return Result.error('Sign in failed: No user data received');
      }
    } on FirebaseAuthException catch (e) {
      return Result.error(_getFirebaseErrorMessage(e));
    } catch (e) {
      return Result.error('Sign in failed: ${e.toString()}');
    }
  }

  Future<Result<FirebasePrincipal>> signUpWithEmail(
    String email,
    String password,
  ) async {
    try {
      final UserCredential credential = await _firebaseAuth
          .createUserWithEmailAndPassword(email: email, password: password);

      if (credential.user != null) {
        return Result.success(
          FirebasePrincipal.fromFirebaseUser(credential.user!),
        );
      } else {
        return Result.error('Sign up failed: No user data received');
      }
    } on FirebaseAuthException catch (e) {
      return Result.error(_getFirebaseErrorMessage(e));
    } catch (e) {
      return Result.error('Sign up failed: ${e.toString()}');
    }
  }

  Future<Result<void>> signOut() async {
    try {
      await _firebaseAuth.signOut();
      return Result.success(null);
    } catch (e) {
      return Result.error('Sign out failed: ${e.toString()}');
    }
  }

  Future<Result<FirebasePrincipal?>> getCurrentUser() async {
    try {
      final user = _firebaseAuth.currentUser;
      if (user != null) {
        return Result.success(FirebasePrincipal.fromFirebaseUser(user));
      } else {
        return Result.success(null);
      }
    } catch (e) {
      return Result.error('Failed to get current user: ${e.toString()}');
    }
  }

  Stream<FirebasePrincipal?> get authStateChanges =>
      _authStateController.stream;
  bool get isSignedIn => _firebaseAuth.currentUser != null;

  String _getFirebaseErrorMessage(FirebaseAuthException e) {
    switch (e.code) {
      case 'user-not-found':
        return 'Akun dengan email ini tidak ditemukan';
      case 'wrong-password':
        return 'Password salah';
      case 'email-already-in-use':
        return 'Email sudah digunakan oleh akun lain';
      case 'weak-password':
        return 'Password terlalu lemah';
      case 'invalid-email':
        return 'Format email tidak valid';
      case 'user-disabled':
        return 'Akun ini telah dinonaktifkan';
      case 'too-many-requests':
        return 'Terlalu banyak percobaan login. Coba lagi nanti';
      case 'operation-not-allowed':
        return 'Metode login ini tidak diizinkan';
      case 'network-request-failed':
        return 'Koneksi internet bermasalah';
      default:
        return 'Error: ${e.message ?? e.code}';
    }
  }

  void dispose() {
    _authStateController.close();
  }
}
