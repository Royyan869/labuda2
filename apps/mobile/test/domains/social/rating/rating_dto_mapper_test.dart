import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/social/rating/data/dto/rating_api_models.dart';
import 'package:labuda/domains/social/rating/data/mappers/rating_api_mapper.dart';

// CANONICAL RATING DTO + MAPPER CONTRACT TEST
//
// Locks the mobile side of the canonical Rating HTTP contract:
// - snake_case JSON keys (id, order_id, buyer_id, seller_id, rating_value,
//   comment, created_at; total_ratings..five_star_count)
// - raw buyer_id / seller_id identity; NO reviewer, NO verified_purchase
// - BARE rating collection; NO items/has_more/next_cursor envelope
// - int64 limit/cursor pagination carried at the datasource layer

void main() {
  group('RatingApiResponse.fromJson', () {
    test('parses canonical snake_case item exactly', () {
      final dto = RatingApiResponse.fromJson(const {
        'id': 'r1',
        'order_id': 'o1',
        'buyer_id': 'b1',
        'seller_id': 's1',
        'rating_value': 5,
        'comment': 'great',
        'created_at': '2026-01-15T10:30:00Z',
      });

      expect(dto.id, 'r1');
      expect(dto.orderId, 'o1');
      expect(dto.buyerId, 'b1');
      expect(dto.sellerId, 's1');
      expect(dto.ratingValue, 5);
      expect(dto.comment, 'great');
      expect(dto.createdAt, DateTime.utc(2026, 1, 15, 10, 30));
    });

    test('parses optional comment as null when absent', () {
      final dto = RatingApiResponse.fromJson(const {
        'id': 'r2',
        'order_id': 'o2',
        'buyer_id': 'b2',
        'seller_id': 's2',
        'rating_value': 3,
        'created_at': '2026-02-01T00:00:00Z',
      });

      expect(dto.comment, isNull);
    });

    test('serializes canonically without reviewer/verified_purchase/envelope', () {
      final dto = RatingApiResponse(
        id: 'r3',
        orderId: 'o3',
        buyerId: 'b3',
        sellerId: 's3',
        ratingValue: 4,
        comment: 'ok',
        createdAt: DateTime.utc(2026, 3, 1),
      );

      final json = dto.toJson();

      expect(json, {
        'id': 'r3',
        'order_id': 'o3',
        'buyer_id': 'b3',
        'seller_id': 's3',
        'rating_value': 4,
        'comment': 'ok',
        'created_at': dto.createdAt.toIso8601String(),
      });
      expect(json.containsKey('reviewer'), isFalse);
      expect(json.containsKey('verified_purchase'), isFalse);
      expect(json.containsKey('has_more'), isFalse);
      expect(json.containsKey('next_cursor'), isFalse);
      expect(json.containsKey('items'), isFalse);
    });
  });

  group('RatingSummaryApiResponse.fromJson', () {
    test('parses canonical aggregate keys', () {
      final dto = RatingSummaryApiResponse.fromJson(const {
        'total_ratings': 10,
        'average_rating': 4.2,
        'one_star_count': 0,
        'two_star_count': 1,
        'three_star_count': 2,
        'four_star_count': 3,
        'five_star_count': 4,
      });

      expect(dto.totalRatings, 10);
      expect(dto.averageRating, 4.2);
      expect(dto.oneStarCount, 0);
      expect(dto.twoStarCount, 1);
      expect(dto.threeStarCount, 2);
      expect(dto.fourStarCount, 3);
      expect(dto.fiveStarCount, 4);
    });
  });

  group('CreateRatingApiRequest', () {
    test('serializes snake_case rating_value + optional comment', () {
      const withComment = CreateRatingApiRequest(ratingValue: 5, comment: 'top');
      const withoutComment = CreateRatingApiRequest(ratingValue: 1);

      expect(withComment.toJson(), {
        'rating_value': 5,
        'comment': 'top',
      });
      expect(withoutComment.toJson(), {
        'rating_value': 1,
      });
    });
  });

  group('RatingApiMapper', () {
    test('toRating maps bare identity and drops nothing', () {
      final response = RatingApiResponse.fromJson(const {
        'id': 'r1',
        'order_id': 'o1',
        'buyer_id': 'b1',
        'seller_id': 's1',
        'rating_value': 5,
        'comment': 'great',
        'created_at': '2026-01-15T10:30:00Z',
      });

      final rating = RatingApiMapper.toRating(response);

      expect(rating.id, 'r1');
      expect(rating.orderId, 'o1');
      expect(rating.buyerId, 'b1');
      expect(rating.sellerId, 's1');
      expect(rating.ratingValue, 5);
      expect(rating.comment, 'great');
      expect(rating.createdAt, DateTime.utc(2026, 1, 15, 10, 30));
    });

    test('toRatingList preserves order and count', () {
      final responses = [
        RatingApiResponse.fromJson(const {
          'id': 'r1',
          'order_id': 'o1',
          'buyer_id': 'b1',
          'seller_id': 's1',
          'rating_value': 5,
          'created_at': '2026-01-15T10:30:00Z',
        }),
        RatingApiResponse.fromJson(const {
          'id': 'r2',
          'order_id': 'o2',
          'buyer_id': 'b2',
          'seller_id': 's2',
          'rating_value': 2,
          'created_at': '2026-01-16T10:30:00Z',
        }),
      ];

      final ratings = RatingApiMapper.toRatingList(responses);

      expect(ratings, hasLength(2));
      expect(ratings.first.id, 'r1');
      expect(ratings.last.buyerId, 'b2');
      expect(ratings.last.sellerId, 's2');
    });

    test('toRatingSummary maps all aggregates', () {
      final response = RatingSummaryApiResponse.fromJson(const {
        'total_ratings': 10,
        'average_rating': 4.2,
        'one_star_count': 0,
        'two_star_count': 1,
        'three_star_count': 2,
        'four_star_count': 3,
        'five_star_count': 4,
      });

      final summary = RatingApiMapper.toRatingSummary(response);

      expect(summary.totalRatings, 10);
      expect(summary.averageRating, 4.2);
      expect(summary.oneStarCount, 0);
      expect(summary.fiveStarCount, 4);
      expect(summary.distribution, {1: 0, 2: 1, 3: 2, 4: 3, 5: 4});
    });
  });

  group('Rating entity canonical semantics', () {
    test('equality covers bare canonical fields only', () {
      final a = RatingApiMapper.toRating(RatingApiResponse.fromJson(const {
        'id': 'r1',
        'order_id': 'o1',
        'buyer_id': 'b1',
        'seller_id': 's1',
        'rating_value': 5,
        'created_at': '2026-01-15T10:30:00Z',
      }));
      final b = RatingApiMapper.toRating(RatingApiResponse.fromJson(const {
        'id': 'r1',
        'order_id': 'o1',
        'buyer_id': 'b1',
        'seller_id': 's1',
        'rating_value': 5,
        'created_at': '2026-01-15T10:30:00Z',
      }));
      final c = RatingApiMapper.toRating(RatingApiResponse.fromJson(const {
        'id': 'r1',
        'order_id': 'o1',
        'buyer_id': 'b1',
        'seller_id': 's1',
        'rating_value': 4,
        'created_at': '2026-01-15T10:30:00Z',
      }));

      expect(a, equals(b));
      expect(a, isNot(equals(c)));
    });

    test('copyWith adjusts only requested canonical fields', () {
      final base = RatingApiMapper.toRating(RatingApiResponse.fromJson(const {
        'id': 'r1',
        'order_id': 'o1',
        'buyer_id': 'b1',
        'seller_id': 's1',
        'rating_value': 5,
        'created_at': '2026-01-15T10:30:00Z',
      }));

      final copy = base.copyWith(comment: 'updated');

      expect(copy.comment, 'updated');
      expect(copy.ratingValue, 5);
      expect(copy.buyerId, 'b1');
      expect(copy.sellerId, 's1');
    });
  });
}