import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';

/// SCOPE 3 — auction status boundary (mobile side).
///
/// The backend now coarsens the public `status` wire field to the public
/// phase vocabulary ({scheduled, active, waiting_settlement, ended,
/// cancelled}) — raw `draft` NEVER crosses the public boundary. The exact
/// internal state crosses ONLY via `seller_status`, and only on owner
/// surfaces. The mapper must prefer `seller_status` (owner precision) and
/// resolve through the public vocabulary otherwise.
void main() {
  Map<String, dynamic> baseJson({String? status, String? sellerStatus}) {
    return <String, dynamic>{
      'id': 'auction-status-boundary',
      'seller_id': 'seller-1',
      'product_id': 'product-1',
      'title': 'Koi Boundary',
      'description': 'desc',
      'media_urls': <String>['https://cdn.example.com/koi.jpg'],
      'start_price': 1500000,
      'bid_increment': 50000,
      'current_bid': null,
      'current_winner_id': null,
      'start_at': '2026-07-28T00:00:00.000Z',
      'end_at': '2026-07-29T00:00:00.000Z',
      'status': status ?? 'active',
      if (sellerStatus != null) 'seller_status': sellerStatus,
      'created_at': '2026-07-28T00:00:00.000Z',
      'updated_at': '2026-07-28T00:00:00.000Z',
    };
  }

  test('public viewer: coarsened status resolves through public vocabulary', () {
    final auction = AuctionMapper.toEntity(AuctionDto.fromJson(baseJson(status: 'scheduled')));
    expect(auction.status, AuctionStatus.scheduled);
  });

  test(
    'owner viewer: seller_status (exact internal state) takes precedence over public status',
    () {
      // Public phase says cancelled; the owner slot carries the exact draft
      // workspace state. The owner must see the true state.
      final auction = AuctionMapper.toEntity(
        AuctionDto.fromJson(baseJson(status: 'cancelled', sellerStatus: 'draft')),
      );
      expect(auction.status, AuctionStatus.draft);
    },
  );

  test('waiting_settlement survives the public vocabulary', () {
    final auction = AuctionMapper.toEntity(AuctionDto.fromJson(baseJson(status: 'waiting_settlement')));
    expect(auction.status, AuctionStatus.waitingSettlement);
  });

  test('anonymous viewer of seller workspace: null seller_status falls back', () {
    // Owner endpoint hit without auth would coarsen seller_status to null —
    // mapper must fall back to public status, never crash.
    final auction = AuctionMapper.toEntity(AuctionDto.fromJson(baseJson(status: 'active')));
    expect(auction.status, AuctionStatus.active);
  });

  test('unknown public value falls back conservatively (existing contract)', () {
    final auction = AuctionMapper.toEntity(AuctionDto.fromJson(baseJson(status: 'mystery_state')));
    expect(auction.status, AuctionStatus.draft);
  });
}
