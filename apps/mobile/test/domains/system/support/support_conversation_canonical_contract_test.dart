/// SUPPORT SCOPE 2 — canonical mobile conversation contract.
///
/// Proves that the mobile conversation path:
///  - reads messages from the Support API (`GET /support/tickets/:id/messages`),
///  - unwraps the canonical `{success, data: {data: [...]}}` envelope,
///  - parses the server-provided canonical `sender_type` / `message_type`
///    instead of guessing the sender from a UUID or local state,
///  - sends the user's reply through the Support API with ONLY the message text
///    (never a client-supplied sender identity).
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/system/support/data/datasources/support_api_datasource.dart';
import 'package:labuda/domains/system/support/data/repositories/support_repository_api.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';

/// Captures every outgoing request and answers with a canned response.
class _CapturingSupportAdapter implements HttpClientAdapter {
  _CapturingSupportAdapter({
    required this.responseBody,
    this.statusCode = 200,
  });

  final Object responseBody;
  final int statusCode;
  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    return ResponseBody.fromString(
      jsonEncode(responseBody),
      statusCode,
      headers: {
        'content-type': ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

SupportRepositoryApi _repository(_CapturingSupportAdapter adapter) {
  final client = ApiClient(logger: null, baseUrl: 'https://labuda.test');
  client.dio.httpClientAdapter = adapter;
  return SupportRepositoryApi(datasource: SupportApiDatasource(client));
}

void main() {
  group('Support conversation — canonical read contract', () {
    test('reads the conversation and parses the canonical sender taxonomy',
        () async {
      final adapter = _CapturingSupportAdapter(
        responseBody: {
          'success': true,
          'data': {
            'data': [
              {
                'id': 'msg-1',
                'room_id': 'room-1',
                'sender_id': 'user-1',
                'sender_type': 'user',
                'message_type': 'text',
                'body': 'Halo, pembayaran saya gagal.',
                'created_at': '2026-09-19T00:00:00Z',
              },
              {
                'id': 'msg-2',
                'room_id': 'room-1',
                'sender_id': 'admin-1',
                'sender_type': 'admin',
                'message_type': 'text',
                'body': 'Baik, kami cek sekarang.',
                'created_at': '2026-09-19T00:05:00Z',
              },
            ],
          },
        },
      );
      final repository = _repository(adapter);

      final result = await repository.getMessages('ticket-1');

      expect(result.isSuccess, isTrue, reason: result.failure?.message ?? '');
      final messages = result.dataOrThrow;
      expect(messages, hasLength(2));

      expect(messages[0].senderType, SupportSenderType.user);
      expect(messages[0].body, 'Halo, pembayaran saya gagal.');
      expect(messages[1].senderType, SupportSenderType.admin);
      expect(messages[1].body, 'Baik, kami cek sekarang.');

      // The user and the agent are distinguishable from the persisted
      // server-provided sender data alone — not from client-side state.
      expect(messages[0].senderType, isNot(messages[1].senderType));
      expect(messages[0].senderId, 'user-1');
      expect(messages[1].senderId, 'admin-1');

      final request = adapter.requests.single;
      expect(request.method, 'GET');
      expect(request.path, '/support/tickets/ticket-1/messages');
    });

    test('tolerates the nested envelope and a bare array', () async {
      final nested = _CapturingSupportAdapter(
        responseBody: {
          'success': true,
          'data': {
            'data': [
              {
                'id': 'msg-1',
                'room_id': 'room-1',
                'sender_id': 'admin-1',
                'sender_type': 'admin',
                'message_type': 'text',
                'body': 'hai',
                'created_at': '2026-09-19T00:00:00Z',
              },
            ],
          },
        },
      );
      expect((await _repository(nested).getMessages('t1')).dataOrThrow,
          hasLength(1));

      final bare = _CapturingSupportAdapter(
        responseBody: {
          'success': true,
          'data': [
            {
              'id': 'msg-2',
              'room_id': 'room-1',
              'sender_id': 'user-1',
              'sender_type': 'user',
              'message_type': 'text',
              'body': 'hai',
              'created_at': '2026-09-19T00:00:00Z',
            },
          ],
        },
      );
      expect(
        (await _repository(bare).getMessages('t1')).dataOrThrow,
        hasLength(1),
      );
    });

    test('sender taxonomy is canonical and never guessed', () {
      expect(SupportSenderType.fromString('user'), SupportSenderType.user);
      expect(SupportSenderType.fromString('admin'), SupportSenderType.admin);
      expect(SupportSenderType.fromString('system'), SupportSenderType.system);
      expect(SupportMessageType.fromString('text'), SupportMessageType.text);
    });
  });

  group('Support conversation — canonical send contract', () {
    test('posts the reply with only the message text', () async {
      final adapter = _CapturingSupportAdapter(
        responseBody: {
          'success': true,
          'message': 'Message sent',
          'data': {'ticket_id': 'ticket-1'},
        },
      );
      final repository = _repository(adapter);

      final result = await repository.sendMessage(
        ticketId: 'ticket-1',
        message: 'Sudah saya coba, masih gagal.',
      );

      expect(result.isSuccess, isTrue, reason: result.failure?.message ?? '');

      final request = adapter.requests.single;
      expect(request.method, 'POST');
      expect(request.path, '/support/tickets/ticket-1/messages');

      final body = request.data as Map<String, dynamic>;
      expect(body['message'], 'Sudah saya coba, masih gagal.');

      // The client never supplies a sender identity as authority.
      expect(body.containsKey('sender_id'), isFalse);
      expect(body.containsKey('senderId'), isFalse);
      expect(body.containsKey('user_id'), isFalse);
      expect(body.containsKey('sender_type'), isFalse);
      expect(body.length, 1, reason: 'only the message text may be sent');
    });

    test('trims the message and rejects an empty reply without a request',
        () async {
      final adapter = _CapturingSupportAdapter(
        responseBody: {'success': true, 'data': <String, dynamic>{}},
      );
      final repository = _repository(adapter);

      final empty = await repository.sendMessage(
        ticketId: 'ticket-1',
        message: '   ',
      );
      expect(empty.isFailure, isTrue);
      expect(adapter.requests, isEmpty,
          reason: 'an empty reply must never hit the API');

      final trimmed = await repository.sendMessage(
        ticketId: 'ticket-1',
        message: '  halo  ',
      );
      expect(trimmed.isSuccess, isTrue);
      expect((adapter.requests.single.data as Map)['message'], 'halo');
    });
  });
}
