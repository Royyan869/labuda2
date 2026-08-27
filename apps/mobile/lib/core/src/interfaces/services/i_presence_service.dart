import 'package:labuda/core/common/result.dart';

/// Interface untuk presence/online status tracking
/// Digunakan secara global di seluruh app
abstract class IPresenceService {
  /// Update user presence status
  Future<Result<bool>> updatePresence({
    required String userId,
    required bool isOnline,
  });

  /// Start presence tracking (called when app becomes active)
  Future<Result<bool>> startTracking(String userId);

  /// Stop presence tracking (called when app goes to background/inactive)
  Future<Result<bool>> stopTracking(String userId);

  /// Get user online status
  Future<Result<bool>> getUserOnlineStatus(String userId);

  /// Get user last seen time
  Future<Result<DateTime?>> getUserLastSeen(String userId);

  /// Watch single user's presence
  Stream<bool> watchUserPresence(String userId);

  /// Watch multiple users' presence
  Stream<Map<String, bool>> watchUsersPresence(List<String> userIds);
}
