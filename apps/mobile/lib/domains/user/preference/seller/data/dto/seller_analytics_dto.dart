/// Seller Analytics DTO — parses the canonical backend wire for
/// GET /api/v1/seller/analytics. The backend is the sole aggregation authority;
/// this DTO only decodes the returned numbers.
library;

import 'package:hishumi/domains/user/preference/seller/domain/entities/seller_analytics_read.dart';

class SellerAnalyticsDto {
  final SellerAnalyticsSummary summary;
  final List<SellerAnalyticsProduct> products;

  const SellerAnalyticsDto({required this.summary, required this.products});

  factory SellerAnalyticsDto.fromJson(Map<String, dynamic> json) {
    final summaryJson =
        json['summary'] as Map<String, dynamic>? ?? const <String, dynamic>{};
    final rawProducts = json['products'] as List? ?? const [];

    return SellerAnalyticsDto(
      summary: SellerAnalyticsSummary(
        totalViews30d: (summaryJson['total_views_30d'] as num?)?.toInt() ?? 0,
        productsWithViews30d:
            (summaryJson['products_with_views_30d'] as num?)?.toInt() ?? 0,
        productsSold30d:
            (summaryJson['products_sold_30d'] as num?)?.toInt() ?? 0,
      ),
      products: rawProducts
          .whereType<Map<String, dynamic>>()
          .map(
            (p) => SellerAnalyticsProduct(
              productId: p['product_id'] as String? ?? '',
              title: p['title'] as String? ?? '',
              surfaceType: p['surface_type'] as String? ?? '',
              state: p['state'] as String? ?? '',
              views30d: (p['views_30d'] as num?)?.toInt() ?? 0,
              sold: p['sold'] as bool? ?? false,
              bidCount30d: (p['bid_count_30d'] as num?)?.toInt() ?? 0,
            ),
          )
          .toList(),
    );
  }
}
