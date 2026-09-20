part of '../screens/checkout_screen_impl.dart';

/// Order Summary Section
class _OrderSummarySection extends ConsumerWidget {
  final String forSaleId;

  /// The applied backend preview — non-null ONLY while it is current.
  final PreviewOrderResult? previewResult;
  final CheckoutReadiness readiness;
  final Duration? remainingTime;
  final VoidCallback onRefreshPricing;
  final bool isAuctionCheckout;

  const _OrderSummarySection({
    required this.forSaleId,
    this.previewResult,
    required this.readiness,
    this.remainingTime,
    required this.onRefreshPricing,
    this.isAuctionCheckout = false,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final forSaleAsync = ref.watch(forSaleDetailProvider(forSaleId));

    return forSaleAsync.when(
      data: (forSale) {
        if (forSale == null) {
          return const SizedBox.shrink();
        }
        return Column(
          children: [
            // Pricing readiness indicator — the ONLY place that explains why
            // checkout is or is not ready, so it can never disagree with the
            // action bar or the create-order guard.
            _TokenValidityIndicator(
              readiness: readiness,
              remainingTime: remainingTime,
              onRefresh: onRefreshPricing,
            ),
            const SizedBox(height: 16),
            _OrderSummaryContent(
              forSale: forSale,
              previewResult: previewResult,
              isAuctionCheckout: isAuctionCheckout,
            ),
          ],
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, stackTrace) => const SizedBox.shrink(),
    );
  }
}

/// Pricing readiness indicator: truthful per-state copy + refresh affordance.
class _TokenValidityIndicator extends StatelessWidget {
  final CheckoutReadiness readiness;
  final Duration? remainingTime;
  final VoidCallback onRefresh;

  const _TokenValidityIndicator({
    required this.readiness,
    this.remainingTime,
    required this.onRefresh,
  });

  String _formatRemainingTime(Duration duration) {
    if (duration <= Duration.zero) return '0:00';
    final minutes = duration.inMinutes;
    final seconds = duration.inSeconds % 60;
    return '$minutes:${seconds.toString().padLeft(2, '0')}';
  }

  /// No current preview yet, but every prerequisite is satisfied.
  Widget _buildLoading(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          const SizedBox(
            width: 16,
            height: 16,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              readiness.message,
              style: TextStyle(
                color: colorScheme.onSurfaceVariant,
                fontSize: 14,
              ),
            ),
          ),
        ],
      ),
    );
  }

  /// A checkout prerequisite is missing — the buyer can act on this.
  Widget _buildPrerequisite(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colorScheme.secondary.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: colorScheme.secondary.withValues(alpha: 0.25),
        ),
      ),
      child: Row(
        children: [
          Icon(Icons.info_outline, color: colorScheme.secondary, size: 20),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  readiness.title,
                  style: TextStyle(
                    fontWeight: FontWeight.bold,
                    color: colorScheme.secondary,
                    fontSize: 14,
                  ),
                ),
                Text(
                  readiness.message,
                  style: TextStyle(
                    color: colorScheme.onSurfaceVariant,
                    fontSize: 12,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  /// The applied preview cannot be used: it failed, is stale, or expired.
  Widget _buildRefreshRequired(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final isExpired = readiness == CheckoutReadiness.expired;
    // Expiry is an error condition (canonical `error` role); every other
    // not-ready reason is a business warning, which Labuda models as its own
    // status colour rather than a Material role.
    final accent = isExpired ? colorScheme.error : AppColors.statusWarning;
    final icon = isExpired
        ? Icons.timer_off_outlined
        : Icons.warning_amber_outlined;

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: accent.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: accent),
      ),
      child: Row(
        children: [
          Icon(icon, color: accent, size: 20),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  readiness.title,
                  style: TextStyle(
                    fontWeight: FontWeight.bold,
                    color: accent,
                    fontSize: 14,
                  ),
                ),
                Text(
                  readiness.message,
                  style: TextStyle(
                    color: colorScheme.onSurfaceVariant,
                    fontSize: 12,
                  ),
                ),
              ],
            ),
          ),
          ElevatedButton(
            onPressed: onRefresh,
            style: ElevatedButton.styleFrom(
              backgroundColor: accent,
              foregroundColor: colorScheme.onPrimary,
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              textStyle: const TextStyle(fontSize: 12),
            ),
            child: const Text('Refresh'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    switch (readiness) {
      case CheckoutReadiness.ready:
        break;
      case CheckoutReadiness.loading:
        return _buildLoading(context);
      case CheckoutReadiness.missingProduct:
      case CheckoutReadiness.missingAddress:
      case CheckoutReadiness.missingShipping:
        return _buildPrerequisite(context);
      case CheckoutReadiness.error:
      case CheckoutReadiness.stale:
      case CheckoutReadiness.expired:
        return _buildRefreshRequired(context);
    }

    // Show countdown with refresh button
    final timeString = remainingTime != null
        ? _formatRemainingTime(remainingTime!)
        : '--:--';
    final isUrgent = remainingTime != null && remainingTime!.inMinutes < 3;

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isUrgent
            ? AppColors.statusWarning.withValues(alpha: 0.1)
            : AppColors.successGreen.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: isUrgent ? AppColors.statusWarning : AppColors.successGreen,
        ),
      ),
      child: Row(
        children: [
          Icon(
            isUrgent ? Icons.timer_outlined : Icons.verified_outlined,
            color: isUrgent ? AppColors.statusWarning : AppColors.successGreen,
            size: 20,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Harga Terkunci',
                  style: TextStyle(
                    fontWeight: FontWeight.bold,
                    color: isUrgent
                        ? AppColors.statusWarning
                        : AppColors.successGreen,
                    fontSize: 14,
                  ),
                ),
                Text(
                  'Berlaku dalam $timeString',
                  style: TextStyle(
                    color: colorScheme.onSurfaceVariant,
                    fontSize: 12,
                  ),
                ),
              ],
            ),
          ),
          OutlinedButton.icon(
            onPressed: onRefresh,
            style: OutlinedButton.styleFrom(
              foregroundColor: colorScheme.onSurfaceVariant,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              minimumSize: const Size(0, 32),
              textStyle: const TextStyle(fontSize: 12),
              side: BorderSide(color: colorScheme.outlineVariant),
            ),
            icon: const Icon(Icons.refresh, size: 16),
            label: const Text('Refresh'),
          ),
        ],
      ),
    );
  }
}

class _OrderSummaryContent extends StatelessWidget {
  final ForSale forSale;
  final PreviewOrderResult? previewResult;
  final bool isAuctionCheckout;

  const _OrderSummaryContent({
    required this.forSale,
    this.previewResult,
    this.isAuctionCheckout = false,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    // Build koi details display string
    String koiDetailsDisplay = '';
    if (forSale.variety != null || forSale.sizeCm != null) {
      final variety = forSale.variety ?? 'Koi';
      final size = forSale.sizeCm != null
          ? '${forSale.sizeCm!.toInt()} cm'
          : '';
      koiDetailsDisplay = size.isNotEmpty ? '$variety - $size' : variety;
    }

    // Use preview pricing if available, otherwise show loading state
    final hasPricing = previewResult != null;
    final subtotal = hasPricing ? previewResult!.subtotal : 0.0;
    final shippingCost = hasPricing ? previewResult!.shippingCost : 0.0;
    final serviceFee = hasPricing
        ? (previewResult!.serviceFeeAmount ?? 0.0)
        : 0.0;
    // CANONICAL TOTAL: total_payable_amount (PD + S + F) is the buyer's gross
    // payable and is emitted by the backend on every canonical order/preview
    // surface. The old `previewResult.total` compatibility alias (which
    // re-derived P+S+F client side) and the legacy discount / coin rows were
    // purged: the seller discount is already folded into the canonical money
    // model, and coins are not an Order snapshot authority.
    final total = hasPricing ? (previewResult!.totalPayableAmount ?? 0.0) : 0.0;

    // SHIPPING MODE INDICATOR: Determine shipping label based on mode
    // - "quote": Manual shipping quote from seller (fixed price)
    // - "standard": Standard forSale shipping options
    //
    // **DEFENSIVE GUARD:** Source of truth is previewResult.shippingMode (backend snapshot)
    // - UI uses snapshot mode, NOT widget.shippingQuoteId param
    // - Ensures correct behavior even with deep link/refresh (param may be lost)
    // - Backend validates quote availability and returns authoritative mode
    final shippingMode = hasPricing ? previewResult!.shippingMode : 'standard';
    final isUsingQuote = shippingMode == 'quote';
    final shippingLabel = isUsingQuote
        ? 'Pengiriman (Hasil Negosiasi)'
        : 'Biaya Pengiriman';

    // **DEFENSIVE ASSERT:** Ensure quote mode state is valid
    // - Verifies snapshot mode is either 'quote' or 'standard'
    // - In debug builds, this will alert developers to unexpected backend responses
    assert(
      !hasPricing || const ['quote', 'standard'].contains(shippingMode),
      'Invalid shipping mode: "$shippingMode". Expected "quote" or "standard". '
      'Backend preview returned unexpected value.',
    );

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text(
                'Ringkasan Pesanan',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
              ),
              const SizedBox(width: 8),
              // Auction badge - shows this is an auction-derived order
              if (isAuctionCheckout)
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 3,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.successGreen.withValues(alpha: 0.15),
                    borderRadius: BorderRadius.circular(4),
                    border: Border.all(
                      color: AppColors.successGreen.withValues(alpha: 0.4),
                      width: 1,
                    ),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Icon(
                        Icons.emoji_events,
                        size: 12,
                        color: AppColors.successGreen,
                      ),
                      const SizedBox(width: 3),
                      Text(
                        'Lelang',
                        style: TextStyle(
                          fontSize: 11,
                          fontWeight: FontWeight.bold,
                          color: AppColors.successGreen,
                        ),
                      ),
                    ],
                  ),
                ),
            ],
          ),
          const SizedBox(height: 12),

          // Product Info
          Row(
            children: [
              // Product Image
              if (forSale.media.isNotEmpty)
                ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: Image.network(
                    forSale.media.first.originalUrl,
                    width: 60,
                    height: 60,
                    fit: BoxFit.cover,
                    errorBuilder: (context, error, stackTrace) {
                      return Container(
                        width: 60,
                        height: 60,
                        color: colorScheme.surfaceContainerHighest,
                        child: const Icon(Icons.image),
                      );
                    },
                  ),
                ),
              const SizedBox(width: 12),

              // Product Details
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      forSale.title,
                      style: const TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 4),
                    if (koiDetailsDisplay.isNotEmpty)
                      Text(
                        koiDetailsDisplay,
                        style: TextStyle(
                          fontSize: 12,
                          color: colorScheme.onSurfaceVariant,
                        ),
                      ),
                  ],
                ),
              ),
            ],
          ),

          const SizedBox(height: 12),
          const Divider(),
          const SizedBox(height: 12),

          // Price Breakdown - using backend preview pricing
          if (hasPricing) ...[
            _PriceRow('Subtotal', 1, subtotal),
            const SizedBox(height: 8),
            _PriceRow(shippingLabel, 1, shippingCost),
            if (serviceFee > 0) ...[
              const SizedBox(height: 8),
              _PriceRow('Biaya Layanan Pembayaran', 1, serviceFee),
            ],
            const SizedBox(height: 12),
            const Divider(),
            const SizedBox(height: 12),
            // Total
            _PriceRow('Total', 1, total, isTotal: true),
          ] else ...[
            // NON-AUTHORITATIVE LOCAL PROJECTION.
            // `forSale.price` is NOT the checkout price: it ignores negotiation,
            // auction settlement, seller discount and payment fee. It is shown
            // only as an explicitly-labelled temporary estimate while the
            // backend preview for the CURRENT inputs has not been applied yet.
            // Nothing here can make checkout READY.
            _PriceRow(
              'Subtotal',
              1,
              forSale.price,
              note: 'Harga lokal sementara',
            ),
            const SizedBox(height: 8),
            const _PriceRow(
              'Biaya Pengiriman',
              1,
              0,
              note: 'Akan dihitung oleh server',
            ),
            const SizedBox(height: 8),
            const _PriceRow(
              'Biaya Layanan Pembayaran',
              1,
              0,
              note: 'Akan dihitung oleh server',
            ),
            const SizedBox(height: 12),
            const Divider(),
            const SizedBox(height: 12),
            // Total
            _PriceRow(
              'Total',
              1,
              forSale.price,
              isTotal: true,
              note: 'Harga lokal sementara',
            ),
            const SizedBox(height: 8),
            Center(
              child: Text(
                'Harga lokal sementara — menunggu harga dari server',
                style: TextStyle(
                  fontSize: 12,
                  color: colorScheme.onSurfaceVariant,
                  fontStyle: FontStyle.italic,
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

/// Single pricing row in the checkout breakdown.
///
/// The legacy `isDiscount` / `color` modifiers were purged together with the
/// obsolete discount row: the seller discount is already embedded in the
/// canonical backend money model (subtotal / total_before_coins_amount), so
/// the summary never renders a separate, client-derived discount line.
class _PriceRow extends StatelessWidget {
  final String label;
  final int quantity;
  final double price;
  final bool isTotal;
  final String? note;

  const _PriceRow(
    this.label,
    this.quantity,
    this.price, {
    this.isTotal = false,
    this.note,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final total = quantity * price;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    label,
                    style: TextStyle(
                      fontSize: isTotal ? 16 : 14,
                      fontWeight: isTotal ? FontWeight.bold : FontWeight.normal,
                      color: isTotal ? colorScheme.primary : null,
                    ),
                  ),
                  if (note != null)
                    Text(
                      note!,
                      style: TextStyle(
                        fontSize: 11,
                        color: colorScheme.onSurfaceVariant,
                      ),
                    ),
                ],
              ),
            ),
            Text(
              AppFormatters.formatCurrency(total),
              style: TextStyle(
                fontSize: isTotal ? 18 : 14,
                fontWeight: isTotal ? FontWeight.bold : FontWeight.w600,
                color: isTotal ? colorScheme.primary : null,
              ),
            ),
          ],
        ),
      ],
    );
  }
}
