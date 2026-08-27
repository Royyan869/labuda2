/// Payment Result Entity
///
/// Pure Dart entity for payment processing result.
library;

import 'package:equatable/equatable.dart';

/// Payment result status
enum PaymentResultStatus { success, failed, pending, cancelled }

/// Payment result entity
class PaymentResult extends Equatable {
  /// Result status
  final PaymentResultStatus status;

  /// Payment ID
  final String? paymentId;

  /// Transaction ID from payment gateway
  final String? transactionId;

  /// Payment number
  final String? paymentNumber;

  /// Error message (if failed)
  final String? errorMessage;

  /// Error code (if failed)
  final String? errorCode;

  /// Additional data
  final Map<String, dynamic>? extraData;

  const PaymentResult({
    required this.status,
    this.paymentId,
    this.transactionId,
    this.paymentNumber,
    this.errorMessage,
    this.errorCode,
    this.extraData,
  });

  /// Create successful result
  factory PaymentResult.success({
    String? paymentId,
    String? transactionId,
    String? paymentNumber,
    Map<String, dynamic>? extraData,
  }) {
    return PaymentResult(
      status: PaymentResultStatus.success,
      paymentId: paymentId,
      transactionId: transactionId,
      paymentNumber: paymentNumber,
      extraData: extraData,
    );
  }

  /// Create failed result
  factory PaymentResult.failure({
    required String errorMessage,
    String? errorCode,
    Map<String, dynamic>? extraData,
  }) {
    return PaymentResult(
      status: PaymentResultStatus.failed,
      errorMessage: errorMessage,
      errorCode: errorCode,
      extraData: extraData,
    );
  }

  /// Create pending result
  factory PaymentResult.pending({
    String? paymentId,
    String? paymentNumber,
    Map<String, dynamic>? extraData,
  }) {
    return PaymentResult(
      status: PaymentResultStatus.pending,
      paymentId: paymentId,
      paymentNumber: paymentNumber,
      extraData: extraData,
    );
  }

  /// Create cancelled result
  factory PaymentResult.cancelled({
    String? paymentId,
    Map<String, dynamic>? extraData,
  }) {
    return PaymentResult(
      status: PaymentResultStatus.cancelled,
      paymentId: paymentId,
      extraData: extraData,
    );
  }

  /// Check if result is successful
  bool get isSuccess => status == PaymentResultStatus.success;

  /// Check if result is failed
  bool get isFailed => status == PaymentResultStatus.failed;

  /// Check if result is pending
  bool get isPending => status == PaymentResultStatus.pending;

  /// Check if result is cancelled
  bool get isCancelled => status == PaymentResultStatus.cancelled;

  @override
  List<Object?> get props => [
    status,
    paymentId,
    transactionId,
    paymentNumber,
    errorMessage,
    errorCode,
  ];
}
