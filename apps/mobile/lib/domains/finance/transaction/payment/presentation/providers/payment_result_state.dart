/// Payment Result State
///
/// States for payment result checking and reconciliation.
/// Backend is the SINGLE SOURCE OF TRUTH for payment status.
///
/// SAFETY:
/// - No client-side calculation of payment status
/// - Explicit states for all scenarios
/// - Error states are distinct and actionable
library;

import 'package:equatable/equatable.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/order.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/order_status.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment.dart';
import 'package:labuda/core/common/types/payment_types.dart';

/// Payment Result Screen Status
///
/// These are UI states derived from BACKEND-CONFIRMED order payment status.
/// Frontend MUST NOT infer or calculate payment status.
enum PaymentResultScreenStatus {
  /// Still checking with backend - user should wait
  checking,

  /// Backend confirmed payment successful
  success,

  /// Backend confirmed payment failed, expired, or refunded
  failed,

  /// Max polling attempts reached - status unknown from backend
  /// User can manually retry or check order detail
  timeout,

  /// Network error - unable to reach backend
  /// User can retry or check order detail
  networkError,
}

/// Payment Result State
///
/// Manages state for payment status reconciliation flow.
/// Uses backend-confirmed payment status from order entity.
class PaymentResultState extends Equatable {
  /// Current UI status
  final PaymentResultScreenStatus status;

  /// The order entity from backend - contains authoritative payment status
  final Order? order;

  /// The payment entity from backend - contains the reusable payment URL
  final Payment? payment;

  /// Number of polling attempts made
  final int pollAttempts;

  /// Maximum polling attempts before timeout
  final int maxPollAttempts;

  /// When polling started
  final DateTime? pollingStartedAt;

  /// Error message if status is networkError
  final String? errorMessage;

  /// Whether a status check is currently in flight
  /// IMMEDIATE LOCK - set synchronously to prevent overlap
  final bool isChecking;

  /// Whether polling has been cancelled by user or navigation
  final bool isCancelled;

  const PaymentResultState({
    this.status = PaymentResultScreenStatus.checking,
    this.order,
    this.payment,
    this.pollAttempts = 0,
    this.maxPollAttempts = 20,
    this.pollingStartedAt,
    this.errorMessage,
    this.isChecking = false,
    this.isCancelled = false,
  });

  /// Initial state - start checking
  factory PaymentResultState.initial() {
    return const PaymentResultState(
      status: PaymentResultScreenStatus.checking,
      pollingStartedAt: null,
    );
  }

  /// Loading state - actively checking backend
  factory PaymentResultState.checking({
    required int pollAttempts,
    DateTime? startedAt,
    int maxAttempts = 20,
    Payment? payment,
  }) {
    return PaymentResultState(
      status: PaymentResultScreenStatus.checking,
      payment: payment,
      pollAttempts: pollAttempts,
      pollingStartedAt: startedAt,
      maxPollAttempts: maxAttempts,
      isChecking: true,
    );
  }

  /// Success state - backend confirmed payment successful
  factory PaymentResultState.success({
    required Order order,
    required int pollAttempts,
    DateTime? startedAt,
    Payment? payment,
  }) {
    return PaymentResultState(
      status: PaymentResultScreenStatus.success,
      order: order,
      payment: payment,
      pollAttempts: pollAttempts,
      pollingStartedAt: startedAt,
      isChecking: false,
    );
  }

  /// Failed state - backend confirmed payment failed/expired/refunded
  factory PaymentResultState.failed({
    required Order order,
    required String reason,
    required int pollAttempts,
    DateTime? startedAt,
    Payment? payment,
  }) {
    return PaymentResultState(
      status: PaymentResultScreenStatus.failed,
      order: order,
      payment: payment,
      pollAttempts: pollAttempts,
      pollingStartedAt: startedAt,
      errorMessage: reason,
      isChecking: false,
    );
  }

  /// Timeout state - max attempts reached, status unknown
  factory PaymentResultState.timeout({
    Order? order,
    required int pollAttempts,
    DateTime? startedAt,
    int maxAttempts = 20,
    Payment? payment,
  }) {
    return PaymentResultState(
      status: PaymentResultScreenStatus.timeout,
      order: order,
      payment: payment,
      pollAttempts: pollAttempts,
      pollingStartedAt: startedAt,
      maxPollAttempts: maxAttempts,
      isChecking: false,
    );
  }

  /// Network error state - unable to reach backend
  factory PaymentResultState.networkError({
    Order? order,
    required String errorMessage,
    required int pollAttempts,
    DateTime? startedAt,
    Payment? payment,
  }) {
    return PaymentResultState(
      status: PaymentResultScreenStatus.networkError,
      order: order,
      payment: payment,
      errorMessage: errorMessage,
      pollAttempts: pollAttempts,
      pollingStartedAt: startedAt,
      isChecking: false,
    );
  }

  /// Cancelled state - polling was cancelled by user or navigation
  factory PaymentResultState.cancelled({
    Order? order,
    required int pollAttempts,
    Payment? payment,
  }) {
    return PaymentResultState(
      status: PaymentResultScreenStatus.checking,
      order: order,
      payment: payment,
      pollAttempts: pollAttempts,
      isCancelled: true,
      isChecking: false,
    );
  }

  /// Check if polling should continue
  bool get shouldContinuePolling =>
      !isCancelled &&
      status == PaymentResultScreenStatus.checking &&
      pollAttempts < maxPollAttempts;

  /// Check if this is a terminal state (no more polling needed)
  bool get isTerminalState =>
      status == PaymentResultScreenStatus.success ||
      status == PaymentResultScreenStatus.failed ||
      isCancelled;

  /// Get elapsed time since polling started
  Duration? get elapsed {
    if (pollingStartedAt == null) return null;
    return DateTime.now().difference(pollingStartedAt!);
  }

  /// Get payment status from backend-confirmed order
  ///
  /// Supporting context only. Do not use as authority for success/failure.
  PaymentStatus? get paymentStatus => order?.paymentStatus;

  /// The reusable payment URL returned by the payment resource, if any.
  String? get paymentUrl => payment?.paymentUrl;

  /// Payment resource ID, useful for debugging and recovery.
  String? get paymentId => payment?.id;

  /// Whether the backend returned a reusable payment URL.
  bool get hasReusablePaymentUrl {
    final url = paymentUrl;
    if (url == null || url.isEmpty) return false;

    final expiresAt = payment?.expiredAt;
    if (expiresAt == null) return true;

    return expiresAt.isAfter(DateTime.now());
  }

  /// Whether "Lanjutkan Pembayaran" (reopen existing payment) should be
  /// offered. Requires a reusable payment_url AND a payment resource still
  /// in a non-terminal state — reopening a URL for an already-resolved
  /// payment (paid/failed/expired/refunded) would be misleading.
  bool get canContinuePayment =>
      hasReusablePaymentUrl && (payment?.status.isOngoing ?? false);

  /// Check if the canonical order reached a success state
  bool get isPaymentSuccessful {
    if (status == PaymentResultScreenStatus.success) {
      return true;
    }
    switch (order?.status) {
      case OrderStatus.paid:
      case OrderStatus.shipped:
      case OrderStatus.delivered:
      case OrderStatus.completed:
        return true;
      case OrderStatus.pending:
      case OrderStatus.cancelled:
      case OrderStatus.cancelledTimeout:
      case OrderStatus.refunded:
      case OrderStatus.disputeOpen:
      case OrderStatus.partiallyRefunded:
      case OrderStatus.expired:
      case null:
        return false;
    }
  }

  /// Check if the canonical order reached a terminal failure state
  bool get isPaymentFailed {
    if (status == PaymentResultScreenStatus.failed) {
      return true;
    }
    switch (order?.status) {
      case OrderStatus.cancelled:
      case OrderStatus.cancelledTimeout:
      case OrderStatus.refunded:
      case OrderStatus.disputeOpen:
      case OrderStatus.partiallyRefunded:
      case OrderStatus.expired:
        return true;
      case OrderStatus.pending:
      case OrderStatus.paid:
      case OrderStatus.shipped:
      case OrderStatus.delivered:
      case OrderStatus.completed:
      case null:
        return false;
    }
  }

  /// Check if payment result should keep polling
  bool get isPaymentPending =>
      status == PaymentResultScreenStatus.checking &&
      order?.status == OrderStatus.pending;

  PaymentResultState copyWith({
    PaymentResultScreenStatus? status,
    Order? order,
    Payment? payment,
    int? pollAttempts,
    int? maxPollAttempts,
    DateTime? pollingStartedAt,
    String? errorMessage,
    bool? isChecking,
    bool? isCancelled,
    bool clearOrder = false,
    bool clearPayment = false,
  }) {
    return PaymentResultState(
      status: status ?? this.status,
      order: clearOrder ? null : (order ?? this.order),
      payment: clearPayment ? null : (payment ?? this.payment),
      pollAttempts: pollAttempts ?? this.pollAttempts,
      maxPollAttempts: maxPollAttempts ?? this.maxPollAttempts,
      pollingStartedAt: pollingStartedAt ?? this.pollingStartedAt,
      errorMessage: errorMessage ?? this.errorMessage,
      isChecking: isChecking ?? this.isChecking,
      isCancelled: isCancelled ?? this.isCancelled,
    );
  }

  @override
  List<Object?> get props => [
    status,
    order,
    payment,
    pollAttempts,
    maxPollAttempts,
    pollingStartedAt,
    errorMessage,
    isChecking,
    isCancelled,
  ];
}
