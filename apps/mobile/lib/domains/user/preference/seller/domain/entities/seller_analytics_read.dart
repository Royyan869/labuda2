/// Seller Analytics read model.
///
/// A 30-day read projection over canonical Product View + sale/auction data.
/// It is NOT a domain authority: every number is computed by the backend from
/// canonical sources (product_view_events, orders, for_sales, auctions,
/// auction_bids). Mobile never re-aggregates these numbers.
library;

import 'package:equatable/equatable.dart';

/// Seller-level rollup for the 30-day window.
class SellerAnalyticsSummary extends Equatable {
  final int totalViews30d;
  final int productsWithViews30d;
  final int productsSold30d;

  const SellerAnalyticsSummary({
    required this.totalViews30d,
    required this.productsWithViews30d,
    required this.productsSold30d,
  });

  static const empty = SellerAnalyticsSummary(
    totalViews30d: 0,
    productsWithViews30d: 0,
    productsSold30d: 0,
  );

  @override
  List<Object?> get props => [
    totalViews30d,
    productsWithViews30d,
    productsSold30d,
  ];
}

/// One product's analytics row.
class SellerAnalyticsProduct extends Equatable {
  final String productId;
  final String title;
  final String surfaceType; // "for_sale" | "auction"
  final String state; // canonical surface status
  final int views30d;
  final bool sold;
  final int bidCount30d; // auction only (0 for for_sale)

  const SellerAnalyticsProduct({
    required this.productId,
    required this.title,
    required this.surfaceType,
    required this.state,
    required this.views30d,
    required this.sold,
    required this.bidCount30d,
  });

  bool get isAuction => surfaceType == 'auction';

  @override
  List<Object?> get props => [
    productId,
    title,
    surfaceType,
    state,
    views30d,
    sold,
    bidCount30d,
  ];
}

/// Seller Analytics aggregate: summary + per-product rows.
class SellerAnalytics extends Equatable {
  final SellerAnalyticsSummary summary;
  final List<SellerAnalyticsProduct> products;

  const SellerAnalytics({required this.summary, required this.products});

  static const empty = SellerAnalytics(
    summary: SellerAnalyticsSummary.empty,
    products: [],
  );

  @override
  List<Object?> get props => [summary, products];
}
