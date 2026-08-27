import 'dart:async';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:google_sign_in/google_sign_in.dart';
import 'package:labuda/core/core.dart';
import '../../domain/entities/firebase_principal.dart';
import 'firebase_auth_core_service.dart';
import 'firebase_auth_social_service.dart';
import 'firebase_auth_password_service.dart';
import 'firebase_auth_token_service.dart';

/// Firebase Authentication Service - Delegating to specialized services
class FirebaseAuthenticationService implements IAuthenticationService {
  final FirebaseAuthCoreService _coreService;
  final FirebaseAuthSocialService _socialService;
  final FirebaseAuthPasswordService _passwordService;
  final FirebaseAuthTokenService _tokenService;

  FirebaseAuthenticationService({
    FirebaseAuth? firebaseAuth,
    GoogleSignIn? googleSignIn,
  }) : _coreService = FirebaseAuthCoreService(firebaseAuth: firebaseAuth),
       _socialService = FirebaseAuthSocialService(
         firebaseAuth: firebaseAuth,
         googleSignIn: googleSignIn,
       ),
       _passwordService = FirebaseAuthPasswordService(
         firebaseAuth: firebaseAuth,
       ),
       _tokenService = FirebaseAuthTokenService(firebaseAuth: firebaseAuth);

  @override
  Future<Result<FirebasePrincipal>> signInWithEmail(
    String email,
    String password,
  ) async {
    return _coreService.signInWithEmail(email, password);
  }

  @override
  Future<Result<FirebasePrincipal>> signUpWithEmail(
    String email,
    String password,
  ) async {
    return _coreService.signUpWithEmail(email, password);
  }

  @override
  Future<Result<FirebasePrincipal>> signInWithGoogle() async {
    return _socialService.signInWithGoogle();
  }

  @override
  Future<Result<FirebasePrincipal>> signInWithApple() async {
    return _socialService.signInWithApple();
  }

  @override
  Future<Result<void>> signOut() async {
    final results = await Future.wait([
      _coreService.signOut(),
      _socialService.signOutGoogle(),
    ]);

    // Return error if any service fails
    for (final result in results) {
      if (result.isFailure) {
        return result;
      }
    }

    return Result.success(null);
  }

  @override
  Future<Result<void>> sendPasswordResetEmail(String email) async {
    return _passwordService.sendPasswordResetEmail(email);
  }

  @override
  Future<Result<void>> sendEmailVerification() async {
    return _passwordService.sendEmailVerification();
  }

  @override
  Future<Result<FirebasePrincipal>> verifyEmail(String code) async {
    return _passwordService.verifyEmail(code);
  }

  @override
  Future<Result<void>> updatePassword(
    String currentPassword,
    String newPassword,
  ) async {
    return _passwordService.updatePassword(currentPassword, newPassword);
  }

  @override
  Future<Result<void>> deleteAccount() async {
    return _tokenService.deleteAccount();
  }

  @override
  Future<Result<FirebasePrincipal?>> getCurrentUser() async {
    return _coreService.getCurrentUser();
  }

  @override
  Future<Result<String>> getIdToken() async {
    return _tokenService.getIdToken();
  }

  @override
  Future<Result<void>> refreshToken() async {
    return _tokenService.refreshToken();
  }

  @override
  Stream<FirebasePrincipal?> get authStateChanges =>
      _coreService.authStateChanges;

  @override
  bool get isSignedIn => _coreService.isSignedIn;

  void dispose() {
    _coreService.dispose();
  }
}
