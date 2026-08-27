/// Types of violations in LABUDA platform
///
/// **BACKEND AUTHORITY:** This enum MUST match the Go backend exactly.
/// Source: backend/internal/domain/user/domain/entity/user_enums.go
///
/// Each violation type has:
/// - Severity level (minor, moderate, severe, critical)
/// - Penalty points (1, 3, 5, 10 respectively)
/// - Potential consequences based on total active points
enum ViolationType {
  /// Spam or repetitive unwanted content
  /// Severity: minor (1 point)
  spam,

  /// Listing fake or misleading products
  /// Severity: critical (10 points)
  fakeProduct,

  /// Fraudulent behavior or scam attempts
  /// Severity: critical (10 points)
  scam,

  /// Harassing other users
  /// Severity: severe (5 points)
  harassment,

  /// Inappropriate content (NSFW, hate speech, etc.)
  /// Severity: moderate (3 points)
  inappropriateContent,

  /// Fraudulent payment methods
  /// Severity: critical (10 points)
  fraudulentPayment,

  /// Non-delivery of paid items
  /// Severity: critical (10 points)
  nonDelivery,

  /// Selling counterfeit goods
  /// Severity: critical (10 points)
  counterfeit,

  /// Manipulating prices or market manipulation
  /// Severity: moderate (3 points)
  priceManipulation,

  /// Other violations not categorized
  /// Severity varies (set by admin)
  other;

  /// Convert from API value (snake_case) to ViolationType enum
  static ViolationType fromApiValue(String value) {
    return ViolationType.values.firstWhere(
      (type) => type.apiValue == value,
      orElse: () => ViolationType.other,
    );
  }

  /// Get API value (snake_case) for backend communication
  String get apiValue {
    switch (this) {
      case ViolationType.spam:
        return 'spam';
      case ViolationType.fakeProduct:
        return 'fake_product';
      case ViolationType.scam:
        return 'scam';
      case ViolationType.harassment:
        return 'harassment';
      case ViolationType.inappropriateContent:
        return 'inappropriate_content';
      case ViolationType.fraudulentPayment:
        return 'fraudulent_payment';
      case ViolationType.nonDelivery:
        return 'non_delivery';
      case ViolationType.counterfeit:
        return 'counterfeit';
      case ViolationType.priceManipulation:
        return 'price_manipulation';
      case ViolationType.other:
        return 'other';
    }
  }

  /// Display name for UI
  String get displayName {
    switch (this) {
      case ViolationType.spam:
        return 'Spam';
      case ViolationType.fakeProduct:
        return 'Fake Product';
      case ViolationType.scam:
        return 'Scam';
      case ViolationType.harassment:
        return 'Harassment';
      case ViolationType.inappropriateContent:
        return 'Inappropriate Content';
      case ViolationType.fraudulentPayment:
        return 'Fraudulent Payment';
      case ViolationType.nonDelivery:
        return 'Non-Delivery';
      case ViolationType.counterfeit:
        return 'Counterfeit';
      case ViolationType.priceManipulation:
        return 'Price Manipulation';
      case ViolationType.other:
        return 'Other';
    }
  }

  /// Get severity level for this violation type
  ViolationSeverity get severity {
    switch (this) {
      case ViolationType.spam:
        return ViolationSeverity.minor;
      case ViolationType.inappropriateContent:
      case ViolationType.priceManipulation:
        return ViolationSeverity.moderate;
      case ViolationType.harassment:
        return ViolationSeverity.severe;
      case ViolationType.fakeProduct:
      case ViolationType.scam:
      case ViolationType.fraudulentPayment:
      case ViolationType.nonDelivery:
      case ViolationType.counterfeit:
        return ViolationSeverity.critical;
      case ViolationType.other:
        return ViolationSeverity.moderate; // Default for "other"
    }
  }

  /// Get penalty points for this violation type
  int get penaltyPoints {
    return severity.points;
  }
}

/// Violation severity levels
enum ViolationSeverity {
  /// Warning only (1 point)
  minor(1),

  /// Temporary restrictions (3 points)
  moderate(3),

  /// Account suspension (5 points)
  severe(5),

  /// Permanent ban (10 points)
  critical(10);

  const ViolationSeverity(this.points);

  /// Penalty points for this severity
  final int points;

  /// Display name for UI
  String get displayName {
    switch (this) {
      case ViolationSeverity.minor:
        return 'Minor';
      case ViolationSeverity.moderate:
        return 'Moderate';
      case ViolationSeverity.severe:
        return 'Severe';
      case ViolationSeverity.critical:
        return 'Critical';
    }
  }

  /// Check if this severity leads to account suspension
  bool get leadsToRestriction =>
      this == ViolationSeverity.severe || this == ViolationSeverity.critical;

  /// Check if this severity leads to ban
  bool get leadsToBan => this == ViolationSeverity.critical;
}

/// Penalty point thresholds (from backend)
///
/// - 3 points: Warning notification
/// - 5 points: Restrict certain actions
/// - 10 points: Suspend account temporarily
/// - 15 points: Permanent ban
class ViolationThresholds {
  /// Send warning notification
  static const int warning = 3;

  /// Restrict certain actions
  static const int restricted = 5;

  /// Suspend account temporarily
  static const int suspended = 10;

  /// Permanent ban
  static const int banned = 15;

  /// Get status description based on active penalty points
  static String getStatusDescription(int activePoints) {
    if (activePoints >= banned) return 'Banned';
    if (activePoints >= suspended) return 'Suspended';
    if (activePoints >= restricted) return 'Restricted';
    if (activePoints >= warning) return 'Warning';
    return 'Good Standing';
  }
}
