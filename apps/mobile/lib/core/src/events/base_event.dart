abstract class BaseEvent {
  final String eventId;
  final DateTime timestamp;
  final String? userId;
  final Map<String, dynamic>? metadata;

  const BaseEvent({
    required this.eventId,
    required this.timestamp,
    this.userId,
    this.metadata,
  });

  String get eventType;

  Map<String, dynamic> toJson() {
    return {
      'eventId': eventId,
      'eventType': eventType,
      'timestamp': timestamp.toIso8601String(),
      'userId': userId,
      'metadata': metadata,
    };
  }

  @override
  String toString() {
    return '$eventType(eventId: $eventId, timestamp: $timestamp, userId: $userId)';
  }
}

class UserEvent extends BaseEvent {
  const UserEvent({
    required super.eventId,
    required super.timestamp,
    super.userId,
    super.metadata,
  });

  @override
  String get eventType => 'user_event';
}

class ListingEvent extends BaseEvent {
  final String listingId;

  const ListingEvent({
    required this.listingId,
    required super.eventId,
    required super.timestamp,
    super.userId,
    super.metadata,
  });

  @override
  String get eventType => 'listing_event';
}

class ChatEvent extends BaseEvent {
  final String conversationId;

  const ChatEvent({
    required this.conversationId,
    required super.eventId,
    required super.timestamp,
    super.userId,
    super.metadata,
  });

  @override
  String get eventType => 'chat_event';
}

class PaymentEvent extends BaseEvent {
  final String transactionId;

  const PaymentEvent({
    required this.transactionId,
    required super.eventId,
    required super.timestamp,
    super.userId,
    super.metadata,
  });

  @override
  String get eventType => 'payment_event';
}

class NavigationEvent extends BaseEvent {
  final String route;
  final Map<String, dynamic>? parameters;

  const NavigationEvent({
    required this.route,
    this.parameters,
    required super.eventId,
    required super.timestamp,
    super.userId,
    super.metadata,
  });

  @override
  String get eventType => 'navigation_event';
}
