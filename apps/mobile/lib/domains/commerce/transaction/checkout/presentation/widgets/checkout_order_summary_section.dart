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
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(AppShape.r8),
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
                fontSize: AppType.s14,
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
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: colorScheme.secondary.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(
          color: colorScheme.secondary.withValues(alpha: 0.25),
        ),
      ),
      child: Row(
        children: [
          Icon(Icons.info_outline, color: colorScheme.secondary, size: AppIconSize.action),
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
                    fontSize: AppType.s14,
                  ),
                ),
                Text(
                  readiness.message,
                  style: TextStyle(
                    color: colorScheme.onSurfaceVariant,
                    fontSize: AppType.s12,
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
    final accent = isExpired ? colorScheme.error : context.statusColors.warning;
    final icon = isExpired
        ? Icons.timer_off_outlined
        : Icons.warning_amber_outlined;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: accent.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(color: accent),
      ),
      child: Row(
        children: [
          Icon(icon, color: accent, size: AppIconSize.action),
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
                    fontSize: AppType.s14,
                  ),
                ),
                Text(
                  readiness.message,
                  style: TextStyle(
                    color: colorScheme.onSurfaceVariant,
                    fontSize: AppType.s12,
                  ),
                ),
              ],
            ),
          ),
          ElevatedButton(
            onPressed: onRefresh,
            style: ElevatedButton.styleFrom(
              backgroundColor: accent,
              padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p8),
              textStyle: const TextStyle(fontSize: AppType.s12),
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
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: isUrgent
            ? context.statusColors.warning.withValues(alpha: 0.1)
            : context.statusColors.success.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(
          color: isUrgent ? context.statusColors.warning : context.statusColors.success,
        ),
      ),
      child: Row(
        children: [
          Icon(
            isUrgent ? Icons.timer_outlined : Icons.verified_outlined,
            color: isUrgent ? context.statusColors.warning : context.statusColors.success,
            size: AppIconSize.action,
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
                        ? context.statusColors.warning
                        : context.statusColors.success,
                    fontSize: AppType.s14,
                  ),
                ),
                Text(
                  'Berlaku dalam $timeString',
                  style: TextStyle(
                    color: colorScheme.onSurfaceVariant,
                    fontSize: AppType.s12,
                  ),
                ),
              ],
            ),
          ),
          OutlinedButton.icon(
            onPressed: onRefresh,
            style: OutlinedButton.styleFrom(
              foregroundColor: colorScheme.onSurfaceVariant,
              padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p8),
              minimumSize: const Size(0, 32),
              textStyle: const TextStyle(fontSize: AppType.s12),
            ),
            icon: const Icon(Icons.refresh, size: AppIconSize.inlineGlyph),
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
    // BUSINESS TRUTH: the seller tariff is ALL-IN (shipping + packing).
    // Buyer-facing surfaces must label it "Ongkir + Packing", never bare
    // "Ongkir", so no cost feels hidden.
    final shippingLabel = isUsingQuote
        ? 'Pengiriman (Hasil Negosiasi)'
        : 'Ongkir + Packing';

    // **DEFENSIVE ASSERT:** Ensure quote mode state is valid
    // - Verifies snapshot mode is either 'quote' or 'standard'
    // - In debug builds, this will alert developers to unexpected backend responses
    assert(
      !hasPricing || const ['quote', 'standard'].contains(shippingMode),
      'Invalid shipping mode: "$shippingMode". Expected "quote" or "standard". '
      'Backend preview returned unexpected value.',
    );

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text(
                'Ringkasan Pesanan',
                style: TextStyle(fontSize: AppType.s20, fontWeight: FontWeight.bold),
              ),
              const SizedBox(width: 8),
              // Auction badge - shows this is an auction-derived order
              if (isAuctionCheckout)
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppMetrics.p8,
                    vertical: AppMetrics.p4,
                  ),
                  decoration: BoxDecoration(
                    color: context.statusColors.success.withValues(alpha: 0.15),
                    borderRadius: BorderRadius.circular(AppShape.r4),
                    border: Border.all(
                      color: context.statusColors.success.withValues(alpha: 0.4),
                      width: 1,
                    ),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        Icons.emoji_events,
                        size: AppIconSize.inlineGlyph,
                        color: context.statusColors.success,
                      ),
                      const SizedBox(width: 3),
                      Text(
                        'Lelang',
                        style: TextStyle(
                          fontSize: AppType.s12,
                          fontWeight: FontWeight.bold,
                          color: context.statusColors.success,
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
                  borderRadius: BorderRadius.circular(AppShape.r8),
                  child: AppImage(
                    imageUrl: forSale.media.first.originalUrl,
                    width: 60,
                    height: 60,
                    fit: BoxFit.cover,
                    backgroundColor: colorScheme.surfaceContainerHighest,
                    errorWidget: Container(
                      width: 60,
                      height: 60,
                      color: colorScheme.surfaceContainerHighest,
                      child: const Icon(Icons.image),
                    ),
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
                        fontSize: AppType.s14,
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
                          fontSize: AppType.s12,
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
              'Ongkir + Packing',
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
                  fontSize: AppType.s12,
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
                      fontSize: isTotal ? AppType.s16 : AppType.s14,
                      fontWeight: isTotal ? FontWeight.bold : FontWeight.normal,
                      color: isTotal ? colorScheme.primary : null,
                    ),
                  ),
                  if (note != null)
                    Text(
                      note!,
                      style: TextStyle(
                        fontSize: AppType.s12,
                        color: colorScheme.onSurfaceVariant,
                      ),
                    ),
                ],
              ),
            ),
            Text(
              AppFormatters.formatCurrency(total),
              style: TextStyle(
                fontSize: isTotal ? AppType.s20 : AppType.s14,
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
