/// Payment Mapper
///
/// Handles conversion between DTOs and entities.
library;

import 'package:labuda/core/core.dart' as core;
import '../dto/payment_dto.dart';
import '../../domain/entities/payment.dart';
import '../../domain/entities/payment_intent.dart';
import '../../domain/entities/payment_method.dart';

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

  /// Map core.PaymentChannel to PaymentMethod entity.
  ///
  /// Legacy metadata only. Fee calculation is no longer client-authoritative.
  static PaymentMethod toPaymentMethod(core.PaymentChannel channel) {
    return PaymentMethod(
      channel: channel,
      displayName: channel.displayName,
      category: _getCategory(channel),
      fee: const PaymentMethodFee.flat(0),
      isAvailable: _isAvailable(channel),
      isComingSoon: _isComingSoon(channel),
      iconAsset: _getIconAsset(channel),
      deepLinkScheme: _getDeepLinkScheme(channel),
    );
  }

  /// Map list of core.PaymentChannel to PaymentMethod entities
  static List<PaymentMethod> toPaymentMethodList(
    List<core.PaymentChannel> channels,
  ) {
    return channels.map((channel) => toPaymentMethod(channel)).toList();
  }

  /// Get category for payment channel
  static PaymentMethodCategory _getCategory(core.PaymentChannel channel) {
    switch (channel.type) {
      case core.PaymentMethodType.bankTransfer:
        return PaymentMethodCategory.bankTransfer;
      case core.PaymentMethodType.manualTransfer:
        return PaymentMethodCategory.convenienceStore;
      case core.PaymentMethodType.eWallet:
        return PaymentMethodCategory.eWallet;
      case core.PaymentMethodType.qris:
        return PaymentMethodCategory.qris;
      case core.PaymentMethodType.creditCard:
      case core.PaymentMethodType.debitCard:
        return PaymentMethodCategory.card;
      default:
        return PaymentMethodCategory.bankTransfer;
    }
  }

  /// Check if payment method is available
  static bool _isAvailable(core.PaymentChannel channel) {
    const supported = [
      core.PaymentChannel.bcaVa,
      core.PaymentChannel.bniVa,
      core.PaymentChannel.briVa,
      core.PaymentChannel.mandiriVa,
      core.PaymentChannel.permataVa,
      core.PaymentChannel.gopay,
      core.PaymentChannel.shopeepay,
      core.PaymentChannel.qris,
    ];

    return supported.contains(channel);
  }

  /// Check if payment method is coming soon
  static bool _isComingSoon(core.PaymentChannel channel) {
    const comingSoon = [
      core.PaymentChannel.dana,
      core.PaymentChannel.ovo,
      core.PaymentChannel.linkAja,
      core.PaymentChannel.cimbVa,
      core.PaymentChannel.bsiVa,
      core.PaymentChannel.danamonVa,
      core.PaymentChannel.maybankVa,
      core.PaymentChannel.btnVa,
      core.PaymentChannel.creditCard,
      core.PaymentChannel.debitCard,
      core.PaymentChannel.akulaku,
      core.PaymentChannel.kredivo,
      core.PaymentChannel.alfamart,
      core.PaymentChannel.indomaret,
    ];

    return comingSoon.contains(channel);
  }

  /// Get icon asset for payment method
  static String? _getIconAsset(core.PaymentChannel channel) {
    return null;
  }

  /// Get deep link scheme for payment method
  static String? _getDeepLinkScheme(core.PaymentChannel channel) {
    switch (channel) {
      case core.PaymentChannel.gopay:
        return 'gopay';
      case core.PaymentChannel.shopeepay:
        return 'shopeeid';
      default:
        return null;
    }
  }

  /// Get all available payment methods
  static List<PaymentMethod> getAllPaymentMethods() {
    return toPaymentMethodList(core.PaymentChannel.values);
  }

  /// Get only supported payment methods (not coming soon)
  static List<PaymentMethod> getSupportedPaymentMethods() {
    return core.PaymentChannel.values
        .where((channel) => !_isComingSoon(channel))
        .map((channel) => toPaymentMethod(channel))
        .toList();
  }
}
