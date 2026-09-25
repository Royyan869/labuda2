import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/mappers/for_sale_dto_mapper.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';

/// SCOPE 3 — for_sale status boundary (mobile side, parity with auction).
///
/// The backend now coarsens the public `status` wire field to the public
/// lifecycle vocabulary ({active, unavailable}) — raw `draft`/`sold`/
/// `withdrawn` NEVER cross the public boundary. The exact internal state
/// crosses ONLY via `seller_status`, and only on owner surfaces. The mapper
/// must prefer `seller_status` (owner precision) and resolve through the
/// public vocabulary otherwise.
void main() {
  Map<String, dynamic> baseJson({String? status, String? sellerStatus}) {
    return <String, dynamic>{
      'id': 'for-sale-status-boundary',
      'seller_id': 'seller-1',
      'title': 'Koi Boundary',
      'description': 'desc',
      'media_urls': <String>['https://cdn.example.com/koi.jpg'],
      'price': 1500000,
      'quantity': 3,
      'visibility': 'public',
      'status': status ?? 'active',
      if (sellerStatus != null) 'seller_status': sellerStatus,
      'created_at': '2026-07-28T00:00:00.000Z',
      'updated_at': '2026-07-28T00:00:00.000Z',
    };
  }

  test('public viewer: coarsened status resolves through public vocabulary', () {
    final forSale = ForSaleDtoMapper.toEntity(
      ForSaleResponseDto.fromJson(baseJson(status: 'active')),
    );
    expect(forSale.status, ForSaleStatus.active);
  });

  test('unavailable coarsens sold/withdrawn/draft into one not-buyable value', () {
    final forSale = ForSaleDtoMapper.toEntity(
      ForSaleResponseDto.fromJson(baseJson(status: 'unavailable')),
    );
    // Public viewers only need "not buyable" — the conservative mapping
    // resolves unavailable to the draft (not-buyable) domain state.
    expect(forSale.status, ForSaleStatus.draft);
    expect(forSale.isAvailable, isFalse);
  });

  test(
    'owner viewer: seller_status (exact internal state) takes precedence over public status',
    () {
      // Public lifecycle says unavailable; the owner slot carries the exact
      // sold terminal state. The owner must see the true state.
      final forSale = ForSaleDtoMapper.toEntity(
        ForSaleResponseDto.fromJson(
          baseJson(status: 'unavailable', sellerStatus: 'sold'),
        ),
      );
      expect(forSale.status, ForSaleStatus.sold);
    },
  );

  test('owner viewer: withdrawn workspace state survives the boundary', () {
    final forSale = ForSaleDtoMapper.toEntity(
      ForSaleResponseDto.fromJson(
        baseJson(status: 'unavailable', sellerStatus: 'withdrawn'),
      ),
    );
    expect(forSale.status, ForSaleStatus.withdrawn);
  });

  test('anonymous viewer: null seller_status falls back to public status', () {
    // Owner endpoint hit without auth coarsens seller_status to null —
    // mapper must fall back to public status, never crash.
    final forSale = ForSaleDtoMapper.toEntity(
      ForSaleResponseDto.fromJson(baseJson(status: 'active')),
    );
    expect(forSale.status, ForSaleStatus.active);
  });

  test('unknown public value falls back conservatively (existing contract)', () {
    final forSale = ForSaleDtoMapper.toEntity(
      ForSaleResponseDto.fromJson(baseJson(status: 'mystery_state')),
    );
    expect(forSale.status, ForSaleStatus.draft);
  });
}
