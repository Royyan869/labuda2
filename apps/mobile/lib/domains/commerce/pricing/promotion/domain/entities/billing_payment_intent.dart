class BillingPaymentIntent {
  final String paymentId;
  final String paymentUrl;
  final int grossAmount;
  final DateTime? expiredAt;

  const BillingPaymentIntent({
    required this.paymentId,
    required this.paymentUrl,
    required this.grossAmount,
    required this.expiredAt,
  });
}
