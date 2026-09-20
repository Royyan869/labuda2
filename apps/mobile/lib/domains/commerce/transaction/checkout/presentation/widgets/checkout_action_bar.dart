part of '../screens/checkout_screen_impl.dart';

/// Checkout Bottom Bar
class _CheckoutBottomBar extends StatelessWidget {
  final bool isCreatingOrder;
  final bool isSubmitting;

  /// Readiness of the pricing step (see CheckoutReadiness). This is the single
  /// gate for the primary action: a local price can never enable it.
  final bool isReady;

  /// Why the action is unavailable. Empty when [isReady].
  final String disabledReason;
  final PreviewOrderResult? previewResult;
  final VoidCallback onCreateOrder;
  final bool isAuctionWinner;

  const _CheckoutBottomBar({
    required this.isCreatingOrder,
    required this.isSubmitting,
    required this.isReady,
    this.disabledReason = '',
    this.previewResult,
    required this.onCreateOrder,
    this.isAuctionWinner = false,
  });

  /// Builds the button text based on auction winner context
  String _buildButtonText(BuildContext context) {
    if (previewResult != null) {
      final total = (previewResult!.totalPayableAmount ?? 0)
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
    // Combine the submission locks with the readiness projection. The reason
    // shown to the buyer is the same value the create-order guard enforces.
    final isDisabled = isSubmitting || isCreatingOrder || !isReady;
    final colorScheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        boxShadow: [
          BoxShadow(
            color: colorScheme.shadow.withValues(alpha: 0.05),
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
                      'Rp ${(previewResult!.totalPayableAmount ?? 0).toStringAsFixed(0).replaceAllMapped(RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'), (Match m) => '${m[1]}.')}',
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                        color: colorScheme.primary,
                      ),
                    ),
                  ],
                ),
              ),
            ],
            if (!isCreatingOrder && disabledReason.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Row(
                  children: [
                    Icon(
                      Icons.info_outline,
                      size: 16,
                      color: colorScheme.onSurfaceVariant,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        disabledReason,
                        style: TextStyle(
                          fontSize: 12,
                          color: colorScheme.onSurfaceVariant,
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
                  backgroundColor: colorScheme.primary,
                  foregroundColor: colorScheme.onPrimary,
                  disabledBackgroundColor: colorScheme.onSurface.withValues(
                    alpha: 0.12,
                  ),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8),
                  ),
                ),
                child: isCreatingOrder
                    ? SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          valueColor: AlwaysStoppedAnimation<Color>(
                            colorScheme.onPrimary,
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
