/// Seller Subscription Entities
///
/// Pure Dart entities for seller subscription - no Firebase/Flutter dependencies.
library;

import 'package:equatable/equatable.dart';

/// Subscription status
enum SubscriptionStatus { inactive, active, expired }

extension SubscriptionStatusExtension on SubscriptionStatus {
  String get apiValue {
    switch (this) {
      case SubscriptionStatus.inactive:
        return 'inactive';
      case SubscriptionStatus.active:
        return 'active';
      case SubscriptionStatus.expired:
        return 'expired';
    }
  }

  static SubscriptionStatus parse(String? value) {
    switch (value?.toLowerCase()) {
      case 'active':
        return SubscriptionStatus.active;
      case 'expired':
        return SubscriptionStatus.expired;
      case 'inactive':
      default:
        return SubscriptionStatus.inactive;
    }
  }
}

/// Seller Subscription Entity
class SellerSubscription extends Equatable {
  final bool isActive;
  final double yearlyFee;
  final DateTime startDate;
  final DateTime expiryDate;
  final SubscriptionStatus status;
  final String paymentId;
  final DateTime createdAt;
  final DateTime? lastRenewalDate;

  const SellerSubscription({
    required this.isActive,
    required this.yearlyFee,
    required this.startDate,
    required this.expiryDate,
    required this.status,
    required this.paymentId,
    required this.createdAt,
    this.lastRenewalDate,
  });

  /// Check if subscription is expired
  bool get isExpired => DateTime.now().isAfter(expiryDate);

  /// Get days until expiry
  int get daysUntilExpiry {
    final difference = expiryDate.difference(DateTime.now()).inDays;
    return difference > 0 ? difference : 0;
  }

  /// Check if subscription is expiring soon (within 30 days)
  bool get isExpiringSoon => daysUntilExpiry > 0 && daysUntilExpiry <= 30;

  /// Create empty subscription
  factory SellerSubscription.empty() {
    final now = DateTime.now();
    return SellerSubscription(
      isActive: false,
      yearlyFee: 0,
      startDate: now,
      expiryDate: now,
      status: SubscriptionStatus.expired,
      paymentId: '',
      createdAt: now,
    );
  }

  /// Copy with
  SellerSubscription copyWith({
    bool? isActive,
    double? yearlyFee,
    DateTime? startDate,
    DateTime? expiryDate,
    SubscriptionStatus? status,
    String? paymentId,
    DateTime? createdAt,
    DateTime? lastRenewalDate,
  }) {
    return SellerSubscription(
      isActive: isActive ?? this.isActive,
      yearlyFee: yearlyFee ?? this.yearlyFee,
      startDate: startDate ?? this.startDate,
      expiryDate: expiryDate ?? this.expiryDate,
      status: status ?? this.status,
      paymentId: paymentId ?? this.paymentId,
      createdAt: createdAt ?? this.createdAt,
      lastRenewalDate: lastRenewalDate ?? this.lastRenewalDate,
    );
  }

  @override
  List<Object?> get props => [
    isActive,
    yearlyFee,
    startDate,
    expiryDate,
    status,
    paymentId,
    createdAt,
    lastRenewalDate,
  ];
}
