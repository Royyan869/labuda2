import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';

/// Use Case: Send Message
///
/// Sends a message in a chat. A commerce product attachment travels as a
/// canonical [ChatResourceOccurrenceRequest] (`direct_commerce_insert_chat`).
class SendMessageUseCase {
  final ChatRepository _repository;

  SendMessageUseCase(this._repository);

  Future<Result<Message>> call({
    required String chatId,
    required String senderId,
    required String senderName,
    required String content,
    MessageType type = MessageType.text,
    List<String> mediaAssetIds = const [],
    ChatResourceOccurrenceRequest? resourceOccurrence,
  }) async {
    try {
      final result = await _repository.sendMessage(
        chatId: chatId,
        senderId: senderId,
        senderName: senderName,
        content: content,
        type: type,
        mediaAssetIds: mediaAssetIds,
        resourceOccurrence: resourceOccurrence,
      );
      return result;
    } catch (e) {
      return Result.error('Failed to send message: $e');
    }
  }
}
