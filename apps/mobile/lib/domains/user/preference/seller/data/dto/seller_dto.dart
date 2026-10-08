/// Seller DTOs
///
/// Data Transfer Objects for API/Firestore serialization.
library;

import 'package:equatable/equatable.dart';

// Re-export existing API models

// Export DTOs used in this module
export 'seller_dto.dart';

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

/// One selectable payment method for a seller subscription payment, as returned
/// by GET /seller/subscription/payment-methods.
///
/// PMF-02: every payment flow carries a payment-method fee, and the backend is
/// the sole fee authority. [serviceFeeAmount] (F) and [grossAmount] (A + F) are
/// computed server-side from the active subscription principal A — the client
/// must never recompute, adjust, or submit either value.
class SellerSubscriptionPaymentMethodDto extends Equatable {
  /// Canonical payment_method_code sent back to the initiation endpoint.
  final String methodCode;

  /// Human-readable method name for the picker.
  final String displayName;

  /// Payment-method fee F the backend will snapshot for this method.
  final int serviceFeeAmount;

  /// Total the gateway will charge for this method: A + F.
  final int grossAmount;

  const SellerSubscriptionPaymentMethodDto({
    required this.methodCode,
    required this.displayName,
    required this.serviceFeeAmount,
    required this.grossAmount,
  });

  factory SellerSubscriptionPaymentMethodDto.fromJson(
    Map<String, dynamic> json,
  ) {
    return SellerSubscriptionPaymentMethodDto(
      methodCode: json['method_code'] as String,
      displayName: json['display_name'] as String,
      serviceFeeAmount: (json['service_fee_amount'] as num).toInt(),
      grossAmount: (json['gross_amount'] as num).toInt(),
    );
  }

  @override
  List<Object?> get props => [
    methodCode,
    displayName,
    serviceFeeAmount,
    grossAmount,
  ];
}

/// Response wrapper for GET /seller/subscription/payment-methods.
///
/// [principalAmount] is the active subscription principal A that the backend
/// will snapshot into the payment; each method's gross is A + its fee.
class SellerSubscriptionPaymentMethodsDto extends Equatable {
  final int principalAmount;
  final String currency;
  final List<SellerSubscriptionPaymentMethodDto> methods;

  const SellerSubscriptionPaymentMethodsDto({
    required this.principalAmount,
    required this.currency,
    required this.methods,
  });

  factory SellerSubscriptionPaymentMethodsDto.fromJson(
    Map<String, dynamic> json,
  ) {
    return SellerSubscriptionPaymentMethodsDto(
      principalAmount: (json['principal_amount'] as num?)?.toInt() ?? 0,
      currency: json['currency'] as String? ?? 'IDR',
      methods: (json['methods'] as List<dynamic>? ?? const [])
          .map(
            (e) => SellerSubscriptionPaymentMethodDto.fromJson(
              e as Map<String, dynamic>,
            ),
          )
          .toList(),
    );
  }

  @override
  List<Object?> get props => [principalAmount, currency, methods];
}
