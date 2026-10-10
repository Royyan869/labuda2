import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:hishumi/domains/chat/chat/domain/repositories/chat_repository.dart';

/// Use Case: Get Chat by ID
///
/// Retrieves a single chat entity by its ID.
class GetChatUseCase {
  final ChatRepository _repository;

  GetChatUseCase(this._repository);

  Future<Result<Chat>> call(String chatId) async {
    try {
      final result = await _repository.getChatById(chatId);
      return result;
    } catch (e) {
      return Result.error('Failed to get chat: $e');
    }
  }
}
