/// Payment Failures
///
/// Pure Dart failure types for error handling.
library;

import 'package:equatable/equatable.dart';

/// Base payment failure
abstract class PaymentFailure extends Equatable {
  final String message;

  const PaymentFailure(this.message);

  @override
  List<Object?> get props => [message];
}

/// Network failure
class NetworkFailure extends PaymentFailure {
  const NetworkFailure(super.message);

  @override
  String toString() => 'Network Error: $message';
}

/// Validation failure
class ValidationFailure extends PaymentFailure {
  final String? field;

  const ValidationFailure(super.message, {this.field});

  @override
  List<Object?> get props => [message, field];

  @override
  String toString() => field != null
      ? 'Validation Error ($field): $message'
      : 'Validation Error: $message';
}

/// Payment gateway failure
class PaymentGatewayFailure extends PaymentFailure {
  final String? errorCode;

  const PaymentGatewayFailure(super.message, {this.errorCode});

  @override
  List<Object?> get props => [message, errorCode];

  @override
  String toString() => errorCode != null
      ? 'Payment Gateway Error ($errorCode): $message'
      : 'Payment Gateway Error: $message';
}

/// Insufficient balance failure
class InsufficientBalanceFailure extends PaymentFailure {
  final int available;
  final int required;

  const InsufficientBalanceFailure({
    required this.available,
    required this.required,
  }) : super(
         'Insufficient balance. Required: $required, Available: $available',
       );

  @override
  List<Object?> get props => [message, available, required];

  @override
  String toString() => message;
}

/// Payment expired failure
class PaymentExpiredFailure extends PaymentFailure {
  final DateTime expiredAt;

  const PaymentExpiredFailure(this.expiredAt)
    : super('Payment expired at $expiredAt');

  @override
  List<Object?> get props => [message, expiredAt];

  @override
  String toString() => message;
}

/// Payment not found failure
class PaymentNotFoundFailure extends PaymentFailure {
  final String paymentId;

  const PaymentNotFoundFailure(this.paymentId)
    : super('Payment not found: $paymentId');

  @override
  List<Object?> get props => [message, paymentId];

  @override
  String toString() => message;
}

/// Unknown failure
class UnknownFailure extends PaymentFailure {
  final dynamic originalError;

  const UnknownFailure(super.message, {this.originalError});

  @override
  List<Object?> get props => [message, originalError];

  @override
  String toString() => 'Unknown Error: $message';
}
