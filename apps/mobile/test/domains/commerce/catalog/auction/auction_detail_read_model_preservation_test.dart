// Auction detail read contract preservation tests.
//
// Proves the canonical GET /api/v1/auctions/:id wire
// (auctionToDetailResponseWithSeller: media_urls, variety, size_cm,
// age_months, gender, breeder, bloodline, certificates, preparation_time,
// preparation_note) is parsed by AuctionDto.fromJson and preserved by
// AuctionMapper.toEntity into the Auction read model — no canonical value may
// be replaced by 'Unknown' / 0 / 'unknown' / [] / null.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/types/preparation_time.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';

Map<String, dynamic> _canonicalDetailPayload() => {
  'id': 'auction-1',
  'seller_id': 'seller-1',
  'product_id': 'product-1',
  'title': 'Showa Koi Auction',
  'description': 'Premium showa from Akira',
  'media_urls': [
    'https://cdn.example.com/koi-1.jpg',
    'https://cdn.example.com/koi-2.jpg',
    'https://cdn.example.com/koi-3.jpg',
  ],
  'variety': 'Showa',
  'size_cm': 28,
  'age_months': 18,
  'gender': 'female',
  'breeder': 'Akira',
  'bloodline': 'Matsunosuke',
  'certificates': ['breeder', 'health'],
  'preparation_time': 'medium',
  'preparation_note': 'Karantina 3 hari sebelum kirim',
  'start_price': 500000,
  'bid_increment': 25000,
  'buy_now_price': 800000,
  'current_bid': 500000,
  'start_at': '2026-07-26T12:20:13+07:00',
  'end_at': '2026-07-31T12:20:13+07:00',
  'status': 'active',
  'created_at': '2026-07-26T12:20:13+07:00',
  'updated_at': '2026-07-26T12:20:13+07:00',
  'seller_username': 'seller_user',
  'seller_farm_name': 'Acme Farm',
  'seller_avatar_url': 'https://cdn.example.com/avatar.jpg',
};

void main() {
  group('AuctionDto parses the canonical detail wire without dropping values', () {
    test('koi, media, and preparation fields survive fromJson', () {
      final dto = AuctionDto.fromJson(_canonicalDetailPayload());

      expect(dto.variety, 'Showa');
      expect(dto.sizeCm, 28);
      expect(dto.ageMonths, 18);
      expect(dto.gender, 'female');
      expect(dto.breeder, 'Akira');
      expect(dto.bloodline, 'Matsunosuke');
      expect(dto.certificates, ['breeder', 'health']);
      expect(dto.preparationTime, 'medium');
      expect(dto.preparationNote, 'Karantina 3 hari sebelum kirim');

      // Full media collection — not just a thumbnail / first image.
      expect(dto.images, [
        'https://cdn.example.com/koi-1.jpg',
        'https://cdn.example.com/koi-2.jpg',
        'https://cdn.example.com/koi-3.jpg',
      ]);
    });

    test('absent canonical content stays absent (no fabricated values)', () {
      final payload = _canonicalDetailPayload()
        ..remove('media_urls')
        ..remove('variety')
        ..remove('size_cm')
        ..remove('age_months')
        ..remove('gender')
        ..remove('breeder')
        ..remove('bloodline')
        ..remove('certificates')
        ..remove('preparation_time')
        ..remove('preparation_note');

      final dto = AuctionDto.fromJson(payload);

      expect(dto.images, isEmpty);
      expect(dto.variety, isNull);
      expect(dto.sizeCm, isNull);
      expect(dto.ageMonths, isNull);
      expect(dto.gender, isNull);
      expect(dto.breeder, isNull);
      expect(dto.bloodline, isNull);
      expect(dto.certificates, isEmpty);
      expect(dto.preparationTime, isNull);
      expect(dto.preparationNote, isNull);
    });
  });

  group('AuctionMapper preserves canonical values into the read model', () {
    test('non-default koi/media/preparation values reach the Auction entity', () {
      final dto = AuctionDto.fromJson(_canonicalDetailPayload());
      final entity = AuctionMapper.toEntity(dto);

      // Koi details: canonical values, NOT synthetic 'Unknown'/0/unknown.
      expect(entity.koiDetails.variety, 'Showa');
      expect(entity.koiDetails.sizeInCm, 28.0);
      expect(entity.koiDetails.ageInMonths, 18);
      expect(entity.koiDetails.gender, 'female');
      expect(entity.koiDetails.breeder, 'Akira');
      expect(entity.koiDetails.bloodline, 'Matsunosuke');
      expect(entity.koiDetails.certificates, ['breeder', 'health']);

      // Full media collection preserved (carousel source).
      expect(entity.media, hasLength(3));
      expect(entity.media[0].originalUrl, 'https://cdn.example.com/koi-1.jpg');
      expect(entity.media[1].originalUrl, 'https://cdn.example.com/koi-2.jpg');
      expect(entity.media[2].originalUrl, 'https://cdn.example.com/koi-3.jpg');

      // Shipping readiness preserved.
      expect(entity.preparationTime, PreparationTime.medium);
      expect(entity.preparationNote, 'Karantina 3 hari sebelum kirim');
    });

    test('absence defaults appear only when the wire omits canonical values', () {
      final payload = _canonicalDetailPayload()
        ..remove('media_urls')
        ..remove('variety')
        ..remove('size_cm')
        ..remove('age_months')
        ..remove('gender')
        ..remove('breeder')
        ..remove('bloodline')
        ..remove('certificates')
        ..remove('preparation_time')
        ..remove('preparation_note');

      final entity = AuctionMapper.toEntity(AuctionDto.fromJson(payload));

      // LEGITIMATE_ABSENCE_DEFAULT: wire truly omits the values, so the read
      // model falls back to its presentational placeholders (unchanged from
      // pre-convergence behavior for list/legacy payloads).
      expect(entity.koiDetails.variety, 'Unknown');
      expect(entity.koiDetails.sizeInCm, 0);
      expect(entity.koiDetails.ageInMonths, 0);
      expect(entity.koiDetails.gender, 'unknown');
      expect(entity.koiDetails.breeder, isNull);
      expect(entity.koiDetails.bloodline, isNull);
      expect(entity.koiDetails.certificates, isEmpty);

      // Absence is NOT masked as "ready to ship immediately".
      expect(entity.preparationTime, isNull);
      expect(entity.preparationNote, isNull);
      expect(entity.media, isEmpty);
    });
  });
}
