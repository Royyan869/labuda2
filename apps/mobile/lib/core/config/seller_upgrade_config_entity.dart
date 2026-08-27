import 'package:labuda/core/config/seller_upgrade_config.dart';

/// Entity representing seller upgrade configuration
///
/// FIRESTORE SUNSET (2025-02-20): Firestore methods removed.
/// Now uses JSON for Backend API communication.
class SellerUpgradeConfigEntity {
  final double yearlyFee;
  final int durationDays;
  final double?
  originalPrice; // Original price before discount (for strikethrough display)
  final int? discountPercentage; // Discount percentage (e.g., 50 for 50% off)
  final bool isEnabled;
  final int renewalReminderDays;
  final String? updatedBy;
  final DateTime? updatedAt;

  const SellerUpgradeConfigEntity({
    required this.yearlyFee,
    required this.durationDays,
    this.originalPrice,
    this.discountPercentage,
    required this.isEnabled,
    required this.renewalReminderDays,
    this.updatedBy,
    this.updatedAt,
  });

  /// Check if there's a discount (has original price higher than yearly fee)
  bool get hasDiscount => originalPrice != null && originalPrice! > yearlyFee;

  /// Calculate discount amount
  double get discountAmount => hasDiscount ? originalPrice! - yearlyFee : 0;

  /// Create default configuration
  factory SellerUpgradeConfigEntity.defaultConfig() {
    return const SellerUpgradeConfigEntity(
      yearlyFee: SellerUpgradeConfig.defaultYearlyFeeRupiah,
      durationDays: SellerUpgradeConfig.subscriptionDurationDays,
      isEnabled: SellerUpgradeConfig.defaultIsEnabled,
      renewalReminderDays: SellerUpgradeConfig.defaultRenewalReminderDays,
    );
  }

  /// Create from JSON (Backend API)
  factory SellerUpgradeConfigEntity.fromJson(Map<String, dynamic> json) {
    final rawYearlyFee = json['yearly_fee_rupiah'] as num?;
    final rawDurationDays =
        (json['durationDays'] as num?) ??
        (json['duration_days'] as num?) ??
        (json['subscription_duration_days'] as num?);
    final rawOriginalPrice =
        (json['originalPrice'] as num?) ?? (json['original_price'] as num?);
    final rawDiscountPercentage =
        (json['discountPercentage'] as num?) ??
        (json['discount_percentage'] as num?);

    return SellerUpgradeConfigEntity(
      yearlyFee: rawYearlyFee != null
          ? rawYearlyFee.toDouble()
          : SellerUpgradeConfig.defaultYearlyFeeRupiah,
      durationDays:
          rawDurationDays?.toInt() ??
          SellerUpgradeConfig.subscriptionDurationDays,
      originalPrice: rawOriginalPrice?.toDouble(),
      discountPercentage: rawDiscountPercentage?.toInt(),
      isEnabled:
          json['enabled'] as bool? ?? SellerUpgradeConfig.defaultIsEnabled,
      renewalReminderDays:
          (json['renewal_reminder_days'] as int?) ??
          SellerUpgradeConfig.defaultRenewalReminderDays,
      updatedBy: json['updatedBy'] as String? ?? json['updated_by'] as String?,
      updatedAt: json['updatedAt'] != null
          ? DateTime.parse(json['updatedAt'] as String)
          : json['updated_at'] != null
          ? DateTime.parse(json['updated_at'] as String)
          : null,
    );
  }

  /// Convert to JSON (Backend API)
  Map<String, dynamic> toJson() {
    return {
      'yearly_fee_rupiah': yearlyFee.round(),
      'duration_days': durationDays,
      if (originalPrice != null) 'original_price': originalPrice!.round(),
      if (discountPercentage != null) 'discount_percentage': discountPercentage,
      'enabled': isEnabled,
      'renewal_reminder_days': renewalReminderDays,
      if (updatedBy != null) 'updated_by': updatedBy,
      if (updatedAt != null) 'updated_at': updatedAt!.toIso8601String(),
    };
  }

  /// Create copy with updated fields
  SellerUpgradeConfigEntity copyWith({
    double? yearlyFee,
    int? durationDays,
    bool? isEnabled,
    int? renewalReminderDays,
    String? updatedBy,
    DateTime? updatedAt,
  }) {
    return SellerUpgradeConfigEntity(
      yearlyFee: yearlyFee ?? this.yearlyFee,
      durationDays: durationDays ?? this.durationDays,
      isEnabled: isEnabled ?? this.isEnabled,
      renewalReminderDays: renewalReminderDays ?? this.renewalReminderDays,
      updatedBy: updatedBy ?? this.updatedBy,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }

  @override
  String toString() {
    return 'SellerUpgradeConfigEntity(yearlyFee: $yearlyFee, durationDays: $durationDays, isEnabled: $isEnabled, renewalReminderDays: $renewalReminderDays)';
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;

    return other is SellerUpgradeConfigEntity &&
        other.yearlyFee == yearlyFee &&
        other.durationDays == durationDays &&
        other.isEnabled == isEnabled &&
        other.renewalReminderDays == renewalReminderDays &&
        other.updatedBy == updatedBy &&
        other.updatedAt == updatedAt;
  }

  @override
  int get hashCode {
    return Object.hash(
      yearlyFee,
      durationDays,
      isEnabled,
      renewalReminderDays,
      updatedBy,
      updatedAt,
    );
  }
}
