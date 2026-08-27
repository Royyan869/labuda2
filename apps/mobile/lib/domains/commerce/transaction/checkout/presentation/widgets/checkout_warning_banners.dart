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
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.primaryBlue.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: AppColors.primaryBlue.withValues(alpha: 0.25),
          width: 1,
        ),
      ),
      child: Row(
        children: [
          Icon(
            Icons.local_shipping_outlined,
            size: 18,
            color: AppColors.primaryBlue,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Pengiriman dikelola oleh penjual',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: AppColors.primaryBlue,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  'Setelah pesanan dibuat, penjual akan menginformasikan opsi pengiriman yang tersedia.',
                  style: TextStyle(
                    fontSize: 11,
                    color: AppColors.neutralGray700,
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
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Colors.green.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: Colors.green.withValues(alpha: 0.3),
          width: 1.5,
        ),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(6),
            decoration: BoxDecoration(
              color: Colors.green.withValues(alpha: 0.15),
              shape: BoxShape.circle,
            ),
            child: Icon(Icons.emoji_events, size: 18, color: Colors.green),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Selamat! Anda Memenangkan Lelang 🎉',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.bold,
                    color: Colors.green.shade700,
                  ),
                ),
                const SizedBox(height: 3),
                Text(
                  'Lengkapi pembayaran untuk mengamankan kemenangan Anda. Harga final sudah terkunci.',
                  style: TextStyle(
                    fontSize: 12,
                    color: AppColors.neutralGray700,
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
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Colors.orange.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: Colors.orange.withValues(alpha: 0.3),
          width: 1.5,
        ),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(6),
            decoration: BoxDecoration(
              color: Colors.orange.withValues(alpha: 0.15),
              shape: BoxShape.circle,
            ),
            child: Icon(Icons.info_outline, size: 18, color: Colors.orange),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Tawaran Sudah Disetujui',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.bold,
                    color: Colors.orange.shade700,
                  ),
                ),
                const SizedBox(height: 3),
                Text(
                  'Tawaran sudah disetujui, tetapi belum diamankan. Selesaikan checkout untuk mengunci produk.',
                  style: TextStyle(
                    fontSize: 12,
                    color: AppColors.neutralGray700,
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
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.statusWarning.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: AppColors.statusWarning.withValues(alpha: 0.3),
          width: 1,
        ),
      ),
      child: Row(
        children: [
          Icon(
            Icons.warning_amber_outlined,
            size: 18,
            color: AppColors.statusWarning,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Stok Terbatas',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: AppColors.statusWarning,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  'Barang bisa habis kapan saja. Segera selesaikan pembayaran.',
                  style: TextStyle(
                    fontSize: 11,
                    color: AppColors.neutralGray700,
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
