// Runtime Honesty Tier 4 — WebSocketService.
//
// Verifies the runtime contract:
//   1. `_resubscribeRooms` failure REMOVES the failed room from
//      `_subscribedRooms` so subscription state no longer lies. The
//      higher-level chat repo's resubscribe (triggered by the
//      now-deferred `connected` broadcast) becomes the second-pass
//      safety net.
//   2. `_handleMessage` parse failures emit a stream error on the
//      `messages` stream (instead of being silently `developer.log`'d)
//      AND the controller stays open so subsequent valid frames still
//      flow through.

import 'dart:async';
import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import 'package:labuda/core/websocket/websocket_message.dart';
import 'package:labuda/core/websocket/websocket_service.dart';

// ---------------------------------------------------------------------------
// Fake channel/sink that throws synchronously on every send so the
// resubscribe pass takes the catch branch deterministically (no need
// to wait 10s for an ACK timeout).
// ---------------------------------------------------------------------------

class _ThrowingSink implements WebSocketSink {
  int addCount = 0;

  @override
  void add(dynamic event) {
    addCount++;
    throw StateError('simulated channel sink failure');
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _ThrowingChannel implements WebSocketChannel {
  final _ThrowingSink _sink = _ThrowingSink();

  @override
  WebSocketSink get sink => _sink;

  @override
  Stream<dynamic> get stream => const Stream<dynamic>.empty();

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

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
  group('WebSocketService — resubscribe honesty (B1)', () {
    test('resubscribe failure removes the room from _subscribedRooms so '
        'state no longer lies, allowing the second-pass resubscribe to '
        're-attempt it cleanly', () async {
      final service = WebSocketService(baseUrl: 'ws://example.invalid');

      service.primeSubscribedRoomsForTest(['room_alpha', 'room_beta']);
      service.markConnectedForTest();
      service.setChannelForTest(_ThrowingChannel());

      // Before the pass: both rooms are "subscribed".
      expect(
        service.subscribedRoomsForTest,
        containsAll(['room_alpha', 'room_beta']),
      );

      await service.resubscribeRoomsForTest();

      expect(
        service.subscribedRoomsForTest,
        isEmpty,
        reason:
            'Every room whose resubscribe failed must be removed from '
            '_subscribedRooms. The chat-repo layer keeps the high-level '
            'intent and re-issues subscribeToRoom on the next connected '
            'broadcast.',
      );
    });
  });

  group('WebSocketService — message parse honesty (C1)', () {
    test('malformed frame emits stream error and the service keeps the '
        'messages stream open for subsequent valid frames', () async {
      final service = WebSocketService(baseUrl: 'ws://example.invalid');
      service.primeMessageControllerForTest();

      final errors = <Object>[];
      final messages = <WebSocketMessage>[];
      final sub = service.messages.listen(messages.add, onError: errors.add);

      // 1) Drive a malformed frame — the parse error path must emit
      // a stream error instead of swallowing it.
      service.handleMessageForTest('this is not JSON');

      // Yield to let the stream deliver the error.
      await Future<void>.delayed(Duration.zero);

      expect(
        errors,
        hasLength(1),
        reason: 'malformed frame must surface as a stream error',
      );
      expect(errors.single, isA<FormatException>());

      // 2) Drive a VALID frame next — the controller must still be
      // open and deliver the message normally.
      service.handleMessageForTest(
        jsonEncode({
          'id': 'msg_recovered',
          'type': 'chat',
          'timestamp': DateTime.utc(2026, 5, 16).toIso8601String(),
          'from': 'user_1',
          'data': {'content': 'after the parse error'},
        }),
      );

      await Future<void>.delayed(Duration.zero);

      expect(
        messages,
        hasLength(1),
        reason:
            'service must remain alive after a parse error — '
            'subsequent valid frames still flow through',
      );
      expect(messages.single.id, equals('msg_recovered'));

      await sub.cancel();
    });
  });

  group('WebSocketService — subscribe ACK contract (P0)', () {
    test(
      'subscribe sends canonical envelope with raw UUID room_id and ACK message_id resolves pending request',
      () async {
        final service = WebSocketService(baseUrl: 'ws://example.invalid');
        final channel = _RecordingChannel();
        service.setChannelForTest(channel);
        service.markConnectedForTest();

        const roomId = '123e4567-e89b-12d3-a456-426614174000';
        final subscribeFuture = service.subscribeToRoom(roomId);
        await Future<void>.delayed(Duration.zero);

        expect(channel.sent, isNotEmpty);
        final outbound =
            jsonDecode(channel.sent.last as String) as Map<String, dynamic>;
        expect(outbound['type'], MessageType.subscribe);
        expect((outbound['data'] as Map<String, dynamic>)['room_id'], roomId);

        final outboundId = outbound['id'] as String;
        service.handleMessageForTest(
          jsonEncode({
            'id': 'server-ack-1',
            'type': MessageType.ack,
            'timestamp': DateTime.utc(2026, 6, 2).toIso8601String(),
            'from': 'server',
            'data': {
              'message_id': outboundId,
              'action': 'subscribe',
              'room_id': roomId,
            },
          }),
        );

        await subscribeFuture;
      },
    );

    test('unsubscribe uses same ACK correlation with message_id', () async {
      final service = WebSocketService(baseUrl: 'ws://example.invalid');
      final channel = _RecordingChannel();
      service.setChannelForTest(channel);
      service.markConnectedForTest();

      const roomId = '123e4567-e89b-12d3-a456-426614174001';
      final subscribeFuture = service.subscribeToRoom(roomId);
      await Future<void>.delayed(Duration.zero);
      final subscribeOutbound =
          jsonDecode(channel.sent.last as String) as Map<String, dynamic>;
      service.handleMessageForTest(
        jsonEncode({
          'id': 'server-ack-sub',
          'type': MessageType.ack,
          'timestamp': DateTime.utc(2026, 6, 2).toIso8601String(),
          'from': 'server',
          'data': {
            'message_id': subscribeOutbound['id'],
            'action': 'subscribe',
            'room_id': roomId,
          },
        }),
      );
      await subscribeFuture;

      final unsubscribeFuture = service.unsubscribeFromRoom(roomId);
      await Future<void>.delayed(Duration.zero);
      final unsubscribeOutbound =
          jsonDecode(channel.sent.last as String) as Map<String, dynamic>;
      expect(unsubscribeOutbound['type'], MessageType.unsubscribe);
      expect(
        (unsubscribeOutbound['data'] as Map<String, dynamic>)['room_id'],
        roomId,
      );

      service.handleMessageForTest(
        jsonEncode({
          'id': 'server-ack-unsub',
          'type': MessageType.ack,
          'timestamp': DateTime.utc(2026, 6, 2).toIso8601String(),
          'from': 'server',
          'data': {
            'message_id': unsubscribeOutbound['id'],
            'action': 'unsubscribe',
            'room_id': roomId,
          },
        }),
      );
      await unsubscribeFuture;
    });
  });
}
