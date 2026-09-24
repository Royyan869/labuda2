import 'package:firebase_auth/firebase_auth.dart';
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
  ///
  /// D1 LINKING: [pendingGoogleCredential] (a Google AuthCredential that
  /// previously hit `account-exists-with-different-credential`) is linked
  /// into the CURRENT Firebase identity via `linkWithCredential` instead of
  /// starting a new sign-in — one Firebase UID per human. When null, this
  /// is a normal Google sign-in.
  Future<Result<void>> signInWithGoogle({AuthCredential? pendingGoogleCredential});

  /// Sign up dengan email dan password
  ///
  /// 🔒 DETERMINISTIC: Only creates Firebase identity.
  /// Returns void - AuthUser domain entity comes from backend via /users/me.
  Future<Result<FirebasePrincipal>> signUpWithEmail({
    required String email,
    required String password,
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

  /// Change password
  Future<Result<void>> changePassword({
    required String currentPassword,
    required String newPassword,
  });

  /// Send email verification
  Future<Result<void>> sendEmailVerification();

  /// Delete user account
  Future<Result<void>> deleteAccount();

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
