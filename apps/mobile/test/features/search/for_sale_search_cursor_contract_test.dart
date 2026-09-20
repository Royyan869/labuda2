// For Sale search cursor contract tests.
//
// Canonical backend contract for GET /api/v1/search/for-sale:
//   { for_sales: [...], next_cursor: "...", has_more: bool }
// There is NO fallback to the legacy offset contract (forSales/total/offset).

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';

void main() {
  group('ForSaleSearchResponseDto — canonical cursor contract', () {
    test('parses for_sales, next_cursor and has_more from canonical payload', () {
      final dto = ForSaleSearchResponseDto.fromJson(const <String, dynamic>{
        'for_sales': <Map<String, dynamic>>[],
        'next_cursor': 'cursor-2',
        'has_more': true,
      });

      expect(dto.forSales, isEmpty);
      expect(dto.nextCursor, 'cursor-2');
      expect(dto.hasMore, isTrue);
    });

    test('terminal page: has_more=false and next_cursor=null', () {
      final dto = ForSaleSearchResponseDto.fromJson(const <String, dynamic>{
        'for_sales': <Map<String, dynamic>>[],
        'has_more': false,
      });

      expect(dto.nextCursor, isNull);
      expect(dto.hasMore, isFalse);
    });

    test('legacy offset keys are NOT parsed (no dual contract fallback)', () {
      // A payload that only speaks the old offset model must NOT populate
      // the canonical page — single authority, no compatibility parser.
      final dto = ForSaleSearchResponseDto.fromJson(const <String, dynamic>{
        'query': 'koi',
        'forSales': <Map<String, dynamic>>[],
        'total': 99,
        'limit': 20,
        'offset': 0,
      });

      expect(dto.forSales, isEmpty);
      expect(dto.nextCursor, isNull);
      expect(dto.hasMore, isFalse);
    });

    test('canonical serialization round-trips cursor fields', () {
      const dto = ForSaleSearchResponseDto(
        forSales: <ForSaleSearchResultDto>[],
        nextCursor: 'cursor-9',
        hasMore: true,
      );

      expect(dto.toJson(), {
        'for_sales': <Object?>[],
        'next_cursor': 'cursor-9',
        'has_more': true,
      });
    });
  });
}
