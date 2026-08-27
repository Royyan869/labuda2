/// Payment Request Abstraction
///
/// Reused from existing payment module with minor modifications.
/// This is already clean - no Firebase/Flutter dependencies.
library;

/// Base class untuk semua tipe payment request di platform.
/// Implements Strategy Pattern untuk extensibility.
abstract class PaymentRequest {
  /// Unique transaction ID
  String get transactionId;

  /// Total amount to be paid (in Rupiah)
  double get totalAmount;

  /// Currency code (default: IDR)
  String get currency => 'IDR';

  /// Breakdown item details untuk tampil di payment gateway
  List<Map<String, dynamic>> toItemDetails();

  /// Customer details untuk Midtrans
  Map<String, dynamic> toCustomerDetails();

  /// Metadata untuk callback identification
  Map<String, dynamic> get metadata;

  /// Optional: Description
  String? get description => null;

  /// Validate payment request
  String? validate() {
    if (totalAmount <= 0) {
      return 'Amount must be greater than 0';
    }
    if (transactionId.isEmpty) {
      return 'Transaction ID is required';
    }
    if (!metadata.containsKey('type')) {
      return 'Metadata must contain "type" field for callback routing';
    }
    return null;
  }
}
