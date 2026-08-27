import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/firebase_principal.dart';

abstract class IAuthenticationService {
  Future<Result<FirebasePrincipal>> signInWithEmail(
    String email,
    String password,
  );
  Future<Result<FirebasePrincipal>> signUpWithEmail(
    String email,
    String password,
  );
  Future<Result<FirebasePrincipal>> signInWithGoogle();
  Future<Result<FirebasePrincipal>> signInWithApple();
  Future<Result<void>> signOut();
  Future<Result<void>> sendPasswordResetEmail(String email);
  Future<Result<void>> sendEmailVerification();
  Future<Result<FirebasePrincipal>> verifyEmail(String code);
  Future<Result<void>> updatePassword(
    String currentPassword,
    String newPassword,
  );
  Future<Result<void>> deleteAccount();
  Future<Result<FirebasePrincipal?>> getCurrentUser();
  Future<Result<String>> getIdToken();
  Future<Result<void>> refreshToken();
  Stream<FirebasePrincipal?> get authStateChanges;
  bool get isSignedIn;
}
