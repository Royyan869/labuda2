import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';

import '../entities/auth_user.dart' as domain;
import '../entities/firebase_principal.dart';
import '../entities/user_profile_patch.dart';

/// Repository interface untuk authentication operations.
///
/// Mengikuti clean architecture dengan interface-first design
/// untuk operasi otentikasi dalam platform LABUDA.
abstract class IAuthRepository {
  /// Sign in dengan email dan password
  Future<Result<FirebasePrincipal>> signInWithEmail({
    required String email,
    required String password,
  });

  /// Sign in dengan Google
  ///
  /// 🔒 DETERMINISTIC: Only creates Firebase identity.
  /// Returns void - AuthUser domain entity comes from backend via /users/me.
  Future<Result<void>> signInWithGoogle();

  /// Sign up dengan email dan password
  ///
  /// 🔒 DETERMINISTIC: Only creates Firebase identity.
  /// Returns void - AuthUser domain entity comes from backend via /users/me.
  Future<Result<FirebasePrincipal>> signUpWithEmail({
    required String email,
    required String password,
    required String username,
  });

  /// Sign out user yang sedang aktif
  Future<Result<void>> signOut();

  /// Revoke the current backend refresh session family for the active user.
  Future<Result<void>> logoutCurrentSession({
    required String refreshToken,
    String? fcmToken,
    String? deviceId,
  });

  /// Revoke all backend refresh sessions for the active user.
  Future<Result<void>> logoutAllSessions({bool deactivateFcmTokens = true});

  /// List active session device summaries for the authenticated user.
  Future<Result<List<AuthSessionDto>>> getActiveSessions();

  /// Revoke a single session family by its family_id.
  Future<Result<void>> revokeSession(String familyId);

  /// Mendapatkan user yang sedang aktif
  Future<Result<domain.AuthUser?>> getCurrentUser();

  /// Reset password melalui email
  Future<Result<void>> resetPassword({required String email});

  /// Verifikasi email user
  Future<Result<void>> verifyEmail();

  /// Update profile user
  Future<Result<UserProfilePatch>> updateProfile({
    String? photoUrl,
    String? phoneNumber,
    DateTime? phoneVerifiedAt,
    String? username,
    String? bio,
    String? location,
    DateTime? dateOfBirth,
  });

  /// Complete the profile after restricted Firebase exchange.
  Future<Result<domain.AuthUser>> completeProfile({required String username});

  /// Change email address
  Future<Result<void>> changeEmail({
    required String newEmail,
    required String currentPassword,
  });

  /// Change password
  Future<Result<void>> changePassword({
    required String currentPassword,
    required String newPassword,
  });

  /// Send email verification
  Future<Result<void>> sendEmailVerification();

  /// Delete user account
  Future<Result<void>> deleteAccount();

  /// Stream untuk mendengarkan perubahan auth state
  Stream<FirebasePrincipal?> get authStateChanges;

  // ============================================
  // User Lookup Operations (moved from user module)
  // ============================================

  /// Get user by ID
  /// Returns null if user not found
  Future<Result<domain.AuthUser?>> getUserById(String userId);

  /// Search users by name or username
  /// Returns list of users matching the query
  Future<Result<List<domain.AuthUser>>> searchUsers({
    required String query,
    int limit = 20,
  });

  /// Deactivate user account with reason
  Future<Result<void>> deactivateAccount({
    required String userId,
    required String reason,
  });

  /// Update user role (for seller upgrade, admin promotion)
  Future<Result<domain.AuthUser>> updateUserRole({
    required String userId,
    required UserRole newRole,
  });
}
