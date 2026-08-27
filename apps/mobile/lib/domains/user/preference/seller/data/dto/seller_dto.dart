/// Seller DTOs
///
/// Data Transfer Objects for API/Firestore serialization.
library;

import 'package:equatable/equatable.dart';

// Re-export existing API models

// Export DTOs used in this module
export 'seller_dto.dart';

/// Dashboard Stats DTO
class DashboardStatsDto extends Equatable {
  final int totalOrders;
  final int pendingOrders;
  final int processingOrders;
  final int completedOrders;
  final int cancelledOrders;
  final int refundedOrders;
  final int problematicOrders;
  final double totalRevenue;
  final double pendingRevenue;
  final double refundedRevenue;
  final int totalListings;
  final int activeListings;
  final int soldListings;
  final int totalAuctions;
  final int activeAuctions;

  const DashboardStatsDto({
    required this.totalOrders,
    required this.pendingOrders,
    required this.processingOrders,
    required this.completedOrders,
    required this.cancelledOrders,
    required this.refundedOrders,
    required this.problematicOrders,
    required this.totalRevenue,
    required this.pendingRevenue,
    required this.refundedRevenue,
    required this.totalListings,
    required this.activeListings,
    required this.soldListings,
    required this.totalAuctions,
    required this.activeAuctions,
  });

  factory DashboardStatsDto.fromJson(Map<String, dynamic> json) {
    return DashboardStatsDto(
      totalOrders: json['total_orders'] as int? ?? 0,
      pendingOrders: json['pending_orders'] as int? ?? 0,
      processingOrders: json['processing_orders'] as int? ?? 0,
      completedOrders: json['completed_orders'] as int? ?? 0,
      cancelledOrders: json['cancelled_orders'] as int? ?? 0,
      refundedOrders: json['refunded_orders'] as int? ?? 0,
      problematicOrders: json['problematic_orders'] as int? ?? 0,
      totalRevenue: (json['total_revenue'] as num?)?.toDouble() ?? 0.0,
      pendingRevenue: (json['pending_revenue'] as num?)?.toDouble() ?? 0.0,
      refundedRevenue: (json['refunded_revenue'] as num?)?.toDouble() ?? 0.0,
      totalListings: json['total_listings'] as int? ?? 0,
      activeListings: json['active_listings'] as int? ?? 0,
      soldListings: json['sold_listings'] as int? ?? 0,
      totalAuctions: json['total_auctions'] as int? ?? 0,
      activeAuctions: json['active_auctions'] as int? ?? 0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'total_orders': totalOrders,
      'pending_orders': pendingOrders,
      'processing_orders': processingOrders,
      'completed_orders': completedOrders,
      'cancelled_orders': cancelledOrders,
      'refunded_orders': refundedOrders,
      'problematic_orders': problematicOrders,
      'total_revenue': totalRevenue,
      'pending_revenue': pendingRevenue,
      'refunded_revenue': refundedRevenue,
      'total_listings': totalListings,
      'active_listings': activeListings,
      'sold_listings': soldListings,
      'total_auctions': totalAuctions,
      'active_auctions': activeAuctions,
    };
  }

  @override
  List<Object?> get props => [
    totalOrders,
    pendingOrders,
    processingOrders,
    completedOrders,
    cancelledOrders,
    refundedOrders,
    problematicOrders,
    totalRevenue,
    pendingRevenue,
    refundedRevenue,
    totalListings,
    activeListings,
    soldListings,
    totalAuctions,
    activeAuctions,
  ];
}

/// Activity Item DTO
class ActivityItemDto extends Equatable {
  final String id;
  final String type;
  final String title;
  final String subtitle;
  final String timestamp;
  final String? imageUrl;
  final double? amount;
  final String? targetId;

  const ActivityItemDto({
    required this.id,
    required this.type,
    required this.title,
    required this.subtitle,
    required this.timestamp,
    this.imageUrl,
    this.amount,
    this.targetId,
  });

  factory ActivityItemDto.fromJson(Map<String, dynamic> json) {
    return ActivityItemDto(
      id: json['id'] as String,
      type: json['type'] as String,
      title: json['title'] as String,
      subtitle: json['subtitle'] as String,
      timestamp: json['timestamp'] as String,
      imageUrl: json['image_url'] as String?,
      amount: (json['amount'] as num?)?.toDouble(),
      targetId: json['target_id'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'type': type,
      'title': title,
      'subtitle': subtitle,
      'timestamp': timestamp,
      if (imageUrl != null) 'image_url': imageUrl,
      if (amount != null) 'amount': amount,
      if (targetId != null) 'target_id': targetId,
    };
  }

  @override
  List<Object?> get props => [
    id,
    type,
    title,
    subtitle,
    timestamp,
    imageUrl,
    amount,
    targetId,
  ];
}

/// Earnings DTO
class EarningsDto extends Equatable {
  final String sellerId;
  final double totalRevenue;
  final double pendingRevenue;
  final double totalPlatformFees;
  final double availableBalance;
  final double totalWithdrawn;
  final double withdrawalFeeAmount;
  final int totalWithdrawals;
  final String? lastWithdrawalDate;
  final String? nextWithdrawalDate;
  final int totalCompletedOrders;
  final double platformFeePercentage;
  final String calculatedAt;

  const EarningsDto({
    required this.sellerId,
    required this.totalRevenue,
    required this.pendingRevenue,
    required this.totalPlatformFees,
    required this.availableBalance,
    required this.totalWithdrawn,
    required this.withdrawalFeeAmount,
    required this.totalWithdrawals,
    this.lastWithdrawalDate,
    this.nextWithdrawalDate,
    required this.totalCompletedOrders,
    required this.platformFeePercentage,
    required this.calculatedAt,
  });

  factory EarningsDto.fromJson(Map<String, dynamic> json) {
    return EarningsDto(
      sellerId: json['seller_id'] as String,
      totalRevenue: (json['total_revenue'] as num?)?.toDouble() ?? 0.0,
      pendingRevenue: (json['pending_revenue'] as num?)?.toDouble() ?? 0.0,
      totalPlatformFees:
          (json['total_platform_fees'] as num?)?.toDouble() ?? 0.0,
      availableBalance: (json['available_balance'] as num?)?.toDouble() ?? 0.0,
      totalWithdrawn: (json['total_withdrawn'] as num?)?.toDouble() ?? 0.0,
      withdrawalFeeAmount:
          (json['withdrawal_fee_amount'] as num?)?.toDouble() ?? 0.0,
      totalWithdrawals: json['total_payouts'] as int? ?? 0,
      lastWithdrawalDate: json['last_payout_date'] as String?,
      nextWithdrawalDate: json['next_payout_date'] as String?,
      totalCompletedOrders: json['total_completed_orders'] as int? ?? 0,
      platformFeePercentage:
          (json['platform_fee_percentage'] as num?)?.toDouble() ?? 4.0,
      calculatedAt: json['calculated_at'] as String,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'seller_id': sellerId,
      'total_revenue': totalRevenue,
      'pending_revenue': pendingRevenue,
      'total_platform_fees': totalPlatformFees,
      'available_balance': availableBalance,
      'total_withdrawn': totalWithdrawn,
      'withdrawal_fee_amount': withdrawalFeeAmount,
      'total_payouts': totalWithdrawals,
      if (lastWithdrawalDate != null) 'last_payout_date': lastWithdrawalDate,
      if (nextWithdrawalDate != null) 'next_payout_date': nextWithdrawalDate,
      'total_completed_orders': totalCompletedOrders,
      'platform_fee_percentage': platformFeePercentage,
      'calculated_at': calculatedAt,
    };
  }

  @override
  List<Object?> get props => [
    sellerId,
    totalRevenue,
    pendingRevenue,
    totalPlatformFees,
    availableBalance,
    totalWithdrawn,
    withdrawalFeeAmount,
    totalWithdrawals,
    lastWithdrawalDate,
    nextWithdrawalDate,
    totalCompletedOrders,
    platformFeePercentage,
    calculatedAt,
  ];
}
