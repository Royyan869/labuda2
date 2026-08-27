/// Seller Analytics Entities
///
/// Pure Dart entities for seller analytics - no Firebase/Flutter dependencies.
library;

import 'package:equatable/equatable.dart';

/// Time period for analytics
enum AnalyticsPeriod { daily, weekly, monthly }

/// Sales data point for charts
class SalesDataPoint extends Equatable {
  final DateTime date;
  final double sales;
  final int orders;

  const SalesDataPoint({
    required this.date,
    required this.sales,
    required this.orders,
  });

  @override
  List<Object> get props => [date, sales, orders];
}

/// Seller Analytics Entity
class SellerAnalytics extends Equatable {
  final String sellerId;
  final AnalyticsPeriod period;
  final double currentPeriodSales;
  final double previousPeriodSales;
  final int currentPeriodOrders;
  final int previousPeriodOrders;
  final List<SalesDataPoint> salesDataPoints;
  final int? bestSellingDay;
  final int? peakSellingHour;
  final double conversionRate;
  final DateTime periodStart;
  final DateTime periodEnd;
  final DateTime calculatedAt;

  const SellerAnalytics({
    required this.sellerId,
    required this.period,
    required this.currentPeriodSales,
    required this.previousPeriodSales,
    required this.currentPeriodOrders,
    required this.previousPeriodOrders,
    required this.salesDataPoints,
    this.bestSellingDay,
    this.peakSellingHour,
    this.conversionRate = 0.0,
    required this.periodStart,
    required this.periodEnd,
    required this.calculatedAt,
  });

  /// Sales growth percentage
  double get salesGrowthPercentage {
    if (previousPeriodSales == 0) return currentPeriodSales > 0 ? 100 : 0;
    return ((currentPeriodSales - previousPeriodSales) / previousPeriodSales) *
        100;
  }

  /// Order growth percentage
  double get orderGrowthPercentage {
    if (previousPeriodOrders == 0) return currentPeriodOrders > 0 ? 100 : 0;
    return ((currentPeriodOrders - previousPeriodOrders) /
            previousPeriodOrders) *
        100;
  }

  /// Average order value
  double get averageOrderValue =>
      currentPeriodOrders > 0 ? currentPeriodSales / currentPeriodOrders : 0;

  /// Projected sales for next period
  double get projectedNextPeriodSales {
    if (salesGrowthPercentage == 0) return currentPeriodSales;
    return currentPeriodSales * (1 + (salesGrowthPercentage / 100));
  }

  /// Create empty analytics
  factory SellerAnalytics.empty(String sellerId) {
    final now = DateTime.now();
    return SellerAnalytics(
      sellerId: sellerId,
      period: AnalyticsPeriod.daily,
      currentPeriodSales: 0,
      previousPeriodSales: 0,
      currentPeriodOrders: 0,
      previousPeriodOrders: 0,
      salesDataPoints: const [],
      periodStart: now,
      periodEnd: now,
      calculatedAt: now,
    );
  }

  @override
  List<Object?> get props => [
    sellerId,
    period,
    currentPeriodSales,
    previousPeriodSales,
    currentPeriodOrders,
    previousPeriodOrders,
    salesDataPoints,
    bestSellingDay,
    peakSellingHour,
    conversionRate,
    periodStart,
    periodEnd,
    calculatedAt,
  ];
}

/// Seller Performance Entity
class SellerPerformance extends Equatable {
  final String sellerId;
  final double customerSatisfactionScore;
  final int totalReviews;
  final int fiveStarReviews;
  final int fourStarReviews;
  final int threeStarReviews;
  final int twoStarReviews;
  final int oneStarReviews;
  final double averageResponseTime;
  final double orderFulfillmentRate;
  final double onTimeDeliveryRate;
  final int totalCompletedOrders;
  final int totalFailedOrders;
  final double averageProcessingTime;
  final double returnRate;
  final double repeatCustomerRate;
  final DateTime calculatedAt;

  const SellerPerformance({
    required this.sellerId,
    required this.customerSatisfactionScore,
    required this.totalReviews,
    required this.fiveStarReviews,
    required this.fourStarReviews,
    required this.threeStarReviews,
    required this.twoStarReviews,
    required this.oneStarReviews,
    required this.averageResponseTime,
    required this.orderFulfillmentRate,
    required this.onTimeDeliveryRate,
    required this.totalCompletedOrders,
    required this.totalFailedOrders,
    required this.averageProcessingTime,
    required this.returnRate,
    required this.repeatCustomerRate,
    required this.calculatedAt,
  });

  /// Average rating (0-5 stars)
  double get averageRating {
    if (totalReviews == 0) return 0;
    final totalStars =
        (fiveStarReviews * 5) +
        (fourStarReviews * 4) +
        (threeStarReviews * 3) +
        (twoStarReviews * 2) +
        (oneStarReviews * 1);
    return totalStars / totalReviews;
  }

  /// Percentage of 5-star reviews
  double get fiveStarPercentage =>
      totalReviews > 0 ? (fiveStarReviews / totalReviews) * 100 : 0;

  /// Order success rate
  double get orderSuccessRate {
    final total = totalCompletedOrders + totalFailedOrders;
    if (total == 0) return 0;
    return (totalCompletedOrders / total) * 100;
  }

  /// Create empty performance
  factory SellerPerformance.empty(String sellerId) {
    return SellerPerformance(
      sellerId: sellerId,
      customerSatisfactionScore: 0,
      totalReviews: 0,
      fiveStarReviews: 0,
      fourStarReviews: 0,
      threeStarReviews: 0,
      twoStarReviews: 0,
      oneStarReviews: 0,
      averageResponseTime: 0,
      orderFulfillmentRate: 0,
      onTimeDeliveryRate: 0,
      totalCompletedOrders: 0,
      totalFailedOrders: 0,
      averageProcessingTime: 0,
      returnRate: 0,
      repeatCustomerRate: 0,
      calculatedAt: DateTime.now(),
    );
  }

  @override
  List<Object?> get props => [
    sellerId,
    customerSatisfactionScore,
    totalReviews,
    fiveStarReviews,
    fourStarReviews,
    threeStarReviews,
    twoStarReviews,
    oneStarReviews,
    averageResponseTime,
    orderFulfillmentRate,
    onTimeDeliveryRate,
    totalCompletedOrders,
    totalFailedOrders,
    averageProcessingTime,
    returnRate,
    repeatCustomerRate,
    calculatedAt,
  ];
}
