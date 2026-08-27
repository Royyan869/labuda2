import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import 'package:labuda/core/websocket/websocket_message.dart';
import 'package:labuda/core/websocket/websocket_service.dart';

class _RecordingSink implements WebSocketSink {
  final List<dynamic> sent = <dynamic>[];

  @override
  void add(dynamic event) {
    sent.add(event);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _RecordingChannel implements WebSocketChannel {
  final _RecordingSink _sink = _RecordingSink();

  @override
  WebSocketSink get sink => _sink;

  @override
  Stream<dynamic> get stream => const Stream<dynamic>.empty();

  List<dynamic> get sent => _sink.sent;

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  group('presence protocol serialization', () {
    test('presence.resume serializes an empty payload', () async {
      final service = WebSocketService(baseUrl: 'ws://example.invalid');
      final channel = _RecordingChannel();
      service.setChannelForTest(channel);
      service.markConnectedForTest();

      await service.sendPresenceResume(requireAck: false);

      final outbound =
          jsonDecode(channel.sent.single as String) as Map<String, dynamic>;
      expect(outbound['type'], MessageType.presenceResume);
      expect(outbound['data'], isEmpty);
      expect(outbound.containsKey('require_ack'), isFalse);
    });

    test('presence.pause serializes an empty payload', () async {
      final service = WebSocketService(baseUrl: 'ws://example.invalid');
      final channel = _RecordingChannel();
      service.setChannelForTest(channel);
      service.markConnectedForTest();

      await service.sendPresencePause(requireAck: false);

      final outbound =
          jsonDecode(channel.sent.single as String) as Map<String, dynamic>;
      expect(outbound['type'], MessageType.presencePause);
      expect(outbound['data'], isEmpty);
      expect(outbound.containsKey('require_ack'), isFalse);
    });

    test('presence.leave serializes an empty payload', () async {
      final service = WebSocketService(baseUrl: 'ws://example.invalid');
      final channel = _RecordingChannel();
      service.setChannelForTest(channel);
      service.markConnectedForTest();

      await service.sendPresenceLeave(requireAck: false);

      final outbound =
          jsonDecode(channel.sent.single as String) as Map<String, dynamic>;
      expect(outbound['type'], MessageType.presenceLeave);
      expect(outbound['data'], isEmpty);
      expect(outbound.containsKey('require_ack'), isFalse);
    });

    test('presence.subscribe serializes canonical user_ids', () async {
      final service = WebSocketService(baseUrl: 'ws://example.invalid');
      final channel = _RecordingChannel();
      service.setChannelForTest(channel);
      service.markConnectedForTest();

      const userIds = <String>[
        '123e4567-e89b-12d3-a456-426614174010',
        '123e4567-e89b-12d3-a456-426614174011',
      ];

      await service.sendPresenceSubscribe(userIds, requireAck: false);

      final outbound =
          jsonDecode(channel.sent.single as String) as Map<String, dynamic>;
      expect(outbound['type'], MessageType.presenceSubscribe);
      expect((outbound['data'] as Map<String, dynamic>)['user_ids'], userIds);
      expect(outbound.containsKey('require_ack'), isFalse);
    });

    test('presence.unsubscribe serializes canonical user_ids', () async {
      final service = WebSocketService(baseUrl: 'ws://example.invalid');
      final channel = _RecordingChannel();
      service.setChannelForTest(channel);
      service.markConnectedForTest();

      const userIds = <String>[
        '123e4567-e89b-12d3-a456-426614174012',
        '123e4567-e89b-12d3-a456-426614174013',
      ];

      await service.sendPresenceUnsubscribe(userIds, requireAck: false);

      final outbound =
          jsonDecode(channel.sent.single as String) as Map<String, dynamic>;
      expect(outbound['type'], MessageType.presenceUnsubscribe);
      expect((outbound['data'] as Map<String, dynamic>)['user_ids'], userIds);
      expect(outbound.containsKey('require_ack'), isFalse);
    });
  });
}
