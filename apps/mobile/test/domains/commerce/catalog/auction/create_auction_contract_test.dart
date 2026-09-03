import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/repositories/auction_repository.dart';

/// PASS_18E/PASS_21B: locks CreateAuctionDto's wire shape to the backend's
/// `CreateAuctionRequest` contract (internal/commerce/auction/delivery/http)
/// — this is a regression guard against the exact silent mismatch PASS_18D
/// found (missing shipping_setup_ids; wrong images/category keys), and
/// against the rejected "auction created from listing" design (PASS_21B):
/// the backend creates the Product inline from item fields, so there must
/// never be a product_id/listing_id key on this request.
void main() {
  group('Create auction contract', () {
    test(
      'CreateAuctionDto serializes the full backend-required/optional field set',
      () {
        final dto = CreateAuctionDto(
          title: 'Showa Auction',
          description: 'Auction request payload',
          mediaUrls: const ['https://cdn.example.com/a.jpg'],
          variety: 'showa',
          sizeCm: 28,
          ageMonths: 18,
          gender: 'female',
          breeder: 'Farm A',
          bloodline: 'Sakai',
          certificates: const ['cert-1'],
          farmAddressId: 'address-1',
          shippingSetupIds: const ['option-1', 'option-2'],
          startPrice: 1000000,
          bidIncrement: 50000,
          buyNowPrice: 2500000,
          startMode: 'now',
          durationHours: 72,
          preparationNote: 'Handle with care',
        );

        final json = dto.toJson();

        expect(json['title'], 'Showa Auction');
        expect(json['media_urls'], const ['https://cdn.example.com/a.jpg']);
        expect(json['variety'], 'showa');
        expect(json['size_cm'], 28);
        expect(json['age_months'], 18);
        expect(json['gender'], 'female');
        expect(json['breeder'], 'Farm A');
        expect(json['bloodline'], 'Sakai');
        expect(json['certificates'], const ['cert-1']);
        expect(json['farm_address_id'], 'address-1');
        expect(json['shipping_setup_ids'], const ['option-1', 'option-2']);
        expect(json['start_price'], 1000000);
        expect(json['bid_increment'], 50000);
        expect(json['buy_now_price'], 2500000);
        expect(json['start_mode'], 'now');
        expect(json['duration_hours'], 72);
        expect(json['preparation_note'], 'Handle with care');
        expect(json.containsKey('scheduled_start_at'), isFalse);

        // Regression guard: the exact stale keys PASS_18D found must never
        // reappear — backend has no 'images'/'category'/'condition' fields.
        expect(json.containsKey('images'), isFalse);
        expect(json.containsKey('category'), isFalse);
        expect(json.containsKey('condition'), isFalse);

        // Regression guard (PASS_21B): auction must never be sourced from a
        // Listing — the backend creates the Product inline, so there is no
        // product_id/listing_id key on this request at all.
        expect(json.containsKey('product_id'), isFalse);
        expect(json.containsKey('productId'), isFalse);
        expect(json.containsKey('listing_id'), isFalse);
        expect(json.containsKey('listingId'), isFalse);
      },
    );

    test(
      'CreateAuctionDto requires shipping_setup_ids even when empty (backend rejects empty list too)',
      () {
        final dto = CreateAuctionDto(
          title: 'Showa Auction',
          mediaUrls: const [],
          shippingSetupIds: const [],
          startPrice: 1000000,
          startMode: 'now',
          durationHours: 24,
        );

        // The DTO itself does not enforce non-empty — that is a UI-layer
        // validation gate (create_auction_screen.dart) before submit. This
        // test locks that the key is always present so an empty selection
        // fails loudly against the backend's `min=1` binding instead of the
        // field silently vanishing from the payload.
        expect(dto.toJson()['shipping_setup_ids'], const []);
        expect(dto.toJson().containsKey('shipping_setup_ids'), isTrue);
      },
    );

    test(
      'CreateAuctionDto serializes scheduled_start_at when start_mode is scheduled',
      () {
        final dto = CreateAuctionDto(
          title: 'Showa Auction',
          description: 'Auction request payload',
          mediaUrls: const ['https://cdn.example.com/a.jpg'],
          shippingSetupIds: const ['option-1'],
          startPrice: 1000000,
          startMode: 'scheduled',
          scheduledStartAt: DateTime.utc(2026, 1, 1, 0, 0),
          durationHours: 24,
        );

        final json = dto.toJson();

        expect(json['start_mode'], 'scheduled');
        expect(json['scheduled_start_at'], '2026-01-01T00:00:00.000Z');
        expect(json['duration_hours'], 24);
      },
    );

    test(
      'AuctionMapper.toCreateDto threads koiDetails and shippingSetupIds through to the DTO, with no product/listing ID',
      () {
        final params = CreateAuctionParams(
          sellerId: 'seller-1',
          title: 'Showa Auction',
          description: 'Auction request payload',
          mediaUrls: const ['https://cdn.example.com/a.jpg'],
          mediaTypes: const [AuctionMediaType.photo],
          koiDetails: const KoiDetails(
            variety: 'showa',
            sizeInCm: 28,
            ageInMonths: 18,
            gender: 'female',
            breeder: 'Farm A',
            bloodline: 'Sakai',
            certificates: ['cert-1'],
          ),
          openingBid: 1000000,
          bidIncrement: 50000,
          buyNowPrice: 2500000,
          startMode: 'now',
          durationHours: 72,
          farmAddressId: 'address-1',
          shippingSetupIds: const ['option-1', 'option-2'],
          preparationNote: 'Handle with care',
        );

        final dto = AuctionMapper.toCreateDto(params);
        final json = dto.toJson();

        expect(json['variety'], 'showa');
        expect(json['size_cm'], 28);
        expect(json['age_months'], 18);
        expect(json['gender'], 'female');
        expect(json['breeder'], 'Farm A');
        expect(json['bloodline'], 'Sakai');
        expect(json['certificates'], const ['cert-1']);
        expect(json['farm_address_id'], 'address-1');
        expect(json['shipping_setup_ids'], const ['option-1', 'option-2']);
        expect(json['media_urls'], const ['https://cdn.example.com/a.jpg']);
        expect(json['preparation_note'], 'Handle with care');
        expect(json.containsKey('product_id'), isFalse);
        expect(json.containsKey('listing_id'), isFalse);
      },
    );

    test(
      'AuctionMapper.toCreateDto omits empty certificates rather than sending []',
      () {
        final params = CreateAuctionParams(
          sellerId: 'seller-1',
          title: 'Showa Auction',
          description: 'Auction request payload',
          mediaUrls: const [],
          mediaTypes: const [],
          koiDetails: const KoiDetails(
            variety: 'showa',
            sizeInCm: 28,
            ageInMonths: 18,
            gender: 'female',
          ),
          openingBid: 1000000,
          bidIncrement: 50000,
          startMode: 'now',
          durationHours: 24,
          shippingSetupIds: const ['option-1'],
        );

        final dto = AuctionMapper.toCreateDto(params);

        expect(dto.certificates, isNull);
        expect(dto.toJson().containsKey('certificates'), isFalse);
      },
    );
  });
}
