import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/pricing/pricing_preview/data/dto/pricing_preview_dto.dart';

// Distinct-ID constants so tests fail if wrong ID is sent in the wrong field.
const _productId = '11111111-1111-1111-1111-111111111111';
const _fixedPriceSaleId = '22222222-2222-2222-2222-222222222222';
const _negotiationId = '33333333-3333-3333-3333-333333333333';
const _addressId = '44444444-4444-4444-4444-444444444444';

void main() {
  group('PricingPreviewRequestDto.toJson', () {
    test('sends product_id, source_type, source_id — no for_sale_id', () {
      final dto = PricingPreviewRequestDto(
        productId: _productId,
        sourceType: 'for_sale',
        sourceId: _fixedPriceSaleId,
        quantity: 1,
        addressId: _addressId,
      );
      final json = dto.toJson();

      expect(json['product_id'], equals(_productId));
      expect(json['source_type'], equals('for_sale'));
      expect(json['source_id'], equals(_fixedPriceSaleId));
      expect(
        json.containsKey('for_sale_id'),
        isFalse,
        reason: 'for_sale_id must not appear in pricing preview request',
      );
    });

    test(
      'product_id and source_id are distinct — fixedPriceSaleId not sent as product_id',
      () {
        final dto = PricingPreviewRequestDto(
          productId: _productId,
          sourceType: 'for_sale',
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
        sourceType: 'for_sale',
        sourceId: _fixedPriceSaleId,
        quantity: 1,
        addressId: _addressId,
      );
      final json = dto.toJson();

      expect(json.containsKey('shipping_option_id'), isFalse);
      expect(json.containsKey('shipping_quote_id'), isFalse);
      expect(json.containsKey('discount_code'), isFalse);
    });

    test('optional fields are included when set', () {
      final dto = PricingPreviewRequestDto(
        productId: _productId,
        sourceType: 'for_sale',
        sourceId: _fixedPriceSaleId,
        quantity: 1,
        addressId: _addressId,
        shippingSetupId: 'opt-123',
        discountCode: 'PROMO10',
      );
      final json = dto.toJson();

      expect(json['shipping_option_id'], equals('opt-123'));
      expect(json['discount_code'], equals('PROMO10'));
    });
  });

  group('NegotiationPricingPreviewRequestDto.toJson', () {
    test('sends negotiation_id — no for_sale_id', () {
      final dto = NegotiationPricingPreviewRequestDto(
        negotiationId: _negotiationId,
        addressId: _addressId,
      );
      final json = dto.toJson();

      expect(json['negotiation_id'], equals(_negotiationId));
      expect(
        json.containsKey('for_sale_id'),
        isFalse,
        reason: 'for_sale_id must not appear in negotiation pricing request',
      );
    });

    test('negotiation quote mode omits shipping_option_id', () {
      final dto = NegotiationPricingPreviewRequestDto(
        negotiationId: _negotiationId,
        addressId: _addressId,
        shippingQuoteId: 'quote-abc',
      );
      final json = dto.toJson();

      expect(json.containsKey('shipping_option_id'), isFalse);
      expect(json['shipping_quote_id'], equals('quote-abc'));
      expect(json.containsKey('shipping_setup_id'), isFalse,
        reason: 'stale wire key must never appear in live pricing preview',
      );
    });
  });

  // ========================================================================
  // STAGE 13 — Shipping option contract proof
  // ========================================================================
  group('Pricing preview shipping option contract', () {
    const _selectedShippingOptionId = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';

    test('standard shipping: shipping_option_id present, shipping_setup_id absent', () {
      final dto = PricingPreviewRequestDto(
        productId: _productId,
        sourceType: 'for_sale',
        sourceId: _fixedPriceSaleId,
        quantity: 1,
        addressId: _addressId,
        shippingSetupId: _selectedShippingOptionId,
      );
      final json = dto.toJson();

      expect(json.containsKey('shipping_option_id'), isTrue);
      expect(json.containsKey('shipping_setup_id'), isFalse,
        reason: 'stale wire key must never appear in live pricing preview',
      );
    });

    test('value trace: selected shipping option ID arrives unchanged as shipping_option_id', () {
      final dto = PricingPreviewRequestDto(
        productId: _productId,
        sourceType: 'for_sale',
        sourceId: _fixedPriceSaleId,
        quantity: 1,
        addressId: _addressId,
        shippingSetupId: _selectedShippingOptionId,
      );
      final json = dto.toJson();

      expect(json['shipping_option_id'], equals(_selectedShippingOptionId));
    });

    test('quote mode: no shipping_option_id when shippingQuoteId is set', () {
      final dto = PricingPreviewRequestDto(
        productId: _productId,
        sourceType: 'for_sale',
        sourceId: _fixedPriceSaleId,
        quantity: 1,
        addressId: _addressId,
        shippingQuoteId: 'quote-xyz',
      );
      final json = dto.toJson();

      expect(json.containsKey('shipping_option_id'), isFalse);
      expect(json.containsKey('shipping_setup_id'), isFalse);
      expect(json['shipping_quote_id'], equals('quote-xyz'));
    });

    test('negative stale-key guard: no live preview request emits shipping_setup_id', () {
      // Exhaustive check — any PricingPreviewRequestDto.toJson() must never
      // contain the stale 'shipping_setup_id' key.
      final withOption = PricingPreviewRequestDto(
        productId: _productId,
        sourceType: 'for_sale',
        sourceId: _fixedPriceSaleId,
        quantity: 1,
        addressId: _addressId,
        shippingSetupId: 'opt-999',
      );
      final withQuote = PricingPreviewRequestDto(
        productId: _productId,
        sourceType: 'for_sale',
        sourceId: _fixedPriceSaleId,
        quantity: 1,
        addressId: _addressId,
        shippingQuoteId: 'quote-999',
      );
      final withoutShipping = PricingPreviewRequestDto(
        productId: _productId,
        sourceType: 'for_sale',
        sourceId: _fixedPriceSaleId,
        quantity: 1,
        addressId: _addressId,
      );

      expect(withOption.toJson().containsKey('shipping_setup_id'), isFalse);
      expect(withQuote.toJson().containsKey('shipping_setup_id'), isFalse);
      expect(withoutShipping.toJson().containsKey('shipping_setup_id'), isFalse);
    });
  });
}
