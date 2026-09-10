// Auction detail seller identity — SINGLE AUTHORITY proof.
//
// Backend authority (GET /api/v1/auctions/:id):
//   auctionToDetailResponseWithSeller emits BOTH flat identity scalars
//   (seller_username / seller_farm_name / seller_avatar_url from
//   sellerdisplay.Info) AND a nested `seller_identity` projection
//   ({store_name, store_image_url, username, avatar_url, public_origin_line?}).
//
// Audit verdict: on the auction detail wire `seller_identity` is DUPLICATE
// TRANSPORT — username == seller_username, store_name == seller_farm_name,
// avatar_url == resolved(seller_avatar_url), and store_image_url +
// public_origin_line are structurally empty on this path (the sellerdisplay
// query hardcodes '' for both). The flat scalars are the single identity
// source that is present on EVERY auction surface (list AND detail), so the
// Auction read model consumes exactly those — never a second identity model,
// never a seller_identity fallback.
//
// These tests pin that decision: flat scalars are the authority, and a
// wire-carrying `seller_identity` block neither overrides nor substitutes for
// them.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';

Map<String, dynamic> _basePayload() {
  return {
    'id': 'auction-1',
    'seller_id': 'seller-1',
    'product_id': 'product-1',
    'title': 'Auction title',
    'description': 'Auction description',
    'media_urls': <String>['https://cdn.example.com/koi.jpg'],
    'start_price': 100000,
    'bid_increment': 5000,
    'current_bid': 100000,
    'start_at': '2026-07-26T12:20:13+07:00',
    'end_at': '2026-07-31T12:20:13+07:00',
    'status': 'active',
    'created_at': '2026-07-26T12:20:13+07:00',
    'updated_at': '2026-07-26T12:20:13+07:00',
    'seller_username': 'qiqijho',
    'seller_farm_name': 'Qiqi Store',
    'seller_avatar_url': 'https://cdn.example.com/avatar.jpg',
  };
}

Map<String, dynamic> _duplicateTransportSellerIdentity() {
  return {
    // Same values as the flat scalars — the only values the current auction
    // backend can emit (store_image_url / public_origin_line are hardcoded ''
    // by the sellerdisplay query on this path).
    'store_name': 'Qiqi Store',
    'store_image_url': '',
    'username': 'qiqijho',
    'avatar_url': 'https://cdn.example.com/avatar-resolved.jpg',
  };
}

void main() {
  test('flat seller scalars are the single identity authority', () {
    final dto = AuctionDto.fromJson(_basePayload());
    final entity = AuctionMapper.toEntity(dto);

    // DTO reads the flat canonical scalars.
    expect(dto.sellerUsername, 'qiqijho');
    expect(dto.sellerFarmName, 'Qiqi Store');
    expect(dto.sellerAvatarUrl, 'https://cdn.example.com/avatar.jpg');

    // Entity identity comes from exactly those scalars — the same values the
    // AuctionSellerCard renders.
    expect(entity.sellerUsername, 'qiqijho');
    expect(entity.sellerFarmName, 'Qiqi Store');
    expect(entity.sellerAvatar, 'https://cdn.example.com/avatar.jpg');
  });

  test(
    'a structurally present seller_identity block never overrides flat scalars',
    () {
      final payload = _basePayload()
        ..['seller_identity'] = _duplicateTransportSellerIdentity();
      final dto = AuctionDto.fromJson(payload);
      final entity = AuctionMapper.toEntity(dto);

      // The duplicate-transport projection does not become a second identity
      // source: flat-scalar identity is preserved verbatim.
      expect(entity.sellerUsername, 'qiqijho');
      expect(entity.sellerFarmName, 'Qiqi Store');
      expect(entity.sellerAvatar, 'https://cdn.example.com/avatar.jpg');
      expect(dto.sellerUsername, 'qiqijho');
      expect(dto.sellerFarmName, 'Qiqi Store');
    },
  );

  test(
    'seller_identity never substitutes for absent flat scalars (no phantom fallback)',
    () {
      // Hypothetical payload carrying ONLY the nested projection — this is
      // not what the current auction backend emits (flat scalars are always
      // present), but even so the read model must NOT fall back to it.
      final payload = _basePayload()
        ..remove('seller_username')
        ..remove('seller_farm_name')
        ..remove('seller_avatar_url')
        ..['seller_identity'] = {
          'store_name': 'Only Store',
          'username': 'only_user',
          'avatar_url': 'https://cdn.example.com/only.jpg',
        };

      final entity = AuctionMapper.toEntity(AuctionDto.fromJson(payload));

      expect(entity.sellerUsername, isNull);
      expect(entity.sellerFarmName, isNull);
      expect(entity.sellerAvatar, isNull);
    },
  );
}