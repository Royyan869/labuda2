/// Canonical Promotion Delivery Analytics DTO
///
/// Maps the backend response from GET /api/v1/promotions/contracts/:id/analytics
library;

import 'package:equatable/equatable.dart';

/// DTO for the canonical promotion delivery analytics response.
///
/// Preserves exact canonical vocabulary (contract_id authority):
/// - included_count: canonical promotion card was included in a returned feed response
/// - impression_count: client explicitly acknowledged the issued canonical exposure
/// - click_count: client explicitly acknowledged an explicit user tap on a delivered card
class CanonicalPromotionAnalyticsDto extends Equatable {
  /// The canonical promotion contract ID this analytics belongs to
  /// (promotion_contracts.id).
  final String contractId;

  /// Number of 'included' observations: the server placed the canonical card
  /// in a returned feed response.
  final int includedCount;

  /// Number of 'impression' observations: the client EXPLICITLY acknowledged
  /// a delivered card by echoing its issued exposure identity.
  final int impressionCount;

  /// Number of 'click' observations: the client EXPLICITLY acknowledged an
  /// explicit user tap on a delivered card by echoing its issued exposure
  /// identity.
  final int clickCount;

  const CanonicalPromotionAnalyticsDto({
    required this.contractId,
    required this.includedCount,
    required this.impressionCount,
    required this.clickCount,
  });

  /// Creates a DTO from JSON response.
  ///
  /// The backend response envelope wraps data in { "success": true, "data": {...} }
  /// This factory parses the inner data object.
  factory CanonicalPromotionAnalyticsDto.fromJson(Map<String, dynamic> json) {
    return CanonicalPromotionAnalyticsDto(
      contractId: json['contract_id'] as String,
      includedCount: json['included_count'] as int,
      impressionCount: json['impression_count'] as int,
      clickCount: json['click_count'] as int,
    );
  }

  /// Converts to JSON for potential caching or logging.
  Map<String, dynamic> toJson() {
    return {
      'contract_id': contractId,
      'included_count': includedCount,
      'impression_count': impressionCount,
      'click_count': clickCount,
    };
  }

  /// Empty analytics for zero state (no events yet).
  factory CanonicalPromotionAnalyticsDto.empty(String contractId) {
    return CanonicalPromotionAnalyticsDto(
      contractId: contractId,
      includedCount: 0,
      impressionCount: 0,
      clickCount: 0,
    );
  }

  /// Whether this analytics represents a zero state (no events).
  bool get isEmpty => includedCount == 0 && impressionCount == 0 && clickCount == 0;

  @override
  List<Object?> get props => [contractId, includedCount, impressionCount, clickCount];
}