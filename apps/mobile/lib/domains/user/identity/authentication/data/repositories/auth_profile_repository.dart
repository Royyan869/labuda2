import 'package:firebase_auth/firebase_auth.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/auth_user.dart'
    as domain;
import 'package:labuda/domains/user/profile/data/mappers/user_api_mapper.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/services/local_storage_service.dart';
import '../datasources/auth_api_datasource.dart';
import '../../domain/entities/account_status.dart';
import '../../domain/entities/seller_tier.dart';
import '../../domain/entities/user_profile_patch.dart';

/// Profile Management Repository - Handles user profile operations
///
/// Migration Strategy (COMPLIANT with MIGRASI_2_GUIDE.md):
/// - User data operations (getUserById, searchUsers, updateProfile data, deactivateAccount, updateUserRole)
///   use Go backend API via AuthApiDatasource
/// - Firebase Auth operations (resetPassword, verifyEmail, changeEmail, changePassword, deleteAccount)
///   remain with Firebase Auth as these are platform/auth services
class AuthProfileRepository {
  final FirebaseAuth _firebaseAuth;
  final AuthApiDatasource _apiDatasource;
  final ILocalStorageService _localStorage;

  AuthProfileRepository({
    FirebaseAuth? firebaseAuth,
    required AuthApiDatasource apiDatasource,
    ILocalStorageService? localStorage,
  }) : _firebaseAuth = firebaseAuth ?? FirebaseAuth.instance,
       _apiDatasource = apiDatasource,
       _localStorage = localStorage ?? LocalStorageService();

  Future<Result<void>> resetPassword({required String email}) async {
    try {
      await _firebaseAuth.sendPasswordResetEmail(email: email);
      return Result.success(null);
    } on FirebaseAuthException catch (e) {
      return Result.error(_mapFirebaseError(e));
    } catch (e) {
      return Result.error('Reset password failed: ${e.toString()}');
    }
  }

  Future<Result<void>> verifyEmail() async {
    try {
      final user = _firebaseAuth.currentUser;
      if (user != null && !user.emailVerified) {
        await user.sendEmailVerification();
        return Result.success(null);
      } else if (user == null) {
        return Result.error('User not found');
      } else {
        return Result.error('Email already verified');
      }
    } on FirebaseAuthException catch (e) {
      return Result.error(_mapFirebaseError(e));
    } catch (e) {
      return Result.error('Verify email failed: ${e.toString()}');
    }
  }

  Future<Result<void>> sendEmailVerification() async {
    try {
      final user = _firebaseAuth.currentUser;
      if (user != null) {
        await user.sendEmailVerification();
        return Result.success(null);
      } else {
        return Result.error('User not found');
      }
    } on FirebaseAuthException catch (e) {
      return Result.error(_mapFirebaseError(e));
    } catch (e) {
      return Result.error('Send email verification failed: ${e.toString()}');
    }
  }

  Future<Result<UserProfilePatch>> updateProfile({
    String? photoUrl,
    String? phoneNumber,
    DateTime? phoneVerifiedAt,
    String? username,
    String? bio,
    String? location,
    DateTime? dateOfBirth,
  }) async {
    try {
      final user = _firebaseAuth.currentUser;
      if (user == null) {
        return Result.error('User not found');
      }

      // 1. Update user profile data via backend API
      // Uses /users/me/profile - backend extracts identity from auth token
      // This endpoint now handles ALL profile updates including username
      // Legacy PATCH /users/{id}/username call has been removed
      final result = await _apiDatasource.updateProfile(
        username: username,
        bio: bio,
        photoUrl: photoUrl,
        phoneNumber: phoneNumber,
        location: location,
        phoneVerifiedAt: phoneVerifiedAt,
        dateOfBirth: dateOfBirth,
      );

      if (result.isError) {
        return Result.error(result.error ?? 'Failed to update profile');
      }

      // 2. Reload Firebase Auth user to get latest profile
      await user.reload();
      final updatedUser = _firebaseAuth.currentUser!;

      // 3. Map API response to a profile patch, not a full account snapshot
      final apiData = result.data!;
      return Result.success(
        _mapApiDataToProfilePatch(
          apiData,
          firebaseUser: updatedUser,
          photoUrl: photoUrl,
          phoneNumber: phoneNumber,
          phoneVerifiedAt: phoneVerifiedAt,
          username: username,
          bio: bio,
          location: location,
          dateOfBirth: dateOfBirth,
        ),
      );
    } on FirebaseAuthException catch (e) {
      return Result.error(_mapFirebaseError(e));
    } catch (e) {
      return Result.error('Update profile failed: ${e.toString()}');
    }
  }

  Future<Result<domain.AuthUser>> completeProfile({
    required String username,
  }) async {
    try {
      final tokenResult = await _localStorage.getRestrictedToken();
      final restrictedToken = tokenResult.data?.trim();
      if (restrictedToken == null || restrictedToken.isEmpty) {
        return Result.error('Restricted completion token not available');
      }

      final result = await _apiDatasource.completeProfile(
        username: username,
        restrictedToken: restrictedToken,
      );
      if (result.isError) {
        return Result.error(result.error ?? 'Failed to complete profile');
      }

      final completeResponse = result.data!;
      final storeResult = await _localStorage.saveLabudaCredential(
        completeResponse.accessToken,
        completeResponse.refreshToken,
      );
      if (storeResult.isError) {
        return Result.error(
          storeResult.error ?? 'Failed to store session credential',
        );
      }

      // Restricted completion token has been consumed — clear its isolated key
      await _localStorage.clearRestrictedToken();

      final currentUserResult = await _apiDatasource.getCurrentUser();
      if (currentUserResult.isError || currentUserResult.data == null) {
        return Result.error(
          currentUserResult.error ?? 'Failed to load current user profile',
          code: currentUserResult.errorCode,
          statusCode: currentUserResult.statusCode,
        );
      }

      final currentUser = UserApiMapper.toAuthUser(currentUserResult.data!);
      if (currentUser.id != completeResponse.userId) {
        await _clearStoredSessionTokens();
        return Result.error(
          'Backend session user mismatch',
          code: 'SESSION_USER_MISMATCH',
          statusCode: 409,
          details: {
            'complete_profile_user_id': completeResponse.userId,
            'current_user_id': currentUser.id,
          },
        );
      }

      return Result.success(currentUser);
    } catch (e) {
      return Result.error('Complete profile failed: ${e.toString()}');
    }
  }

  Future<Result<void>> changeEmail({
    required String newEmail,
    required String currentPassword,
  }) async {
    try {
      final user = _firebaseAuth.currentUser;
      if (user == null) {
        return Result.error('User not found');
      }

      // Reauthenticate first for security
      final credential = EmailAuthProvider.credential(
        email: user.email!,
        password: currentPassword,
      );
      await user.reauthenticateWithCredential(credential);

      // Update email
      await user.verifyBeforeUpdateEmail(newEmail);

      // Send verification email to new address
      await user.sendEmailVerification();

      return Result.success(null);
    } on FirebaseAuthException catch (e) {
      return Result.error(_mapFirebaseError(e));
    } catch (e) {
      return Result.error('Change email failed: ${e.toString()}');
    }
  }

  Future<Result<void>> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async {
    try {
      final user = _firebaseAuth.currentUser;
      if (user == null) {
        return Result.error('User not found');
      }

      // Reauthenticate first for security
      final credential = EmailAuthProvider.credential(
        email: user.email!,
        password: currentPassword,
      );
      await user.reauthenticateWithCredential(credential);

      // Update password
      await user.updatePassword(newPassword);

      return Result.success(null);
    } on FirebaseAuthException catch (e) {
      return Result.error(_mapFirebaseError(e));
    } catch (e) {
      return Result.error('Change password failed: ${e.toString()}');
    }
  }

  /// Delete account — backend-first ordering.
  ///
  /// 1. Soft-delete on backend (sets deleted_at, emits user.deleted event).
  /// 2. Delete Firebase credential so the UID can never be re-linked.
  ///
  /// If step 2 fails the user is already locked out at the backend because
  /// UserLookupMiddleware filters deleted_at IS NULL and
  /// hasSoftDeletedUserByFirebaseUID returns ACCOUNT_DELETED. Caller must
  /// still sign out locally.
  Future<Result<void>> deleteAccount() async {
    // Step 1: backend soft-delete
    final backendResult = await _apiDatasource.selfDeleteAccount();
    if (backendResult.isError) {
      return Result.error(
        backendResult.error ?? 'Failed to delete account on server',
      );
    }

    // Step 2: Firebase credential delete (best-effort)
    try {
      final user = _firebaseAuth.currentUser;
      if (user != null) {
        await user.delete();
      }
    } on FirebaseAuthException catch (e) {
      // requires-recent-login: user is already locked out on backend; surface
      // a specific message so the caller can prompt re-auth if desired.
      if (e.code == 'requires-recent-login') {
        return Result.error('requires-recent-login');
      }
      // All other Firebase errors: backend is authoritative, treat as success.
    } catch (_) {
      // Non-Firebase errors: backend is authoritative, proceed.
    }

    return Result.success(null);
  }

  /// Revoke the current backend refresh session family.
  Future<Result<void>> logoutCurrentSession({
    required String refreshToken,
    String? fcmToken,
    String? deviceId,
  }) {
    return _apiDatasource.logoutCurrentSession(
      refreshToken: refreshToken,
      fcmToken: fcmToken,
      deviceId: deviceId,
    );
  }

  /// Revoke all backend refresh sessions for the active user.
  Future<Result<void>> logoutAllSessions({bool deactivateFcmTokens = true}) {
    return _apiDatasource.logoutAllSessions(
      deactivateFcmTokens: deactivateFcmTokens,
    );
  }

  /// List active session device summaries for the authenticated user.
  Future<Result<List<AuthSessionDto>>> getActiveSessions() {
    return _apiDatasource.getActiveSessions();
  }

  /// Revoke a single session family by its family_id.
  Future<Result<void>> revokeSession(String familyId) {
    return _apiDatasource.revokeSession(familyId);
  }

  /// Map Firebase Auth errors to user-friendly English messages
  String _mapFirebaseError(FirebaseAuthException e) {
    switch (e.code) {
      case 'requires-recent-login':
        return 'Please sign in again to make this change';
      case 'user-disabled':
        return 'Account disabled';
      case 'too-many-requests':
        return 'Too many attempts, try again later';
      case 'weak-password':
        return 'Password too weak';
      case 'email-already-in-use':
        return 'Email already used';
      default:
        return 'Error occurred: ${e.message}';
    }
  }

  List<UserRole> _parseUserRoles(dynamic rolesData) {
    // Default to user role (canonical, replaces legacy "buyer")
    const defaultRole = UserRole.user;

    if (rolesData == null) {
      return [defaultRole];
    }

    if (rolesData is List) {
      return rolesData.map((r) {
        try {
          return UserRoleParser.fromApiValue(r.toString());
        } catch (e) {
          return defaultRole;
        }
      }).toList();
    } else if (rolesData is String) {
      if (rolesData.isEmpty) return [defaultRole];
      try {
        return [UserRoleParser.fromApiValue(rolesData)];
      } catch (e) {
        return [defaultRole];
      }
    }

    return [defaultRole];
  }

  // ============================================
  // User Lookup Operations (moved from user module)
  // ============================================

  /// Get user by ID from backend API
  Future<Result<AuthUser?>> getUserById(String userId) async {
    final result = await _apiDatasource.getUserById(userId);
    return result.map((data) => _mapApiDataToAuthUser(data));
  }

  /// Search users by name or username using backend API
  Future<Result<List<AuthUser>>> searchUsers({
    required String query,
    int limit = 20,
  }) async {
    if (query.trim().isEmpty) {
      return Result.success([]);
    }

    final result = await _apiDatasource.searchUsers(
      query: query,
      page: 1,
      limit: limit,
    );

    return result.map(
      (usersData) =>
          usersData.map((data) => _mapApiDataToAuthUser(data)).toList(),
    );
  }

  /// Deactivate user account with reason using backend API
  Future<Result<void>> deactivateAccount({
    required String userId,
    required String reason,
  }) async {
    final result = await _apiDatasource.deactivateAccount(
      userId: userId,
      reason: reason,
    );
    if (result.isError) {
      return Result.error(result.error ?? 'Failed to deactivate account');
    }
    return Result.success(null);
  }

  /// Update user role (for seller upgrade, admin promotion) using backend API
  Future<Result<AuthUser>> updateUserRole({
    required String userId,
    required UserRole newRole,
  }) async {
    final result = await _apiDatasource.updateUserRole(
      userId: userId,
      role: newRole.name,
    );
    return result.map((data) => _mapApiDataToAuthUser(data));
  }

  /// Map backend API response data to AuthUser entity
  /// Backend uses snake_case (avatar_url, created_at, etc.)
  AuthUser _mapApiDataToAuthUser(Map<String, dynamic> data) {
    // Handle both snake_case (API) and camelCase (Firestore) formats
    final id = data['id']?.toString() ?? data['user_id']?.toString() ?? '';
    final email = data['email'] as String? ?? '';
    final username = data['username'] as String? ?? '';
    final avatarUrl = data['avatar_url'] as String?;
    final bio = data['bio'] as String?;
    final phoneNumber =
        data['phone_number'] as String? ?? data['phoneNumber'] as String?;
    final isEmailVerified =
        data['email_verified'] as bool? ??
        data['is_email_verified'] as bool? ??
        data['isEmailVerified'] as bool? ??
        false;

    // Parse roles - backend returns array of strings or single string
    final rolesData = data['roles'] ?? data['role'];
    final roles = _parseUserRoles(rolesData);

    // Parse provider
    final providerStr = data['provider'] as String? ?? 'email';
    final provider = domain.AuthProvider.values.firstWhere(
      (p) => p.name == providerStr,
      orElse: () => domain.AuthProvider.email,
    );

    // Parse timestamps - API returns ISO strings, Firestore returns Timestamp
    DateTime? createdAt;
    DateTime? updatedAt;
    if (data['created_at'] is String) {
      createdAt = DateTime.tryParse(data['created_at'] as String);
    } else if (data['createdAt'] is String) {
      createdAt = DateTime.tryParse(data['createdAt'] as String);
    }

    if (data['updated_at'] is String) {
      updatedAt = DateTime.tryParse(data['updated_at'] as String);
    } else if (data['updatedAt'] is String) {
      updatedAt = DateTime.tryParse(data['updatedAt'] as String);
    }

    // Parse account_status — default to suspended when absent (fail-closed).
    // Guards (AuthGuard/SellerGuard) rely on backend rejection for enforcement;
    // this parse provides the correct local state for display paths.
    final accountStatusStr = data['account_status'] as String?;
    final accountStatus = accountStatusStr != null
        ? AccountStatus.fromApiValue(accountStatusStr)
        : AccountStatus.suspended;

    // E5.2 — Parse canonical public lifecycle from response.identity.lifecycle.
    //
    // GET /users/:id (PublicUserResponse) now carries an `identity` block
    // shaped like publiccard.UserCard, with lifecycle ∈ {active, unavailable,
    // removed} coarsened server-side via viewercontext.CoarsenLifecycle.
    //
    // /users/me and other legacy surfaces still return the flat UserDTO with
    // no `identity` block; for them this falls through to the safe default
    // ContentLifecycle.active (fromWire null → active). The mapper MUST NOT
    // coarsen from raw `account_status` — that would replicate the
    // coarsening rule client-side and violate ADR-006 §11.
    String? lifecycleWire;
    final identityRaw = data['identity'];
    if (identityRaw is Map) {
      final identity = identityRaw.cast<String, dynamic>();
      final v = identity['lifecycle'];
      if (v is String) lifecycleWire = v;
    }
    final lifecycle = ContentLifecycleParse.fromWire(lifecycleWire);

    // Parse seller_tier from public profile response (GET /users/:id).
    // Backend emits "pro" or "elite" only when feature flag + lifecycle gates
    // pass; null means no badge. Safe: SellerTier.fromApiValue handles null.
    final sellerTierRaw = data['seller_tier'] as String?;
    final sellerTier = SellerTier.fromApiValue(sellerTierRaw);

    return AuthUser(
      id: id,
      email: email,
      username: username,
      avatarUrl: avatarUrl,
      bio: bio,
      phoneNumber: phoneNumber,
      isEmailVerified: isEmailVerified,
      accountStatus: accountStatus,
      roles: roles,
      provider: provider,
      createdAt: createdAt ?? DateTime.now(),
      updatedAt: updatedAt ?? DateTime.now(),
      lifecycle: lifecycle,
      sellerTier: sellerTier,
    );
  }

  UserProfilePatch _mapApiDataToProfilePatch(
    Map<String, dynamic> data, {
    required User firebaseUser,
    String? photoUrl,
    String? phoneNumber,
    DateTime? phoneVerifiedAt,
    String? username,
    String? bio,
    String? location,
    DateTime? dateOfBirth,
  }) {
    final profile = data['profile'];
    final profileMap = profile is Map ? profile.cast<String, dynamic>() : null;

    final resolvedPhotoUrl = photoUrl ?? profileMap?['avatar_url'] as String?;
    final resolvedPhoneNumber =
        firebaseUser.phoneNumber ??
        phoneNumber ??
        profileMap?['phone_number'] as String? ??
        profileMap?['phoneNumber'] as String?;
    final resolvedUsername =
        username ??
        data['username'] as String? ??
        profileMap?['username'] as String?;
    final resolvedBio =
        bio ?? data['bio'] as String? ?? profileMap?['bio'] as String?;
    final resolvedLocation =
        location ??
        data['location'] as String? ??
        profileMap?['location'] as String?;

    return UserProfilePatch(
      photoUrl: resolvedPhotoUrl,
      phoneNumber: resolvedPhoneNumber,
      phoneVerifiedAt: phoneVerifiedAt,
      username: resolvedUsername,
      bio: resolvedBio,
      location: resolvedLocation,
      dateOfBirth: dateOfBirth,
    );
  }

  Future<void> _clearStoredSessionTokens() async {
    await _localStorage.clearLabudaCredential();
  }
}
