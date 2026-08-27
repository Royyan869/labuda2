part of '../screens/checkout_screen_impl.dart';

/// DISCOUNT HONESTY: Discount Section for Checkout
///
/// This section provides honest discount code input during checkout.
///
/// HONESTY PRINCIPLES:
/// - Shows for listing and auction checkout, not negotiation
/// - DiscountInputField validates codes via backend
/// - Shows success/error state from backend validation
/// - Displays applied discount honestly with description only
/// - Does NOT calculate or invent discount savings locally
///
/// FLOW:
/// 1. User enters discount code
/// 2. Code validated by backend (not frontend)
/// 3. If valid: store state, trigger preview refresh
/// 4. Backend preview returns final pricing with discount applied
class _DiscountSection extends ConsumerWidget {
  final String fixedPriceSaleId;
  final String? sellerId;
  final String contextType;
  final String? auctionId;
  final double subtotal;
  final void Function(Discount? discount, double amount) onDiscountApplied;
  final Discount? appliedDiscount;
  final double appliedDiscountAmount;

  const _DiscountSection({
    required this.fixedPriceSaleId,
    required this.sellerId,
    required this.contextType,
    this.auctionId,
    required this.subtotal,
    required this.onDiscountApplied,
    required this.appliedDiscount,
    required this.appliedDiscountAmount,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Header
        const Text(
          'Kode Promo',
          style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
        ),
        const SizedBox(height: 12),

        // Discount Input Field
        // HONESTY: This widget handles validation and display honestly
        // - Validates codes via backend
        // - Shows error messages from backend
        // - Displays success state only when backend confirms valid code
        // - Never invents discount amounts or fake savings
        DiscountInputField(
          subtotal: subtotal,
          contextType: contextType,
          sellerId: sellerId ?? '',
          listingId: contextType == 'listing' ? fixedPriceSaleId : null,
          auctionId: contextType == 'auction' ? auctionId : null,
          onDiscountApplied: onDiscountApplied,
        ),

        // HONESTY: Show applied discount summary ONLY if discount is applied
        // - Display comes from backend validation result
        // - Shows clear description (e.g., "10% off", "Rp50.000 off")
        // - No fake "you save X" calculations - amounts come from backend
        if (appliedDiscount != null) ...[
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: AppColors.successGreen.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(
                color: AppColors.successGreen.withValues(alpha: 0.3),
                width: 1,
              ),
            ),
            child: Row(
              children: [
                const Icon(
                  Icons.check_circle,
                  color: AppColors.successGreen,
                  size: 20,
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Kode "${appliedDiscount!.code}" berhasil diterapkan',
                        style: const TextStyle(
                          fontSize: 13,
                          fontWeight: FontWeight.w600,
                          color: AppColors.successGreen,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        _getDiscountDescription(appliedDiscount!),
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.neutralGray700,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ],
      ],
    );
  }

  /// HONESTY: Get discount description based on backend-validated discount type
  ///
  /// Returns a clear, non-misleading description of the discount:
  /// - percentage: "Diskon 10%" (from backend value)
  /// - flat amount: "Diskon nominal" (backend value)
  /// - free shipping: "Gratis ongkir" (backend flag)
  String _getDiscountDescription(Discount discount) {
    switch (discount.type) {
      case DiscountType.percentage:
        final percentage = discount.value;
        final maxDiscount = discount.maxDiscount;
        if (maxDiscount != null && maxDiscount > 0) {
          // Percentage with cap - show both
          return 'Diskon $percentage% (maks. Rp${maxDiscount.toInt()})';
        }
        return 'Diskon $percentage%';
      case DiscountType.flatAmount:
        return 'Diskon nominal';
      case DiscountType.freeShipping:
        return 'Gratis ongkir';
    }
  }
}
