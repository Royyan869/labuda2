import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/presentation/shipping_quote_intent.dart';

// Canonical distinct UUIDs for ID-confusion proof.
const _productId = '11111111-1111-1111-1111-111111111111';
const _forSaleId = '22222222-2222-2222-2222-222222222222';
const _auctionId = '33333333-3333-3333-3333-333333333333';

void main() {
  group('SellerShippingQuoteTarget', () {
    test('forSale target carries the for-sale surface only', () {
      const target = SellerShippingQuoteTarget.forSale(
        forSaleId: _forSaleId,
        title: 'Kohaku 50cm',
      );
      expect(target.forSaleId, _forSaleId);
      expect(target.auctionId, isNull);
      expect(target.title, 'Kohaku 50cm');
    });

    test('auction target carries the auction surface only', () {
      const target = SellerShippingQuoteTarget.auction(
        auctionId: _auctionId,
        title: 'Lelang Kohaku',
      );
      expect(target.auctionId, _auctionId);
      expect(target.forSaleId, isNull);
      expect(target.title, 'Lelang Kohaku');
    });
  });

  group('canonical create-quote wire (Shipping-owned builder)', () {
    test('for_sale request uses canonical backend fields', () {
      final request = buildShippingQuoteRequest(
        productId: _productId,
        sourceType: 'for_sale',
        sourceId: _forSaleId,
        cost: 25000,
        note: 'termasuk oksigen',
        destinationCityId: '3171',
        destinationProvinceId: '31',
      );

      final json = request.toJson();
      expect(json['product_id'], _productId);
      expect(json['source_type'], 'for_sale');
      expect(json['source_id'], _forSaleId);
      expect(json['cost'], 25000);
      expect(json['note'], 'termasuk oksigen');
      // Destination lock travels on every FPS quote wire (kota/kabupaten).
      expect(json['destination_city_id'], '3171');
      expect(json['destination_province_id'], '31');
      // Stale aliases must never be emitted.
      expect(json.containsKey('for_sale_id'), isFalse);
      expect(json.containsKey('auction_id'), isFalse);
    });

    test('optional note is omitted when absent', () {
      final request = buildShippingQuoteRequest(
        productId: _productId,
        sourceType: 'for_sale',
        sourceId: _forSaleId,
        cost: 10000,
        destinationCityId: '3171',
        destinationProvinceId: '31',
      );
      final json = request.toJson();
      expect(json.containsKey('note'), isFalse);
    });

    test('auction request keeps source_id = auction id, never product id', () {
      final request = buildShippingQuoteRequest(
        productId: _productId,
        sourceType: 'auction',
        sourceId: _auctionId,
        cost: 20000,
        destinationCityId: '3171',
        destinationProvinceId: '31',
      );
      final json = request.toJson();
      expect(json['source_type'], 'auction');
      expect(json['source_id'], _auctionId);
      expect(json['source_id'], isNot(equals(_productId)));
    });
  });
}
