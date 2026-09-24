/// Order Detail Handlers
///
/// Implements order action handlers for the order detail screen.
/// Uses Riverpod providers for dependency injection - no ServiceLocator.
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/transaction/order/order.dart';
import 'package:labuda/domains/finance/transaction/payment/payment.dart';
import 'package:labuda/domains/social/rating/rating.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
// Payment URLs are presented exclusively inside Labuda's internal WebView.

mixin OrderDetailHandlersMixin on ConsumerState<OrderDetailScreen> {
  /// Generic handler for order actions
  Future<void> handleOrderAction(
    String action,
    Map<String, dynamic> params,
  ) async {
    // Delegate to specific handlers based on action type
    switch (action) {
      case 'accept':
        final orderId = params['orderId'] as String;
        final sellerId = params['sellerId'] as String;
        await handleAcceptOrder(orderId, sellerId);
        break;
      case 'reject':
        final orderId = params['orderId'] as String;
        final sellerId = params['sellerId'] as String;
        final reason = params['reason'] as String;
        await handleRejectOrder(orderId, sellerId, reason);
        break;
      case 'ship':
        final orderId = params['orderId'] as String;
        final sellerId = params['sellerId'] as String;
        final proofData = ShippingProofData(
          shippingReference: params['trackingNumber'] as String? ?? '',
          referenceType: params['referenceType'] as String?,
          note: params['note'] as String?,
        );
        await handleShipOrder(orderId, sellerId, proofData);
        break;
      case 'confirm_delivery':
        final orderId = params['orderId'] as String;
        final buyerId = params['buyerId'] as String;
        await handleConfirmDelivery(orderId, buyerId);
        break;
      case 'cancel':
        final orderId = params['orderId'] as String;
        final reason = params['reason'] as String?;
        await handleCancelOrder(orderId, reason ?? 'User cancelled');
        break;
      default:
        debugPrint('Unknown action: $action');
    }
  }

  Future<void> handleRefundRequest(String orderId, String reason) async {
    // Refund request is handled by OrderRefundHandler.showRefundDialog
    // This method is kept for interface compatibility
    debugPrint(
      'handleRefundRequest called - use OrderRefundHandler.showRefundDialog',
    );
  }

  Future<void> handleShippingUpdate(
    String orderId,
    String trackingNumber,
  ) async {
    // Shipping update is part of handleShipOrder
    debugPrint('handleShippingUpdate called - use handleShipOrder');
  }

  /// Cancel order - for both buyer and seller
  Future<void> handleCancelOrder(String orderId, String reason) async {
    try {
      final notifier = ref.read(orderProvider.notifier);
      await notifier.cancelOrder(orderId, reason);

      // Trigger order refresh to get updated status from backend
      ref.invalidate(orderRefreshTriggerProvider(orderId));

      if (mounted) {
        AppSnackBar.showSuccess(context, 'Order cancelled successfully');
      }
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Failed to cancel order: ${e.toString()}',
        );
      }
    }
  }

  /// Seller: Accept order
  Future<void> handleAcceptOrder(String orderId, String sellerId) async {
    if (mounted) {
      AppSnackBar.showError(
        context,
        'Order accept action is not supported by current backend contract.',
      );
    }
  }

  /// Seller: Reject order
  Future<void> handleRejectOrder(
    String orderId,
    String sellerId,
    String reason,
  ) async {
    try {
      // Cancel order to reject it
      await handleCancelOrder(orderId, 'Seller rejected: $reason');
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Failed to reject order: ${e.toString()}',
        );
      }
    }
  }

  /// Seller: Ship order with proof
  Future<void> handleShipOrder(
    String orderId,
    String sellerId,
    ShippingProofData proofData,
  ) async {
    try {
      final shippingReference = proofData.shippingReference;
      if (shippingReference.isEmpty) {
        if (mounted) {
          AppSnackBar.showError(context, 'Referensi pengiriman wajib diisi');
        }
        return;
      }

      final notifier = ref.read(orderProvider.notifier);
      await notifier.shipOrder(
        orderId,
        shippingReference,
        referenceType: proofData.referenceType,
        note: proofData.note,
      );

      // Trigger order refresh to get updated status from backend
      ref.invalidate(orderRefreshTriggerProvider(orderId));

      if (mounted) {
        AppSnackBar.showSuccess(context, 'Pesanan berhasil dikirim');
      }
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Gagal mengirim pesanan: ${e.toString()}',
        );
      }
    }
  }

  /// Buyer: Confirm delivery
  Future<void> handleConfirmDelivery(String orderId, String buyerId) async {
    try {
      final notifier = ref.read(orderProvider.notifier);
      await notifier.confirmDelivery(orderId);

      // Trigger order refresh to get updated status from backend
      ref.invalidate(orderRefreshTriggerProvider(orderId));

      if (mounted) {
        AppSnackBar.showSuccess(context, 'Delivery confirmed successfully');
      }
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Failed to confirm delivery: ${e.toString()}',
        );
      }
    }
  }

  /// Buyer: Extend confirmation deadline (Decision V2 action)
  Future<void> handleExtendConfirmation(String orderId) async {
    try {
      final notifier = ref.read(orderProvider.notifier);
      await notifier.extendOrderConfirmation(orderId);

      // Trigger order refresh to get updated status from backend
      ref.invalidate(orderRefreshTriggerProvider(orderId));

      if (mounted) {
        AppSnackBar.showSuccess(
          context,
          'Masa konfirmasi berhasil diperpanjang',
        );
      }
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Gagal memperpanjang konfirmasi: ${e.toString()}',
        );
      }
    }
  }

  /// Buyer: Submit rating
  Future<void> handleSubmitRating(
    String orderId,
    String fromUserId,
    String toUserId,
    int rating,
    String? review,
  ) async {
    try {
      final result = await ref
          .read(ratingRepositoryProvider)
          .createRatingForOrder(
            orderId: orderId,
            ratingValue: rating,
            comment: review,
          );

      if (result.isError) {
        if (result.errorCode == 'EMAIL_VERIFICATION_REQUIRED') {
          // Backend-rejection handler (defense-in-depth): the backend stays
          // the single authority for EMAIL_VERIFICATION_REQUIRED.
          if (mounted) {
            AppSnackBar.showError(
              context,
              'Verifikasi email kamu diperlukan sebelum memberi rating.',
            );
          }
          return;
        }
        if (mounted) {
          AppSnackBar.showError(
            context,
            'Failed to submit rating: ${result.error}',
          );
        }
        return;
      }
      if (mounted) {
        AppSnackBar.showSuccess(context, 'Rating submitted successfully');
        Navigator.of(context).pop();
      }
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Failed to submit rating: ${e.toString()}',
        );
      }
    }
  }

  /// Buyer: Pay now - Initiates payment flow with safety mechanisms
  ///
  /// PAYMENT INITIATION FLOW WITH SAFETY:
  /// 1. Validates order is pending payment
  /// 2. Creates payment intent via backend API with idempotency key
  /// 3. Launches payment URL for external payment gateway
  /// 4. Navigates to payment result screen for status polling
  ///
  /// SAFETY MECHANISMS:
  /// - Double-tap guard via PaymentInitiationNotifier.isInitiating
  /// - Idempotency key preserved for retry scenarios
  /// - Backend authority - payment state from backend only
  /// - Explicit error handling with user-friendly messages
  Future<void> handlePayNow(Order order) async {
    // SAFETY: Validate order status before attempting payment.
    // Guard on canonical order status (not payment status) — payment status
    // may be absent/stale; order status is the authoritative lifecycle state.
    if (order.status != OrderStatus.pending) {
      if (mounted) {
        _showPaymentErrorDialog(
          order.id,
          'Status pesanan tidak valid untuk pembayaran',
          'Pesanan ini sudah diproses atau dibatalkan.',
        );
      }
      return;
    }

    // SAFETY: Validate order total.
    // Canonical payable is total_payable_amount; when the backend has not
    // emitted it we fail closed (0) and refuse to start a payment.
    final payableTotal = order.pricing.totalPayableAmount ?? 0;

    if (payableTotal <= 0) {
      if (mounted) {
        _showPaymentErrorDialog(
          order.id,
          'Total pembayaran tidak valid',
          'Mohon hubungi customer service untuk bantuan.',
        );
      }
      return;
    }

    // PASS_18V: buyer must select a payment method before payment creation —
    // backend calculates the fee per method, never the client.
    final paymentRepo = ref.read(paymentRepositoryProvider);
    final methodsResult = await paymentRepo.getPaymentMethodOptions(order.id);
    if (!mounted) return;
    final methods = methodsResult.fold<List<PaymentMethodOption>>(
      (options) => options,
      (_) => const [],
    );
    if (methods.isEmpty) {
      if (mounted) {
        _showPaymentErrorDialog(
          order.id,
          'Metode pembayaran tidak tersedia',
          'Silakan coba lagi nanti atau hubungi customer service.',
        );
      }
      return;
    }
    final selectedMethodCode = await PaymentMethodPickerSheet.show(
      context,
      methods: methods,
    );
    if (!mounted || selectedMethodCode == null) return;

    // Get payment initiation notifier
    final paymentInitiationNotifier = ref.read(
      paymentInitiationProvider.notifier,
    );

    // Create payment initiation request — backend derives gross_amount and
    // buyer payment fee from the selected method (PASS_18V).
    final request = InitiatePaymentRequest(
      orderId: order.id,
      paymentMethodCode: selectedMethodCode,
      priceSnapshotId: order.priceSnapshotId,
    );

    // Initiate payment with safety mechanisms
    final intent = await paymentInitiationNotifier.initiatePayment(request);

    if (intent == null || !mounted) {
      // Payment initiation failed or widget unmounted
      // Error is already set in state, show to user if needed
      final initiationState = ref.read(paymentInitiationProvider);
      if (mounted && initiationState.error != null) {
        AppSnackBar.showError(context, initiationState.error!);
      }
      return;
    }

    // SUCCESS: Payment intent created
    // Now launch payment URL and navigate to result screen
    await _handlePaymentIntent(intent, order);
  }

  /// Handles payment intent after successful initiation.
  ///
  /// Payment URLs are presented exclusively inside Labuda's internal WebView
  /// (PaymentWebviewScreen). External-browser payment navigation is obsolete
  /// and must not be reintroduced. Completion remains backend-authoritative.
  Future<void> _handlePaymentIntent(PaymentIntent intent, Order order) async {
    final paymentUrl = intent.paymentUrl;

    if (paymentUrl != null && paymentUrl.isNotEmpty && mounted) {
      await context.push(
        '/payment-webview?url=${Uri.encodeComponent(paymentUrl)}&orderId=${Uri.encodeComponent(order.id)}',
      );
    }

    // Navigate to payment result screen for status polling (backend-authoritative)
    if (mounted) {
      context.push('/payment-result/${order.id}', extra: order.orderNumber);
    }
  }

  /// Shows error dialog for payment initiation errors
  void _showPaymentErrorDialog(String orderId, String title, String message) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        icon: Icon(
          Icons.error_outline,
          color: core.AppColors.statusError,
          size: 48,
        ),
        title: Text(title),
        content: Text(message),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Tutup'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.of(context).pop();
              // Navigate to order list
              context.push('/orders');
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: core.AppColors.primaryRed,
              foregroundColor: Colors.white,
            ),
            child: const Text('Lihat Pesanan Saya'),
          ),
        ],
      ),
    );
  }

  /// Buyer: Change payment method
  /// NOTE: Payment processing is handled separately - not in scope for this task
  Future<void> handleChangePaymentMethod(Order order) async {
    // Payment flow is handled by checkout/payment feature
    if (mounted) {
      AppSnackBar.showInfo(
        context,
        'Payment method change: Navigate to payment settings',
      );
    }
  }
}
