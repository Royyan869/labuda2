import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';

/// Use Case: Manage User Presence
///
/// Manages presence tracking for online/offline status.
class ManagePresenceUseCase {
  final ChatRepository _repository;

  ManagePresenceUseCase(this._repository);

  Future<Result<bool>> startTracking(String userId) async {
    try {
      final result = await _repository.startPresenceTracking(userId);
      return result;
    } catch (e) {
      return Result.error('Failed to start presence tracking: $e');
    }
  }

  Future<Result<bool>> stopTracking(String userId) async {
    try {
      final result = await _repository.stopPresenceTracking(userId);
      return result;
    } catch (e) {
      return Result.error('Failed to stop presence tracking: $e');
    }
  }
}
