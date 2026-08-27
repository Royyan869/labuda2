import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';

/// Use Case: Mark Messages as Read
///
/// Marks all messages in a chat as read for a user.
class MarkMessagesAsReadUseCase {
  final ChatRepository _repository;

  MarkMessagesAsReadUseCase(this._repository);

  Future<Result<bool>> call({
    required String chatId,
    required String userId,
  }) async {
    try {
      final result = await _repository.markMessagesAsRead(
        chatId: chatId,
        userId: userId,
      );
      return result;
    } catch (e) {
      return Result.error('Failed to mark messages as read: $e');
    }
  }
}
