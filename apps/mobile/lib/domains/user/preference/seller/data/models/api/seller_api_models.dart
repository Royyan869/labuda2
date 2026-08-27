/// Seller API Models
///
/// RECOVERY STEP 2E - Stub models for compiler compatibility
/// These are placeholder models to satisfy type checking during refactoring.
/// TODO: Migrate from old implementation or implement proper API models
library;

/// Seller Analytics API Model
///
/// Stub model for seller analytics data from API
class SellerAnalyticsApiModel {
  final double totalSales;
  final int totalOrders;
  final double averageOrderValue;
  final List<SalesDataPointApiModel> salesData;
  final DateTime calculatedAt;

  const SellerAnalyticsApiModel({
    required this.totalSales,
    required this.totalOrders,
    required this.averageOrderValue,
    required this.salesData,
    required this.calculatedAt,
  });

  factory SellerAnalyticsApiModel.fromJson(Map<String, dynamic> json) {
    return SellerAnalyticsApiModel(
      totalSales: (json['total_sales'] as num?)?.toDouble() ?? 0.0,
      totalOrders: json['total_orders'] as int? ?? 0,
      averageOrderValue:
          (json['average_order_value'] as num?)?.toDouble() ?? 0.0,
      salesData:
          (json['sales_data'] as List?)
              ?.map(
                (e) =>
                    SalesDataPointApiModel.fromJson(e as Map<String, dynamic>),
              )
              .toList() ??
          [],
      calculatedAt: json['calculated_at'] != null
          ? DateTime.parse(json['calculated_at'] as String)
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'total_sales': totalSales,
      'total_orders': totalOrders,
      'average_order_value': averageOrderValue,
      'sales_data': salesData.map((e) => e.toJson()).toList(),
      'calculated_at': calculatedAt.toIso8601String(),
    };
  }
}

/// Sales Data Point API Model
class SalesDataPointApiModel {
  final String date;
  final double sales;
  final int orders;

  const SalesDataPointApiModel({
    required this.date,
    required this.sales,
    required this.orders,
  });

  factory SalesDataPointApiModel.fromJson(Map<String, dynamic> json) {
    return SalesDataPointApiModel(
      date: json['date'] as String,
      sales: (json['sales'] as num?)?.toDouble() ?? 0.0,
      orders: json['orders'] as int? ?? 0,
    );
  }

  Map<String, dynamic> toJson() {
    return {'date': date, 'sales': sales, 'orders': orders};
  }
}

/// Seller Earnings API Model
///
/// Matches backend response from GET /api/v1/seller/earnings
/// Backend returns: available_balance, pending_balance, total_withdrawn, total_earned
/// Plus balance breakdown: gross_payable, active_dispute_freeze, withdrawable_balance
class SellerEarningsApiModel {
  final double availableBalance; // Withdrawable balance (freeze-aware)
  final double
  pendingBalance; // Sum of escrow amounts for shipped/delivered orders
  final double totalWithdrawn; // Sum of all COMPLETED withdrawal amounts
  final double
  totalEarned; // Total credits ever received to SELLER_PAYABLE account
  final double withdrawalFeeAmount; // Fixed seller withdrawal fee

  // Balance breakdown (J1-C): explains why availableBalance may be < grossPayable.
  final double? grossPayable; // Raw SELLER_PAYABLE ledger balance
  final double? activeDisputeFreeze; // Funds frozen by active disputes
  final double? withdrawableBalance; // gross - freeze (== availableBalance)

  const SellerEarningsApiModel({
    required this.availableBalance,
    required this.pendingBalance,
    required this.totalWithdrawn,
    required this.totalEarned,
    required this.withdrawalFeeAmount,
    this.grossPayable,
    this.activeDisputeFreeze,
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
      activeDisputeFreeze: (json['active_dispute_freeze'] as num?)?.toDouble(),
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
      if (activeDisputeFreeze != null)
        'active_dispute_freeze': activeDisputeFreeze,
      if (withdrawableBalance != null)
        'withdrawable_balance': withdrawableBalance,
    };
  }
}

/// Top Product Item API Model
///
/// Stub model for top product data from API
class TopProductItemApiModel {
  final String productId;
  final String productName;
  final String? imageUrl;
  final int totalSold;
  final double totalRevenue;

  const TopProductItemApiModel({
    required this.productId,
    required this.productName,
    this.imageUrl,
    required this.totalSold,
    required this.totalRevenue,
  });

  factory TopProductItemApiModel.fromJson(Map<String, dynamic> json) {
    return TopProductItemApiModel(
      productId: json['product_id'] as String,
      productName: json['product_name'] as String,
      imageUrl: json['image_url'] as String?,
      totalSold: json['total_sold'] as int? ?? 0,
      totalRevenue: (json['total_revenue'] as num?)?.toDouble() ?? 0.0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'product_id': productId,
      'product_name': productName,
      if (imageUrl != null) 'image_url': imageUrl,
      'total_sold': totalSold,
      'total_revenue': totalRevenue,
    };
  }
}
