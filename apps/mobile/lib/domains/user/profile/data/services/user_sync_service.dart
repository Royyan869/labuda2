import 'package:firebase_auth/firebase_auth.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/mappers/user_api_mapper.dart';

/// Result of user sync operation - includes user data, creation flag, and profile completion
class SyncUserResult {
  final AuthUser? user;
  final String userId;
  final String? email;
  final bool created;
  final bool
  profileComplete; // Backend-authoritative: true if username is set and not empty
  final String username; // Current username from profile

  const SyncUserResult({
    required this.user,
    required this.userId,
    this.email,
    required this.created,
    required this.profileComplete,
    required this.username,
  });
}

/// Service to sync user data between Firebase Auth and Go Backend
///
/// **SOURCE OF TRUTH (Single Source):**
/// - Identity (email, password, provider): Firebase Auth
/// - Profile Data (username, bio, avatarUrl): PostgreSQL via Backend API
/// - Roles & Permissions: PostgreSQL via Backend API
///
/// This service ensures that:
/// 1. After Firebase sign-in/sign-up, user exists in PostgreSQL
/// 2. Profile updates are persisted to PostgreSQL via Backend API
/// 3. User data is fetched from PostgreSQL (NOT from Firestore)
///
/// Usage:
/// ```dart
/// // After successful Firebase auth
/// final syncResult = await userSyncService.syncUser(username: username);
/// ```
class UserSyncService {
  final FirebaseAuth _firebaseAuth;
  final UserApiDatasource _datasource;
  final ILoggerService? _logger;
  final ILocalStorageService? _localStorage;

  UserSyncService({
    FirebaseAuth? firebaseAuth,
    required UserApiDatasource datasource,
    ILoggerService? logger,
    ILocalStorageService? localStorage,
  }) : _firebaseAuth = firebaseAuth ?? FirebaseAuth.instance,
       _datasource = datasource,
       _logger = logger,
       _localStorage = localStorage;

  /// Sync user to backend after Firebase authentication
  ///
  /// This should be called after:
  /// - Successful sign-in (to ensure user exists in backend)
  /// - Successful sign-up (to create user in backend)
  /// - Profile updates that need to be persisted
  ///
  /// This method NOW uses the new /users/sync endpoint which:
  /// - Verifies Firebase ID token from the API client
  /// - Extracts UID and email from the verified token
  /// - Creates user if not exists, or returns existing user (idempotent)
  ///
  /// IMPORTANT: This will fail if sync fails - NOT silent!
  /// Caller should handle the error appropriately.
  ///
  /// Returns SyncUserResult containing the synced AuthUser and a flag indicating
  /// if the user was just created (true) or already existed (false)
  Future<Result<SyncUserResult>> syncUser({
    required String username,
    String? phoneNumber,
  }) async {
    // 🔍 COLD START AUDIT: Sync started

    // 🔎 BAGIAN 1: Log Firebase ID Token
    final firebaseUser = _firebaseAuth.currentUser;

    final idToken = await firebaseUser?.getIdToken();

    // Security: Firebase ID token obtained - NOT logged for security
    // Security: Token null/empty - NOT logged for security

    if (idToken == null || idToken.isEmpty) {
      // Security: Firebase token error - NOT logged for security
      return Result.error('Firebase ID token is null - user not authenticated');
    }

    // 🔎 BAGIAN 2: Log username being sent
    _logger?.log(
      '[SYNC] Syncing user: username length=${username.length}',
      level: LogLevel.debug,
    );
    _logger?.log(
      '[SYNC] Username bytes: ${username.codeUnits.length}',
      level: LogLevel.debug,
    );
    _logger?.log(
      '[SYNC] Username trimmed: "${username.trim()}"',
      level: LogLevel.debug,
    );
    _logger?.info('Syncing user to backend: username=$username');

    _logger?.info('Syncing user to backend via Firebase exchange');

    final result = await _datasource.exchangeFirebaseSession(
      firebaseIdToken: idToken,
      username: username,
    );

    // Log error for debugging
    if (result.isError) {
      _logger?.error('[SYNC] Failed: ${result.error}');
      _logger?.error(
        'Backend sync failed: ${result.error}',
        extra: {'username': username},
      );
      // PASS 2A: forward errorCode/statusCode from the datasource result —
      // AuthController's classifyAuthSyncError needs the backend's
      // structured code (INVALID_TOKEN/ACCOUNT_DELETED/ACCOUNT_INACTIVE) to
      // classify this failure correctly instead of guessing from text.
      return Result.error(
        result.error ?? 'Backend sync failed',
        code: result.errorCode,
        statusCode: result.statusCode,
        details: result.errorDetails,
      );
    }

    _logger?.info('[SYNC] Success');
    _logger?.info(
      'User synced successfully: ${result.data?.userId}, created: ${result.data?.created}, profileComplete: ${!(result.data?.requiresProfileCompletion ?? false)}',
    );

    // Persist platform tokens (fire-and-forget; Firebase token remains authoritative for API calls).
    if (result.isSuccess) {
      final response = result.data!;
      final storage = _localStorage;
      if (storage != null) {
        if (response.accessToken.isNotEmpty) {
          await storage.setAuthToken(response.accessToken);
        }
        if (response.refreshToken != null &&
            response.refreshToken!.isNotEmpty) {
          await storage.setRefreshToken(response.refreshToken!);
        } else {
          await storage.clearRefreshToken();
        }
      }
    }

    final response = result.data!;
    if (response.requiresProfileCompletion) {
      return Result.success(
        SyncUserResult(
          user: null,
          userId: response.userId,
          email: response.email,
          created: response.created,
          profileComplete: false,
          username: '',
        ),
      );
    }

    final currentUserResult = await getCurrentUser();
    if (currentUserResult.isError || currentUserResult.data == null) {
      return Result.error(
        currentUserResult.error ?? 'Failed to load current user profile',
        code: currentUserResult.errorCode,
        statusCode: currentUserResult.statusCode,
        details: currentUserResult.errorDetails,
      );
    }

    final currentUser = currentUserResult.data!;
    if (currentUser.id != response.userId) {
      await _clearStoredSessionTokens();
      return Result.error(
        'Backend session user mismatch',
        code: 'SESSION_USER_MISMATCH',
        statusCode: 409,
        details: {
          'exchange_user_id': response.userId,
          'current_user_id': currentUser.id,
        },
      );
    }

    return Result.success(
      SyncUserResult(
        user: currentUser,
        userId: response.userId,
        email: response.email,
        created: response.created,
        profileComplete: true,
        username: currentUser.username,
      ),
    );
  }

  /// Get current user from backend
  ///
  /// Use this to fetch the most up-to-date user data from PostgreSQL
  Future<Result<AuthUser>> getCurrentUser() async {
    // 🔍 LANGKAH 2: Di awal method
    _logger?.log('[SYNC] getCurrentUser called', level: LogLevel.debug);

    final result = await _datasource.getCurrentUser();

    // 🔍 LANGKAH 2: Setelah API call
    _logger?.log(
      '[SYNC] getCurrentUser: response received',
      level: LogLevel.debug,
    );
    _logger?.log(
      '[SYNC] getCurrentUser: isSuccess=${result.isSuccess}',
      level: LogLevel.debug,
    );
    _logger?.log(
      '[SYNC] getCurrentUser: error=${result.error}',
      level: LogLevel.debug,
    );

    if (result.isSuccess && result.data != null) {
      _logger?.info('[SYNC] getCurrentUser: success');
      _logger?.log(
        '[SYNC] getCurrentUser: user.id=${result.data!.id}',
        level: LogLevel.debug,
      );
      _logger?.log(
        '[SYNC] getCurrentUser: emailVerified=${result.data!.isEmailVerified}',
        level: LogLevel.debug,
      );
    } else {
      _logger?.error('[SYNC] getCurrentUser: failed');
    }

    return result.map((response) => UserApiMapper.toAuthUser(response));
  }

  /// Get user by ID from backend
  Future<Result<AuthUser?>> getUserById(String userId) async {
    final result = await _datasource.getUserById(userId);

    if (result.isError) {
      // User not found is not an error, return null
      if (result.error?.contains('not found') == true) {
        return Result.success(null);
      }
      return Result.error(result.error ?? 'Failed to get user');
    }

    return Result.success(UserApiMapper.toAuthUser(result.data!));
  }

  /// Update user profile in backend
  Future<Result<AuthUser>> updateProfile({
    required String userId,
    String? bio,
    String? avatarUrl,
    DateTime? dateOfBirth,
    String? gender,
    String? location,
    String? instagramHandle,
    String? facebookHandle,
    String? twitterHandle,
    String? tiktokHandle,
    String? youtubeHandle,
    String? websiteUrl,
    String? visibility,
    bool? showPhoneNumber,
    bool? showEmail,
    bool? showLocation,
    String? allowMessagesFrom,
    bool? allowTagging,
    bool? showActivityStatus,
    bool? showTransactionCount,
  }) async {
    _logger?.info('Updating profile for user: $userId');

    final request = UserApiMapper.toUpdateProfileRequest(
      bio: bio,
      avatarUrl: avatarUrl,
      dateOfBirth: dateOfBirth,
      gender: gender,
      location: location,
      instagramHandle: instagramHandle,
      facebookHandle: facebookHandle,
      twitterHandle: twitterHandle,
      tiktokHandle: tiktokHandle,
      youtubeHandle: youtubeHandle,
      websiteUrl: websiteUrl,
      visibility: visibility,
      showPhoneNumber: showPhoneNumber,
      showEmail: showEmail,
      showLocation: showLocation,
      allowMessagesFrom: allowMessagesFrom,
      allowTagging: allowTagging,
      showActivityStatus: showActivityStatus,
      showTransactionCount: showTransactionCount,
    );

    final result = await _datasource.updateProfile(userId, request);

    return result.map((response) {
      _logger?.info('Profile updated successfully');
      return UserApiMapper.toAuthUser(response);
    });
  }

  /// Update current user's profile
  Future<Result<AuthUser>> updateMyProfile({
    String? bio,
    String? avatarUrl,
    DateTime? dateOfBirth,
    String? gender,
    String? location,
  }) async {
    final request = UserApiMapper.toUpdateProfileRequest(
      bio: bio,
      avatarUrl: avatarUrl,
      dateOfBirth: dateOfBirth,
      gender: gender,
      location: location,
    );

    final result = await _datasource.updateMyProfile(request);
    return result.map((response) => UserApiMapper.toAuthUser(response));
  }

  /// Search users
  Future<Result<List<AuthUser>>> searchUsers({
    required String query,
    int page = 1,
    int limit = 20,
  }) async {
    final result = await _datasource.searchUsers(
      query: query,
      page: page,
      limit: limit,
    );

    return result.map(
      (responses) => responses.map(UserApiMapper.toAuthUser).toList(),
    );
  }

  /// Check if username is available
  Future<Result<bool>> checkUsernameAvailability(String username) async {
    return _datasource.checkUsernameAvailability(username);
  }

  /// Update avatar URL
  Future<Result<AuthUser>> updateAvatar(String userId, String avatarUrl) async {
    final result = await _datasource.updateAvatar(userId, avatarUrl);
    return result.map((response) => UserApiMapper.toAuthUser(response));
  }

  // ========================================
  // Role Management Operations
  // ========================================

  /// Update user roles (replaces all roles with the provided roles)
  /// This is the preferred method for managing user roles from backend API
  Future<Result<AuthUser>> updateUserRoles({
    required String userId,
    required List<String> roles,
  }) async {
    _logger?.info('Updating user roles: userId=$userId, roles=$roles');

    final result = await _datasource.updateUserRoles(
      userId: userId,
      roles: roles,
    );

    if (result.isError) {
      _logger?.error(
        'Failed to update user roles: ${result.error}',
        extra: {'userId': userId, 'roles': roles},
      );
      return Result.error(result.error ?? 'Failed to update user roles');
    }

    _logger?.info('User roles updated successfully: ${result.data?.id}');
    return result.map((response) => UserApiMapper.toAuthUser(response));
  }

  /// Add roles to user (doesn't remove existing roles)
  Future<Result<AuthUser>> addUserRoles({
    required String userId,
    required List<String> roles,
  }) async {
    _logger?.info('Adding user roles: userId=$userId, roles=$roles');

    final result = await _datasource.addUserRoles(userId: userId, roles: roles);

    if (result.isError) {
      _logger?.error(
        'Failed to add user roles: ${result.error}',
        extra: {'userId': userId, 'roles': roles},
      );
      return Result.error(result.error ?? 'Failed to add user roles');
    }

    _logger?.info('User roles added successfully: ${result.data?.id}');
    return result.map((response) => UserApiMapper.toAuthUser(response));
  }

  /// Remove roles from user
  Future<Result<AuthUser>> removeUserRoles({
    required String userId,
    required List<String> roles,
  }) async {
    _logger?.info('Removing user roles: userId=$userId, roles=$roles');

    final result = await _datasource.removeUserRoles(
      userId: userId,
      roles: roles,
    );

    if (result.isError) {
      _logger?.error(
        'Failed to remove user roles: ${result.error}',
        extra: {'userId': userId, 'roles': roles},
      );
      return Result.error(result.error ?? 'Failed to remove user roles');
    }

    _logger?.info('User roles removed successfully: ${result.data?.id}');
    return result.map((response) => UserApiMapper.toAuthUser(response));
  }

  Future<void> _clearStoredSessionTokens() async {
    final storage = _localStorage;
    if (storage == null) return;

    await storage.clearAuthToken();
    await storage.clearRefreshToken();
  }
}
