part of '../screens/checkout_screen_impl.dart';

/// Display facts the order summary renders for the purchased product.
///
/// Sourced from the sale surface authority of the checkout context: ForSale
/// for for_sale/negotiation, Auction for auction buy-now/bid-win. Checkout
/// never derives product identity itself — it only forwards what the
/// authoritative detail provider returns.
class _SummaryProductDisplay {
  final String title;
  final String? imageUrl;
  final String koiDetailsDisplay;

  const _SummaryProductDisplay({
    required this.title,
    this.imageUrl,
    this.koiDetailsDisplay = '',
  });
}

_SummaryProductDisplay _summaryDisplayFromForSale(ForSale forSale) {
  String koiDetailsDisplay = '';
  if (forSale.variety != null || forSale.sizeCm != null) {
    final variety = forSale.variety ?? 'Koi';
    final size = forSale.sizeCm != null ? '${forSale.sizeCm!.toInt()} cm' : '';
    koiDetailsDisplay = size.isNotEmpty ? '$variety - $size' : variety;
  }
  return _SummaryProductDisplay(
    title: forSale.title,
    imageUrl: forSale.media.isNotEmpty ? forSale.media.first.originalUrl : null,
    koiDetailsDisplay: koiDetailsDisplay,
  );
}

_SummaryProductDisplay _summaryDisplayFromAuction(Auction auction) {
  final koi = auction.koiDetails;
  String koiDetailsDisplay = '';
  if (koi.variety.isNotEmpty || koi.sizeInCm > 0) {
    final size = koi.sizeInCm > 0 ? '${koi.sizeInCm.toInt()} cm' : '';
    koiDetailsDisplay = size.isNotEmpty ? '${koi.variety} - $size' : koi.variety;
  }
  return _SummaryProductDisplay(
    title: auction.title,
    imageUrl: auction.media.isNotEmpty
        ? auction.media.first.originalUrl
        : null,
    koiDetailsDisplay: koiDetailsDisplay,
  );
}

/// F5-local fit measure: whether a single-line title + chip pair fits the
/// incoming width. Local copy (no new shared authority).
bool _fitsTitleBadgeSingleLine({
  required BuildContext context,
  required double maxWidth,
  required String title,
  required String badge,
  required TextStyle? titleStyle,
  required TextStyle? badgeStyle,
  required double fixedExtrasWidth,
}) {
  if (!maxWidth.isFinite) {
    return false;
  }
  final TextDirection direction = Directionality.of(context);
  final TextScaler scaler = MediaQuery.textScalerOf(context);

  double singleLineWidth(String text, TextStyle? style) {
    final TextPainter painter = TextPainter(
      text: TextSpan(text: text, style: style),
      textDirection: direction,
      textScaler: scaler,
      maxLines: 1,
    )..layout();
    return painter.width;
  }

  const double safetyMargin = 2;
  return singleLineWidth(title, titleStyle) +
          fixedExtrasWidth +
          singleLineWidth(badge, badgeStyle) +
          safetyMargin <=
      maxWidth;
}

/// Order Summary Section
class _OrderSummarySection extends ConsumerWidget {
  /// Sale-surface id of this checkout context: the for-sale id for for_sale /
  /// negotiation, the AUCTION id for auction buy-now/bid-win (the route's
  /// :id path slot).
  final String forSaleId;

  /// The applied backend preview — non-null ONLY while it is current.
  final PreviewOrderResult? previewResult;
  final CheckoutReadiness readiness;
  final VoidCallback onRefreshPricing;
  final bool isAuctionCheckout;

  /// Canonical pre-order payment pricing + the buyer's selected method, used to
  /// show the final payable amount. Null until the methods are loaded (always
  /// null for bid-win, which binds no method at creation).
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
    // AUCTION CONTEXT: the sale-surface id is the AUCTION id — resolving
    // forSaleDetailProvider with it always yields null and (previously)
    // collapsed the ENTIRE summary, including the readiness indicator. Auction
    // display data comes from the auction authority instead.
    final Widget child;
    if (isAuctionCheckout) {
      final auctionAsync = ref.watch(auctionDetailProvider(forSaleId));
      child = auctionAsync.when(
        data: (auction) {
          if (auction == null) {
            return const SizedBox.shrink();
          }
          return _buildBody(context, _summaryDisplayFromAuction(auction));
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, stackTrace) => const SizedBox.shrink(),
      );
    } else {
      final forSaleAsync = ref.watch(forSaleDetailProvider(forSaleId));
      child = forSaleAsync.when(
        data: (forSale) {
          if (forSale == null) {
            return const SizedBox.shrink();
          }
          return _buildBody(context, _summaryDisplayFromForSale(forSale));
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, stackTrace) => const SizedBox.shrink(),
      );
    }
    return child;
  }

  Widget _buildBody(BuildContext context, _SummaryProductDisplay product) {
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
          product: product,
          previewResult: previewResult,
          isAuctionCheckout: isAuctionCheckout,
          preOrderPricing: preOrderPricing,
          selectedMethodCode: selectedMethodCode,
        ),
      ],
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
  final _SummaryProductDisplay product;
  final PreviewOrderResult? previewResult;
  final bool isAuctionCheckout;
  final PreOrderPaymentPricing? preOrderPricing;
  final String? selectedMethodCode;

  const _OrderSummaryContent({
    required this.product,
    this.previewResult,
    this.isAuctionCheckout = false,
    this.preOrderPricing,
    this.selectedMethodCode,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    final koiDetailsDisplay = product.koiDetailsDisplay;

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
          // F5 header (adaptive): section title + auction chip share one row
          // when the single-line pair fits, else the title stacks over the
          // intact chip.
          LayoutBuilder(
            builder: (BuildContext context, BoxConstraints constraints) {
              final TextStyle titleStyle = context.typeRoles.titleSection
                  .copyWith(fontWeight: FontWeight.bold);
              final TextStyle chipStyle = context.typeRoles.labelMicro
                  .copyWith(fontWeight: FontWeight.bold);
              final bool fits =
                  !isAuctionCheckout ||
                  _fitsTitleBadgeSingleLine(
                    context: context,
                    maxWidth: constraints.maxWidth,
                    title: 'Ringkasan Pesanan',
                    badge: 'Lelang',
                    titleStyle: titleStyle,
                    badgeStyle: chipStyle,
                    fixedExtrasWidth:
                        8 +
                        AppMetrics.p8 * 2 +
                        AppIconSize.inlineGlyph +
                        3,
                  );
              if (fits) {
                return Row(
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
                          color: context.statusColors.success.withValues(
                            alpha: 0.15,
                          ),
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
                );
              }
              return Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Ringkasan Pesanan',
                    style: context.typeRoles.titleSection.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  // Auction badge - shows this is an auction-derived order
                  if (isAuctionCheckout) ...[
                    const SizedBox(height: 4),
                    Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: AppMetrics.p8,
                        vertical: AppMetrics.p4,
                      ),
                      decoration: BoxDecoration(
                        color: context.statusColors.success.withValues(
                          alpha: 0.15,
                        ),
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
                            softWrap: true,
                          ),
                        ],
                      ),
                    ),
                  ],
                ],
              );
            },
          ),
          const SizedBox(height: 12),

          // Product Info
          Row(
            children: [
              // Product Image
              if (product.imageUrl != null)
                ClipRRect(
                  borderRadius: BorderRadius.circular(AppShape.r8),
                  child: AppImage(
                    imageUrl: product.imageUrl!,
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
                      product.title,
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
