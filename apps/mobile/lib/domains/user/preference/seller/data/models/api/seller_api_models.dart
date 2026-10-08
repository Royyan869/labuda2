/// Seller API Models
///
/// Canonical API models for the seller domain.
///
/// Seller analytics models were PURGED: there is no seller-analytics endpoint,
/// and Product View is owned by the canonical `product_view_events` authority,
/// not by this domain.
library;

/// Seller Earnings API Model
///
/// Matches backend response from GET /api/v1/seller/earnings
/// Backend returns: available_balance, pending_balance, total_withdrawn, total_earned
/// Plus balance breakdown: gross_payable, withdrawable_balance
class SellerEarningsApiModel {
  final double availableBalance; // Withdrawable balance
  final double
  pendingBalance; // Sum of escrow amounts for shipped/delivered orders
  final double totalWithdrawn; // Sum of all COMPLETED withdrawal amounts
  final double
  totalEarned; // Total credits ever received to SELLER_PAYABLE account
  final double withdrawalFeeAmount; // Fixed seller withdrawal fee

  // Balance breakdown (J1-C).
  final double? grossPayable; // Raw SELLER_PAYABLE ledger balance
  final double? withdrawableBalance; // == availableBalance

  const SellerEarningsApiModel({
    required this.availableBalance,
    required this.pendingBalance,
    required this.totalWithdrawn,
    required this.totalEarned,
    required this.withdrawalFeeAmount,
    this.grossPayable,
    this.withdrawableBalance,
  });

  factory SellerEarningsApiModel.fromJson(Map<String, dynamic> json) {
    return SellerEarningsApiModel(
      availableBalance: (json['available_balance'] as num?)?.toDouble() ?? 0.0,
      pendingBalance: (json['pending_balance'] as num?)?.toDouble() ?? 0.0,
      totalWithdrawn: (json['total_withdrawn'] as num?)?.toDouble() ?? 0.0,
      totalEarned: (json['total_earned'] as num?)?.toDouble() ?? 0.0,
      withdrawalFeeAmount:
          (json['withdrawal_fee_amount'] as num?)?.toDouble() ?? 0.0,
      grossPayable: (json['gross_payable'] as num?)?.toDouble(),
      withdrawableBalance: (json['withdrawable_balance'] as num?)?.toDouble(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'available_balance': availableBalance,
      'pending_balance': pendingBalance,
      'total_withdrawn': totalWithdrawn,
      'total_earned': totalEarned,
      'withdrawal_fee_amount': withdrawalFeeAmount,
      if (grossPayable != null) 'gross_payable': grossPayable,
      if (withdrawableBalance != null)
        'withdrawable_balance': withdrawableBalance,
    };
  }
}
