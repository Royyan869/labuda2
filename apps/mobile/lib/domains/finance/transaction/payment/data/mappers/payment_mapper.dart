/// Payment Mapper
///
/// Handles conversion between DTOs and entities.
///
/// NOTE: the legacy hardcoded channel→PaymentMethod mapping (core.PaymentChannel,
/// getAllPaymentMethods) was PURGED. The canonical payment-method authority is
/// the backend (GET /payments/methods → PaymentMethodOption); the client never
/// maintains its own method list.
library;

import '../dto/payment_dto.dart';
import '../../domain/entities/payment.dart';
import '../../domain/entities/payment_intent.dart';

/// Payment mapper class
class PaymentMapper {
  /// Convert PaymentDto to Payment entity
  static Payment toPaymentEntity(PaymentDto dto) {
    return dto.toEntity();
  }

  /// Convert list of PaymentDto to list of Payment entities
  static List<Payment> toPaymentEntityList(List<PaymentDto> dtos) {
    return dtos.map((dto) => dto.toEntity()).toList();
  }

  /// Convert PaymentIntentDto to PaymentIntent entity
  static PaymentIntent toPaymentIntentEntity(PaymentIntentDto dto) {
    return dto.toEntity();
  }

  /// Convert CreatePaymentRequest to DTO
  static CreatePaymentRequestDto toCreatePaymentDto(
    CreatePaymentRequest request,
  ) {
    return CreatePaymentRequestDto.fromRequest(request);
  }
}
