import 'package:labuda/core/common/result.dart';
import 'package:labuda/shared/chat/chat_gateway.dart';

/// Link Order to Chat Use Case
///
/// **DOMAIN:** Commerce → Transaction
/// **RESPONSIBILITY:** Request order linking to chat through gateway
/// **BOUNDARY:** Commerce domain uses ChatGateway interface (no direct chat dependency)
///
/// **RULES:**
/// - Commerce domain requests linking through ChatGateway
/// - No direct dependency on chat repository
/// - Uses interface-based dependency inversion
class LinkOrderToChatUseCase {
  final ChatGateway _chatGateway;

  LinkOrderToChatUseCase(this._chatGateway);

  /// Execute the use case
  ///
  /// Links an order to a chat for commerce continuity
  /// This enables showing order status in chat context
  Future<Result<void>> execute(String roomId, String orderId) async {
    try {
      await _chatGateway.linkOrderToChat(roomId, orderId);
      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to link order to chat: $e');
    }
  }

  /// Unlink an order from chat
  ///
  /// Removes the order link when order is completed/cancelled
  Future<Result<void>> unlink(String roomId) async {
    try {
      // Gateway handles unlinking
      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to unlink order: $e');
    }
  }
}
