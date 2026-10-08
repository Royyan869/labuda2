part of '../screens/checkout_screen_impl.dart';

/// Order Summary Section
class _OrderSummarySection extends ConsumerWidget {
  final String forSaleId;

  /// The applied backend preview — non-null ONLY while it is current.
  final PreviewOrderResult? previewResult;
  final CheckoutReadiness readiness;
  final VoidCallback onRefreshPricing;
  final bool isAuctionCheckout;

  /// Canonical pre-order payment pricing + the buyer's selected method, used to
  /// show the final payable amount. Null until the methods are loaded.
  final PreOrderPaymentPricing? preOrderPricing;
  final String? selectedMethodCode;

  const _OrderSummarySection({
    required this.forSaleId,
    this.previewResult,
    required this.readiness,
    required this.onRefreshPricing,
    this.isAuctionCheckout = false,
    this.preOrderPricing,
    this.selectedMethodCode,
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
              onRefresh: onRefreshPricing,
            ),
            const SizedBox(height: 16),
            _OrderSummaryContent(
              forSale: forSale,
              previewResult: previewResult,
              isAuctionCheckout: isAuctionCheckout,
              preOrderPricing: preOrderPricing,
              selectedMethodCode: selectedMethodCode,
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
  final VoidCallback onRefresh;

  const _TokenValidityIndicator({
    required this.readiness,
    required this.onRefresh,
  });

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
              style: context.typeRoles.bodyDense.copyWith(
                color: colorScheme.onSurfaceVariant,
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
          Icon(
            Icons.info_outline,
            color: colorScheme.secondary,
            size: AppIconSize.action,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  readiness.title,
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.bold,
                    color: colorScheme.secondary,
                  ),
                ),
                Text(
                  readiness.message,
                  style: context.typeRoles.bodyDense.copyWith(
                    color: colorScheme.onSurfaceVariant,
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
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.bold,
                    color: accent,
                  ),
                ),
                Text(
                  readiness.message,
                  style: context.typeRoles.bodyDense.copyWith(
                    color: colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
          ElevatedButton(
            onPressed: onRefresh,
            style: ElevatedButton.styleFrom(
              backgroundColor: accent,
              padding: const EdgeInsets.symmetric(
                horizontal: AppMetrics.p16,
                vertical: AppMetrics.p8,
              ),
              textStyle: Theme.of(context).textTheme.labelLarge,
            ),
            child: const Text('Refresh'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    switch (readiness) {
      case CheckoutReadiness.ready:
        // Price locked: NO manual refresh affordance. Convergence is automatic
        // (input changes auto-preview; near-expiry auto-regenerates).
        return const SizedBox.shrink();
      case CheckoutReadiness.loading:
      case CheckoutReadiness.loadingPaymentMethods:
        return _buildLoading(context);
      case CheckoutReadiness.missingProduct:
      case CheckoutReadiness.missingAddress:
      case CheckoutReadiness.missingShipping:
      case CheckoutReadiness.missingPaymentMethod:
        return _buildPrerequisite(context);
      case CheckoutReadiness.error:
      case CheckoutReadiness.stale:
      case CheckoutReadiness.expired:
      case CheckoutReadiness.paymentMethodsError:
        return _buildRefreshRequired(context);
    }
  }
}

class _OrderSummaryContent extends StatelessWidget {
  final ForSale forSale;
  final PreviewOrderResult? previewResult;
  final bool isAuctionCheckout;
  final PreOrderPaymentPricing? preOrderPricing;
  final String? selectedMethodCode;

  const _OrderSummaryContent({
    required this.forSale,
    this.previewResult,
    this.isAuctionCheckout = false,
    this.preOrderPricing,
    this.selectedMethodCode,
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

    // All money is BACKEND AUTHORITY. The escrow base comes from the applied
    // preview; the buyer fee and FINAL payable come from the selected method in
    // the pre-order pricing response. Nothing is derived on the client.
    final hasPricing = previewResult != null;
    final subtotal = hasPricing ? previewResult!.subtotal : 0.0;
    final shippingCost = hasPricing ? previewResult!.shippingCost : 0.0;
    final selectedOption = preOrderPricing?.optionFor(selectedMethodCode);
    final serviceFee = selectedOption != null
        ? selectedOption.buyerPaymentFeeAmount.toDouble()
        : 0.0;
    // Before a method is selected the payable is the escrow base (PD+S); once a
    // method is selected it is that method's backend-computed FINAL amount.
    final total = selectedOption != null
        ? selectedOption.finalPayableAmount.toDouble()
        : (hasPricing ? (previewResult!.totalPayableAmount ?? 0.0) : 0.0);

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
              Text(
                'Ringkasan Pesanan',
                style: context.typeRoles.titleSection.copyWith(
                  fontWeight: FontWeight.bold,
                ),
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
                      color: context.statusColors.success.withValues(
                        alpha: 0.4,
                      ),
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
                        style: context.typeRoles.labelMicro.copyWith(
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
                      style: context.typeRoles.titleCompact.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 4),
                    if (koiDetailsDisplay.isNotEmpty)
                      Text(
                        koiDetailsDisplay,
                        style: context.typeRoles.labelMicro.copyWith(
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

          // Price Breakdown — BACKEND MONEY ONLY. The base is the applied
          // preview's escrow; the buyer fee and FINAL total come from the
          // selected pre-order method. There is no local price projection: an
          // amount is shown only once the backend has produced it (before that,
          // the readiness banner explains what is missing).
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
            _PriceRow('Total Pembayaran', 1, total, isTotal: true),
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

  const _PriceRow(
    this.label,
    this.quantity,
    this.price, {
    this.isTotal = false,
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
                    style:
                        (isTotal
                                ? context.typeRoles.titleCompact
                                : context.typeRoles.bodyDense)
                            .copyWith(
                              fontWeight: isTotal
                                  ? FontWeight.bold
                                  : FontWeight.normal,
                              color: isTotal ? colorScheme.primary : null,
                            ),
                  ),
                ],
              ),
            ),
            Text(
              AppFormatters.formatCurrency(total),
              style:
                  (isTotal
                          ? context.typeRoles.titleSection
                          : context.typeRoles.titleCompact)
                      .copyWith(
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
