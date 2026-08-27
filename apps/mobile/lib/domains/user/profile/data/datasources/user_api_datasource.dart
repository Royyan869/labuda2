import 'package:dio/dio.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';

/// Data source for User API operations against Go backend
///
/// Handles HTTP calls to:
/// - POST /api/v1/users - Create/sync user
/// - GET /api/v1/users/:id - Get user by ID
/// - GET /api/v1/users/me - Get current user
/// - PATCH /api/v1/users/:id - Update user
/// - PATCH /api/v1/users/:id/profile - Update profile
/// - GET /api/v1/users/search - Search users
/// - GET /api/v1/users/:id - Public profile counts for stats surfaces
/// - GET /api/v1/users/:id/addresses - Get user addresses
/// - And more...
class UserApiDatasource extends BaseApiRepository {
  UserApiDatasource(super.apiClient, {super.logger});

  // ========================================
  // Core User Operations
  // ========================================

  /// Exchange a Firebase ID token for a backend session.
  ///
  /// [username] is the canonical registration username chosen during
  /// email/password signup. It is optional: it is only included in the request
  /// body when non-empty, and the backend assigns it exactly once when the
  /// user profile has no username yet. Login / Google-first-sync omit it so the
  /// backend decides profile completion on its own.
  Future<Result<FirebaseExchangeResponse>> exchangeFirebaseSession({
    required String firebaseIdToken,
    String? username,
  }) async {
    final body = <String, dynamic>{'firebase_id_token': firebaseIdToken};
    if (username != null && username.trim().isNotEmpty) {
      body['username'] = username;
    }
    return executeRequest(
      () => apiClient.post(
        '/auth/firebase/exchange',
        data: body,
        options: Options(extra: {'skipAuth': true}),
      ),
      parser: (data) =>
          FirebaseExchangeResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Get current authenticated user
  Future<Result<UserApiResponse>> getCurrentUser() async {
    final result = await executeRequest(
      () => apiClient.get('/users/me'),
      parser: (data) {
        final json = data as Map<String, dynamic>;
        // /users/me returns {user: UserDTO, profile: ProfileDTO} envelope
        final userMap = Map<String, dynamic>.from(
          json['user'] as Map<String, dynamic>,
        );
        final profileData = json['profile'] as Map<String, dynamic>?;
        if (profileData != null) {
          userMap['profile'] = profileData;
        }
        return UserApiResponse.fromJson(userMap);
      },
    );

    return result;
  }

  /// Get user by ID
  Future<Result<UserApiResponse>> getUserById(String userId) async {
    return executeRequest(
      () => apiClient.get('/users/$userId'),
      parser: (data) => UserApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Update user profile
  Future<Result<UserApiResponse>> updateProfile(
    String userId,
    UpdateProfileApiRequest request,
  ) async {
    return executeRequest(
      () => apiClient.patch('/users/$userId/profile', data: request.toJson()),
      parser: (data) => UserApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Update current user's profile
  Future<Result<UserApiResponse>> updateMyProfile(
    UpdateProfileApiRequest request,
  ) async {
    return executeRequest(
      () => apiClient.patch('/users/me/profile', data: request.toJson()),
      parser: (data) => UserApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  // ========================================
  // Search & Discovery Operations
  // ========================================

  /// Search users by query
  Future<Result<List<UserApiResponse>>> searchUsers({
    required String query,
    int page = 1,
    int limit = 20,
  }) async {
    return executeListRequest(
      () => apiClient.get(
        '/users/search',
        queryParameters: {'q': query, 'page': page, 'limit': limit},
      ),
      itemParser: (json) => UserApiResponse.fromJson(json),
    );
  }

  /// Get users by role/type
  Future<Result<List<UserApiResponse>>> getUsersByRole({
    required String role,
    int page = 1,
    int limit = 20,
  }) async {
    return executeListRequest(
      () => apiClient.get(
        '/users',
        queryParameters: {'role': role, 'page': page, 'limit': limit},
      ),
      itemParser: (json) => UserApiResponse.fromJson(json),
    );
  }

  /// Get trending/popular users
  Future<Result<List<UserApiResponse>>> getTrendingUsers({
    int limit = 10,
  }) async {
    return executeListRequest(
      () => apiClient.get('/users/trending', queryParameters: {'limit': limit}),
      itemParser: (json) => UserApiResponse.fromJson(json),
    );
  }

  /// Get multiple users by IDs
  Future<Result<List<UserApiResponse>>> getMultipleUsers(
    List<String> userIds,
  ) async {
    return executeListRequest(
      () => apiClient.post('/users/batch', data: {'user_ids': userIds}),
      itemParser: (json) => UserApiResponse.fromJson(json),
    );
  }

  /// Get verified sellers
  Future<Result<List<UserApiResponse>>> getVerifiedSellers({
    int page = 1,
    int limit = 20,
  }) async {
    return executeListRequest(
      () => apiClient.get(
        '/users/sellers/verified',
        queryParameters: {'page': page, 'limit': limit},
      ),
      itemParser: (json) => UserApiResponse.fromJson(json),
    );
  }

  // ========================================
  // Profile Stats Operations
  // ========================================

  /// Get profile stats for a user
  Future<Result<ProfileStats>> getProfileStats(String userId) async {
    return executeRequest(
      () => apiClient.get('/users/$userId'),
      parser: (data) => _parseProfileStats(data as Map<String, dynamic>),
    );
  }

  ProfileStats _parseProfileStats(Map<String, dynamic> json) {
    return ProfileStats(
      followersCount: (json['followers_count'] as num?)?.toInt() ?? 0,
      followingCount: (json['following_count'] as num?)?.toInt() ?? 0,
      // REMOVED: postsCount (no backend support, deleted in PROFILE PURGE)
      // REMOVED: averageRating (use rating module instead, deleted in PROFILE PURGE)
      // REMOVED: totalReviews (use rating module instead, deleted in PROFILE PURGE)
      // REMOVED: collectionsCount, transactionsCount (PROFILE PURGE)
    );
  }

  // ========================================
  // Farm/Seller Operations
  // ========================================

  /// Update farm info for seller
  Future<Result<UserApiResponse>> updateFarmInfo(
    String userId,
    FarmInfo farmInfo,
  ) async {
    return executeRequest(
      () => apiClient.patch(
        '/users/$userId/farm',
        data: {
          'farm_name': farmInfo.farmName,
          'farm_photo_url': farmInfo.farmPhotoUrl,
          'farm_website': farmInfo.farmWebsite,
          'specialties': farmInfo.specialties,
          'established_date': farmInfo.establishedDate?.toIso8601String(),
        },
      ),
      parser: (data) => UserApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  // ========================================
  // Utility Operations
  // ========================================

  /// Check if username is available
  Future<Result<bool>> checkUsernameAvailability(String username) async {
    return executeRequest(
      () => apiClient.get(
        '/users/check-username',
        queryParameters: {'username': username},
      ),
      parser: (data) {
        if (data is Map<String, dynamic>) {
          return data['available'] as bool? ?? false;
        }
        return false;
      },
    );
  }

  /// Update user avatar
  Future<Result<UserApiResponse>> updateAvatar(
    String userId,
    String avatarUrl,
  ) async {
    return executeRequest(
      () => apiClient.patch(
        '/users/$userId/avatar',
        data: {'avatar_url': avatarUrl},
      ),
      parser: (data) => UserApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  // ========================================
  // Role & Account Operations
  // ========================================

  /// Update user roles (replaces all roles with the provided roles)
  /// This is the preferred method for managing user roles
  Future<Result<UserApiResponse>> updateUserRoles({
    required String userId,
    required List<String> roles,
  }) async {
    return executeRequest(
      () => apiClient.patch('/users/$userId/role', data: {'roles': roles}),
      parser: (data) => UserApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Update user role (e.g., buyer -> seller upgrade)
  /// DEPRECATED: Use updateUserRoles for multiple roles support
  Future<Result<UserApiResponse>> updateUserRole({
    required String userId,
    required String role,
  }) async {
    return executeRequest(
      () => apiClient.patch(
        '/users/$userId/role',
        data: {
          'roles': [role], // Wrap in array for new endpoint
        },
      ),
      parser: (data) => UserApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Add roles to user (doesn't remove existing roles)
  Future<Result<UserApiResponse>> addUserRoles({
    required String userId,
    required List<String> roles,
  }) async {
    return executeRequest(
      () => apiClient.post('/users/$userId/roles', data: {'roles': roles}),
      parser: (data) => UserApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Remove roles from user
  Future<Result<UserApiResponse>> removeUserRoles({
    required String userId,
    required List<String> roles,
  }) async {
    return executeRequest(
      () => apiClient.delete('/users/$userId/roles', data: {'roles': roles}),
      parser: (data) => UserApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Deactivate user account
  Future<Result<void>> deactivateAccount({
    required String userId,
    required String reason,
  }) async {
    return executeVoidRequest(
      () =>
          apiClient.post('/users/$userId/deactivate', data: {'reason': reason}),
    );
  }
}
