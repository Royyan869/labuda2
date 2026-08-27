import '../../domain/domain.dart';
import '../dto/dto_barrel.dart';

/// Refund Mapper - converts between DTOs and Domain Entities
///
/// BACKEND AUTHORITY: Refund Fields
/// ========================================
/// All refund calculation fields come from backend.
/// - sellerApprovedPercent, sellerApprovedAmount
/// - adminApprovedPercent, adminApprovedAmount
/// - finalRefundAmount
///
/// RefundDto parses these fields from backend API response.
/// Client only uses what backend sends - NO derivation or calculation.
class RefundMapper {
  /// Convert RefundDto to RefundRequest domain entity
  ///
  /// Uses RefundDto.toEntity() which handles all field mappings
  /// including amount conversions (int64 -> double).
  static RefundRequest toRefundRequest(RefundDto dto) {
    return dto.toEntity();
  }

  /// Convert list of RefundDto to RefundRequest list
  static List<RefundRequest> toRefundList(List<RefundDto> dtos) {
    return dtos.map(toRefundRequest).toList();
  }

  /// Convert CreateRefundParams to CreateRefundDto for API request
  static CreateRefundDto toCreateRefundDto({
    required String orderId,
    required String reason,
    required String description,
    List<String>? evidence,
  }) {
    return CreateRefundDto(
      orderId: orderId,
      reason: reason,
      description: description,
      evidence: evidence,
    );
  }

  /// Convert refund percentage to RefundActionDto
  ///
  /// Format: "50%" for partial refund, "reject" for rejection
  static RefundActionDto toRefundActionDto(String response) {
    return RefundActionDto(response: response);
  }
}
