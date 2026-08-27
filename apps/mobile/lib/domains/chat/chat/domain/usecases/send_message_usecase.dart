import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';

/// Use Case: Send Message
///
/// Sends a message in a chat. Supports text messages and object attachments.
class SendMessageUseCase {
  final ChatRepository _repository;

  SendMessageUseCase(this._repository);

  Future<Result<Message>> call({
    required String chatId,
    required String senderId,
    required String senderName,
    required String content,
    MessageType type = MessageType.text,
    ShareReference? objectReference,
    Map<String, dynamic>? workflowAttachment,
  }) async {
    try {
      final result = await _repository.sendMessage(
        chatId: chatId,
        senderId: senderId,
        senderName: senderName,
        content: content,
        type: type,
        objectReference: objectReference,
        workflowAttachment: workflowAttachment,
      );
      return result;
    } catch (e) {
      return Result.error('Failed to send message: $e');
    }
  }
}
