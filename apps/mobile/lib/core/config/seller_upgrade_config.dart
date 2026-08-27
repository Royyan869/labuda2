/// Seller Upgrade Fee Configuration
///
/// Contains the current seller activation defaults and helper methods used by
/// the mobile seller upgrade flow.
/// Runtime config is surfaced through [SellerUpgradeConfigService] and falls
/// back to these defaults when remote config is unavailable.
class SellerUpgradeConfig {
  // Default values used by the seller upgrade flow when remote config is unavailable.
  static const double defaultYearlyFeeRupiah = 70000; // Rp 70k/year
  static const int defaultRenewalReminderDays = 7; // 7 days before expiry
  static const int subscriptionDurationDays = 365; // 1 year
  static const bool defaultIsEnabled = true;

  /// Calculate expiry date from start date
  static DateTime calculateExpiryDate(DateTime startDate) {
    return startDate.add(const Duration(days: subscriptionDurationDays));
  }

  /// Check if subscription is active (before expiry date)
  static bool isSubscriptionActive(DateTime? expiryDate) {
    if (expiryDate == null) return false;
    return DateTime.now().isBefore(expiryDate);
  }

  /// Calculate days until expiry
  static int daysUntilExpiry(DateTime? expiryDate) {
    if (expiryDate == null) return 0;
    final difference = expiryDate.difference(DateTime.now()).inDays;
    return difference > 0 ? difference : 0;
  }

  /// Check if subscription is within the renewal reminder window.
  static bool isWithinRenewalReminderWindow(
    DateTime? expiryDate,
    int reminderDays,
  ) {
    if (expiryDate == null) return false;
    final now = DateTime.now();
    if (!now.isBefore(expiryDate)) return false;
    final reminderWindowStart = expiryDate.subtract(
      Duration(days: reminderDays),
    );
    return !now.isBefore(reminderWindowStart);
  }

  /// Check if subscription is expiring soon using the canonical reminder window.
  static bool isExpiringSoon(DateTime? expiryDate) {
    return isWithinRenewalReminderWindow(
      expiryDate,
      defaultRenewalReminderDays,
    );
  }

  /// Calculate the renewal reminder start date.
  static DateTime calculateRenewalReminderStart(
    DateTime expiryDate,
    int reminderDays,
  ) {
    return expiryDate.subtract(Duration(days: reminderDays));
  }
}
