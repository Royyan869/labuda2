// PRODUCT REFERENCE LIFECYCLE LABEL — shared presentation mapping.
//
// `commerceLifecycleLabel` is the ONE mapping used by both the Chat and the
// Comment product reference cards. It reads the Commerce projection verbatim
// (ForSaleLivePayload.status / AuctionLivePayload.lifecycle + hasWinner) and
// maps it to a display label. It never calculates lifecycle.

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';

ForSaleLivePayload _forSale(String status) => ForSaleLivePayload(
  title: 'Koi',
  media: const [],
  price: const LivePrice(amount: 1000, currency: 'IDR'),
  status: status,
  quantityAvailable: 1,
  seller: const ResourceSellerCard(
    user: ResourceUserCard(id: 's1', username: 'seller'),
  ),
);

AuctionLivePayload _auction(String lifecycle, {bool hasWinner = false}) =>
    AuctionLivePayload(
      title: 'Lelang Koi',
      media: const [],
      endAt: '2026-12-10T12:34:56Z',
      lifecycle: lifecycle,
      hasWinner: hasWinner,
      seller: const ResourceSellerCard(
        user: ResourceUserCard(id: 's1', username: 'seller'),
      ),
    );

void main() {
  group('For Sale (PublicLifecycle)', () {
    test('active → Tersedia', () {
      expect(commerceLifecycleLabel(_forSale('active')), 'Tersedia');
    });

    test('available synonym → Tersedia (wire synonym tolerated)', () {
      expect(commerceLifecycleLabel(_forSale('available')), 'Tersedia');
    });

    test('sold → Terjual', () {
      expect(commerceLifecycleLabel(_forSale('sold')), 'Terjual');
    });

    test('unavailable → Tidak tersedia', () {
      expect(commerceLifecycleLabel(_forSale('unavailable')), 'Tidak tersedia');
    });

    test('unknown value → neutral (never reinterpreted)', () {
      expect(
        commerceLifecycleLabel(_forSale('something-new')),
        'Status tidak tersedia',
      );
    });
  });

  group('Auction (PublicPhase + has_winner)', () {
    test('scheduled → Terjadwal', () {
      expect(commerceLifecycleLabel(_auction('scheduled')), 'Terjadwal');
    });

    test('active → Berlangsung', () {
      expect(commerceLifecycleLabel(_auction('active')), 'Berlangsung');
    });

    test('waiting_settlement → Menunggu Penyelesaian', () {
      expect(
        commerceLifecycleLabel(_auction('waiting_settlement')),
        'Menunggu Penyelesaian',
      );
    });

    test('ended + has_winner=true → Terjual', () {
      expect(
        commerceLifecycleLabel(_auction('ended', hasWinner: true)),
        'Terjual',
      );
    });

    test('ended + has_winner=false → Berakhir tanpa pemenang', () {
      expect(
        commerceLifecycleLabel(_auction('ended', hasWinner: false)),
        'Berakhir tanpa pemenang',
      );
    });

    test('cancelled → Dibatalkan', () {
      expect(commerceLifecycleLabel(_auction('cancelled')), 'Dibatalkan');
    });

    test('unknown value → neutral (never reinterpreted)', () {
      expect(
        commerceLifecycleLabel(_auction('mystery')),
        'Status tidak tersedia',
      );
    });

    test('has_winner does not alter non-ended labels', () {
      for (final phase in const [
        'scheduled',
        'active',
        'waiting_settlement',
        'cancelled',
      ]) {
        expect(
          commerceLifecycleLabel(_auction(phase, hasWinner: true)),
          commerceLifecycleLabel(_auction(phase, hasWinner: false)),
          reason: 'has_winner must not change the $phase label',
        );
      }
    });
  });

  test('non-commerce payload → null (surface keeps its own fallback)', () {
    expect(
      commerceLifecycleLabel(
        const ProfileLivePayload(username: 'alice', lifecycle: 'active'),
      ),
      isNull,
    );
  });
}
