import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/presentation/screens/chat_detail_screen.dart'
    show resolveChatFixedPriceSaleAttachmentId;
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_picker_bottom_sheet.dart';

ForSale _listing({
  String forSaleId = 'fps-1',
  String? productId = 'product-1',
  ForSaleStatus status = ForSaleStatus.active,
}) {
  return ForSale(
    forSaleId: forSaleId,
    productId: productId,
    title: 'Showa 28cm',
    description: 'Premium showa',
    price: 1500000,
    stock: 1,
    sellerId: 'seller-1',
    status: status,
    visibility: ForSaleVisibility.public,
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
    updatedAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
  );
}

void main() {
  group('ListingPicker caller intent contract', () {
    test('chat fixed-price-sale intent returns forSaleId explicitly', () {
      final listing = _listing();
      final selection = ListingPickerSelection.fromListing(listing);

      expect(
        ListingPickerIntent.fixedPriceSaleAttachment.matches(listing),
        isTrue,
      );
      expect(resolveChatFixedPriceSaleAttachmentId(selection), 'fps-1');
      expect(selection.forSaleId, 'fps-1');
    });

    // PASS_21C: the Listing.listingType field was removed entirely — the
    // backend never emits it (every real Listing is definitionally
    // fixed-price; FixedPriceSaleType has no "auction" value on the
    // backend either, see fixed_price_sale_type.go). There is no longer a
    // "listingType == auction" scenario to construct or exclude; type
    // safety now prevents it instead of a runtime filter.

    // PASS_21B: ListingPickerIntent.auctionSourceProduct was removed —
    // auction creation must never be sourced from a Listing. There is no
    // longer an "auction source" picker intent to test; CreateAuctionScreen
    // now takes inline product fields directly (see
    // create_auction_screen_contract_test.dart).
  });
}
