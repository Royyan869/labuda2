import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';

/// Get or Create Commerce Chat Use Case
///
/// **DOMAIN:** Chat domain
/// **RESPONSIBILITY:** Handle commerce chat operations (get/create room + link order)
/// **BOUNDARY:** Encapsulates the business logic of creating chat context for commerce
class GetOrCreateCommerceChatUseCase {
  final ChatRepository _chatRepository;

  GetOrCreateCommerceChatUseCase(this._chatRepository);

  /// Execute the use case
  ///
  /// Gets or creates a chat room between two participants and links an order to it.
  /// This encapsulates the business logic of:
  /// 1. Finding existing direct commerce room or creating new one
  /// 2. Linking the order to the chat for commerce continuity
  Future<Result<Chat>> call({
    required String currentUserId,
    required String otherUserId,
    required String orderId,
  }) async {
    try {
      // Step 1: Find or create canonical direct commerce room
      final roomResult = await _chatRepository.getOrCreateChat(
        participantIds: [currentUserId, otherUserId],
      );

      if (roomResult.isError) {
        return Result.error(roomResult.error ?? 'Failed to get or create chat');
      }

      final room = roomResult.data!;

      // Step 2: Link order to chat (LATEST ACTIVE ORDER RULE)
      final linkResult = await _chatRepository.linkOrderToChat(
        roomId: room.id,
        orderId: orderId,
      );

      if (linkResult.isError) {
        // Non-fatal: link failed but room was created, continue
      }

      return Result.success(room);
    } catch (e) {
      return Result.error('Failed to create commerce chat: $e');
    }
  }
}
