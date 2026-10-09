part of '../screens/checkout_screen_impl.dart';

/// **CV3:** Shipping Clarity Banner
///
/// Provides clear expectation setting about seller-managed shipping model.
/// This helps users understand:
/// - Why they're providing shipping address
/// - What happens next (seller coordination)
/// - That they're still in the purchase flow
class _ShippingClarityBanner extends StatelessWidget {
  const _ShippingClarityBanner();

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: colorScheme.secondary.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(
          color: colorScheme.secondary.withValues(alpha: 0.25),
          width: 1,
        ),
      ),
      child: Row(
        children: [
          Icon(
            Icons.local_shipping_outlined,
            size: AppIconSize.action,
            color: colorScheme.secondary,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Pengiriman dikelola oleh penjual',
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.w600,
                    color: colorScheme.secondary,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  'Setelah pesanan dibuat, penjual akan menginformasikan opsi pengiriman yang tersedia.',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: colorScheme.onSurfaceVariant,
                    height: 1.4,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// **AW1/AW2:** Auction Winner Banner
///
/// Provides winner-specific context when checking out an auction win.
/// Frames the experience as "securing your victory" rather than generic purchase.
/// This maintains payoff continuity from the auction win celebration.
///
/// Winner messaging principles:
/// - Celebratory: "Selamat! Anda Memenangkan Lelang 🎉"
/// - Victory-focused: "Amankan kemenangan" not "Complete purchase"
/// - Finality: "Harga final sudah terkunci" (winner's price is secure)
class _AuctionWinnerBanner extends StatelessWidget {
  const _AuctionWinnerBanner();

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: context.statusColors.success.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(
          color: context.statusColors.success.withValues(alpha: 0.3),
          width: 1.5,
        ),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(AppMetrics.p8),
            decoration: BoxDecoration(
              color: context.statusColors.success.withValues(alpha: 0.15),
              shape: BoxShape.circle,
            ),
            child: Icon(
              Icons.emoji_events,
              size: AppIconSize.action,
              color: context.statusColors.success,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Selamat! Anda Memenangkan Lelang 🎉',
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.bold,
                    color: context.statusColors.success,
                  ),
                ),
                const SizedBox(height: 3),
                Text(
                  'Harga final sudah terkunci. Selesaikan checkout untuk mengamankan kemenangan Anda.',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: colorScheme.onSurfaceVariant,
                    height: 1.3,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Bid-win payment-method honesty note.
///
/// Owner canonical: an auction bid-win order is created WITHOUT a payment
/// method. The winner chooses the method on the created Order (Order Detail's
/// PaymentMethodPicker) and the first payment binds it. Checkout therefore
/// shows the escrow base honestly and states exactly where the method and its
/// fee come from — it never fakes a fee-inclusive final total.
class _BidWinPaymentMethodNote extends StatelessWidget {
  const _BidWinPaymentMethodNote();

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: colorScheme.secondary.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(
          color: colorScheme.secondary.withValues(alpha: 0.25),
          width: 1,
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            Icons.account_balance_wallet_outlined,
            size: AppIconSize.action,
            color: colorScheme.secondary,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Metode Pembayaran Dipilih Setelah Pesanan',
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.w600,
                    color: colorScheme.secondary,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  'Total di atas adalah harga barang + pengiriman. '
                  'Pilih metode pembayaran (beserta biaya layanannya) '
                  'setelah pesanan dibuat, saat menyelesaikan pembayaran.',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: colorScheme.onSurfaceVariant,
                    height: 1.4,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// **NEGOTIATION UX FIX:** Negotiation Warning Banner
///
/// Shows a warning when checking out from a negotiation source.
/// Negotiation acceptance does NOT reserve the product - checkout is required to secure it.
///
/// Display conditions:
/// - Show when negotiationId is present
/// - Warns that product is not reserved until checkout completes
class _NegotiationWarningBanner extends StatelessWidget {
  const _NegotiationWarningBanner();

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: context.statusColors.warning.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(
          color: context.statusColors.warning.withValues(alpha: 0.3),
          width: 1.5,
        ),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(AppMetrics.p8),
            decoration: BoxDecoration(
              color: context.statusColors.warning.withValues(alpha: 0.15),
              shape: BoxShape.circle,
            ),
            child: Icon(
              Icons.info_outline,
              size: AppIconSize.action,
              color: context.statusColors.warning,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Tawaran Sudah Disetujui',
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.bold,
                    color: context.statusColors.warning,
                  ),
                ),
                const SizedBox(height: 3),
                Text(
                  'Tawaran sudah disetujui, tetapi belum diamankan. Selesaikan checkout untuk mengunci produk.',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: colorScheme.onSurfaceVariant,
                    height: 1.3,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// **STOCK WARNING UX FIX 2:** Stock Warning Banner
///
/// Shows a warning about limited stock availability after preview succeeds.
/// This prevents user surprise when stock runs out during checkout.
///
/// Display conditions:
/// - Show after preview succeeds
/// - Show once per checkout session
/// - Use warning color to indicate urgency
class _StockWarningBanner extends StatelessWidget {
  const _StockWarningBanner();

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: context.statusColors.warning.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(
          color: context.statusColors.warning.withValues(alpha: 0.3),
          width: 1,
        ),
      ),
      child: Row(
        children: [
          Icon(
            Icons.warning_amber_outlined,
            size: AppIconSize.action,
            color: context.statusColors.warning,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Stok Terbatas',
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.w600,
                    color: context.statusColors.warning,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  'Barang bisa habis kapan saja. Segera selesaikan pembayaran.',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: colorScheme.onSurfaceVariant,
                    height: 1.3,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
