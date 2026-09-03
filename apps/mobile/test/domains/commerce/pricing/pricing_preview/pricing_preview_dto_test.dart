import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/pricing/pricing_preview/data/dto/pricing_preview_dto.dart';

// Distinct-ID constants so tests fail if wrong ID is sent in the wrong field.
const _productId = '11111111-1111-1111-1111-111111111111';
const _fixedPriceSaleId = '22222222-2222-2222-2222-222222222222';
const _negotiationId = '33333333-3333-3333-3333-333333333333';
const _addressId = '44444444-4444-4444-4444-444444444444';

void main() {
  group('PricingPreviewRequestDto.toJson', () {
    test('sends product_id, source_type, source_id — no listing_id', () {
      final dto = PricingPreviewRequestDto(
        productId: _productId,
        sourceType: 'fixed_price_sale',
        sourceId: _fixedPriceSaleId,
        quantity: 1,
        addressId: _addressId,
      );
      final json = dto.toJson();

      expect(json['product_id'], equals(_productId));
      expect(json['source_type'], equals('fixed_price_sale'));
      expect(json['source_id'], equals(_fixedPriceSaleId));
      expect(
        json.containsKey('listing_id'),
        isFalse,
        reason: 'listing_id must not appear in pricing preview request',
      );
    });

    test(
      'product_id and source_id are distinct — fixedPriceSaleId not sent as product_id',
      () {
        final dto = PricingPreviewRequestDto(
          productId: _productId,
          sourceType: 'fixed_price_sale',
          sourceId: _fixedPriceSaleId,
          quantity: 2,
          addressId: _addressId,
        );
        final json = dto.toJson();

        expect(json['product_id'], equals(_productId));
        expect(json['source_id'], equals(_fixedPriceSaleId));
        expect(
          json['product_id'],
          isNot(equals(_fixedPriceSaleId)),
          reason: 'product_id must not equal fixedPriceSaleId',
        );
      },
    );

    test('optional fields are omitted when null', () {
      final dto = PricingPreviewRequestDto(
        productId: _productId,
        sourceType: 'fixed_price_sale',
        sourceId: _fixedPriceSaleId,
        quantity: 1,
        addressId: _addressId,
      );
      final json = dto.toJson();

      expect(json.containsKey('shipping_setup_id'), isFalse);
      expect(json.containsKey('shipping_quote_id'), isFalse);
      expect(json.containsKey('discount_code'), isFalse);
    });

    test('optional fields are included when set', () {
      final dto = PricingPreviewRequestDto(
        productId: _productId,
        sourceType: 'fixed_price_sale',
        sourceId: _fixedPriceSaleId,
        quantity: 1,
        addressId: _addressId,
        shippingSetupId: 'opt-123',
        discountCode: 'PROMO10',
      );
      final json = dto.toJson();

      expect(json['shipping_setup_id'], equals('opt-123'));
      expect(json['discount_code'], equals('PROMO10'));
    });
  });

  group('NegotiationPricingPreviewRequestDto.toJson', () {
    test('sends negotiation_id — no listing_id', () {
      final dto = NegotiationPricingPreviewRequestDto(
        negotiationId: _negotiationId,
        addressId: _addressId,
      );
      final json = dto.toJson();

      expect(json['negotiation_id'], equals(_negotiationId));
      expect(
        json.containsKey('listing_id'),
        isFalse,
        reason: 'listing_id must not appear in negotiation pricing request',
      );
    });
  });
}
