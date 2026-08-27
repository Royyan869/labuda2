import 'dart:math';

class WebSocketMessage {
  final String id;
  final String type;
  final DateTime timestamp;
  final String from;
  final String? to;
  final String? roomId;
  final Map<String, dynamic> data;
  bool requireAck;

  WebSocketMessage({
    String? id,
    required this.type,
    DateTime? timestamp,
    required this.from,
    this.to,
    this.roomId,
    required this.data,
    this.requireAck = false,
  }) : id = id ?? _generateId(),
       timestamp = timestamp ?? DateTime.now();

  factory WebSocketMessage.fromJson(Map<String, dynamic> json) {
    return WebSocketMessage(
      id: json['id'] as String,
      type: json['type'] as String,
      timestamp: DateTime.parse(json['timestamp'] as String),
      from: json['from'] as String,
      to: json['to'] as String?,
      roomId: json['room_id'] as String?,
      data: json['data'] as Map<String, dynamic>,
      requireAck: json['require_ack'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'type': type,
      'timestamp': timestamp.toIso8601String(),
      'from': from,
      if (to != null) 'to': to,
      if (roomId != null) 'room_id': roomId,
      'data': data,
      if (requireAck) 'require_ack': true,
    };
  }

  static String _generateId() {
    final random = Random();
    return '${DateTime.now().millisecondsSinceEpoch}_${random.nextInt(99999)}';
  }
}

// Message types constants
class MessageType {
  static const String chat = 'chat';
  static const String auctionBid = 'auction_bid';
  static const String notification = 'notification';
  static const String presence = 'presence';
  static const String typing = 'typing';
  static const String subscribe = 'subscribe';
  static const String unsubscribe = 'unsubscribe';
  static const String ack = 'ack';
  static const String error = 'error';
  static const String ping = 'ping';
  static const String pong = 'pong';
}
