/// Seller Mappers
///
/// Converts between Domain Entities and DTOs.
library;

import 'package:labuda/domains/user/preference/seller/data/models/api/seller_api_models.dart';

import '../../domain/entities/seller_earnings.dart';
import '../../domain/entities/seller_performance.dart';
import '../dto/seller_dto.dart';

/// Performance Mapper
///
/// Decodes the canonical backend wire for GET /seller/performance. The backend
/// is the sole aggregation authority (Reputation + Rating); this mapper only
/// decodes the returned numbers.
class SellerPerformanceMapper {
  /// Convert JSON to Entity
  static SellerPerformance toEntity({
    required String sellerId,
    required Map<String, dynamic> json,
  }) {
    return SellerPerformance(
      sellerId: sellerId,
      tier: json['tier'] as String? ?? 'basic',
      fulfillmentRate: (json['fulfillment_rate'] as num?)?.toDouble() ?? 0.0,
      completedOrders: (json['completed_orders'] as num?)?.toInt() ?? 0,
      cancelledTimeout: (json['cancelled_timeout'] as num?)?.toInt() ?? 0,
      averageRating: (json['average_rating'] as num?)?.toDouble() ?? 0.0,
      reviewCount: (json['review_count'] as num?)?.toInt() ?? 0,
      oneStarCount: (json['one_star_count'] as num?)?.toInt() ?? 0,
      twoStarCount: (json['two_star_count'] as num?)?.toInt() ?? 0,
      threeStarCount: (json['three_star_count'] as num?)?.toInt() ?? 0,
      fourStarCount: (json['four_star_count'] as num?)?.toInt() ?? 0,
      fiveStarCount: (json['five_star_count'] as num?)?.toInt() ?? 0,
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
    );
  }
}
