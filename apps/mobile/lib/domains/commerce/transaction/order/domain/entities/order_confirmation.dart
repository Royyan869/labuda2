import 'package:equatable/equatable.dart';

/// Order Confirmation Entity - Represents the confirmation period for an order
///
/// Business Rules:
/// - 5-day confirmation period starting from shipment date
/// - Buyer can extend once by 3 days (max 8 days total)
/// - Payment auto-released to seller if buyer doesn't confirm within period
/// - Buyer can confirm anytime to immediately release payment to seller
///
/// UI Terms:
/// - "Menunggu Konfirmasi" = Active confirmation
/// - "Waktu Diperpanjang" = Extended confirmation
/// - "Batas Konfirmasi Berakhir" = Confirmation expired
///
/// This is NOT a product quality guarantee, but a buyer confirmation period
/// where payment is held in escrow until buyer confirms receipt.
class OrderConfirmation extends Equatable {
  /// Confirmation ID (same as orderId for 1-to-1 mapping)
  final String id;

  /// Associated order ID
  final String orderId;

  /// Buyer user ID
  final String buyerId;

  /// Seller user ID
  final String sellerId;

  /// When confirmation period started (from order.shippedAt)
  final DateTime startDate;

  /// Original end date (startDate + 5 days)
  final DateTime originalEndDate;

  /// Extended end date if buyer extends (originalEndDate + 3 days)
  final DateTime? extendedEndDate;

  /// Whether extension has been used (max 1 time)
  final bool extensionUsed;

  /// Current confirmation status
  final ConfirmationStatus status;

  /// When confirmation was created
  final DateTime createdAt;

  /// When confirmation was completed (if completed)
  final DateTime? completedAt;

  /// Reason for completion ('buyer_confirmed' or 'auto_released')
  final String? completionReason;

  /// Whether day-5 notification has been sent to buyer
  final bool day5NotificationSent;

  const OrderConfirmation({
    required this.id,
    required this.orderId,
    required this.buyerId,
    required this.sellerId,
    required this.startDate,
    required this.originalEndDate,
    this.extendedEndDate,
    required this.extensionUsed,
    required this.status,
    required this.createdAt,
    this.completedAt,
    this.completionReason,
    required this.day5NotificationSent,
  });

  // ========== Computed Properties ==========

  /// Get the active end date (considers extension)
  DateTime get activeEndDate => extendedEndDate ?? originalEndDate;

  /// Check if confirmation period has expired
  bool get isExpired => DateTime.now().isAfter(activeEndDate);

  /// Check if buyer can extend the confirmation
  /// Requirements: Not used, status is active, not expired
  bool get canExtend =>
      !extensionUsed && status == ConfirmationStatus.active && !isExpired;

  /// Get remaining days (rounded down)
  int get daysRemaining {
    final remaining = activeEndDate.difference(DateTime.now());
    return remaining.isNegative ? 0 : remaining.inDays;
  }

  /// Get remaining hours (for countdown display)
  int get hoursRemaining {
    final remaining = activeEndDate.difference(DateTime.now());
    return remaining.isNegative ? 0 : remaining.inHours;
  }

  /// Get remaining duration (for precise countdown)
  Duration get remainingDuration {
    final remaining = activeEndDate.difference(DateTime.now());
    return remaining.isNegative ? Duration.zero : remaining;
  }

  /// Check if extension button should be shown
  /// Show when: can extend AND already 3 days since shipped
  /// This means show from day 3 onwards, giving buyer time to anticipate
  bool shouldShowExtensionButton() {
    if (!canExtend) return false;

    // Show button from day 3 onwards (not last minute)
    // Calculate days since shipment
    final daysSinceShipped = DateTime.now().difference(startDate).inDays;

    // Show button on day 3 or later (but before extension is used)
    return daysSinceShipped >= 3 && !extensionUsed;
  }

  /// Check if confirmation is in urgent state (< 24 hours remaining)
  bool get isUrgent => hoursRemaining < 24 && hoursRemaining > 0;

  /// Check if should send day-5 notification
  /// Send when: 5 days since shipped, status active, notification not yet sent
  bool get shouldSendDay5Notification {
    if (day5NotificationSent || status != ConfirmationStatus.active) {
      return false;
    }

    final daysSinceShipped = DateTime.now().difference(startDate).inDays;
    return daysSinceShipped >= 5;
  }

  /// Check if confirmation is terminal (no more state changes expected)
  bool get isTerminal =>
      status == ConfirmationStatus.completed ||
      status == ConfirmationStatus.autoReleased ||
      status == ConfirmationStatus.cancelled;

  // ========== Methods ==========

  /// Create a copy with updated fields
  OrderConfirmation copyWith({
    String? id,
    String? orderId,
    String? buyerId,
    String? sellerId,
    DateTime? startDate,
    DateTime? originalEndDate,
    DateTime? extendedEndDate,
    bool? extensionUsed,
    ConfirmationStatus? status,
    DateTime? createdAt,
    DateTime? completedAt,
    String? completionReason,
    bool? day5NotificationSent,
  }) {
    return OrderConfirmation(
      id: id ?? this.id,
      orderId: orderId ?? this.orderId,
      buyerId: buyerId ?? this.buyerId,
      sellerId: sellerId ?? this.sellerId,
      startDate: startDate ?? this.startDate,
      originalEndDate: originalEndDate ?? this.originalEndDate,
      extendedEndDate: extendedEndDate ?? this.extendedEndDate,
      extensionUsed: extensionUsed ?? this.extensionUsed,
      status: status ?? this.status,
      createdAt: createdAt ?? this.createdAt,
      completedAt: completedAt ?? this.completedAt,
      completionReason: completionReason ?? this.completionReason,
      day5NotificationSent: day5NotificationSent ?? this.day5NotificationSent,
    );
  }

  @override
  List<Object?> get props => [
    id,
    orderId,
    buyerId,
    sellerId,
    startDate,
    originalEndDate,
    extendedEndDate,
    extensionUsed,
    status,
    createdAt,
    completedAt,
    completionReason,
    day5NotificationSent,
  ];
}

/// Confirmation status enumeration
enum ConfirmationStatus {
  /// Confirmation period is active, waiting for confirmation or expiry
  active,

  /// Buyer confirmed delivery, payment released
  completed,

  /// Auto-released after confirmation period expired
  autoReleased,

  /// Order cancelled, confirmation void
  cancelled,
}

/// Extension for ConfirmationStatus display names
extension ConfirmationStatusExtension on ConfirmationStatus {
  /// Get Indonesian display name for UI
  String get displayName {
    switch (this) {
      case ConfirmationStatus.active:
        return 'Aktif';
      case ConfirmationStatus.completed:
        return 'Selesai';
      case ConfirmationStatus.autoReleased:
        return 'Auto-Released';
      case ConfirmationStatus.cancelled:
        return 'Dibatalkan';
    }
  }

  /// Get status from string name (for deserialization)
  static ConfirmationStatus fromString(String value) {
    switch (value) {
      case 'active':
        return ConfirmationStatus.active;
      case 'completed':
        return ConfirmationStatus.completed;
      case 'autoReleased':
      case 'auto_released':
        return ConfirmationStatus.autoReleased;
      case 'cancelled':
        return ConfirmationStatus.cancelled;
      default:
        throw ArgumentError('Invalid ConfirmationStatus: $value');
    }
  }

  /// Convert to storage string
  String toStorageString() {
    switch (this) {
      case ConfirmationStatus.active:
        return 'active';
      case ConfirmationStatus.completed:
        return 'completed';
      case ConfirmationStatus.autoReleased:
        return 'auto_released';
      case ConfirmationStatus.cancelled:
        return 'cancelled';
    }
  }
}
