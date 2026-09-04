import 'package:dio/dio.dart';
import 'package:labuda/core/api/api.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';

/// API Data Source for Authentication operations against Go backend
///
/// This datasource handles operations that have been migrated from Firebase
/// to the Go backend API:
/// - GET /api/v1/users/:id - Get user by ID
/// - GET /api/v1/users/search - Search users
/// - PATCH /api/v1/users/me/profile - Update profile (username is immutable
///   once established; the backend enforces establishment-only assignment)
/// - PATCH /api/v1/users/:id/role - Update user role
/// - POST /api/v1/users/:id/deactivate - Deactivate account
/// - GET /api/v1/users/:id/subscription - Get seller subscription
///
/// Note: Core auth operations (sign in, sign out, password reset) remain
/// with Firebase Auth as defined in the migration guide.
class AuthApiDatasource extends BaseApiRepository {
  AuthApiDatasource(super.apiClient, {super.logger});

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

  /// Complete the profile using the restricted backend token.
  Future<Result<FirebaseExchangeCompleteResponse>> completeProfile({
    required String username,
    required String restrictedToken,
  }) async {
    return executeRequest(
      () => apiClient.post(
        '/auth/complete-profile',
        data: {'username': username},
        options: Options(
          extra: {'skipAuth': true},
          headers: {'Authorization': 'Bearer $restrictedToken'},
        ),
      ),
      parser: (data) => FirebaseExchangeCompleteResponse.fromJson(
        data as Map<String, dynamic>,
      ),
    );
  }

  /// Get current authenticated user from /users/me.
  Future<Result<UserApiResponse>> getCurrentUser() async {
    return executeRequest(
      () => apiClient.get('/users/me'),
      parser: (data) {
        final json = data as Map<String, dynamic>;
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
  }

  /// Get user by ID from backend
  Future<Result<Map<String, dynamic>>> getUserById(String userId) async {
    return executeRequest(
      () => apiClient.get('/users/$userId'),
      parser: (data) => data,
    );
  }

  /// Search users by query
  Future<Result<List<Map<String, dynamic>>>> searchUsers({
    required String query,
    int page = 1,
    int limit = 20,
  }) async {
    return executeListRequest(
      () => apiClient.get(
        '/users/search',
        queryParameters: {'q': query, 'page': page, 'limit': limit},
      ),
      itemParser: (json) => json,
    );
  }

  /// Update current user's profile
  /// Uses /users/me/profile - backend extracts identity from auth token
  Future<Result<Map<String, dynamic>>> updateProfile({
    String? username,
    String? bio,
    String? photoUrl,
    String? phoneNumber,
    String? location,
    DateTime? phoneVerifiedAt,
    DateTime? dateOfBirth,
  }) async {
    // Build request body with only non-null fields
    final body = <String, dynamic>{};
    if (username != null) body['username'] = username;
    if (bio != null) body['bio'] = bio;
    if (photoUrl != null) body['avatar_url'] = photoUrl;
    if (phoneNumber != null) body['phone_number'] = phoneNumber;
    if (location != null) body['location'] = location;

    return executeRequest(
      () => apiClient.patch('/users/me/profile', data: body),
      parser: (data) => data,
    );
  }

  /// Update user role (e.g., buyer -> seller upgrade)
  Future<Result<Map<String, dynamic>>> updateUserRole({
    required String userId,
    required String role,
  }) async {
    return executeRequest(
      () => apiClient.patch('/users/$userId/role', data: {'role': role}),
      parser: (data) => data,
    );
  }

  /// Deactivate user account
  Future<Result<Map<String, dynamic>>> deactivateAccount({
    required String userId,
    required String reason,
  }) async {
    return executeRequest(
      () =>
          apiClient.post('/users/$userId/deactivate', data: {'reason': reason}),
      parser: (data) => data,
    );
  }

  /// Get seller subscription info
  Future<Result<Map<String, dynamic>>> getSellerSubscription({
    required String userId,
  }) async {
    return executeRequest(
      () => apiClient.get('/users/$userId/subscription'),
      parser: (data) => data,
    );
  }

  /// Self-delete the authenticated user's account (soft delete on backend).
  /// Returns Success(null) on HTTP 204, Failure on any error.
  Future<Result<void>> selfDeleteAccount() async {
    return executeRequest(() => apiClient.delete('/users/me'), parser: (_) {});
  }

  /// Rotate a platform refresh token via POST /auth/refresh.
  ///
  /// The old [refreshToken] is consumed atomically. Both returned tokens
  /// are new single-use values that must be persisted and used on subsequent
  /// calls. Callers must discard [refreshToken] immediately after this call.
  ///
  /// Returns [BackendRefreshResponse] with new access_token + refresh_token.
  Future<Result<BackendRefreshResponse>> refreshPlatformToken(
    String refreshToken,
  ) async {
    return executeRequest(
      () => apiClient.post(
        '/auth/refresh',
        data: {'refresh_token': refreshToken},
        options: Options(extra: {'skipAuth': true}),
      ),
      parser: (data) =>
          BackendRefreshResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Revoke the current refresh session family via POST /auth/logout.
  ///
  /// The caller must pass the current refresh token so the backend can
  /// revoke only the current session family and optionally deactivate the
  /// current FCM token / device pairing.
  Future<Result<void>> logoutCurrentSession({
    required String refreshToken,
    String? fcmToken,
    String? deviceId,
  }) async {
    final body = <String, dynamic>{'refresh_token': refreshToken};
    if (fcmToken != null && fcmToken.isNotEmpty) {
      body['fcm_token'] = fcmToken;
    }
    if (deviceId != null && deviceId.isNotEmpty) {
      body['device_id'] = deviceId;
    }

    return executeRequest(
      () => apiClient.post('/auth/logout', data: body),
      parser: (_) {},
    );
  }

  /// Revoke all refresh sessions via POST /auth/logout-all.
  ///
  /// The caller does not need to provide a refresh token. FCM deactivation is
  /// optional and defaults to true on the backend.
  Future<Result<void>> logoutAllSessions({
    bool deactivateFcmTokens = true,
  }) async {
    return executeRequest(
      () => apiClient.post(
        '/auth/logout-all',
        data: {'deactivate_fcm_tokens': deactivateFcmTokens},
      ),
      parser: (_) {},
    );
  }

  /// List active sessions via GET /auth/sessions.
  ///
  /// Returns safe device/session snapshot fields only.
  /// Sensitive fields (token_hash, jti, ip_hash) are never included.
  Future<Result<List<AuthSessionDto>>> getActiveSessions() async {
    return executeRequest(
      () => apiClient.get('/auth/sessions'),
      parser: (data) {
        final raw = data as Map<String, dynamic>;
        final list = raw['sessions'] as List<dynamic>? ?? [];
        return list
            .whereType<Map<String, dynamic>>()
            .map(AuthSessionDto.fromJson)
            .toList();
      },
    );
  }

  /// Revoke a single session family via DELETE /auth/sessions/:family_id.
  ///
  /// The family_id is scoped to the authenticated user on the backend.
  Future<Result<void>> revokeSession(String familyId) async {
    return executeRequest(
      () => apiClient.delete('/auth/sessions/$familyId'),
      parser: (_) {},
    );
  }
}
