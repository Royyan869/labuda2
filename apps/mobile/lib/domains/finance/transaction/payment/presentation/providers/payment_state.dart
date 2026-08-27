/// Payment State
///
/// States for payment operations using Riverpod.
library;

import 'package:freezed_annotation/freezed_annotation.dart';
import '../../domain/entities/payment.dart';
import '../../domain/entities/payment_intent.dart';
import '../../domain/entities/payment_result.dart';

part 'payment_state.freezed.dart';

/// Base payment state
@freezed
class PaymentState with _$PaymentState {
  const factory PaymentState.initial() = PaymentInitial;

  const factory PaymentState.loading() = PaymentLoading;

  const factory PaymentState.paymentCreated(PaymentIntent intent) =
      PaymentCreated;

  const factory PaymentState.paymentLoaded(Payment payment) = PaymentLoaded;

  const factory PaymentState.paymentsLoaded(List<Payment> payments) =
      PaymentsLoaded;

  const factory PaymentState.paymentSuccess(PaymentResult result) =
      PaymentSuccess;

  const factory PaymentState.error(String message) = PaymentError;
}
