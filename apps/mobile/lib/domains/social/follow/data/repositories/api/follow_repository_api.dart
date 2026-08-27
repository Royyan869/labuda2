import 'dart:async';

import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';
import 'package:labuda/domains/social/follow/data/datasources/follow_api_datasource.dart';
import 'package:labuda/domains/social/follow/data/mappers/follow_api_mapper.dart';

/// API-based implementation of IFollowRepository
///
/// Handles Follow, Block, and Mute operations through the Go backend API.
/// Uses polling for real-time streams until WebSocket support is added.
class FollowRepositoryApi implements IFollowRepository {
  final FollowApiDatasource _datasource;
  final ILoggerService? _logger;

  // Polling timers for real-time streams
  final Map<String, Timer> _activePollingTimers = {};
  final Map<String, StreamController> _activeStreamControllers = {};

  FollowRepositoryApi(this._datasource, {ILoggerService? logger})
    : _logger = logger;

  // ===========================================
  // FOLLOW OPERATIONS
  // ===========================================

  @override
  Future<Result<bool>> followUser({
    required String followerId,
    required String followingId,
  }) async {
    _logger?.info('Following user: $followingId');

    final result = await _datasource.followUser(followingId);

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(true),
    );
  }

  @override
  Future<Result<bool>> unfollowUser({
    required String followerId,
    required String followingId,
  }) async {
    _logger?.info('Unfollowing user: $followingId');

    final result = await _datasource.unfollowUser(followingId);

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(true),
    );
  }

  @override
  Future<Result<List<FollowableUser>>> getFollowers({
    required String userId,
    int limit = 20,
    String? lastFollowId,
  }) async {
    _logger?.info('Getting followers for user: $userId');

    final result = await _datasource.getFollowers(
      userId,
      limit: limit,
      cursor: lastFollowId,
    );

    return result.fold(
      (error) => Result.error(error),
      (response) =>
          Result.success(FollowApiMapper.fromFollowListCards(response.items)),
    );
  }

  @override
  Future<Result<List<FollowableUser>>> getFollowing({
    required String userId,
    int limit = 20,
    String? lastFollowId,
  }) async {
    _logger?.info('Getting following for user: $userId');

    final result = await _datasource.getFollowing(
      userId,
      limit: limit,
      cursor: lastFollowId,
    );

    return result.fold(
      (error) => Result.error(error),
      (response) =>
          Result.success(FollowApiMapper.fromFollowListCards(response.items)),
    );
  }

  @override
  Future<Result<FollowStats>> getFollowStats({
    required String userId,
    String? currentUserId,
  }) async {
    _logger?.info('Getting follow stats for user: $userId');

    final result = await _datasource.getFollowStats(userId);

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(
        FollowStats(
          userId: response.userId,
          followersCount: response.followersCount,
          followingCount: response.followingCount,
          mutualFollowsCount: 0,
          topFollowers: const [],
          suggestedUsers: const [],
          lastUpdated: DateTime.now(),
        ),
      ),
    );
  }

  @override
  Future<Result<bool>> checkFollowStatus({
    required String followerId,
    required String followingId,
  }) async {
    _logger?.info('Checking follow status: $followerId -> $followingId');

    final result = await _datasource.getFollowStatus(followingId);

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(response.isFollowing),
    );
  }

  // ===========================================
  // BLOCK OPERATIONS
  // ===========================================

  @override
  Future<Result<bool>> blockUser({
    required String userId,
    required String targetUserId,
  }) async {
    _logger?.info('Blocking user: $targetUserId');

    final result = await _datasource.blockUser(targetUserId);

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(true),
    );
  }

  @override
  Future<Result<bool>> unblockUser({
    required String userId,
    required String targetUserId,
  }) async {
    _logger?.info('Unblocking user: $targetUserId');

    final result = await _datasource.unblockUser(targetUserId);

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(true),
    );
  }

  // ===========================================
  // MUTE OPERATIONS
  // ===========================================

  @override
  Future<Result<bool>> muteUser({
    required String userId,
    required String targetUserId,
  }) async {
    _logger?.info('Muting user: $targetUserId');

    final result = await _datasource.muteUser(targetUserId);

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(true),
    );
  }

  @override
  Future<Result<bool>> unmuteUser({
    required String userId,
    required String targetUserId,
  }) async {
    _logger?.info('Unmuting user: $targetUserId');

    final result = await _datasource.unmuteUser(targetUserId);

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(true),
    );
  }

  @override
  Future<Result<List<FollowableUser>>> searchUsers({
    required String query,
    String? currentUserId,
    UserType? filterByType,
    int limit = 20,
  }) async {
    _logger?.info('Searching users: $query');

    // filterByType is not supported by the backend search/users endpoint.
    // Backend matches by username only and returns all active, non-deleted users.
    final result = await _datasource.searchUsers(query, limit: limit);

    return result.fold(
      (error) => Result.error(error),
      (response) => Result.success(
        FollowApiMapper.fromUserSearchPreviews(response.users),
      ),
    );
  }

  // ===========================================
  // REAL-TIME STREAMS (POLLING-BASED)
  // ===========================================

  @override
  Stream<List<FollowableUser>> watchFollowers(String userId) {
    final key = 'followers_$userId';
    _cleanupExistingStream(key);

    final controller = StreamController<List<FollowableUser>>.broadcast(
      onCancel: () => _cleanupStream(key),
    );
    _activeStreamControllers[key] = controller;

    // Initial fetch
    _fetchAndEmitFollowers(userId, controller);

    // Poll every 30 seconds
    _activePollingTimers[key] = Timer.periodic(
      const Duration(seconds: 30),
      (_) => _fetchAndEmitFollowers(userId, controller),
    );

    return controller.stream;
  }

  @override
  Stream<List<FollowableUser>> watchFollowing(String userId) {
    final key = 'following_$userId';
    _cleanupExistingStream(key);

    final controller = StreamController<List<FollowableUser>>.broadcast(
      onCancel: () => _cleanupStream(key),
    );
    _activeStreamControllers[key] = controller;

    // Initial fetch
    _fetchAndEmitFollowing(userId, controller);

    // Poll every 30 seconds
    _activePollingTimers[key] = Timer.periodic(
      const Duration(seconds: 30),
      (_) => _fetchAndEmitFollowing(userId, controller),
    );

    return controller.stream;
  }

  @override
  Stream<FollowStats> watchFollowStats(String userId) {
    final key = 'followStats_$userId';
    _cleanupExistingStream(key);

    final controller = StreamController<FollowStats>.broadcast(
      onCancel: () => _cleanupStream(key),
    );
    _activeStreamControllers[key] = controller;

    // Initial fetch
    _fetchAndEmitFollowStats(userId, controller);

    // Poll every 15 seconds (more frequent for stats)
    _activePollingTimers[key] = Timer.periodic(
      const Duration(seconds: 15),
      (_) => _fetchAndEmitFollowStats(userId, controller),
    );

    return controller.stream;
  }

  @override
  Stream<List<FollowActivity>> watchFollowActivities(String userId) {
    // TODO: Implement when backend API supports follow activities
    _logger?.info('watchFollowActivities not yet implemented in API');
    return Stream.value([]);
  }

  // ===========================================
  // STREAM HELPERS
  // ===========================================

  void _fetchAndEmitFollowers(
    String userId,
    StreamController<List<FollowableUser>> controller,
  ) async {
    if (controller.isClosed) return;

    final result = await getFollowers(userId: userId);
    result.fold(
      (error) => _logger?.warning('Failed to fetch followers: $error'),
      (followers) {
        if (!controller.isClosed) {
          controller.add(followers);
        }
      },
    );
  }

  void _fetchAndEmitFollowing(
    String userId,
    StreamController<List<FollowableUser>> controller,
  ) async {
    if (controller.isClosed) return;

    final result = await getFollowing(userId: userId);
    result.fold(
      (error) => _logger?.warning('Failed to fetch following: $error'),
      (following) {
        if (!controller.isClosed) {
          controller.add(following);
        }
      },
    );
  }

  void _fetchAndEmitFollowStats(
    String userId,
    StreamController<FollowStats> controller,
  ) async {
    if (controller.isClosed) return;

    final result = await getFollowStats(userId: userId);
    result.fold(
      (error) => _logger?.warning('Failed to fetch follow stats: $error'),
      (stats) {
        if (!controller.isClosed) {
          controller.add(stats);
        }
      },
    );
  }

  void _cleanupStream(String key) {
    _activePollingTimers[key]?.cancel();
    _activePollingTimers.remove(key);
    _activeStreamControllers.remove(key);
  }

  void _cleanupExistingStream(String key) {
    _activePollingTimers[key]?.cancel();
    _activePollingTimers.remove(key);
    _activeStreamControllers[key]?.close();
    _activeStreamControllers.remove(key);
  }

  /// Cleanup all active streams and timers
  void dispose() {
    for (final timer in _activePollingTimers.values) {
      timer.cancel();
    }
    _activePollingTimers.clear();

    for (final controller in _activeStreamControllers.values) {
      controller.close();
    }
    _activeStreamControllers.clear();
  }
}
