import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/bidding_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/bidding_mapper.dart';

/// PASS 1 — AUCTION NUMERIC READ CANONICAL CONVERGENCE proof.
///
/// Backend factual authority (proven in the forensic trace):
///   PostgreSQL bigint → Go int64 / *int64 → JSON integer / null
///
/// Canonical mobile representation: int / int? (nullable follows factual
/// backend nullability). These tests lock that an integer wire literal
/// parses to Dart int — NOT double — and flows through the mapper into the
/// entity unchanged. No fallback/compat/coercion is asserted anywhere.
Map<String, dynamic> _auctionPayload() => {
  'id': 'a1',
  'seller_id': 's1',
  'product_id': 'p1',
  'title': 'Kohaku Auction',
  'description': 'desc',
  'start_price': 1000000,
  'bid_increment': 50000,
  'buy_now_price': 2000000,
  'current_bid': 1100000,
  'current_winner_id': 'u9',
  'total_bids': 2,
  'start_at': '2026-06-01T00:00:00Z',
  'end_at': '2026-06-02T00:00:00Z',
  'status': 'active',
  'created_at': '2026-06-01T00:00:00Z',
  'updated_at': '2026-06-01T00:10:00Z',
  'images': ['https://img/1.jpg'],
};

void main() {
  group('AuctionDto — integer wire parses to canonical int', () {
    test('pricing fields are Dart int, not double', () {
      final dto = AuctionDto.fromJson(_auctionPayload());

      expect(dto.startPrice, 1000000);
      expect(dto.startPrice, isA<int>());
      expect(dto.startPrice, isNot(isA<double>()));

      expect(dto.bidIncrement, 50000);
      expect(dto.bidIncrement, isA<int>());

      expect(dto.currentBid, 1100000);
      expect(dto.currentBid, isA<int>());

      // minimumBid is phantom and purged — minimum bid is derived via
      // entity.minimumNextBid (currentBid + bidIncrement), not wire.
    });

    test('buy_now_price integer wire parses to int', () {
      final dto = AuctionDto.fromJson(_auctionPayload());

      expect(dto.buyNowPrice, 2000000);
      expect(dto.buyNowPrice, isA<int>());
      expect(dto.buyNowPrice, isNot(isA<double>()));
    });

    test('nullable buy_now_price null is preserved as int? null', () {
      // Type/null preservation proof only — no UI/business null semantics
      // are decided in PASS 1.
      final payload = _auctionPayload()..['buy_now_price'] = null;
      final dto = AuctionDto.fromJson(payload);

      expect(dto.buyNowPrice, isNull);
    });
  });

  group('BidDto — bid amount integer wire parses to canonical int', () {
    test('amount: 150000 becomes Dart int on DTO', () {
      final dto = BidDto.fromJson({
        'id': 'b1',
        'auction_id': 'a1',
        'bidder_id': 'u2',
        'amount': 150000,
        'created_at': '2026-06-01T01:00:00Z',
        'bidder': {'id': 'u2', 'username': 'alice'},
      });

      expect(dto.amount, 150000);
      expect(dto.amount, isA<int>());
      expect(dto.amount, isNot(isA<double>()));
    });
  });

  group('BiddingItemDto — /bidding int64 wire parses to canonical int', () {
    test('your_last_bid and current_bid are Dart int', () {
      final dto = BiddingItemDto.fromJson({
        'auction_id': 'a1',
        'title': 'Kohaku',
        'your_last_bid': 1150000,
        'current_bid': 1200000,
        'status': 'leading',
        'end_at': '2026-06-02T00:00:00Z',
        'updated_at': '2026-06-01T02:00:00Z',
      });

      expect(dto.yourLastBid, 1150000);
      expect(dto.yourLastBid, isA<int>());
      expect(dto.currentBid, 1200000);
      expect(dto.currentBid, isA<int>());
    });
  });

  group('Mapper pass-through — int DTO to int entity, zero conversion', () {
    test('AuctionMapper.toEntity carries int pricing unchanged', () {
      final dto = AuctionDto.fromJson(_auctionPayload());
      final entity = AuctionMapper.toEntity(dto);

      expect(entity.openingBid, 1000000);
      expect(entity.openingBid, isA<int>());
      expect(entity.currentBid, 1100000);
      expect(entity.currentBid, isA<int>());
      expect(entity.bidIncrement, 50000);
      expect(entity.bidIncrement, isA<int>());
      expect(entity.buyNowPrice, 2000000);
      expect(entity.buyNowPrice, isA<int>());
      // Minimum sum is int arithmetic, no conversion bridge.
      expect(entity.minimumNextBid, 1150000);
      expect(entity.minimumNextBid, isA<int>());
    });

    test('AuctionMapper.toBidEntity carries int amount unchanged', () {
      final dto = BidDto.fromJson({
        'id': 'b1',
        'auction_id': 'a1',
        'bidder_id': 'u2',
        'amount': 150000,
        'created_at': '2026-06-01T01:00:00Z',
        'bidder': {'id': 'u2', 'username': 'alice'},
      });
      final entity = AuctionMapper.toBidEntity(dto);

      expect(entity.amount, 150000);
      expect(entity.amount, isA<int>());
      expect(entity.amount, isNot(isA<double>()));
    });

    test('BiddingMapper.toItemEntity carries int bids unchanged', () {
      final dto = BiddingItemDto.fromJson({
        'auction_id': 'a1',
        'title': 'Kohaku',
        'your_last_bid': 1150000,
        'current_bid': 1200000,
        'status': 'leading',
        'end_at': '2026-06-02T00:00:00Z',
        'updated_at': '2026-06-01T02:00:00Z',
      });
      final entity = BiddingMapper.toItemEntity(dto);

      expect(entity.yourLastBid, 1150000);
      expect(entity.yourLastBid, isA<int>());
      expect(entity.currentBid, 1200000);
      expect(entity.currentBid, isA<int>());
    });
  });
}
