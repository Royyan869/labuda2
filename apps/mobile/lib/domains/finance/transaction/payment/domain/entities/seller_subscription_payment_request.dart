import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment_request.dart';
import 'package:uuid/uuid.dart';

/// Payment request for seller subscription upgrade
class SellerSubscriptionPaymentRequest extends PaymentRequest {
  final String userId;
  final String userDisplayName;
  final String userEmail;
  final String? userPhone;
  final double yearlyFee;
  final String _transactionId;

  SellerSubscriptionPaymentRequest({
    required this.userId,
    required this.userDisplayName,
    required this.userEmail,
    this.userPhone,
    required this.yearlyFee,
  }) : _transactionId = 'seller_sub_${const Uuid().v4()}';

  @override
  String get transactionId => _transactionId;

  @override
  double get totalAmount => yearlyFee;

  @override
  String? get description => 'Seller Subscription - 1 Year Access';

  @override
  List<Map<String, dynamic>> toItemDetails() {
    return [
      {
        'id': 'seller_subscription',
        'name': 'Seller Subscription (1 Year)',
        'price': yearlyFee.toInt(),
        'quantity': 1,
      },
    ];
  }

  @override
  Map<String, dynamic> toCustomerDetails() {
    return {
      'first_name': userDisplayName,
      'email': userEmail,
      if (userPhone != null) 'phone': userPhone,
    };
  }

  @override
  Map<String, dynamic> get metadata => {
    'type': 'seller_subscription',
    'userId': userId,
    'yearlyFee': yearlyFee,
    'subscriptionDuration': 365, // days
  };

  @override
  String? validate() {
    // Call parent validation first
    final parentValidation = super.validate();
    if (parentValidation != null) {
      return parentValidation;
    }

    // Custom validation
    if (userId.isEmpty) {
      return 'User ID is required';
    }

    if (userDisplayName.isEmpty) {
      return 'User display name is required';
    }

    if (userEmail.isEmpty) {
      return 'User email is required';
    }

    if (yearlyFee < 0) {
      return 'Yearly fee cannot be negative';
    }

    return null;
  }

  @override
  String toString() {
    return 'SellerSubscriptionPaymentRequest(userId: $userId, yearlyFee: $yearlyFee, transactionId: $transactionId)';
  }
}
