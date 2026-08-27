part of '../screens/checkout_screen_impl.dart';

/// Checkout Bottom Bar
class _CheckoutBottomBar extends StatelessWidget {
  final bool isCreatingOrder;
  final bool isSubmitting;
  final bool isPricingAvailable;
  final bool isTokenExpired;
  final PreviewOrderResult? previewResult;
  final VoidCallback onCreateOrder;
  final bool isAuctionWinner;

  const _CheckoutBottomBar({
    required this.isCreatingOrder,
    required this.isSubmitting,
    required this.isPricingAvailable,
    this.isTokenExpired = false,
    this.previewResult,
    required this.onCreateOrder,
    this.isAuctionWinner = false,
  });

  /// Builds the button text based on auction winner context
  String _buildButtonText(BuildContext context) {
    if (previewResult != null) {
      final total = previewResult!.total
          .toStringAsFixed(0)
          .replaceAllMapped(
            RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'),
            (Match m) => '${m[1]}.',
          );

      if (isAuctionWinner) {
        // Winner framing: "Secure Your Victory - Rp X"
        return 'Amankan Kemenangan - Rp $total';
      }
      // Regular purchase: "Create Order - Rp X"
      return 'Buat Pesanan - Rp $total';
    }
    return isAuctionWinner ? 'Amankan Kemenangan' : 'Buat Pesanan';
  }

  @override
  Widget build(BuildContext context) {
    // Combine both state locks for immediate UI feedback
    // Disable when: submitting, creating order, no pricing available, OR token expired
    final isDisabled =
        isSubmitting ||
        isCreatingOrder ||
        !isPricingAvailable ||
        isTokenExpired;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: AppColors.neutralWhite,
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.05),
            blurRadius: 10,
            offset: const Offset(0, -2),
          ),
        ],
      ),
      child: SafeArea(
        top: false,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Show pricing summary if available
            if (previewResult != null) ...[
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    const Text(
                      'Total Pembayaran',
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    Text(
                      'Rp ${previewResult!.total.toStringAsFixed(0).replaceAllMapped(RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'), (Match m) => '${m[1]}.')}',
                      style: const TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                        color: AppColors.primaryRed,
                      ),
                    ),
                  ],
                ),
              ),
            ],
            if (!isPricingAvailable && !isCreatingOrder)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Row(
                  children: [
                    Icon(
                      Icons.info_outline,
                      size: 16,
                      color: AppColors.neutralGray600,
                    ),
                    const SizedBox(width: 8),
                    const Expanded(
                      child: Text(
                        'Memuat harga dari server...',
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.neutralGray600,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            SizedBox(
              width: double.infinity,
              height: 48,
              child: ElevatedButton(
                onPressed: isDisabled ? null : onCreateOrder,
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primaryRed,
                  foregroundColor: Colors.white,
                  disabledBackgroundColor: AppColors.neutralGray300,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8),
                  ),
                ),
                child: isCreatingOrder
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          valueColor: AlwaysStoppedAnimation<Color>(
                            Colors.white,
                          ),
                        ),
                      )
                    : Text(
                        _buildButtonText(context),
                        style: const TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
