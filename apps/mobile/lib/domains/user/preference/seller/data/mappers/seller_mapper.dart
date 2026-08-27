/// Seller Mappers
///
/// Converts between Domain Entities and DTOs.
library;

import 'package:labuda/domains/user/preference/seller/data/models/api/seller_api_models.dart';

import '../../domain/entities/seller_activity.dart';
import '../../domain/entities/seller_analytics.dart';
import '../../domain/entities/seller_dashboard.dart';
import '../../domain/entities/seller_earnings.dart';
import '../dto/seller_dto.dart';

/// Dashboard Stats Mapper
class DashboardStatsMapper {
  /// Convert DTO to Entity
  static SellerDashboardStats toEntity(DashboardStatsDto dto) {
    return SellerDashboardStats(
      totalOrders: dto.totalOrders,
      pendingOrders: dto.pendingOrders,
      processingOrders: dto.processingOrders,
      completedOrders: dto.completedOrders,
      cancelledOrders: dto.cancelledOrders,
      refundedOrders: dto.refundedOrders,
      problematicOrders: dto.problematicOrders,
      totalRevenue: dto.totalRevenue,
      pendingRevenue: dto.pendingRevenue,
      refundedRevenue: dto.refundedRevenue,
      totalListings: dto.totalListings,
      activeListings: dto.activeListings,
      soldListings: dto.soldListings,
      totalAuctions: dto.totalAuctions,
      activeAuctions: dto.activeAuctions,
    );
  }

  /// Convert Entity to DTO
  static DashboardStatsDto toDto(SellerDashboardStats entity) {
    return DashboardStatsDto(
      totalOrders: entity.totalOrders,
      pendingOrders: entity.pendingOrders,
      processingOrders: entity.processingOrders,
      completedOrders: entity.completedOrders,
      cancelledOrders: entity.cancelledOrders,
      refundedOrders: entity.refundedOrders,
      problematicOrders: entity.problematicOrders,
      totalRevenue: entity.totalRevenue,
      pendingRevenue: entity.pendingRevenue,
      refundedRevenue: entity.refundedRevenue,
      totalListings: entity.totalListings,
      activeListings: entity.activeListings,
      soldListings: entity.soldListings,
      totalAuctions: entity.totalAuctions,
      activeAuctions: entity.activeAuctions,
    );
  }
}

/// Sales Data Point Mapper
class SalesDataPointMapper {
  /// Convert API SalesDataPoint to Entity
  static SalesDataPoint toEntity(Map<String, dynamic> json) {
    return SalesDataPoint(
      date: DateTime.parse(json['date'] as String),
      sales: (json['sales'] as num).toDouble(),
      orders: json['orders'] as int,
    );
  }

  /// Convert SalesDataPointApiModel to Entity
  static SalesDataPoint fromApiModel(SalesDataPointApiModel model) {
    return SalesDataPoint(
      date: DateTime.parse(model.date),
      sales: model.sales,
      orders: model.orders,
    );
  }

  /// Convert Entity to Map
  static Map<String, dynamic> toJson(SalesDataPoint entity) {
    return {
      'date': entity.date.toIso8601String(),
      'sales': entity.sales,
      'orders': entity.orders,
    };
  }
}

/// Analytics Mapper
class SellerAnalyticsMapper {
  /// Convert API model to Entity
  static SellerAnalytics toEntity({
    required String sellerId,
    required AnalyticsPeriod period,
    required Map<String, dynamic> currentPeriodData,
    required Map<String, dynamic> previousPeriodData,
    required List<Map<String, dynamic>> salesDataJson,
    required DateTime periodStart,
    required DateTime periodEnd,
  }) {
    return SellerAnalytics(
      sellerId: sellerId,
      period: period,
      currentPeriodSales:
          (currentPeriodData['sales'] as num?)?.toDouble() ?? 0.0,
      previousPeriodSales:
          (previousPeriodData['sales'] as num?)?.toDouble() ?? 0.0,
      currentPeriodOrders: currentPeriodData['orders'] as int? ?? 0,
      previousPeriodOrders: previousPeriodData['orders'] as int? ?? 0,
      salesDataPoints: salesDataJson
          .map((e) => SalesDataPointMapper.toEntity(e))
          .toList(),
      bestSellingDay: currentPeriodData['best_selling_day'] as int?,
      peakSellingHour: currentPeriodData['peak_selling_hour'] as int?,
      conversionRate:
          (currentPeriodData['conversion_rate'] as num?)?.toDouble() ?? 0.0,
      periodStart: periodStart,
      periodEnd: periodEnd,
      calculatedAt: DateTime.now(),
    );
  }
}

/// Performance Mapper
class SellerPerformanceMapper {
  /// Convert JSON to Entity
  static SellerPerformance toEntity({
    required String sellerId,
    required Map<String, dynamic> json,
  }) {
    return SellerPerformance(
      sellerId: sellerId,
      customerSatisfactionScore:
          (json['satisfactionScore'] as num?)?.toDouble() ?? 0.0,
      totalReviews: json['totalReviews'] as int? ?? 0,
      fiveStarReviews: json['fiveStarReviews'] as int? ?? 0,
      fourStarReviews: json['fourStarReviews'] as int? ?? 0,
      threeStarReviews: json['threeStarReviews'] as int? ?? 0,
      twoStarReviews: json['twoStarReviews'] as int? ?? 0,
      oneStarReviews: json['oneStarReviews'] as int? ?? 0,
      averageResponseTime: (json['avgResponseTime'] as num?)?.toDouble() ?? 0.0,
      orderFulfillmentRate:
          (json['fulfillmentRate'] as num?)?.toDouble() ?? 0.0,
      onTimeDeliveryRate:
          (json['onTimeDeliveryRate'] as num?)?.toDouble() ?? 0.0,
      totalCompletedOrders: json['completedOrders'] as int? ?? 0,
      totalFailedOrders: json['failedOrders'] as int? ?? 0,
      averageProcessingTime:
          (json['avgProcessingTime'] as num?)?.toDouble() ?? 0.0,
      returnRate: (json['returnRate'] as num?)?.toDouble() ?? 0.0,
      repeatCustomerRate:
          (json['repeatCustomerRate'] as num?)?.toDouble() ?? 0.0,
      calculatedAt: json['updatedAt'] != null
          ? DateTime.parse(json['updatedAt'] as String)
          : DateTime.now(),
    );
  }
}

/// Earnings Mapper
class SellerEarningsMapper {
  /// Convert DTO to Entity
  static SellerEarnings toEntity(EarningsDto dto) {
    return SellerEarnings(
      sellerId: dto.sellerId,
      totalRevenue: dto.totalRevenue,
      pendingRevenue: dto.pendingRevenue,
      totalPlatformFees: dto.totalPlatformFees,
      availableBalance: dto.availableBalance,
      withdrawalFeeAmount: dto.withdrawalFeeAmount,
      totalWithdrawn: dto.totalWithdrawn,
      totalWithdrawals: dto.totalWithdrawals,
      lastWithdrawalDate: dto.lastWithdrawalDate != null
          ? DateTime.parse(dto.lastWithdrawalDate!)
          : null,
      nextWithdrawalDate: dto.nextWithdrawalDate != null
          ? DateTime.parse(dto.nextWithdrawalDate!)
          : null,
      totalCompletedOrders: dto.totalCompletedOrders,
      platformFeePercentage: dto.platformFeePercentage,
      calculatedAt: DateTime.parse(dto.calculatedAt),
    );
  }

  /// Convert Entity to DTO
  static EarningsDto toDto(SellerEarnings entity) {
    return EarningsDto(
      sellerId: entity.sellerId,
      totalRevenue: entity.totalRevenue,
      pendingRevenue: entity.pendingRevenue,
      totalPlatformFees: entity.totalPlatformFees,
      availableBalance: entity.availableBalance,
      withdrawalFeeAmount: entity.withdrawalFeeAmount,
      totalWithdrawn: entity.totalWithdrawn,
      totalWithdrawals: entity.totalWithdrawals,
      lastWithdrawalDate: entity.lastWithdrawalDate?.toIso8601String(),
      nextWithdrawalDate: entity.nextWithdrawalDate?.toIso8601String(),
      totalCompletedOrders: entity.totalCompletedOrders,
      platformFeePercentage: entity.platformFeePercentage,
      calculatedAt: entity.calculatedAt.toIso8601String(),
    );
  }

  /// Convert API model to Entity
  /// Matches backend response from GET /api/v1/seller/earnings
  static SellerEarnings fromApiModel({
    required String sellerId,
    required SellerEarningsApiModel apiModel,
  }) {
    return SellerEarnings(
      sellerId: sellerId,
      totalRevenue: apiModel.totalEarned,
      pendingRevenue: apiModel.pendingBalance,
      totalPlatformFees: 0, // Not provided in simplified API response
      availableBalance: apiModel.availableBalance,
      withdrawalFeeAmount: apiModel.withdrawalFeeAmount,
      totalWithdrawn: apiModel.totalWithdrawn,
      totalWithdrawals: 0, // Not in API model
      lastWithdrawalDate: null, // Not in API model
      nextWithdrawalDate: null, // Not in API model
      totalCompletedOrders: 0, // Not in API model
      platformFeePercentage: 4.0, // Default platform fee
      calculatedAt: DateTime.now(),
      // Balance breakdown (J1-C)
      grossPayable: apiModel.grossPayable,
      activeDisputeFreeze: apiModel.activeDisputeFreeze,
    );
  }
}

/// Activity Item Mapper
class ActivityItemMapper {
  /// Parse activity type from string
  static ActivityType parseActivityType(String type) {
    switch (type.toLowerCase()) {
      case 'new_order':
        return ActivityType.newOrder;
      case 'order_paid':
        return ActivityType.orderPaid;
      case 'order_shipped':
        return ActivityType.orderShipped;
      case 'order_completed':
        return ActivityType.orderCompleted;
      case 'order_cancelled':
        return ActivityType.orderCancelled;
      case 'order_refunded':
        return ActivityType.orderRefunded;
      case 'order_disputed':
        return ActivityType.orderDisputed;
      case 'new_bid':
        return ActivityType.newBid;
      case 'auction_ended':
        return ActivityType.auctionEnded;
      case 'collection_sold':
        return ActivityType.collectionSold;
      case 'new_review':
        return ActivityType.newReview;
      default:
        return ActivityType.newOrder;
    }
  }

  /// Convert activity type to string
  static String activityTypeToString(ActivityType type) {
    return type.name;
  }

  /// Convert DTO to Entity
  static RecentActivityItem toEntity(ActivityItemDto dto) {
    return RecentActivityItem(
      id: dto.id,
      type: parseActivityType(dto.type),
      title: dto.title,
      subtitle: dto.subtitle,
      timestamp: DateTime.parse(dto.timestamp),
      imageUrl: dto.imageUrl,
      amount: dto.amount,
      targetId: dto.targetId,
    );
  }

  /// Convert Entity to DTO
  static ActivityItemDto toDto(RecentActivityItem entity) {
    return ActivityItemDto(
      id: entity.id,
      type: activityTypeToString(entity.type),
      title: entity.title,
      subtitle: entity.subtitle,
      timestamp: entity.timestamp.toIso8601String(),
      imageUrl: entity.imageUrl,
      amount: entity.amount,
      targetId: entity.targetId,
    );
  }
}
