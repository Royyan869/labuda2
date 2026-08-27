import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/user/profile/data/models/blocked_user_model.dart';
import 'package:labuda/domains/user/profile/data/services/user_lookup_service.dart';

/// Service for managing blocked users.
///
/// Canonical backend routes (backend/cmd/core_server/routes_core.go):
///   GET    /api/v1/blocks            list blocked-user UUIDs
///   POST   /api/v1/users/:id/block   block a user
///   DELETE /api/v1/users/:id/block   unblock a user
///
/// Backend has no block-status check endpoint. Callers must derive
/// "is X blocked" from the cached UUID set surfaced by
/// `blockedUserIdsProvider` / `isUserBlockedProvider`.
class BlockedUsersService {
  final ApiClient _apiClient;
  final UserLookupService? _userLookupService;
  final ILoggerService? _logger;

  BlockedUsersService(
    this._apiClient, {
    UserLookupService? userLookupService,
    ILoggerService? logger,
  }) : _userLookupService = userLookupService,
       _logger = logger;

  /// Stream blocked users (API not realtime; one-shot via Stream.fromFuture).
  Stream<List<BlockedUserModel>> watchBlockedUsers(String userId) {
    return Stream.fromFuture(getBlockedUsers());
  }

  /// GET /api/v1/blocks
  ///
  /// Wire payload: `{data: {blocked: [uuid, ...], limit: N}}`. The backend
  /// returns UUID strings only — no display name / avatar / timestamp — so
  /// each row materializes as a minimal [BlockedUserModel]. The filter
  /// cache only needs `.id`; manage-list UX that wants rich identity must
  /// hydrate per-UUID via a separate user-detail fetch.
  Future<List<BlockedUserModel>> getBlockedUsers() async {
    try {
      final response = await _apiClient.get('/blocks');
      final body = response.data as Map<String, dynamic>;
      final data = body['data'] as Map<String, dynamic>?;
      final list = data?['blocked'] as List<dynamic>?;
      if (list == null) return [];
      final blockedIds = list.cast<String>();
      final now = DateTime.now();
      final lookupService = _userLookupService;

      if (lookupService == null) {
        return blockedIds
            .map(
              (id) => BlockedUserModel(
                id: id,
                username: id.substring(0, id.length > 8 ? 8 : id.length),
                avatarUrl: null,
                blockedAt: now,
              ),
            )
            .toList();
      }

      final users = await lookupService.getUsersByIds(blockedIds);
      final byId = {for (final user in users) user.userId: user};

      return blockedIds
          .map(
            (id) => BlockedUserModel(
              id: id,
              username:
                  byId[id]?.username ??
                  id.substring(0, id.length > 8 ? 8 : id.length),
              avatarUrl: byId[id]?.avatarUrl,
              blockedAt: now,
            ),
          )
          .toList();
    } catch (e) {
      _logger?.error(
        'Failed to get blocked users',
        extra: {'error': e.toString()},
      );
      return [];
    }
  }

  /// POST /api/v1/users/:id/block
  ///
  /// Backend identifies the blocker from the JWT subject; no body needed.
  /// The username / avatar params are retained on this method
  /// surface for caller stability — they are intentionally not sent.
  Future<void> blockUser({
    required String currentUserId,
    required String blockedUserId,
    required String blockedUserUsername,
    String? blockedUserAvatarUrl,
  }) async {
    try {
      await _apiClient.post('/users/$blockedUserId/block');
    } catch (e) {
      _logger?.error('Failed to block user', extra: {'error': e.toString()});
      rethrow;
    }
  }

  /// DELETE /api/v1/users/:id/block
  Future<void> unblockUser({
    required String currentUserId,
    required String blockedUserId,
  }) async {
    try {
      await _apiClient.delete('/users/$blockedUserId/block');
    } catch (e) {
      _logger?.error('Failed to unblock user', extra: {'error': e.toString()});
      rethrow;
    }
  }
}
