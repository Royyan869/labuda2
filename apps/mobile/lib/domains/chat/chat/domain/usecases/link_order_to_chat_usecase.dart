import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';

/// Link Order to Chat Use Case
///
/// **DOMAIN:** Chat domain
/// **RESPONSIBILITY:** Link orders to chat conversations for commerce continuity
/// **BOUNDARY:** Chat domain manages its own data, commerce domain requests linking
class LinkOrderToChatUseCase {
  final ChatRepository _chatRepository;

  LinkOrderToChatUseCase(this._chatRepository);

  /// Execute the use case
  ///
  /// Links an order to a chat for commerce continuity
  /// This enables showing order status in chat context
  Future<Result<void>> call({
    required String chatId,
    required String orderId,
  }) async {
    try {
      final result = await _chatRepository.linkOrderToChat(
        roomId: chatId,
        orderId: orderId,
      );

      if (result.isSuccess) {
        // Publish event for commerce domain to listen
        return Result.success(null);
      } else {
        return Result.error(result.error ?? 'Failed to link order to chat');
      }
    } catch (e) {
      return Result.error('Failed to link order to chat: $e');
    }
  }

  /// Unlink an order from chat
  ///
  /// Removes the order link when order is completed/cancelled
  Future<Result<void>> unlink(String chatId) async {
    try {
      // This would call a repository method to remove the link
      // For now, returning success as placeholder
      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to unlink order: $e');
    }
  }
}
