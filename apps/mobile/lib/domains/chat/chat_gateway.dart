import 'package:labuda/shared/attachment/entities/share_reference.dart';

/// Chat Gateway Interface
///
/// Provides abstraction layer for commerce→chat communication.
/// Commerce domain can interact with chat through this interface
/// without creating circular dependencies.
abstract class ChatGateway {
  /// Link order to chat for commerce continuity
  Future<void> linkOrderToChat(String chatId, String orderId);

  /// Get for-sale share reference for chat attachments
  Future<ShareReference?> getForSaleShareReference(String forSaleId);

  /// Stream chat state changes
  Stream<Map<String, dynamic>> watchChatState(String chatId);
}

/// Chat Event Bus
///
/// Event-based communication between chat and commerce domains.
/// Eliminates direct dependencies and enables loose coupling.
class ChatEventBus {
  static final ChatEventBus _instance = ChatEventBus._internal();
  factory ChatEventBus() => _instance;
  ChatEventBus._internal();

  final Map<String, List<Function>> _listeners = {};

  /// Subscribe to chat events
  void subscribe(String eventType, Function callback) {
    _listeners.putIfAbsent(eventType, () => []).add(callback);
  }

  /// Publish chat event
  void publish(String eventType, dynamic data) {
    final listeners = _listeners[eventType];
    if (listeners != null) {
      for (final listener in listeners) {
        try {
          listener(data);
        } catch (_) {
          // Continue notifying other listeners on error
        }
      }
    }
  }

  /// Unsubscribe from chat events
  void unsubscribe(String eventType, Function callback) {
    final listeners = _listeners[eventType];
    if (listeners != null) {
      listeners.remove(callback);
    }
  }
}

/// Chat Event Types
class ChatEventTypes {
  static const String orderLinked = 'order_linked';
  static const String forSaleAttached = 'for_sale_attached';
  static const String messageSent = 'message_sent';
  static const String chatCreated = 'chat_created';
}
