import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/user/preference/seller/data/dto/seller_analytics_dto.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/seller_analytics_read.dart';

void main() {
  group('SellerAnalyticsDto decode (canonical wire)', () {
    test('parses summary + mixed For Sale/Auction products', () {
      final dto = SellerAnalyticsDto.fromJson({
        'summary': {
          'total_views_30d': 120,
          'products_with_views_30d': 3,
          'products_sold_30d': 1,
        },
        'products': [
          {
            'product_id': 'p1',
            'title': 'Koi A',
            'surface_type': 'for_sale',
            'state': 'sold',
            'views_30d': 100,
            'sold': true,
            'bid_count_30d': 0,
          },
          {
            'product_id': 'p2',
            'title': 'Koi B',
            'surface_type': 'auction',
            'state': 'active',
            'views_30d': 20,
            'sold': false,
            'bid_count_30d': 7,
          },
          {
            'product_id': 'p3',
            'title': 'Koi C',
            'surface_type': 'auction',
            'state': 'ended',
            'views_30d': 0,
            'sold': true,
            'bid_count_30d': 0,
          },
        ],
      });

      expect(dto.summary.totalViews30d, 120);
      expect(dto.summary.productsWithViews30d, 3);
      expect(dto.summary.productsSold30d, 1);

      expect(dto.products, hasLength(3));

      final a = dto.products[0];
      expect(a.productId, 'p1');
      expect(a.title, 'Koi A');
      expect(a.surfaceType, 'for_sale');
      expect(a.state, 'sold');
      expect(a.views30d, 100);
      expect(a.sold, isTrue);
      expect(a.bidCount30d, 0);
      expect(a.isAuction, isFalse);

      final b = dto.products[1];
      expect(b.surfaceType, 'auction');
      expect(b.bidCount30d, 7);
      expect(b.sold, isFalse);
      expect(b.isAuction, isTrue);

      final c = dto.products[2];
      expect(c.state, 'ended');
      expect(c.sold, isTrue);
    });

    test('missing fields degrade to zero/empty (no crash)', () {
      final dto = SellerAnalyticsDto.fromJson({});
      expect(dto.summary.totalViews30d, 0);
      expect(dto.summary.productsWithViews30d, 0);
      expect(dto.summary.productsSold30d, 0);
      expect(dto.products, isEmpty);
    });

    test('empty aggregate entity is a truthful zero state', () {
      expect(SellerAnalytics.empty.summary.totalViews30d, 0);
      expect(SellerAnalytics.empty.summary.productsWithViews30d, 0);
      expect(SellerAnalytics.empty.summary.productsSold30d, 0);
      expect(SellerAnalytics.empty.products, isEmpty);
    });
  });
}
