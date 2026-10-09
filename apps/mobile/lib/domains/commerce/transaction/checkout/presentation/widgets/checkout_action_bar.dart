part of '../screens/checkout_screen_impl.dart';

/// Checkout Bottom Bar — business state only.
///
/// Chrome (surface, separator, padding, Safe Area, keyboard, button height,
/// loading presentation) is owned by [BottomActionBar]. This widget only
/// decides CONTENT: the final payable header, the readiness gate, and the label.
///
/// The label/header show the FINAL payable amount for the buyer's selected
/// payment method (backend-computed). Before a method is selected the amount is
/// unknown, so no amount is shown and the CTA stays gated by [isReady].
class _CheckoutBottomBar extends StatelessWidget {
  final bool isCreatingOrder;
  final bool isSubmitting;

  /// Readiness of the checkout step (see CheckoutReadiness). This is the single
  /// gate for the primary action: no local value can enable it.
  final bool isReady;

  /// Why the action is unavailable. Empty when [isReady].
  final String disabledReason;

  /// The FINAL payable amount (backend) for the selected method, or null when
  /// no method is selected yet.
  final int? finalPayableAmount;
  final VoidCallback onCreateOrder;

  /// Auction bid-win: winner framing on the CTA. Buy-now uses the regular
  /// purchase label.
  final bool isBidWin;

  const _CheckoutBottomBar({
    required this.isCreatingOrder,
    required this.isSubmitting,
    required this.isReady,
    this.disabledReason = '',
    this.finalPayableAmount,
    required this.onCreateOrder,
    this.isBidWin = false,
  });

  /// Builds the button text based on auction bid-win context.
  String _buildButtonText(BuildContext context) {
    if (finalPayableAmount != null) {
      // The label owns the 'Rp ' prefix so no caller can interpolate a bare
      // number behind it (money authority: formatGroupedAmount).
      final total = 'Rp ${formatGroupedAmount(finalPayableAmount!)}';

      if (isBidWin) {
        // Winner framing: "Secure Your Victory - Rp X"
        return 'Amankan Kemenangan - $total';
      }
      // Regular purchase: "Create Order - Rp X"
      return 'Buat Pesanan - $total';
    }
    return isBidWin ? 'Amankan Kemenangan' : 'Buat Pesanan';
  }

  @override
  Widget build(BuildContext context) {
    // Combine the submission locks with the readiness projection. The reason
    // shown to the buyer is the same value the create-order guard enforces.
    // Checkout's ONLY durable write is order creation; while it is on the wire
    // the CTA is disabled AND spinning, so a buyer cannot double-submit.
    final isDisabled = isSubmitting || isCreatingOrder || !isReady;
    final colorScheme = Theme.of(context).colorScheme;
    final showReason = !isCreatingOrder && disabledReason.isNotEmpty;

    return BottomActionBar(
      header: (finalPayableAmount != null || showReason)
          ? Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                // Show final payable if a method is selected.
                if (finalPayableAmount != null)
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        'Total Pembayaran',
                        style: context.typeRoles.bodyDense.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      Text(
                        'Rp ${formatGroupedAmount(finalPayableAmount!)}',
                        style: context.typeRoles.titleCompact.copyWith(
                          fontWeight: FontWeight.bold,
                          color: colorScheme.primary,
                        ),
                      ),
                    ],
                  ),
                if (finalPayableAmount != null && showReason)
                  const SizedBox(height: AppMetrics.p8),
                if (showReason)
                  Row(
                    children: [
                      Icon(
                        Icons.info_outline,
                        size: AppIconSize.inlineGlyph,
                        color: colorScheme.onSurfaceVariant,
                      ),
                      const SizedBox(width: AppMetrics.p8),
                      Expanded(
                        child: Text(
                          disabledReason,
                          style: context.typeRoles.bodyDense.copyWith(
                            color: colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ),
                    ],
                  ),
              ],
            )
          : null,
      primary: BottomBarAction(
        label: _buildButtonText(context),
        onPressed: isDisabled ? null : onCreateOrder,
        isLoading: isCreatingOrder,
      ),
    );
  }
}
