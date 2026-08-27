part of '../screens/checkout_screen_impl.dart';

/// Order Summary Section
class _OrderSummarySection extends ConsumerWidget {
  final String fixedPriceSaleId;
  final PreviewOrderResult? previewResult;
  final bool isTokenExpired;
  final Duration? remainingTime;
  final bool isFetchingPreview;
  final String? previewError;
  final VoidCallback onRefreshPricing;
  final bool supportsDiscounts;
  final bool isAuctionCheckout;

  const _OrderSummarySection({
    required this.fixedPriceSaleId,
    this.previewResult,
    required this.isTokenExpired,
    this.remainingTime,
    required this.isFetchingPreview,
    this.previewError,
    required this.onRefreshPricing,
    this.supportsDiscounts = true,
    this.isAuctionCheckout = false,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final listingAsync = ref.watch(
      forSaleDetailProvider(fixedPriceSaleId),
    );

    return listingAsync.when(
      data: (listing) {
        if (listing == null) {
          return const SizedBox.shrink();
        }
        return Column(
          children: [
            // Token validity indicator with refresh button
            _TokenValidityIndicator(
              hasPricing: previewResult != null,
              isTokenExpired: isTokenExpired,
              remainingTime: remainingTime,
              isFetching: isFetchingPreview,
              error: previewError,
              onRefresh: onRefreshPricing,
            ),
            if (previewResult != null) const SizedBox(height: 16),
            _OrderSummaryContent(
              listing: listing,
              previewResult: previewResult,
              supportsDiscounts: supportsDiscounts,
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

/// Token validity indicator widget showing countdown and refresh button
class _TokenValidityIndicator extends StatelessWidget {
  final bool hasPricing;
  final bool isTokenExpired;
  final Duration? remainingTime;
  final bool isFetching;
  final String? error;
  final VoidCallback onRefresh;

  const _TokenValidityIndicator({
    required this.hasPricing,
    required this.isTokenExpired,
    this.remainingTime,
    required this.isFetching,
    this.error,
    required this.onRefresh,
  });

  String _formatRemainingTime(Duration duration) {
    if (duration <= Duration.zero) return '0:00';
    final minutes = duration.inMinutes;
    final seconds = duration.inSeconds % 60;
    return '$minutes:${seconds.toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    // ERROR STATE: Show error when preview fetch failed
    if (error != null && !hasPricing) {
      return Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: AppColors.statusWarning.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: AppColors.statusWarning),
        ),
        child: Row(
          children: [
            Icon(
              Icons.warning_amber_outlined,
              color: AppColors.statusWarning,
              size: 20,
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Gagal Memuat Harga',
                    style: TextStyle(
                      fontWeight: FontWeight.bold,
                      color: AppColors.statusWarning,
                      fontSize: 14,
                    ),
                  ),
                  Text(
                    'Tap refresh untuk mencoba lagi',
                    style: TextStyle(
                      color: AppColors.neutralGray600,
                      fontSize: 12,
                    ),
                  ),
                ],
              ),
            ),
            ElevatedButton(
              onPressed: onRefresh,
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.statusWarning,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 8,
                ),
                textStyle: const TextStyle(fontSize: 12),
              ),
              child: const Text('Refresh'),
            ),
          ],
        ),
      );
    }

    // LOADING/FETCHING STATE: Show loading when fetching preview
    if (!hasPricing || isFetching) {
      return Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: AppColors.neutralGray100,
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
            Text(
              isFetching ? 'Memperbarui harga...' : 'Memuat harga...',
              style: TextStyle(color: AppColors.neutralGray600, fontSize: 14),
            ),
          ],
        ),
      );
    }

    if (isTokenExpired) {
      // Token expired - show urgent refresh
      return Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: AppColors.statusError.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: AppColors.statusError),
        ),
        child: Row(
          children: [
            Icon(
              Icons.timer_off_outlined,
              color: AppColors.statusError,
              size: 20,
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Harga Kadaluarsa',
                    style: TextStyle(
                      fontWeight: FontWeight.bold,
                      color: AppColors.statusError,
                      fontSize: 14,
                    ),
                  ),
                  Text(
                    'Silakan refresh harga terbaru',
                    style: TextStyle(
                      color: AppColors.neutralGray600,
                      fontSize: 12,
                    ),
                  ),
                ],
              ),
            ),
            ElevatedButton(
              onPressed: onRefresh,
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.statusError,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 8,
                ),
                textStyle: const TextStyle(fontSize: 12),
              ),
              child: const Text('Refresh'),
            ),
          ],
        ),
      );
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
                    color: AppColors.neutralGray600,
                    fontSize: 12,
                  ),
                ),
              ],
            ),
          ),
          OutlinedButton.icon(
            onPressed: onRefresh,
            style: OutlinedButton.styleFrom(
              foregroundColor: AppColors.neutralGray700,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              minimumSize: const Size(0, 32),
              textStyle: const TextStyle(fontSize: 12),
              side: BorderSide(color: AppColors.neutralGray300),
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
  final ForSale listing;
  final PreviewOrderResult? previewResult;
  final bool supportsDiscounts;
  final bool isAuctionCheckout;

  const _OrderSummaryContent({
    required this.listing,
    this.previewResult,
    this.supportsDiscounts = true,
    this.isAuctionCheckout = false,
  });

  @override
  Widget build(BuildContext context) {
    // Build koi details display string
    String koiDetailsDisplay = '';
    if (listing.variety != null || listing.sizeCm != null) {
      final variety = listing.variety ?? 'Koi';
      final size = listing.sizeCm != null
          ? '${listing.sizeCm!.toInt()} cm'
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
    final coinDiscount = hasPricing ? previewResult!.coinDiscount : 0.0;
    final discount = hasPricing ? previewResult!.discount : 0.0;
    final total = hasPricing
        ? (previewResult!.totalPayableAmount ?? previewResult!.total)
        : 0.0;

    // SHIPPING MODE INDICATOR: Determine shipping label based on mode
    // - "quote": Manual shipping quote from seller (fixed price)
    // - "standard": Standard listing shipping options
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
        color: AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.neutralGray200),
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
                    color: Colors.green.withValues(alpha: 0.15),
                    borderRadius: BorderRadius.circular(4),
                    border: Border.all(
                      color: Colors.green.withValues(alpha: 0.4),
                      width: 1,
                    ),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        Icons.emoji_events,
                        size: 12,
                        color: Colors.green.shade700,
                      ),
                      const SizedBox(width: 3),
                      Text(
                        'Lelang',
                        style: TextStyle(
                          fontSize: 11,
                          fontWeight: FontWeight.bold,
                          color: Colors.green.shade700,
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
              if (listing.media.isNotEmpty)
                ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: Image.network(
                    listing.media.first.originalUrl,
                    width: 60,
                    height: 60,
                    fit: BoxFit.cover,
                    errorBuilder: (context, error, stackTrace) {
                      return Container(
                        width: 60,
                        height: 60,
                        color: AppColors.neutralGray200,
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
                      listing.title,
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
                        style: const TextStyle(
                          fontSize: 12,
                          color: AppColors.neutralGray600,
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
            if (coinDiscount > 0) ...[
              const SizedBox(height: 8),
              _PriceRow(
                'Diskon Koin',
                1,
                coinDiscount,
                isDiscount: true,
                color: AppColors.coinPrimary,
              ),
            ],
            // SOURCE GUARD: Only show discount row if source supports it AND discount > 0
            if (supportsDiscounts && discount > 0) ...[
              const SizedBox(height: 8),
              _PriceRow('Diskon', 1, discount, isDiscount: true),
            ],
            const SizedBox(height: 12),
            const Divider(),
            const SizedBox(height: 12),
            // Total
            _PriceRow('Total', 1, total, isTotal: true),
          ] else ...[
            // Loading state - placeholder values
            _PriceRow('Subtotal', 1, listing.price),
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
            _PriceRow('Total', 1, listing.price, isTotal: true),
            const SizedBox(height: 8),
            Center(
              child: Text(
                'Memuat harga dari server...',
                style: TextStyle(
                  fontSize: 12,
                  color: AppColors.neutralGray600,
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

class _PriceRow extends StatelessWidget {
  final String label;
  final int quantity;
  final double price;
  final bool isTotal;
  final bool isDiscount;
  final Color? color;
  final String? note;

  const _PriceRow(
    this.label,
    this.quantity,
    this.price, {
    this.isTotal = false,
    this.isDiscount = false,
    this.color,
    this.note,
  });

  @override
  Widget build(BuildContext context) {
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
                      color: isTotal
                          ? AppColors.primaryRed
                          : (isDiscount
                                ? (color ?? AppColors.successGreen)
                                : null),
                      decoration: isDiscount
                          ? TextDecoration.lineThrough
                          : null,
                    ),
                  ),
                  if (note != null)
                    Text(
                      note!,
                      style: const TextStyle(
                        fontSize: 11,
                        color: AppColors.neutralGray600,
                      ),
                    ),
                ],
              ),
            ),
            Text(
              isDiscount && total > 0
                  ? '-${AppFormatters.formatCurrency(total)}'
                  : AppFormatters.formatCurrency(total),
              style: TextStyle(
                fontSize: isTotal ? 18 : 14,
                fontWeight: isTotal ? FontWeight.bold : FontWeight.w600,
                color: isTotal
                    ? AppColors.primaryRed
                    : (isDiscount ? (color ?? AppColors.successGreen) : null),
              ),
            ),
          ],
        ),
      ],
    );
  }
}
