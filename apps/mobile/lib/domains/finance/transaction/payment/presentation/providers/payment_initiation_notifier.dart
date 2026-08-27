/// Payment Initiation Notifier
///
/// Handles payment initiation flow with safety mechanisms:
/// - Double-tap guard via immediate lock flag
/// - Idempotency key handling for retry scenarios
/// - Backend authority for payment state
/// - Clear error handling with user-friendly messages
library;

import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:uuid/uuid.dart';
import 'package:labuda/core/core.dart' as core;
import '../../domain/entities/payment.dart';
import '../../domain/entities/payment_intent.dart';
import '../../domain/failures/payment_failure.dart' as payment_failures;
import 'payment_initiation_state.dart';
import 'payment_providers.dart' show paymentRepositoryProvider;

part 'payment_initiation_notifier.g.dart';

// ============================================================================
// PAYMENT INITIATION REQUEST
// ============================================================================

/// Request for initiating payment from order
///
/// PASS_18V: backend is sole authority for the buyer payment fee and gross
/// amount, both derived from [paymentMethodCode]. The client never computes
/// or sends an amount — see PaymentRepository.getAvailablePaymentMethods for
/// the method list + fee the buyer picks from before calling this.
class InitiatePaymentRequest {
  /// Order ID to create payment for
  final String orderId;

  /// Canonical payment method code the buyer selected (required).
  final String paymentMethodCode;

  /// Coin discount to apply (optional)
  final int? coinDiscount;

  /// Price snapshot ID from order (optional, for backend validation)
  final String? priceSnapshotId;

  const InitiatePaymentRequest({
    required this.orderId,
    required this.paymentMethodCode,
    this.coinDiscount,
    this.priceSnapshotId,
  });

  /// Validate request before sending to backend
  String? validate() {
    if (orderId.isEmpty) {
      return 'Order ID is required';
    }
    if (paymentMethodCode.isEmpty) {
      return 'Metode pembayaran belum dipilih';
    }
    return null;
  }

  /// Convert to CreatePaymentRequest for repository — matches backend struct
  CreatePaymentRequest toCreatePaymentRequest() {
    return CreatePaymentRequest(
      orderId: orderId,
      paymentMethodCode: paymentMethodCode,
      coinDiscount: coinDiscount ?? 0,
      priceSnapshotId: priceSnapshotId,
    );
  }
}

// ============================================================================
// PAYMENT INITIATION NOTIFIER
// ============================================================================

/// Payment initiation notifier with safety mechanisms
///
/// SAFETY GUARDS:
/// 1. IMMEDIATE LOCK - isInitiating set synchronously before async operation
/// 2. IDEMPOTENCY KEY - UUID v4 generated once per attempt, preserved for retry
/// 3. BACKEND AUTHORITY - All payment state from backend, no client calculation
/// 4. EXPLICIT ERROR - All errors mapped to user-friendly messages
/// 5. COOLDOWN - 5 second cooldown between attempts to prevent spam
@riverpod
class PaymentInitiationNotifier extends _$PaymentInitiationNotifier {
  /// UUID v4 generator for idempotency keys
  static const _uuid = Uuid();

  /// Minimum cooldown between payment initiation attempts (seconds)
  static const _cooldownSeconds = 5;

  @override
  PaymentInitiationState build() {
    return const PaymentInitiationState();
  }

  /// Initiate payment for an order with safety mechanisms
  ///
  /// FLOW:
  /// 1. Check immediate lock (isInitiating) - return early if locked
  /// 2. Check cooldown - return early if within cooldown period
  /// 3. Validate request - return error if invalid
  /// 4. Generate idempotency key if not exists
  /// 5. Set initiating state (IMMEDIATE LOCK)
  /// 6. Call backend API
  /// 7. Handle response (success/failure)
  /// 8. Clear initiating lock
  ///
  /// IDEMPOTENCY:
  /// - Idempotency key generated once per payment attempt
  /// - Preserved in state for retry scenarios
  /// - Cleared on successful initiation
  Future<PaymentIntent?> initiatePayment(InitiatePaymentRequest request) async {
    // SAFETY GUARD 1: IMMEDIATE LOCK CHECK
    if (state.isInitiating) {
      _logger?.warning(
        'Payment initiation already in progress - ignoring duplicate request',
      );
      return null;
    }

    // SAFETY GUARD 2: COOLDOWN CHECK
    if (state.isInitiated && !state.hasCooldownPassed) {
      _logger?.warning('Payment initiation cooldown active - ignoring request');
      state = state.copyWith(
        error: 'Mohon tunggu $_cooldownSeconds detik sebelum mencoba lagi',
      );
      return null;
    }

    // SAFETY GUARD 3: REQUEST VALIDATION
    final validationError = request.validate();
    if (validationError != null) {
      state = state.copyWith(error: validationError);
      return null;
    }

    // IDEMPOTENCY: Generate key only if not present (retry scenario)
    final idempotencyKey = state.idempotencyKey ?? _uuid.v4();

    // IMMEDIATE LOCK: Set loading state BEFORE async operation
    state = state.copyWith(
      isInitiating: true,
      idempotencyKey: idempotencyKey,
      lastInitiatedAt: DateTime.now(),
      error: null,
    );

    try {
      final repo = ref.read(paymentRepositoryProvider);
      final createRequest = request.toCreatePaymentRequest();

      _logger?.info(
        'Initiating payment for order ${request.orderId} with idempotency key',
      );

      final result = await repo.createPayment(createRequest);

      return result.fold(
        (intent) {
          _logger?.info('Payment initiated successfully: ${intent.id}');

          // SUCCESS: Store intent and clear idempotency key
          state = PaymentInitiationState.success(
            intent: intent,
            idempotencyKey: null, // Clear key on success
          );
          return intent;
        },
        (failure) {
          _logger?.error('Payment initiation failed: ${failure.message}');

          // FAILURE: Keep idempotency key for potential retry
          state = state.copyWith(
            error: _getUserFriendlyErrorMessage(failure),
            isInitiating: false,
          );

          return null;
        },
      );
    } catch (e, stackTrace) {
      _logger?.error(
        'Unexpected error during payment initiation',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );

      // UNEXPECTED ERROR: Keep idempotency key for potential retry
      state = state.copyWith(
        error: 'Terjadi kesalahan. Silakan coba lagi.',
        isInitiating: false,
      );
      return null;
    }
  }

  /// Retry payment initiation with same idempotency key
  ///
  /// Uses the same idempotency key from previous attempt
  /// to ensure backend treats this as retry, not duplicate
  Future<PaymentIntent?> retryPayment(InitiatePaymentRequest request) async {
    if (state.idempotencyKey == null) {
      state = state.copyWith(
        error: 'Tidak dapat mencoba ulang. Silakan mulai pembayaran baru.',
      );
      return null;
    }

    _logger?.info(
      'Retrying payment initiation with idempotency key ${state.idempotencyKey}',
    );

    return initiatePayment(request);
  }

  /// Reset state for new payment initiation
  ///
  /// Clears all state including idempotency key
  /// Call this when starting a completely new payment flow
  void reset() {
    state = const PaymentInitiationState();
  }

  /// Clear error but preserve other state
  void clearError() {
    state = state.copyWith(error: '');
  }

  /// Convert payment failure to user-friendly message
  String _getUserFriendlyErrorMessage(payment_failures.PaymentFailure failure) {
    // Map payment failures to user-friendly Indonesian messages
    if (failure is payment_failures.NetworkFailure) {
      return 'Koneksi internet bermasalah. Silakan cek koneksi Anda.';
    }
    if (failure is payment_failures.PaymentNotFoundFailure) {
      return 'Pembayaran tidak ditemukan.';
    }
    if (failure is payment_failures.PaymentExpiredFailure) {
      return 'Pembayaran telah kadaluarsa.';
    }
    if (failure is payment_failures.ValidationFailure) {
      return failure.message.isNotEmpty
          ? failure.message
          : 'Data pembayaran tidak valid.';
    }
    // Default for UnknownFailure and others
    return failure.message.isNotEmpty
        ? failure.message
        : 'Terjadi kesalahan. Silakan coba lagi.';
  }

  core.ILoggerService? get _logger => ref.read(core.loggerServiceProvider);
}
