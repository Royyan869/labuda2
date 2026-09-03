import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/remote/auction_remote_datasource.dart';

class _RecordingApiClient implements ApiClient {
  String? lastGetPath;
  String? lastPostPath;
  String? lastPutPath;
  String? lastDeletePath;
  Map<String, dynamic>? lastGetQuery;
  dynamic lastPostData;

  dynamic getPayload = <String, dynamic>{
    'success': true,
    'data': <String, dynamic>{},
  };
  dynamic postPayload = <String, dynamic>{
    'success': true,
    'data': <String, dynamic>{},
  };
  dynamic putPayload = <String, dynamic>{
    'success': true,
    'data': <String, dynamic>{},
  };
  dynamic deletePayload = <String, dynamic>{'success': true, 'data': null};

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
      statusCode: 200,
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
      statusCode: 200,
    );
  }

  @override
  Future<Response<T>> put<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPutPath = path;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: putPayload as T,
      statusCode: 200,
    );
  }

  @override
  Future<Response<T>> delete<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastDeletePath = path;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: deletePayload as T,
      statusCode: 200,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Map<String, dynamic> _auctionPayload() => {
  'id': 'a1',
  'seller_id': 's1',
  'product_id': 'p1',
  'title': 'Auction',
  'description': 'desc',
  'start_price': 1000,
  'bid_increment': 100,
  'current_bid': 1200,
  'current_winner_id': 'u9',
  'total_bids': 2,
  'minimum_bid': 1300,
  'start_at': '2026-06-01T00:00:00Z',
  'end_at': '2026-06-02T00:00:00Z',
  'status': 'active',
  'created_at': '2026-06-01T00:00:00Z',
  'updated_at': '2026-06-01T00:10:00Z',
  'images': ['https://img/1.jpg'],
};

void main() {
  group('Auction P1 route contract', () {
    test('canonical route calls are used', () async {
      final client = _RecordingApiClient();
      final ds = AuctionRemoteDatasource(client);

      client.getPayload = {'success': true, 'data': _auctionPayload()};
      await ds.getAuctionById('a1');
      expect(client.lastGetPath, '/auctions/a1');

      client.getPayload = {
        'success': true,
        'data': [
          {
            'id': 'b1',
            'auction_id': 'a1',
            'bidder_id': 'u2',
            'amount': 1400,
            'created_at': '2026-06-01T01:00:00Z',
            'bidder': {'id': 'u2', 'username': 'alice'},
          },
        ],
      };
      await ds.getBidHistory('a1');
      expect(client.lastGetPath, '/auctions/a1/bids');

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

      client.postPayload = {'success': true, 'data': null};
      await ds.cancelAuction('a1', const CancelAuctionDto(reason: 'x'));
      expect(client.lastPostPath, '/auctions/a1/cancel');

      client.postPayload = {
        'success': true,
        'data': {'order_id': 'o1'},
      };
      await ds.claimAuction(
        'a1',
        addressId: 'addr1',
        shippingSetupId: 'ship1',
      );
      expect(client.lastPostPath, '/auctions/a1/claim');
    });

    test(
      'unsupported endpoints throw UnsupportedError and do not call API',
      () async {
        // PASS_21C: deleteAuction/getMyAuctions/startAuction/buyNow/getWinner
        // were removed entirely (dead stubs, confirmed unreachable from any
        // UI — see auction_repository.dart PASS_21C comment). getAuctionsByIds
        // remains because the backend genuinely has no /auctions/batch
        // endpoint; its only caller (object_preview_batch_provider.dart) was
        // fixed to no longer call it, since the call was guaranteed to throw.
        final client = _RecordingApiClient();
        final ds = AuctionRemoteDatasource(client);

        expect(
          () => ds.getAuctionsByIds(['a1']),
          throwsA(isA<UnsupportedError>()),
        );

        expect(client.lastPostPath, isNull);
        expect(client.lastDeletePath, isNull);
        expect(client.lastGetPath, isNull);
      },
    );
  });

  group('Auction DTO contract', () {
    test('AuctionDto parses backend canonical keys', () {
      final dto = AuctionDto.fromJson(_auctionPayload());
      expect(
        dto.startTime.toUtc().toIso8601String(),
        '2026-06-01T00:00:00.000Z',
      );
      expect(dto.endTime.toUtc().toIso8601String(), '2026-06-02T00:00:00.000Z');
      expect(dto.currentHighestBid, 1200);
      expect(dto.highestBidderId, 'u9');
    });

    test('BidDto parses backend created_at and nested bidder', () {
      final dto = BidDto.fromJson({
        'id': 'b1',
        'auction_id': 'a1',
        'bidder_id': 'u2',
        'amount': 1500,
        'created_at': '2026-06-01T01:00:00Z',
        'bidder': {'id': 'u2', 'username': 'alice'},
      });
      expect(
        dto.createdAt.toUtc().toIso8601String(),
        '2026-06-01T01:00:00.000Z',
      );
      expect(dto.bidTime.toUtc().toIso8601String(), '2026-06-01T01:00:00.000Z');
      expect(dto.bidder?.username, 'alice');
    });
  });
}
