/// Payment Result Notifier
///
/// Manages payment status reconciliation flow with safety mechanisms:
/// - Backend authority for payment status
/// - Payment resource recovery when order state lags behind the payment row
/// - Immediate lock to prevent overlapping status checks
/// - Cancellation token for graceful cleanup
/// - Exponential backoff for polling
/// - Explicit error states
library;

import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/transaction/order/data/order_providers.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/order_status.dart';
import 'package:labuda/core/common/types/payment_types.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment.dart';
import 'package:labuda/domains/finance/transaction/payment/presentation/providers/payment_providers.dart';
import 'payment_result_state.dart';

part 'payment_result_notifier.g.dart';

/// Payment Result Notifier
///
/// SAFETY MECHANISMS:
/// 1. IMMEDIATE LOCK - isChecking set synchronously before async operations
/// 2. CANCELLATION TOKEN - polling can be aborted gracefully
/// 3. BACKEND AUTHORITY - order status is canonical; payment row is used for recovery
/// 4. EXPLICIT ERRORS - distinct states for timeout, network error, failed
/// 5. EXPONENTIAL BACKOFF - dynamic polling intervals to reduce server load
@riverpod
class PaymentResultNotifier extends _$PaymentResultNotifier {
  /// Maximum polling attempts before giving up
  static const _maxPollAttempts = 20;

  /// Initial polling interval (fast) - first 10 seconds
  static const _fastPollInterval = Duration(seconds: 3);

  /// Slow polling interval (after initial period) - 10-15 seconds
  static const _slowPollMinInterval = Duration(seconds: 10);

  /// Timer for polling
  Timer? _pollTimer;

  /// Cancellation token - if true, all polling should stop
  bool _isCancelled = false;

  /// In-flight check guard - prevents overlapping status checks
  bool _isCheckingInFlight = false;

  /// True only while an actual network fetch is awaited inside
  /// [_checkPaymentStatus] - used to skip a resume-triggered recheck instead
  /// of racing it against a call already in progress.
  bool _isFetchInFlight = false;

  @override
  PaymentResultState build() {
    // Clean up timer when provider is disposed
    ref.onDispose(() {
      _cancelTimer();
    });

    return PaymentResultState.initial();
  }

  /// Start checking payment status for an order
  ///
  /// FLOW:
  /// 1. Check immediate lock - return if already checking
  /// 2. Reset cancellation token
  /// 3. Set checking state (IMMEDIATE LOCK)
  /// 4. Perform first status check
  /// 5. If pending/processing, schedule next poll
  ///
  /// CANCELLATION: Call stopChecking() to abort polling
  Future<void> startChecking(String orderId) async {
    // SAFETY GUARD 1: IMMEDIATE LOCK CHECK
    if (_isCheckingInFlight) {
      _logger?.warning(
        'Payment status check already in flight - ignoring duplicate request',
      );
      return;
    }

    // SAFETY GUARD 2: Reset cancellation token for new check
    _isCancelled = false;

    // IMMEDIATE LOCK: Set checking state BEFORE async operation
    final previousPayment = state.payment;
    _isCheckingInFlight = true;
    state = PaymentResultState.checking(
      pollAttempts: 0,
      startedAt: DateTime.now(),
      maxAttempts: _maxPollAttempts,
      payment: previousPayment,
    );

    // Perform first check
    await _checkPaymentStatus(orderId);
  }

  /// Stop checking payment status
  ///
  /// Cancels any ongoing polling and cleans up timers.
  /// Call this when user navigates away or manually stops.
  void stopChecking() {
    _isCancelled = true;
    _cancelTimer();
    _isCheckingInFlight = false;

    // Update state to cancelled if we were checking
    if (state.status == PaymentResultScreenStatus.checking) {
      state = PaymentResultState.cancelled(
        order: state.order,
        pollAttempts: state.pollAttempts,
      );
    }
  }

  /// Manually retry payment status check
  ///
  /// User-initiated retry - resets attempt counter and starts fresh.
  /// STATUS-ONLY: never opens the payment URL - that is an explicit,
  /// separate user action (see payment_result_screen_impl's continue-payment
  /// handler).
  Future<void> retry(String orderId) async {
    // Cancel any existing polling
    stopChecking();

    // Small delay to ensure cleanup
    await Future.delayed(const Duration(milliseconds: 100));

    // Start fresh
    await startChecking(orderId);
  }

  /// Status-only recheck triggered when the app resumes from background
  /// (e.g. the user returns from the external payment browser/app).
  ///
  /// SAFETY:
  /// - No-ops on a terminal (success/failed) or cancelled state - nothing to
  ///   recheck, and restarting would regress an already-resolved screen.
  /// - No-ops while a fetch is already in flight - avoids racing an
  ///   overlapping request instead of scheduling a duplicate poll loop.
  /// - Never opens the payment URL.
  Future<void> recheckOnResume(String orderId) async {
    if (_isCancelled || state.isCancelled) {
      return;
    }
    if (state.status == PaymentResultScreenStatus.success ||
        state.status == PaymentResultScreenStatus.failed) {
      return;
    }
    if (_isFetchInFlight) {
      _logger?.info(
        'Resume recheck skipped - a payment status fetch is already in flight',
      );
      return;
    }

    _cancelTimer();
    await _checkPaymentStatus(orderId);
  }

  /// Check payment status with backend
  ///
  /// CORE LOGIC:
  /// 1. Fetch order from backend (authoritative source)
  /// 2. Optionally fetch the linked payment row for recovery URL/status
  /// 3. Determine if terminal state reached or continue polling
  ///
  /// BACKEND AUTHORITY: Order status is canonical; payment row only helps recovery
  Future<void> _checkPaymentStatus(String orderId) async {
    // SAFETY: Check cancellation before making network call
    if (_isCancelled) {
      _logger?.info('Payment status check cancelled - aborting');
      return;
    }

    // SAFETY: Check if we've exceeded max attempts
    if (state.pollAttempts >= _maxPollAttempts) {
      state = PaymentResultState.timeout(
        order: state.order,
        pollAttempts: state.pollAttempts,
        startedAt: state.pollingStartedAt,
        maxAttempts: _maxPollAttempts,
      );
      _isCheckingInFlight = false;
      return;
    }

    _isFetchInFlight = true;
    try {
      // Fetch order from backend - this is the SINGLE SOURCE OF TRUTH
      final orderRepo = ref.read(orderRepositoryProvider);
      final orderResult = await orderRepo.getOrderById(orderId);
      if (!orderResult.isSuccess || orderResult.data == null) {
        final error = orderResult.error ?? 'Unknown error';

        // SAFETY: Check cancellation
        if (_isCancelled) {
          _logger?.info('Payment status check cancelled on error - aborting');
          return;
        }

        // ERROR: Network or API error
        _logger?.error('Failed to fetch order for payment status: $error');

        state = PaymentResultState.networkError(
          order: state.order,
          payment: state.payment,
          errorMessage: error.toString(),
          pollAttempts: state.pollAttempts,
          startedAt: state.pollingStartedAt,
        );
        _isCheckingInFlight = false;
        return;
      }

      final order = orderResult.data!;

      // SAFETY: Check cancellation after async call
      if (_isCancelled) {
        _logger?.info('Payment status check cancelled after fetch - aborting');
        return;
      }

      Payment? payment = state.payment;
      final paymentId = order.paymentId;
      if (paymentId != null && paymentId.isNotEmpty) {
        final paymentRepo = ref.read(paymentRepositoryProvider);
        final paymentResult = await paymentRepo.getPayment(paymentId);

        if (paymentResult.isSuccess && paymentResult.data != null) {
          payment = paymentResult.data!;
        } else if (paymentResult.isFailure) {
          _logger?.warning(
            'Failed to fetch payment for payment result recovery: ${paymentResult.failure}',
          );
        }
      }

      // SAFETY: Check cancellation before updating state
      if (_isCancelled) {
        _logger?.info('Payment status check cancelled before state update');
        return;
      }

      state = state.copyWith(order: order, payment: payment);

      if (_isOrderSuccessful(order.status) || _isPaymentSuccessful(payment)) {
        // SUCCESS: Backend confirmed paid state either on order or payment resource
        state = PaymentResultState.success(
          order: order,
          payment: payment,
          pollAttempts: state.pollAttempts + 1,
          startedAt: state.pollingStartedAt,
        );
        _cancelTimer();
        _isCheckingInFlight = false;
        return;
      }

      if (_isOrderFailed(order.status) || _isPaymentFailed(payment)) {
        // FAILED: Backend confirmed terminal failure state
        final reason = _isOrderFailed(order.status)
            ? _getFailureReason(order.status)
            : _getPaymentFailureReason(payment);
        state = PaymentResultState.failed(
          order: order,
          payment: payment,
          reason: reason,
          pollAttempts: state.pollAttempts + 1,
          startedAt: state.pollingStartedAt,
        );
        _cancelTimer();
        _isCheckingInFlight = false;
        return;
      }

      // STILL PENDING: Continue polling
      final nextAttempt = state.pollAttempts + 1;

      if (nextAttempt >= _maxPollAttempts) {
        // TIMEOUT: Max attempts reached
        state = PaymentResultState.timeout(
          order: order,
          payment: payment,
          pollAttempts: nextAttempt,
          startedAt: state.pollingStartedAt,
          maxAttempts: _maxPollAttempts,
        );
        _isCheckingInFlight = false;
      } else {
        // CONTINUE: Schedule next poll
        state = state.copyWith(
          order: order,
          payment: payment,
          pollAttempts: nextAttempt,
        );

        final interval = _getNextPollInterval();
        _scheduleNextPoll(orderId, interval);
      }
    } catch (e, stackTrace) {
      // SAFETY: Check cancellation
      if (_isCancelled) {
        _logger?.info('Payment status check cancelled on exception - aborting');
        return;
      }

      // UNEXPECTED ERROR
      _logger?.error(
        'Unexpected error during payment status check',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );

      state = PaymentResultState.networkError(
        order: state.order,
        payment: state.payment,
        errorMessage: 'Terjadi kesalahan. Silakan coba lagi.',
        pollAttempts: state.pollAttempts,
        startedAt: state.pollingStartedAt,
      );
      _isCheckingInFlight = false;
    } finally {
      _isFetchInFlight = false;
    }
  }

  /// Schedule next poll with exponential backoff
  void _scheduleNextPoll(String orderId, Duration interval) {
    // Cancel any existing timer
    _cancelTimer();

    // Schedule next poll
    _pollTimer = Timer(interval, () {
      if (!_isCancelled) {
        _checkPaymentStatus(orderId);
      }
    });
  }

  /// Cancel polling timer
  void _cancelTimer() {
    _pollTimer?.cancel();
    _pollTimer = null;
  }

  /// Get next polling interval based on elapsed time
  ///
  /// STRATEGY:
  /// - First 10 seconds: Fast poll (3 seconds)
  /// - After 10 seconds: Slow poll (10-15 seconds with jitter)
  /// This reduces server load while providing quick initial feedback
  Duration _getNextPollInterval() {
    final elapsed = state.elapsed;
    if (elapsed == null || elapsed.inSeconds < 10) {
      return _fastPollInterval;
    }

    // Add jitter to prevent thundering herd
    final jitter = (DateTime.now().millisecond % 5);
    return _slowPollMinInterval + Duration(seconds: jitter);
  }

  /// Get user-friendly failure reason from payment status
  String _getFailureReason(OrderStatus status) {
    switch (status) {
      case OrderStatus.cancelled:
        return 'Pesanan dibatalkan.';
      case OrderStatus.cancelledTimeout:
        return 'Pesanan dibatalkan karena batas waktu telah terlewati.';
      case OrderStatus.refunded:
        return 'Pembayaran telah dikembalikan.';
      case OrderStatus.partiallyRefunded:
        return 'Pembayaran telah dikembalikan sebagian.';
      case OrderStatus.disputeOpen:
        return 'Pesanan sedang dalam sengketa.';
      case OrderStatus.expired:
        return 'Pembayaran kadaluarsa. Silakan buat pesanan baru.';
      case OrderStatus.pending:
      case OrderStatus.paid:
      case OrderStatus.shipped:
      case OrderStatus.delivered:
      case OrderStatus.completed:
        return 'Status pesanan tidak valid.';
    }
  }

  String _getPaymentFailureReason(Payment? payment) {
    switch (payment?.status) {
      case PaymentStatus.failed:
        return 'Pembayaran gagal.';
      case PaymentStatus.expired:
        return 'Pembayaran kadaluarsa. Silakan buat pesanan baru.';
      case PaymentStatus.refunded:
        return 'Pembayaran telah dikembalikan.';
      case PaymentStatus.pending:
      case PaymentStatus.processing:
      case PaymentStatus.paid:
      case null:
        return 'Status pembayaran tidak valid.';
    }
  }

  bool _isOrderSuccessful(OrderStatus status) {
    switch (status) {
      case OrderStatus.paid:
      case OrderStatus.shipped:
      case OrderStatus.delivered:
      case OrderStatus.completed:
        return true;
      case OrderStatus.pending:
      case OrderStatus.cancelled:
      case OrderStatus.cancelledTimeout:
      case OrderStatus.refunded:
      case OrderStatus.partiallyRefunded:
      case OrderStatus.disputeOpen:
      case OrderStatus.expired:
        return false;
    }
  }

  bool _isOrderFailed(OrderStatus status) {
    switch (status) {
      case OrderStatus.cancelled:
      case OrderStatus.cancelledTimeout:
      case OrderStatus.refunded:
      case OrderStatus.partiallyRefunded:
      case OrderStatus.disputeOpen:
      case OrderStatus.expired:
        return true;
      case OrderStatus.pending:
      case OrderStatus.paid:
      case OrderStatus.shipped:
      case OrderStatus.delivered:
      case OrderStatus.completed:
        return false;
    }
  }

  bool _isPaymentSuccessful(Payment? payment) {
    switch (payment?.status) {
      case PaymentStatus.paid:
        return true;
      case PaymentStatus.pending:
      case PaymentStatus.processing:
      case PaymentStatus.failed:
      case PaymentStatus.expired:
      case PaymentStatus.refunded:
      case null:
        return false;
    }
  }

  bool _isPaymentFailed(Payment? payment) {
    switch (payment?.status) {
      case PaymentStatus.failed:
      case PaymentStatus.expired:
      case PaymentStatus.refunded:
        return true;
      case PaymentStatus.pending:
      case PaymentStatus.processing:
      case PaymentStatus.paid:
      case null:
        return false;
    }
  }

  /// Clear error and reset to checking state
  void clearError() {
    if (state.status == PaymentResultScreenStatus.networkError) {
      state = state.copyWith(status: PaymentResultScreenStatus.checking);
    }
  }

  core.ILoggerService? get _logger => ref.read(core.loggerServiceProvider);
}
