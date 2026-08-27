import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/common/types/payment_types.dart';

/// Interface for handling payment webhook callbacks from payment gateway
///
/// This interface defines the contract for order module to handle
/// payment notifications without breaking module boundaries.
///
/// Implementation: Order module (OrderRepositoryImpl implements this)
/// Consumer: Cloud Functions (Midtrans webhook handler)
///
/// Architecture:
/// ```
/// Midtrans Webhook
///     ↓
/// Cloud Function (handleMidtransWebhook)
///     ↓
/// Cloud Function (updateOrderPaymentStatus) - via HTTPS callable
///     ↓
/// IOrderPaymentHandler.handlePaymentNotification()
///     ↓
/// OrderRepository updates order in Firestore
/// ```
///
/// This ensures:
/// - Order module controls its own data
/// - Payment updates go through proper business logic
/// - Transaction safety maintained
/// - No direct Firestore writes from webhook
abstract interface class IOrderPaymentHandler {
  /// Handle payment notification from payment gateway webhook
  ///
  /// This method is called when a payment status changes (from Midtrans webhook).
  /// The implementation should:
  /// 1. Validate the order exists
  /// 2. Update order payment status using Firestore transaction
  /// 3. Update order status based on payment result (paid → paid, failed → cancelled)
  /// 4. Handle any business logic (notifications, analytics, etc.)
  ///
  /// Parameters:
  /// - [orderId]: The order ID to update
  /// - [paymentStatus]: New payment status from payment gateway
  /// - [transactionId]: Payment gateway transaction ID
  /// - [paymentType]: Type of payment used (optional)
  /// - [metadata]: Additional payment metadata (optional)
  ///
  /// Returns:
  /// - Success: Result.success(null)
  /// - Error: Result.error with error message
  ///
  /// Example:
  /// ```dart
  /// final result = await orderPaymentHandler.handlePaymentNotification(
  ///   orderId: 'order_123',
  ///   paymentStatus: PaymentStatus.paid,
  ///   transactionId: 'midtrans_tx_456',
  ///   paymentType: 'gopay',
  ///   metadata: {'settlement_time': '2024-01-15 10:30:00'},
  /// );
  /// ```
  Future<Result<void>> handlePaymentNotification({
    required String orderId,
    required PaymentStatus paymentStatus,
    required String transactionId,
    String? paymentType,
    Map<String, dynamic>? metadata,
  });

  /// Verify payment status directly with payment gateway
  ///
  /// This method queries the payment gateway to check the current
  /// payment status of an order. Useful for:
  /// - Manual verification when webhook fails
  /// - Polling payment status
  /// - Reconciliation
  ///
  /// Parameters:
  /// - [orderId]: The order ID to verify
  ///
  /// Returns:
  /// - Success: Result.success(PaymentStatus)
  /// - Error: Result.error with error message
  ///
  /// Example:
  /// ```dart
  /// final result = await orderPaymentHandler.verifyPaymentStatus('order_123');
  /// if (result.isSuccess) {
  ///   print('Current payment status: ${result.data}');
  /// }
  /// ```
  Future<Result<PaymentStatus>> verifyPaymentStatus(String orderId);
}
