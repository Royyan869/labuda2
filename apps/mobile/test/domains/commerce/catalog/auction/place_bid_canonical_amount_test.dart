import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/remote/auction_remote_datasource.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_action_modal.dart';

/// PLACE BID CANONICAL NUMERIC CONVERGENCE — proof tests.
///
/// Backend contract (auction_handler.go PlaceBidRequest):
///   Amount int64 `json:"amount" binding:"required,min=1"`
/// PostgreSQL persistence: `amount bigint`.
///
/// Therefore the single canonical representation of the Place Bid amount on
/// the entire live write chain is integer. A JSON literal like 1000000.0 is
/// REJECTED by the int64 binding, and any silent truncation
/// (1000000.9 -> 1000000) is forbidden: fractional input must be rejected
/// explicitly at the input boundary instead.

class _RecordingApiClient implements ApiClient {
  String? lastPostPath;
  dynamic lastPostData;

  dynamic postPayload = <String, dynamic>{
    'success': true,
    'data': <String, dynamic>{},
  };

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
      statusCode: 200,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  group('PlaceBidDto JSON serialization proof', () {
    test('amount serializes as an integer wire literal (no .0)', () {
      final dto = const PlaceBidDto(amount: 1000000);
      final json = dto.toJson();

      expect(json['amount'], 1000000);
      expect(json['amount'], isA<int>());
      expect(json['amount'], isNot(isA<double>()));

      final encoded = jsonEncode(json);
      expect(encoded, contains('"amount":1000000'));
      expect(encoded, isNot(contains('1000000.0')));
    });

    test('idempotency key defaults but amount stays integer', () {
      final json = const PlaceBidDto(amount: 1500).toJson();

      expect(json['amount'], 1500);
      expect(json['amount'], isA<int>());
      expect(json['idempotency_key'], isNotEmpty);
    });
  });

  group('parseCanonicalBidAmount — fraction rejection at input boundary', () {
    test('fractional input 1000000.9 is rejected, never truncated', () {
      expect(parseCanonicalBidAmount('1000000.9'), isNull);
    });

    test('even a clean fractional literal 1000000.0 is rejected', () {
      // int.tryParse rejects "1000000.0" — the user must retype an integer.
      // This is the explicit-rejection contract, not a silent coercion.
      expect(parseCanonicalBidAmount('1000000.0'), isNull);
    });

    test('no round/floor/ceil coercion exists: ".5" variants rejected', () {
      expect(parseCanonicalBidAmount('.9'), isNull);
      expect(parseCanonicalBidAmount('0.5'), isNull);
      // "12,5" in id-ID locale means 12.5 — it must NOT be silently
      // reinterpreted as 125. Only strict thousands grouping ("1,000,000")
      // is normalized; anything else is rejected.
      expect(parseCanonicalBidAmount('12,5'), isNull);
      expect(parseCanonicalBidAmount('1,00'), isNull);
      expect(parseCanonicalBidAmount('1000,5'), isNull);
    });

    test('canonical integer input parses losslessly', () {
      expect(parseCanonicalBidAmount('1000000'), 1000000);
      expect(parseCanonicalBidAmount('1,000,000'), 1000000);
      expect(parseCanonicalBidAmount('  150000  '), 150000);
    });

    test('malformed input is rejected, not coerced', () {
      expect(parseCanonicalBidAmount('abc'), isNull);
      expect(parseCanonicalBidAmount(''), isNull);
      expect(parseCanonicalBidAmount('12a'), isNull);
    });

    test('negative integers parse (caller must gate them against minimum)', () {
      // Parsing is canonical conversion only; the minimum-bid gate remains
      // downstream. The parse itself never mutates the value.
      expect(parseCanonicalBidAmount('-100'), -100);
    });
  });

  group('Live datasource Place Bid wire path', () {
    test('placeBid posts to /auctions/:id/bid with an integer amount', () async {
      final client = _RecordingApiClient();
      final ds = AuctionRemoteDatasource(client);

      client.postPayload = {
        'success': true,
        'data': {
          'id': 'b2',
          'auction_id': 'a1',
          'bidder_id': 'u2',
          'amount': 1500,
          'created_at': '2026-06-01T01:10:00Z',
          'bidder': {'id': 'u2', 'username': 'alice'},
        },
      };

      await ds.placeBid('a1', const PlaceBidDto(amount: 1500));

      expect(client.lastPostPath, '/auctions/a1/bid');
      final data = client.lastPostData as Map<String, dynamic>;
      expect(data['amount'], 1500);
      expect(data['amount'], isA<int>());
      expect(data['amount'], isNot(isA<double>()));
    });

    test('jsonEncode of the recorded payload emits an integer literal', () {
      const dto = PlaceBidDto(amount: 1000000);
      final encoded = jsonEncode(dto.toJson());

      expect(encoded, contains('"amount":1000000'));
      expect(encoded, isNot(contains('"amount":1000000.0')));
    });
  });
}
