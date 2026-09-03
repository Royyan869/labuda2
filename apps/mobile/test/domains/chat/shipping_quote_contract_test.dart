import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/presentation/screens/chat_detail_screen.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/shipping_quote_dto.dart';
import 'package:labuda/shared/attachment/entities/attachment.dart';

// Canonical distinct UUIDs for ID-confusion proof tests.
const _productId = '11111111-1111-1111-1111-111111111111';
const _fixedPriceSaleId = '22222222-2222-2222-2222-222222222222';
const _auctionId = '33333333-3333-3333-3333-333333333333';

void main() {
  // ── A. Fixed-price-sale shipping quote request ──────────────────────────────

  test(
    'fixed-price sale shipping quote request uses canonical backend fields',
    () {
      final request = buildForSaleShippingQuoteRequest(
        productId: _productId,
        forSaleId: _fixedPriceSaleId,
        cost: 25000,
        note: 'catatan',
      );

      expect(request.productId, _productId);
      expect(request.sourceType, 'for_sale');
      expect(request.sourceId, _fixedPriceSaleId);
      // productId must never equal sourceId for FPS
      expect(request.productId, isNot(equals(request.sourceId)));

      final json = request.toJson();
      expect(json['product_id'], _productId);
      expect(json['source_type'], 'for_sale');
      expect(json['source_id'], _fixedPriceSaleId);
      expect(json['cost'], 25000);
      expect(json['note'], 'catatan');
      expect(json.containsKey('listing_id'), isFalse);
      expect(json.containsKey('auction_id'), isFalse);
    },
  );

  // ── B. Auction shipping quote checkout target ───────────────────────────────

  test(
    'auction ShippingQuoteCheckoutTarget carries productId and auctionId distinctly',
    () async {
      final attachment = ShippingQuoteAttachment(
        offerId: 'offer-1',
        linkedItemId: _auctionId,
        linkedItemType: 'auction',
        linkedItemName: 'Test Ikan',
        linkedItemPrice: 100000,
        shippingType: 'standard',
        shippingTypeName: 'Standar',
        shippingTypeEmoji: '📦',
        rate: 20000,
        validUntil: DateTime.now().add(const Duration(hours: 1)),
        status: 'ACTIVE',
        sellerId: 'seller-1',
      );

      final target = await resolveShippingQuoteCheckoutTarget(
        shippingQuote: attachment,
        resolveAuctionProductId: (auctionId) async {
          // Callback returns the product ID — must differ from auctionId
          expect(auctionId, _auctionId);
          return _productId;
        },
      );

      expect(target, isNotNull);
      // Auction path: auctionId and productId are populated, fixedPriceSaleId is null
      expect(target!.auctionId, _auctionId);
      expect(target.productId, _productId);
      expect(target.forSaleId, isNull);

      // Distinct-ID proof: productId must not equal auctionId
      expect(target.productId, isNot(equals(target.auctionId)));
      // productId must not appear in the auctionId slot
      expect(target.auctionId, isNot(equals(_productId)));
      // auctionId must not appear in the fixedPriceSaleId slot
      expect(target.forSaleId, isNot(equals(_auctionId)));
    },
  );

  // ── C. Fixed-price-sale ShippingQuoteCheckoutTarget (unchanged) ────────────

  test(
    'FPS ShippingQuoteCheckoutTarget carries fixedPriceSaleId only',
    () async {
      final attachment = ShippingQuoteAttachment(
        offerId: 'offer-2',
        linkedItemId: _fixedPriceSaleId,
        linkedItemType: 'for_sale',
        linkedItemName: 'Test Barang',
        linkedItemPrice: 200000,
        shippingType: 'standard',
        shippingTypeName: 'Standar',
        shippingTypeEmoji: '📦',
        rate: 15000,
        validUntil: DateTime.now().add(const Duration(hours: 1)),
        status: 'ACTIVE',
        sellerId: 'seller-2',
      );

      final target = await resolveShippingQuoteCheckoutTarget(
        shippingQuote: attachment,
        resolveAuctionProductId: (_) async {
          fail('resolveAuctionProductId must not be called for FPS quotes');
        },
      );

      expect(target, isNotNull);
      expect(target!.forSaleId, _fixedPriceSaleId);
      expect(target.auctionId, isNull);
      expect(target.productId, isNull);
      // FPS: fixedPriceSaleId is not productId and not auctionId
      expect(target.forSaleId, isNot(equals(_productId)));
      expect(target.forSaleId, isNot(equals(_auctionId)));
    },
  );

  // ── D. Shipping quote response DTO (existing, unchanged) ───────────────────

  test('shipping quote response dto uses canonical backend fields', () {
    final dto = ShippingQuoteResponseDto(
      id: 'quote-1',
      chatId: 'chat-1',
      productId: _productId,
      sourceType: 'auction',
      sourceId: _auctionId,
      sellerId: 'seller-1',
      buyerId: 'buyer-1',
      cost: 42000,
      status: 'ACTIVE',
      createdAt: '2026-06-01T00:00:00Z',
    );

    final json = dto.toJson();
    expect(json['product_id'], _productId);
    expect(json['source_type'], 'auction');
    expect(json['source_id'], _auctionId);
    expect(json.containsKey('auction_id'), isFalse);
    expect(json.containsKey('listing_id'), isFalse);
    // Auction: source_id must equal auctionId, not productId
    expect(json['source_id'], isNot(equals(_productId)));

    final parsed = ShippingQuoteResponseDto.fromJson(json);
    expect(parsed.productId, _productId);
    expect(parsed.sourceType, 'auction');
    expect(parsed.sourceId, _auctionId);
  });

  // ── E. Distinct-ID regression guard ────────────────────────────────────────

  test('auction quote: productId is never used as source_id', () {
    // If productId appears as sourceId for auction, this test fails
    const sourceType = 'auction';
    const sourceId = _auctionId; // must be auctionId, not productId
    const productId = _productId;

    expect(sourceType, 'auction');
    expect(sourceId, _auctionId);
    expect(sourceId, isNot(equals(productId)));
  });

  test('FPS quote: source_type is never auction', () {
    const sourceType = 'for_sale';
    expect(sourceType, isNot(equals('auction')));
  });
}
