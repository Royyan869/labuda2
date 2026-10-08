import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';

/// SCOPE 3 — auction status boundary (mobile side).
///
/// The backend coarsens the public `status` wire field to the public
/// phase vocabulary ({scheduled, active, waiting_settlement, ended,
/// cancelled}) — raw internal states NEVER cross the public boundary. The
/// exact internal state crosses ONLY via `seller_status`, and only on owner
/// surfaces. The mapper must prefer `seller_status` (owner precision) and
/// resolve through the public vocabulary otherwise.
///
/// DRAFT IS PURGED (owner decision, Oct 2026): create = publish. A legacy
/// 'draft' value now maps to 'lapsed' (never-live, relistable).
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
      // Public phase says cancelled; the owner slot carries the exact lapsed
      // state (market authority expired before activation). The owner must
      // see the true state.
      final auction = AuctionMapper.toEntity(
        AuctionDto.fromJson(baseJson(status: 'cancelled', sellerStatus: 'lapsed')),
      );
      expect(auction.status, AuctionStatus.lapsed);
    },
  );

  test('lapsed is a first-class state (never-live, relistable)', () {
    final auction = AuctionMapper.toEntity(AuctionDto.fromJson(baseJson(status: 'lapsed')));
    expect(auction.status, AuctionStatus.lapsed);
    expect(auction.isRelistable, isTrue);
  });

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

  test('legacy draft value maps to lapsed, never to a phantom enum state', () {
    final auction = AuctionMapper.toEntity(AuctionDto.fromJson(baseJson(status: 'draft')));
    expect(auction.status, AuctionStatus.lapsed);
  });

  test('unknown public value falls back to the backend PublicPhase default (cancelled)', () {
    final auction = AuctionMapper.toEntity(AuctionDto.fromJson(baseJson(status: 'mystery_state')));
    expect(auction.status, AuctionStatus.cancelled);
  });
}
