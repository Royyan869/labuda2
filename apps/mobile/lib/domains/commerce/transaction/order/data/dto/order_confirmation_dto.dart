/// Data Transfer Object for OrderConfirmation
///
/// Handles serialization/deserialization from/to API (Go Backend)
class OrderConfirmationDto {
  final String id;
  final String orderId;
  final String buyerId;
  final String sellerId;
  final DateTime startDate;
  final DateTime originalEndDate;
  final DateTime? extendedEndDate;
  final bool extensionUsed;
  final String status;
  final DateTime createdAt;
  final DateTime? completedAt;
  final String? completionReason;
  final bool day5NotificationSent;

  const OrderConfirmationDto({
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

  /// Create new confirmation DTO
  factory OrderConfirmationDto.create({
    required String orderId,
    required String buyerId,
    required String sellerId,
    required DateTime shippedAt,
  }) {
    final now = DateTime.now();
    final startDate = shippedAt;
    final originalEndDate = startDate.add(const Duration(days: 5));

    return OrderConfirmationDto(
      id: orderId,
      orderId: orderId,
      buyerId: buyerId,
      sellerId: sellerId,
      startDate: startDate,
      originalEndDate: originalEndDate,
      extendedEndDate: null,
      extensionUsed: false,
      status: 'active',
      createdAt: now,
      completedAt: null,
      completionReason: null,
      day5NotificationSent: false,
    );
  }

  // ============================================================
  // API SERIALIZATION (Go Backend)
  // ============================================================

  /// Convert to JSON for API request
  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'order_id': orderId,
      'buyer_id': buyerId,
      'seller_id': sellerId,
      'start_date': startDate.toIso8601String(),
      'original_end_date': originalEndDate.toIso8601String(),
      'extended_end_date': extendedEndDate?.toIso8601String(),
      'extension_used': extensionUsed,
      'status': status,
      'created_at': createdAt.toIso8601String(),
      'completed_at': completedAt?.toIso8601String(),
      'completion_reason': completionReason,
      'day5_notification_sent': day5NotificationSent,
    };
  }

  /// Create DTO from API response (snake_case to camelCase)
  factory OrderConfirmationDto.fromJson(Map<String, dynamic> json) {
    DateTime parseDateTime(dynamic value) {
      if (value == null) {
        return DateTime.now();
      }
      if (value is DateTime) {
        return value;
      }
      return DateTime.parse(value.toString());
    }

    return OrderConfirmationDto(
      id: json['id'] as String,
      orderId: json['order_id'] as String? ?? json['orderId'] as String? ?? '',
      buyerId: json['buyer_id'] as String? ?? json['buyerId'] as String? ?? '',
      sellerId:
          json['seller_id'] as String? ?? json['sellerId'] as String? ?? '',
      startDate: parseDateTime(json['start_date'] ?? json['startDate']),
      originalEndDate: parseDateTime(
        json['original_end_date'] ?? json['originalEndDate'],
      ),
      extendedEndDate:
          json['extended_end_date'] != null || json['extendedEndDate'] != null
          ? parseDateTime(json['extended_end_date'] ?? json['extendedEndDate'])
          : null,
      extensionUsed:
          json['extension_used'] as bool? ??
          json['extensionUsed'] as bool? ??
          false,
      status: json['status'] as String,
      createdAt: parseDateTime(json['created_at'] ?? json['createdAt']),
      completedAt: json['completed_at'] != null || json['completedAt'] != null
          ? parseDateTime(json['completed_at'] ?? json['completedAt'])
          : null,
      completionReason:
          json['completion_reason'] as String? ??
          json['completionReason'] as String?,
      day5NotificationSent:
          json['day5_notification_sent'] as bool? ??
          json['day5NotificationSent'] as bool? ??
          false,
    );
  }
}
