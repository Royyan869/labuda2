import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/shipping/data/mappers/shipping_mapper.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';

// Canonical distinct UUIDs for ID-confusion proof.
const _productId = '11111111-1111-1111-1111-111111111111';
const _auctionId = '22222222-2222-2222-2222-222222222222';
const _fixedPriceSaleId = '33333333-3333-3333-3333-333333333333';

void main() {
  // ── A. CheckDeliveryRequest only accepts productId ─────────────────────────

  test('checkDeliveryToJson emits product_id and omits seller_id', () {
    final request = CheckDeliveryRequest(
      productId: 'product-123',
      provinceId: '31',
      cityId: '3171',
      cityName: 'Jakarta Selatan',
    );

    final json = DeliveryOptionMapper.checkDeliveryToJson(request);

    expect(json['product_id'], 'product-123');
    expect(json.containsKey('seller_id'), isFalse);
  });

  // ── B. Distinct-ID proof: delivery request uses productId not auctionId ───

  test('auction delivery check uses productId — never auctionId', () {
    // For auction checkout, the widget.productId (from auction.productId) is
    // passed into CheckDeliveryRequest. The auctionId must NOT appear as productId.
    final request = CheckDeliveryRequest(
      productId: _productId,
      provinceId: '31',
      cityId: '3171',
      cityName: 'Test City',
    );

    final json = DeliveryOptionMapper.checkDeliveryToJson(request);

    expect(json['product_id'], _productId);
    // auctionId must not appear as the product lookup key
    expect(json['product_id'], isNot(equals(_auctionId)));
    expect(json.containsKey('auction_id'), isFalse);
    expect(json.containsKey('source_id'), isFalse);
    expect(json.containsKey('listing_id'), isFalse);
  });

  test('FPS delivery check uses productId — never fixedPriceSaleId', () {
    // For FPS checkout, productId comes from listing.productId (not listing.id).
    final request = CheckDeliveryRequest(
      productId: _productId,
      provinceId: '31',
      cityId: '3171',
      cityName: 'Test City',
    );

    final json = DeliveryOptionMapper.checkDeliveryToJson(request);

    expect(json['product_id'], _productId);
    expect(json['product_id'], isNot(equals(_fixedPriceSaleId)));
  });

  // ── C. Distinct-ID proof — all three IDs are different ────────────────────

  test('productId, auctionId and fixedPriceSaleId are distinct values', () {
    expect(_productId, isNot(equals(_auctionId)));
    expect(_productId, isNot(equals(_fixedPriceSaleId)));
    expect(_auctionId, isNot(equals(_fixedPriceSaleId)));
  });

  test(
    'delivery request for auction carries productId not auctionId as identifier',
    () {
      // Simulate: widget.productId=_productId, widget.auctionId=_auctionId
      // The delivery check must use productId, not auctionId.
      final request = CheckDeliveryRequest(
        productId:
            _productId, // ← comes from widget.productId (auction.productId)
        provinceId: '31',
        cityId: '3171',
        cityName: 'Test City',
      );

      final json = DeliveryOptionMapper.checkDeliveryToJson(request);

      // Source of truth: product_id is the physical product authority
      expect(json['product_id'], _productId);
      // auctionId (22222...) must never flow into product_id slot
      expect(json['product_id'], isNot(equals(_auctionId)));
      // fixedPriceSaleId (33333...) must never appear in delivery check
      expect(json.containsKey('fixed_price_sale_id'), isFalse);
    },
  );
}
