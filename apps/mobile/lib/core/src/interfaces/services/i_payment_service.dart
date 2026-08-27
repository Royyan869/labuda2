import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/common/types/payment_types.dart';

abstract class IPaymentService {
  Future<Result<PaymentIntent>> createPaymentIntent(
    CreatePaymentRequest request,
  );
  Future<Result<PaymentResult>> processPayment(ProcessPaymentRequest request);
  Future<Result<void>> cancelPayment(String paymentIntentId);
  Future<Result<PaymentStatus>> getPaymentStatus(String paymentIntentId);
  Future<Result<List<PaymentMethod>>> getSavedPaymentMethods();
  Future<Result<PaymentMethod>> savePaymentMethod(
    SavePaymentMethodRequest request,
  );
  Future<Result<void>> deletePaymentMethod(String paymentMethodId);
  Future<Result<RefundResult>> requestRefund(RefundRequest request);
  Future<Result<List<TransactionEntity>>> getTransactionHistory({
    int page = 1,
    int limit = 20,
  });
  Future<Result<TransactionEntity>> getTransactionById(String transactionId);
}

abstract class PaymentIntent {
  String get id;
  String get clientSecret;
  double get amount;
  String get currency;
  PaymentIntentStatus get status;
  String? get description;
  Map<String, dynamic>? get metadata;
  DateTime get createdAt;
}

abstract class PaymentResult {
  String get paymentIntentId;
  PaymentStatus get status;
  String? get paymentMethodId;
  String? get transactionId;
  String? get errorMessage;
  DateTime get processedAt;
}

abstract class PaymentMethod {
  String get id;
  PaymentMethodType get type;
  String get lastFourDigits;
  String get brand;
  DateTime get expiryDate;
  bool get isDefault;
  DateTime get createdAt;
}

abstract class TransactionEntity {
  String get id;
  String get paymentIntentId;
  String get buyerId;
  String get sellerId;
  String? get listingId;
  double get amount;
  double get platformFee;
  String get currency;
  PaymentTransactionType get type;
  TransactionStatus get status;
  String? get description;
  DateTime get createdAt;
  DateTime? get completedAt;
}

abstract class CreatePaymentRequest {
  double get amount;
  String get currency;
  String get buyerId;
  String get sellerId;
  String? get listingId;
  String? get description;
  Map<String, dynamic>? get metadata;
}

abstract class ProcessPaymentRequest {
  String get paymentIntentId;
  String get paymentMethodId;
  String? get confirmationToken;
}

abstract class SavePaymentMethodRequest {
  PaymentMethodType get type;
  String get token;
  bool get setAsDefault;
}

abstract class RefundRequest {
  String get transactionId;
  double? get amount;
  String get reason;
}

abstract class RefundResult {
  String get refundId;
  String get transactionId;
  double get amount;
  RefundStatus get status;
  DateTime get processedAt;
}

// PaymentMethodType and PaymentStatus now imported from core payment_types.dart

enum PaymentIntentStatus {
  requiresPaymentMethod,
  requiresConfirmation,
  requiresAction,
  processing,
  succeeded,
  canceled,
}

enum PaymentTransactionType { purchase, auction, escrow, subscription }

enum TransactionStatus {
  pending,
  processing,
  completed,
  failed,
  canceled,
  refunded,
  disputed,
}

enum RefundStatus { pending, processing, completed, failed }
