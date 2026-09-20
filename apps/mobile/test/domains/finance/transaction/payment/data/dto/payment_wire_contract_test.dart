import 'dart:convert';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/types/payment_types.dart';
import 'package:labuda/domains/finance/transaction/payment/data/dto/payment_dto.dart';
import 'package:labuda/domains/finance/transaction/payment/data/remote/payment_remote_datasource.dart';
import 'package:labuda/domains/finance/transaction/payment/data/repositories/payment_repository_impl.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment.dart';
import 'package:labuda/shared/services/logger_service.dart';

/// CROSS-LANGUAGE PAYMENT WIRE CONTRACT PROOF (Dart half).
///
/// Single source of truth: test/fixtures/payment_wire_contract.json.
/// The Go half (backend/internal/serverboot/payment_wire_contract_test.go)
/// proves the canonical handler (CorePaymentHandler.CreatePayment /
/// GetPayment) emits exactly the keys declared in that fixture. This half
/// proves the mobile parsers consume exactly those keys — through the REAL
/// datasource and repository, not through a hand-rolled map.
///
/// This test fails if:
///   - the backend renames payment_id / drops payment_url (Go half fails, and
///     the fixture values below stop matching the real response),
///   - mobile re-introduces a required `net_amount` (parsing the fixture, which
///     has no such key, would throw),
///   - mobile expects the stale `id` / `amount` / `currency` on the create
///     response (parsing would throw / assert wrong values),
///   - the request body stops using the canonical `coins_to_use` key.
const String _fixturePath = 'test/fixtures/payment_wire_contract.json';

Map<String, dynamic> _fixture() =>
    jsonDecode(File(_fixturePath).readAsStringSync()) as Map<String, dynamic>;

Map<String, dynamic> _block(String name) =>
    _fixture()[name] as Map<String, dynamic>;

Map<String, dynamic> _response(String name) =>
    Map<String, dynamic>.from(_block(name)['response'] as Map<String, dynamic>);

List<String> _forbiddenKeys(String name) =>
    (_block(name)['forbidden_response_keys'] as List<dynamic>)
        .map((e) => e.toString())
        .toList();

/// Envelope-wrapped response, exactly as the backend `response.Success` helper
/// sends it (`{success, data, timestamp}`).
Map<String, dynamic> _envelope(Map<String, dynamic> data) => {
  'success': true,
  'data': data,
  'timestamp': '2026-09-18T10:00:00Z',
};

class _RecordingApiClient implements ApiClient {
  String? lastGetPath;
  String? lastPostPath;
  dynamic lastPostData;
  Map<String, dynamic>? lastGetQuery;
  int responseStatusCode = 200;
  dynamic getPayload = <String, dynamic>{'data': <String, dynamic>{}};
  dynamic postPayload = <String, dynamic>{'data': <String, dynamic>{}};

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastGetPath = path;
    lastGetQuery = queryParameters;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: getPayload as T,
      statusCode: responseStatusCode,
    );
  }

  @override
  Future<Response<T>> post<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPostPath = path;
    lastPostData = data;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: postPayload as T,
      statusCode: responseStatusCode,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

({PaymentRepositoryImpl repo, _RecordingApiClient client}) _repo() {
  final client = _RecordingApiClient();
  return (
    repo: PaymentRepositoryImpl(
      datasource: PaymentRemoteDatasource(client),
      logger: LoggerService.instance,
    ),
    client: client,
  );
}

void main() {
  group('payment wire contract fixture is single-source and alias-free', () {
    test('the canonical responses never carry obsolete keys', () {
      // Guards the fixture itself: it can never legitimize a dropped or
      // renamed wire key (net_amount was dropped by migration 000037).
      for (final block in ['create_payment', 'get_payment']) {
        final response = _response(block);
        for (final forbidden in _forbiddenKeys(block)) {
          expect(
            response.containsKey(forbidden),
            isFalse,
            reason: '$block response must not declare "$forbidden"',
          );
        }
      }
    });
  });

  group('POST /payments (create payment intent)', () {
    test('the canonical response parses into the payment intent', () async {
      final response = _response('create_payment');
      final r = _repo();
      r.client.postPayload = _envelope(response);

      final result = await r.repo.createPayment(
        const CreatePaymentRequest(
          orderId: '5c1e2f3a-4b5c-4d6e-8f90-1a2b3c4d5e6f',
          paymentMethodCode: 'bank_transfer',
        ),
      );

      expect(r.client.lastPostPath, '/payments');
      expect(result.isSuccess, isTrue, reason: result.failure?.message);

      final intent = result.data!;
      expect(intent.paymentId, response['payment_id']);
      expect(intent.paymentNumber, response['payment_number']);
      expect(intent.status, 'pending');
      expect(intent.paymentUrl, response['payment_url']);
      expect(intent.paymentMethodCode, 'bank_transfer');
      expect(intent.buyerPaymentFeeAmount, 4000);
      expect(intent.grossAmount, 114000);
      expect(intent.coinsToUse, 0);
      expect(intent.coinDiscountAmount, 0);
      expect(intent.referenceType, 'order');
      expect(intent.referenceId, response['reference_id']);
      expect(intent.expiredAt, DateTime.utc(2026, 9, 19, 10, 0, 0));

      // Checkout navigates to the in-app WebView only when a payment URL is
      // present and it is passed through unmodified.
      expect(intent.requiresRedirect, isTrue);
      expect(intent.paymentUrl, contains('midtrans.com'));
    });

    test('the request body uses the canonical create-payment keys', () async {
      final request = _block('create_payment_request');
      final expectedKeys = request.keys.toSet();
      final r = _repo();
      r.client.postPayload = _envelope(_response('create_payment'));

      await r.repo.createPayment(
        CreatePaymentRequest(
          orderId: request['order_id'] as String,
          paymentMethodCode: request['payment_method_code'] as String,
          priceSnapshotId: request['price_snapshot_id'] as String?,
        ),
      );

      final body = r.client.lastPostData as Map<String, dynamic>;
      expect(body.keys.toSet(), expectedKeys);
      expect(body, equals(request));
      // PAY-B: K is fixed at Order creation; the payment request must not carry
      // a client coin authority. The stale `coin_discount` key is never sent.
      expect(body.containsKey('coins_to_use'), isFalse);
      expect(body.containsKey('coin_discount'), isFalse);
    });

    test('a payload without the stale id/amount/currency keys still parses', () {
      final response = _response('create_payment');
      expect(response.containsKey('id'), isFalse);
      expect(response.containsKey('amount'), isFalse);
      expect(response.containsKey('currency'), isFalse);

      final dto = PaymentIntentDto.fromJson(response);
      expect(dto.paymentId, response['payment_id']);
      expect(dto.grossAmount, 114000);
    });

    test('unknown future response keys do not break parsing', () {
      final response = _response('create_payment')
        ..['some_future_field'] = 'ignored';
      final dto = PaymentIntentDto.fromJson(response);
      expect(dto.paymentId, response['payment_id']);
    });
  });

  group('GET /payments/:id (payment resource)', () {
    test('the canonical response parses into the payment entity', () async {
      final response = _response('get_payment');
      final r = _repo();
      r.client.getPayload = _envelope(response);

      final result = await r.repo.getPayment('9f6f0b0a-1d2e-4c3b-8a4f-2b7c1e5d6a90');

      expect(
        r.client.lastGetPath,
        '/payments/9f6f0b0a-1d2e-4c3b-8a4f-2b7c1e5d6a90',
      );
      expect(result.isSuccess, isTrue, reason: result.failure?.message);

      final payment = result.data!;
      expect(payment.id, response['id']);
      expect(payment.paymentNumber, response['payment_number']);
      expect(payment.userId, response['user_id']);
      expect(payment.grossAmount, 114000);
      expect(payment.coinsToUse, 0);
      expect(payment.coinDiscountAmount, 0);
      expect(payment.status, PaymentStatus.pending);
      expect(payment.referenceType, 'order');
      expect(payment.paymentUrl, response['payment_url']);
      expect(payment.expiredAt, DateTime.utc(2026, 9, 19, 10, 0, 0));
      expect(payment.midtransOrderId, response['midtrans_order_id']);
      expect(payment.paidAt, isNull);
    });

    test('parsing must not require the dropped net_amount key', () {
      final response = _response('get_payment');
      expect(response.containsKey('net_amount'), isFalse);

      final dto = PaymentDto.fromJson(response);
      expect(dto.grossAmount, 114000);
      // gross_amount is the canonical charged amount on this wire; the mobile
      // model has no net_amount surface at all.
      expect(dto.coinsToUse, 0);
    });

    test('unknown future response keys do not break parsing', () {
      final response = _response('get_payment')..['some_future_field'] = 1;
      final dto = PaymentDto.fromJson(response);
      expect(dto.id, response['id']);
    });
  });

  group('GET /payments/methods (fee disclosure)', () {
    test('per-method fee/total are consumed as backend-presented values', () async {
      final response = _response('list_payment_methods');
      final r = _repo();
      r.client.getPayload = _envelope(response);

      final result = await r.repo.getPaymentMethodOptions(
        response['order_id'] as String,
      );

      expect(r.client.lastGetPath, '/payments/methods');
      expect(r.client.lastGetQuery, {'order_id': response['order_id']});
      expect(result.isSuccess, isTrue, reason: result.failure?.message);

      final option = result.data!.single;
      final method =
          (response['methods'] as List<dynamic>).single
              as Map<String, dynamic>;
      expect(option.methodCode, method['method_code']);
      expect(option.displayName, method['display_name']);
      expect(option.buyerPaymentFeeAmount, method['buyer_payment_fee_amount']);
      expect(option.totalPayableAmount, method['total_payable_amount']);
      // PAY-B CLOSED: the backend no longer accepts a client `coins_to_use`
      // query input on this endpoint. The buyer fee is computed on
      // `cash_amount = base_amount − K`, where K is read from the pricing-token
      // snapshot written at Order creation (pricing_tokens.coins_used). The
      // client expresses coin intent only once, as the boolean `use_coins` on
      // POST /orders, and never manufactures a coin count downstream.
      //
      // This client therefore sends only the required `order_id`, and the guard
      // below pins that it can never re-acquire a payment-time coin authority.
      expect(r.client.lastGetQuery!.containsKey('coins_to_use'), isFalse);
    });
  });
}
