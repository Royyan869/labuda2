/// SUPPORT SCOPE 1 — canonical mobile ticket creation contract.
///
/// Proves that the mobile creation path:
///  - uses the Support API (`POST /support/tickets`) as the sole authority,
///  - serializes the canonical category taxonomy as its canonical wire value,
///  - sends no client-supplied identity as an authority,
///  - parses the canonical snake_case API response back into the domain entity.
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/system/support/data/datasources/support_api_datasource.dart';
import 'package:labuda/domains/system/support/data/dto/support_ticket_dto.dart';
import 'package:labuda/domains/system/support/data/repositories/support_repository_api.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';

/// Captures every outgoing request and answers with a canned canonical ticket.
class _CapturingSupportAdapter implements HttpClientAdapter {
  _CapturingSupportAdapter({
    required this.responseBody,
    this.statusCode = 201,
  });

  final Map<String, dynamic> responseBody;
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

ApiClient _fakeClient(_CapturingSupportAdapter adapter) {
  final client = ApiClient(logger: null, baseUrl: 'https://labuda.test');
  client.dio.httpClientAdapter = adapter;
  return client;
}

void main() {
  group('Support ticket creation — canonical contract', () {
    test('posts to the Support API with canonical category and linked order',
        () async {
      final adapter = _CapturingSupportAdapter(
        responseBody: {
          'success': true,
          'data': {
            'id': 'ticket-1',
            'user_id': 'user-1',
            'username': 'Ayu',
            'category': 'payment_issue',
            'priority': 'high',
            'status': 'open',
            'chat_room_id': 'room-1',
            'linked_order_id': 'order-1',
            'created_at': '2026-09-19T00:00:00Z',
          },
        },
      );
      final repository = SupportRepositoryApi(
        datasource: SupportApiDatasource(_fakeClient(adapter)),
      );

      final result = await repository.createTicket(
        userId: 'user-1', // local display context only
        userName: 'Ayu',
        category: SupportCategory.paymentIssue,
        priority: SupportPriority.high,
        description: 'Kartu saya selalu ditolak',
        linkedOrderId: 'order-1',
      );

      expect(result.isSuccess, isTrue);
      expect(result.data, 'ticket-1');

      final request = adapter.requests.single;
      expect(request.method, 'POST');
      expect(request.path, '/support/tickets');

      final body = request.data as Map<String, dynamic>;
      expect(body['category'], 'payment_issue');
      expect(body['priority'], 'high');
      expect(body['description'], 'Kartu saya selalu ditolak');
      expect(body['linked_order_id'], 'order-1');

      // The client never sends identity/owner as authority.
      expect(body.containsKey('userId'), isFalse);
      expect(body.containsKey('userName'), isFalse);
      expect(body.containsKey('userAvatar'), isFalse);
    });

    test('canonical taxonomy serializes 1:1 with the backend enum', () {
      expect(
        SupportCategory.values.map((c) => c.wireValue).toList(),
        <String>[
          'order_issue',
          'payment_issue',
          'account_issue',
          'listing_issue',
          'shipping_issue',
          'refund_request',
          'dispute',
          'technical_issue',
          'other',
        ],
      );
    });

    test('legacy divergent category values are not part of the taxonomy', () {
      for (final legacy in <String>[
        'payment',
        'order',
        'technical',
        'account',
        'general',
      ]) {
        expect(
          SupportCategory.fromWire(legacy),
          isNull,
          reason: 'legacy value "$legacy" must not resolve to a canonical '
              'category — no silent translation is allowed',
        );
      }
    });

    test('canonical API response parses back into the domain entity', () {
      final dto = SupportTicketDto.fromMap('ticket-9', <String, dynamic>{
        'user_id': 'u1',
        'username': 'Ayu',
        'category': 'refund_request',
        'priority': 'medium',
        'status': 'waiting_user',
        'linked_order_id': 'order-9',
        'created_at': '2026-09-19T00:00:00Z',
      });

      final entity = dto.toEntity();
      expect(entity.id, 'ticket-9');
      expect(entity.userId, 'u1');
      expect(entity.category, SupportCategory.refundRequest);
      expect(entity.priority, SupportPriority.medium);
      expect(entity.status, SupportStatus.waitingUser);
      expect(entity.linkedOrderId, 'order-9');
    });
  });
}
