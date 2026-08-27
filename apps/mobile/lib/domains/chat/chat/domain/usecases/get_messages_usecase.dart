import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';

/// Use Case: Get Messages for Chat
///
/// Retrieves messages for a specific chat with pagination support.
class GetMessagesUseCase {
  final ChatRepository _repository;

  GetMessagesUseCase(this._repository);

  Future<Result<List<Message>>> call({
    required String chatId,
    required String userId,
    int page = 1,
    int limit = 50,
    DateTime? cursorCreatedAt,
    String? cursorId,
  }) async {
    try {
      final result = await _repository.getMessages(
        chatId: chatId,
        userId: userId,
        page: page,
        limit: limit,
        cursorCreatedAt: cursorCreatedAt,
        cursorId: cursorId,
      );
      return result;
    } catch (e) {
      return Result.error('Failed to get messages: $e');
    }
  }
}
