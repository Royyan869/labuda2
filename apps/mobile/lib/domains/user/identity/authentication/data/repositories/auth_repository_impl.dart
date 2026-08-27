import 'package:firebase_auth/firebase_auth.dart';
import 'package:google_sign_in/google_sign_in.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import '../../domain/entities/user_profile_patch.dart';
import '../../domain/entities/firebase_principal.dart';
import '../repositories/auth_core_repository.dart';
import '../repositories/auth_google_repository.dart';
import '../repositories/auth_signup_repository.dart';
import '../repositories/auth_profile_repository.dart';
import '../datasources/auth_api_datasource.dart';

/// Authentication Repository Implementation - Delegating to specialized repositories
///
/// R4.1B DI MIGRATION: Removed ApiDI fallback from constructor
/// Previous implementation had fallback to ApiDI.apiClient and ApiDI.logger
/// which violated canonical DI path. Now requires explicit dependencies.
class AuthRepositoryImpl implements IAuthRepository {
  final AuthCoreRepository _coreRepository;
  final AuthGoogleRepository _googleRepository;
  final AuthSignUpRepository _signUpRepository;
  final AuthProfileRepository _profileRepository;

  AuthRepositoryImpl({
    FirebaseAuth? firebaseAuth,
    GoogleSignIn? googleSignIn,
    ILocalStorageService? localStorage,
    required AuthApiDatasource apiDatasource,
  }) : _coreRepository = AuthCoreRepository(firebaseAuth: firebaseAuth),
       _googleRepository = AuthGoogleRepository(
         firebaseAuth: firebaseAuth,
         googleSignIn: googleSignIn,
       ),
       _signUpRepository = AuthSignUpRepository(firebaseAuth: firebaseAuth),
       _profileRepository = AuthProfileRepository(
         firebaseAuth: firebaseAuth,
         apiDatasource: apiDatasource,
         localStorage: localStorage,
       );

  // Core authentication operations
  @override
  Future<Result<FirebasePrincipal>> signInWithEmail({
    required String email,
    required String password,
  }) => _coreRepository.signInWithEmail(email: email, password: password);

  @override
  Future<Result<void>> signInWithGoogle() =>
      _googleRepository.signInWithGoogle();

  @override
  Future<Result<FirebasePrincipal>> signUpWithEmail({
    required String email,
    required String password,
    required String username,
  }) => _signUpRepository.signUpWithEmail(
    email: email,
    password: password,
    username: username,
  );

  @override
  Future<Result<void>> signOut() async {
    try {
      await Future.wait([
        _coreRepository.signOut(),
        _googleRepository.signOutGoogle(),
      ]);
      return Result.success(null);
    } catch (e) {
      return Result.error('Sign out gagal: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> logoutCurrentSession({
    required String refreshToken,
    String? fcmToken,
    String? deviceId,
  }) {
    return _profileRepository.logoutCurrentSession(
      refreshToken: refreshToken,
      fcmToken: fcmToken,
      deviceId: deviceId,
    );
  }

  @override
  Future<Result<void>> logoutAllSessions({bool deactivateFcmTokens = true}) {
    return _profileRepository.logoutAllSessions(
      deactivateFcmTokens: deactivateFcmTokens,
    );
  }

  @override
  Future<Result<List<AuthSessionDto>>> getActiveSessions() {
    return _profileRepository.getActiveSessions();
  }

  @override
  Future<Result<void>> revokeSession(String familyId) {
    return _profileRepository.revokeSession(familyId);
  }

  @override
  Future<Result<AuthUser?>> getCurrentUser() =>
      _coreRepository.getCurrentUser();

  // Profile operations
  @override
  Future<Result<void>> resetPassword({required String email}) =>
      _profileRepository.resetPassword(email: email);

  @override
  Future<Result<void>> verifyEmail() => _profileRepository.verifyEmail();

  @override
  Future<Result<void>> sendEmailVerification() =>
      _profileRepository.sendEmailVerification();

  @override
  Future<Result<UserProfilePatch>> updateProfile({
    String? photoUrl,
    String? phoneNumber,
    DateTime? phoneVerifiedAt,
    String? username,
    String? bio,
    String? location,
    DateTime? dateOfBirth,
  }) => _profileRepository.updateProfile(
    photoUrl: photoUrl,
    phoneNumber: phoneNumber,
    phoneVerifiedAt: phoneVerifiedAt,
    username: username,
    bio: bio,
    location: location,
    dateOfBirth: dateOfBirth,
  );

  @override
  Future<Result<AuthUser>> completeProfile({required String username}) =>
      _profileRepository.completeProfile(username: username);

  @override
  Future<Result<void>> changeEmail({
    required String newEmail,
    required String currentPassword,
  }) => _profileRepository.changeEmail(
    newEmail: newEmail,
    currentPassword: currentPassword,
  );

  @override
  Future<Result<void>> changePassword({
    required String currentPassword,
    required String newPassword,
  }) => _profileRepository.changePassword(
    currentPassword: currentPassword,
    newPassword: newPassword,
  );

  @override
  Future<Result<void>> deleteAccount() => _profileRepository.deleteAccount();

  @override
  Stream<FirebasePrincipal?> get authStateChanges =>
      _coreRepository.authStateChanges;

  // User lookup operations (moved from user module)
  @override
  Future<Result<AuthUser?>> getUserById(String userId) =>
      _profileRepository.getUserById(userId);

  @override
  Future<Result<List<AuthUser>>> searchUsers({
    required String query,
    int limit = 20,
  }) => _profileRepository.searchUsers(query: query, limit: limit);

  @override
  Future<Result<void>> deactivateAccount({
    required String userId,
    required String reason,
  }) => _profileRepository.deactivateAccount(userId: userId, reason: reason);

  @override
  Future<Result<AuthUser>> updateUserRole({
    required String userId,
    required UserRole newRole,
  }) => _profileRepository.updateUserRole(userId: userId, newRole: newRole);

  void dispose() {
    _coreRepository.dispose();
  }
}
