import 'package:labuda/core/api/base_api_repository.dart';
import 'package:labuda/core/api/models/common_api_models.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/social/follow/data/dto/follow_api_models.dart';

/// API Datasource for Follow, Block, and Mute operations
///
/// Provides HTTP operations for:
/// - Follow/Unfollow users
/// - Block/Unblock users
/// - Mute/Unmute users
/// - Query followers and following
/// - Get follow statistics and status
class FollowApiDatasource extends BaseApiRepository {
  static const String _basePath = '';

  FollowApiDatasource(super.apiClient, {super.logger});

  // ===========================================
  // FOLLOW OPERATIONS
  // ===========================================

  /// Follow a user
  ///
  /// Endpoint: POST /api/v1/users/{userId}/follow
  Future<Result<void>> followUser(String userId) async {
    logger?.info('Following user: $userId');

    return executeVoidRequest(
      () => apiClient.post('$_basePath/users/$userId/follow'),
    );
  }

  /// Unfollow a user
  ///
  /// Endpoint: DELETE /api/v1/users/{userId}/follow
  Future<Result<void>> unfollowUser(String userId) async {
    logger?.info('Unfollowing user: $userId');

    return executeVoidRequest(
      () => apiClient.delete('$_basePath/users/$userId/follow'),
    );
  }

  /// Get user's followers
  ///
  /// Endpoint: GET /api/v1/users/{userId}/followers
  /// Returns lifecycle-aware UserCard list. Uses cursor-based pagination
  /// matching the backend (limit + optional cursor RFC3339 timestamp).
  Future<Result<FollowListResponseDto>> getFollowers(
    String userId, {
    int limit = 20,
    String? cursor,
  }) async {
    logger?.info('Getting followers for user: $userId');

    return executeRequest(
      () => apiClient.get(
        '$_basePath/users/$userId/followers',
        queryParameters: {
          'limit': limit,
          ...?(cursor == null ? null : {'cursor': cursor}),
        },
      ),
      parser: (data) =>
          FollowListResponseDto.fromFollowersJson(data as Map<String, dynamic>),
    );
  }

  /// Get users that a user is following
  ///
  /// Endpoint: GET /api/v1/users/{userId}/following
  /// Returns lifecycle-aware UserCard list. Uses cursor-based pagination.
  Future<Result<FollowListResponseDto>> getFollowing(
    String userId, {
    int limit = 20,
    String? cursor,
  }) async {
    logger?.info('Getting following for user: $userId');

    return executeRequest(
      () => apiClient.get(
        '$_basePath/users/$userId/following',
        queryParameters: {
          'limit': limit,
          ...?(cursor == null ? null : {'cursor': cursor}),
        },
      ),
      parser: (data) =>
          FollowListResponseDto.fromFollowingJson(data as Map<String, dynamic>),
    );
  }

  /// Get follow statistics for a user.
  ///
  /// Canonical source: the public profile endpoint returns live counts
  /// derived from user_follows, so this method simply projects the count
  /// fields out of that response.
  Future<Result<FollowStatsApiResponse>> getFollowStats(String userId) async {
    logger?.info('Getting follow stats for user: $userId');

    return executeRequest(
      () => apiClient.get('$_basePath/users/$userId'),
      parser: (data) {
        final json = data as Map<String, dynamic>;
        return FollowStatsApiResponse(
          userId: json['id'] as String? ?? userId,
          followersCount: (json['followers_count'] as num?)?.toInt() ?? 0,
          followingCount: (json['following_count'] as num?)?.toInt() ?? 0,
        );
      },
    );
  }

  /// Check follow status between current user and target user
  ///
  /// Endpoint: GET /api/v1/follows/status/{userId}
  Future<Result<FollowStatusApiResponse>> getFollowStatus(String userId) async {
    logger?.info('Checking follow status with user: $userId');

    return executeRequest(
      () => apiClient.get('$_basePath/follows/status/$userId'),
      parser: (data) =>
          FollowStatusApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  // ===========================================
  // BLOCK OPERATIONS
  // ===========================================

  /// Block a user
  ///
  /// Endpoint: POST /api/v1/users/{userId}/block
  Future<Result<void>> blockUser(String userId) async {
    logger?.info('Blocking user: $userId');

    return executeVoidRequest(
      () => apiClient.post('$_basePath/users/$userId/block'),
    );
  }

  /// Unblock a user
  ///
  /// Endpoint: DELETE /api/v1/users/{userId}/block
  Future<Result<void>> unblockUser(String userId) async {
    logger?.info('Unblocking user: $userId');

    return executeVoidRequest(
      () => apiClient.delete('$_basePath/users/$userId/block'),
    );
  }

  /// Get list of blocked users
  ///
  /// Endpoint: GET /api/v1/blocks
  Future<Result<PaginatedApiResponse<BlockApiResponse>>> getBlockedUsers({
    int page = 1,
    int pageSize = 20,
  }) async {
    logger?.info('Getting blocked users (page: $page)');

    return executeRequest(
      () => apiClient.get(
        '$_basePath/blocks',
        queryParameters: {'page': page, 'page_size': pageSize},
      ),
      parser: (data) => PaginatedApiResponse.fromJson(
        data as Map<String, dynamic>,
        (json) => BlockApiResponse.fromJson(json as Map<String, dynamic>),
      ),
    );
  }

  // ===========================================
  // MUTE OPERATIONS
  // ===========================================

  /// Mute a user
  ///
  /// Endpoint: POST /api/v1/users/{userId}/mute
  Future<Result<void>> muteUser(String userId) async {
    logger?.info('Muting user: $userId');

    return executeVoidRequest(
      () => apiClient.post('$_basePath/users/$userId/mute'),
    );
  }

  /// Unmute a user
  ///
  /// Endpoint: DELETE /api/v1/users/{userId}/mute
  Future<Result<void>> unmuteUser(String userId) async {
    logger?.info('Unmuting user: $userId');

    return executeVoidRequest(
      () => apiClient.delete('$_basePath/users/$userId/mute'),
    );
  }

  // ===========================================
  // USER SEARCH
  // ===========================================

  /// Search users by username.
  ///
  /// Endpoint: GET /api/v1/search/users?q=&limit=&offset=
  /// Auth: required (v1 group has AuthMiddleware).
  /// Backend filters: account_status='active', deleted_at IS NULL,
  /// and blocked users are excluded using caller's viewerID.
  Future<Result<UserSearchResponseDto>> searchUsers(
    String query, {
    int limit = 20,
    int offset = 0,
  }) async {
    logger?.info('Searching users: $query');

    return executeRequest(
      () => apiClient.get(
        '$_basePath/search/users',
        queryParameters: {'q': query, 'limit': limit, 'offset': offset},
      ),
      parser: (data) =>
          UserSearchResponseDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Get list of muted users
  ///
  /// Endpoint: GET /api/v1/mutes
  Future<Result<PaginatedApiResponse<MuteApiResponse>>> getMutedUsers({
    int page = 1,
    int pageSize = 20,
  }) async {
    logger?.info('Getting muted users (page: $page)');

    return executeRequest(
      () => apiClient.get(
        '$_basePath/mutes',
        queryParameters: {'page': page, 'page_size': pageSize},
      ),
      parser: (data) => PaginatedApiResponse.fromJson(
        data as Map<String, dynamic>,
        (json) => MuteApiResponse.fromJson(json as Map<String, dynamic>),
      ),
    );
  }
}
