/// Pricing Breakdown Widget
///
/// **STRICT MODE - READ ONLY DISPLAY**
///
/// Displays pricing breakdown from backend pricing snapshot.
/// All values are sourced from backend - NO calculations in UI.
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/pricing/pricing_preview/domain/entities/pricing_snapshot.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/utils/currency_utils.dart';

/// Pricing Breakdown Widget
///
/// Displays complete pricing breakdown with:
/// - Item price (unit price × quantity)
/// - Shipping cost with context
/// - Discount (if applied)
/// - Coins (if applied)
/// - Savings display (total savings from backend)
/// - Negotiation visual (original vs negotiated price)
/// - Commission
/// - Total amount
///
/// **TRUST LABELS:**
/// - Shows "Negotiated price" for negotiation checkout
/// - Shows discount limits (max 50%)
/// - Shows coins limits (max 20%)
/// - Shows shipping context for live fish
class PricingBreakdown extends StatelessWidget {
  final PricingSnapshot snapshot;
  final bool isNegotiatedPrice;
  final bool showCommission;
  final bool showShippingContext;
  final String? negotiatedPriceLabel;

  const PricingBreakdown({
    super.key,
    required this.snapshot,
    this.isNegotiatedPrice = false,
    this.showCommission = false,
    this.showShippingContext = false,
    this.negotiatedPriceLabel,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.colorScheme.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: theme.colorScheme.outline.withValues(alpha: 0.3),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header
          Text(
            'Rincian Harga',
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 12),

          // Negotiation visual (original vs negotiated price)
          if (isNegotiatedPrice && snapshot.isNegotiated) ...[
            _buildNegotiationVisual(context),
            const SizedBox(height: 12),
          ],

          // Item price breakdown
          _buildPriceRow(
            context,
            'Harga Barang (${snapshot.quantity}x)',
            CurrencyUtils.formatInt(snapshot.unitPrice ~/ 100),
          ),

          // Subtotal
          _buildPriceRow(
            context,
            'Subtotal',
            CurrencyUtils.formatInt(snapshot.subtotal ~/ 100),
          ),

          // Shipping cost with context
          if (snapshot.shippingTotal > 0) ...[
            _buildPriceRow(
              context,
              'Ongkos Kirim',
              CurrencyUtils.formatInt(snapshot.shippingTotal ~/ 100),
            ),
            if (showShippingContext) _buildShippingContextLabel(context),
          ],

          // Discount
          if (snapshot.hasDiscount) ...[
            const SizedBox(height: 8),
            _buildDiscountRow(context),
          ],

          // Coins
          if (snapshot.hasCoins) ...[
            const SizedBox(height: 8),
            _buildCoinsRow(context),
          ],

          // Total Savings Display
          if (snapshot.totalSavings != null && snapshot.totalSavings! > 0) ...[
            const SizedBox(height: 8),
            _buildTotalSavingsRow(context),
            const SizedBox(height: 8),
          ],

          const Divider(height: 24),

          // Commission (optional, for seller view)
          if (showCommission)
            _buildPriceRow(
              context,
              'Biaya Layanan (${snapshot.commissionPercent.toStringAsFixed(0)}%)',
              CurrencyUtils.formatInt(snapshot.commissionAmount ~/ 100),
            ),

          // Total
          _buildTotalRow(context),
        ],
      ),
    );
  }

  Widget _buildPriceRow(BuildContext context, String label, String value) {
    final theme = Theme.of(context);

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            label,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          Text(
            value,
            style: theme.textTheme.bodyMedium?.copyWith(
              fontWeight: FontWeight.w500,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildNegotiationVisual(BuildContext context) {
    final theme = Theme.of(context);
    final originalPrice = snapshot.originalPrice ?? 0;
    final negotiatedPrice = snapshot.totalAmount;
    final savings = originalPrice - negotiatedPrice;

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: [
            theme.colorScheme.primaryContainer.withValues(alpha: 0.5),
            theme.colorScheme.primaryContainer.withValues(alpha: 0.2),
          ],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: theme.colorScheme.primary.withValues(alpha: 0.3),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.handshake_outlined,
                size: 20,
                color: theme.colorScheme.primary,
              ),
              const SizedBox(width: 8),
              Text(
                'Harga Negosiasi',
                style: theme.textTheme.titleSmall?.copyWith(
                  color: theme.colorScheme.primary,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Harga Awal',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    CurrencyUtils.formatInt(originalPrice ~/ 100),
                    style: theme.textTheme.bodyMedium?.copyWith(
                      decoration: TextDecoration.lineThrough,
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
              Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Text(
                    'Harga Deal',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.primary,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    CurrencyUtils.formatInt(negotiatedPrice ~/ 100),
                    style: theme.textTheme.titleLarge?.copyWith(
                      fontWeight: FontWeight.w700,
                      color: theme.colorScheme.primary,
                      fontSize: 20,
                    ),
                  ),
                ],
              ),
            ],
          ),
          if (savings > 0) ...[
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
              decoration: BoxDecoration(
                color: theme.colorScheme.primary.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(6),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    Icons.trending_down_outlined,
                    size: 14,
                    color: theme.colorScheme.primary,
                  ),
                  const SizedBox(width: 4),
                  Text(
                    'Hemat ${CurrencyUtils.formatInt(savings ~/ 100)} dari negosiasi',
                    style: theme.textTheme.labelSmall?.copyWith(
                      color: theme.colorScheme.primary,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildShippingContextLabel(BuildContext context) {
    final theme = Theme.of(context);

    return Padding(
      padding: const EdgeInsets.only(left: 0, top: 4),
      child: Row(
        children: [
          Icon(
            Icons.info_outline,
            size: 12,
            color: theme.colorScheme.onSurfaceVariant,
          ),
          const SizedBox(width: 4),
          Expanded(
            child: Text(
              'Ongkir ditentukan oleh seller (pengiriman ikan hidup)',
              style: theme.textTheme.labelSmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
                fontStyle: FontStyle.italic,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildDiscountRow(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: theme.colorScheme.primaryContainer.withValues(alpha: 0.3),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Row(
            children: [
              Icon(
                Icons.local_offer_outlined,
                size: 18,
                color: theme.colorScheme.primary,
              ),
              const SizedBox(width: 8),
              Text(
                snapshot.discountCode?.toUpperCase() ?? 'Diskon',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: theme.colorScheme.primary,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
          Text(
            '-${CurrencyUtils.formatInt(snapshot.discountAmount ~/ 100)}',
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.primary,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildCoinsRow(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: theme.colorScheme.tertiaryContainer.withValues(alpha: 0.3),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Row(
                children: [
                  Icon(
                    Icons.monetization_on_outlined,
                    size: 18,
                    color: theme.colorScheme.tertiary,
                  ),
                  const SizedBox(width: 8),
                  Text(
                    'Coins',
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: theme.colorScheme.tertiary,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ],
              ),
              Text(
                '-${CurrencyUtils.formatInt(snapshot.coinsAmount ~/ 100)}',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: theme.colorScheme.tertiary,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Row(
            children: [
              const SizedBox(width: 26),
              Expanded(
                child: Text(
                  'Coins digunakan sebagai potongan harga tambahan',
                  style: theme.textTheme.labelSmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildTotalSavingsRow(BuildContext context) {
    final theme = Theme.of(context);
    final savings = snapshot.totalSavings ?? 0;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: theme.colorScheme.secondaryContainer.withValues(alpha: 0.3),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: theme.colorScheme.secondary.withValues(alpha: 0.3),
        ),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Row(
            children: [
              Icon(
                Icons.local_offer_outlined,
                size: 18,
                color: theme.colorScheme.secondary,
              ),
              const SizedBox(width: 8),
              Text(
                'Total Potongan (diskon + coins)',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: theme.colorScheme.secondary,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
          Text(
            CurrencyUtils.formatInt(savings ~/ 100),
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.secondary,
              fontWeight: FontWeight.w700,
              fontSize: 16,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTotalRow(BuildContext context) {
    final theme = Theme.of(context);

    return Padding(
      padding: const EdgeInsets.only(top: 8),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            'Total Pembayaran',
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w700,
            ),
          ),
          Text(
            CurrencyUtils.formatInt(snapshot.totalAmount ~/ 100),
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w700,
              color: theme.colorScheme.primary,
              fontSize: 18,
            ),
          ),
        ],
      ),
    );
  }
}

/// Pricing Trust Labels Widget
///
/// Displays trust badges and information about pricing:
/// - "Negotiated price" badge for negotiation checkout
/// - "Max coins 20%" information
/// - "Discount max 50%" information
class PricingTrustLabels extends StatelessWidget {
  final bool isNegotiatedPrice;
  final bool showCoinsInfo;
  final bool showDiscountInfo;

  const PricingTrustLabels({
    super.key,
    this.isNegotiatedPrice = false,
    this.showCoinsInfo = true,
    this.showDiscountInfo = true,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Wrap(
      spacing: 8,
      runSpacing: 8,
      children: [
        if (isNegotiatedPrice)
          _buildTrustLabel(
            context,
            icon: Icons.handshake_outlined,
            label: 'Harga Nego',
            color: theme.colorScheme.primary,
          ),
        if (showCoinsInfo)
          _buildTrustLabel(
            context,
            icon: Icons.monetization_on_outlined,
            label: 'Max koin 20%',
            color: theme.colorScheme.tertiary,
          ),
        if (showDiscountInfo)
          _buildTrustLabel(
            context,
            icon: Icons.local_offer_outlined,
            label: 'Diskon max 50%',
            color: theme.colorScheme.secondary,
          ),
      ],
    );
  }

  Widget _buildTrustLabel(
    BuildContext context, {
    required IconData icon,
    required String label,
    required Color color,
  }) {
    final theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: color.withValues(alpha: 0.3)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 14, color: color),
          const SizedBox(width: 6),
          Text(
            label,
            style: theme.textTheme.labelSmall?.copyWith(
              color: color,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }
}

/// Token Expiry Widget
///
/// Displays pricing token expiry countdown with urgency indicators
class TokenExpiryWidget extends StatefulWidget {
  final DateTime expiresAt;
  final VoidCallback? onRefresh;

  const TokenExpiryWidget({super.key, required this.expiresAt, this.onRefresh});

  @override
  State<TokenExpiryWidget> createState() => _TokenExpiryWidgetState();
}

class _TokenExpiryWidgetState extends State<TokenExpiryWidget> {
  late DateTime _expiresAt;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _expiresAt = widget.expiresAt;
    _startTimer();
  }

  @override
  void didUpdateWidget(TokenExpiryWidget oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.expiresAt != oldWidget.expiresAt) {
      _expiresAt = widget.expiresAt;
    }
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  void _startTimer() {
    _timer = Timer.periodic(const Duration(seconds: 1), (timer) {
      if (mounted) {
        setState(() {});
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final now = DateTime.now();
    final remaining = _expiresAt.difference(now);

    final isExpired = remaining.isNegative;
    final isCritical = remaining.inMinutes < 2 && !isExpired;
    final isWarning = remaining.inMinutes < 5 && !isExpired && !isCritical;

    Color getColor() {
      if (isExpired) return theme.colorScheme.error;
      if (isCritical) return theme.colorScheme.error;
      if (isWarning) return theme.colorScheme.secondary;
      return theme.colorScheme.primary;
    }

    IconData getIcon() {
      if (isExpired) return Icons.error_outline;
      if (isCritical) return Icons.warning_amber_rounded;
      if (isWarning) return Icons.access_time_filled;
      return Icons.access_time_rounded;
    }

    String getMessage() {
      if (isExpired) return 'Waktu harga habis';
      if (isCritical) {
        final seconds = remaining.inSeconds % 60;
        return '$seconds detik lagi untuk mengunci harga ini';
      }
      if (isWarning) {
        final minutes = remaining.inMinutes;
        final seconds = remaining.inSeconds % 60;
        return 'Harga berlaku $minutes menit $seconds detik';
      }
      return 'Harga berlaku 10 menit';
    }

    String getSubMessage() {
      if (isExpired) return 'Silakan refresh harga terbaru';
      if (isCritical) return 'Selesaikan pesanan untuk mengunci harga ini';
      if (isWarning) return 'Harga mungkin berubah sewaktu-waktu';
      return 'Segarkan harga jika waktu habis';
    }

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: isCritical
              ? [
                  theme.colorScheme.errorContainer.withValues(alpha: 0.6),
                  theme.colorScheme.errorContainer.withValues(alpha: 0.3),
                ]
              : isWarning
              ? [
                  theme.colorScheme.secondaryContainer.withValues(alpha: 0.5),
                  theme.colorScheme.secondaryContainer.withValues(alpha: 0.2),
                ]
              : [
                  theme.colorScheme.primaryContainer.withValues(alpha: 0.4),
                  theme.colorScheme.primaryContainer.withValues(alpha: 0.1),
                ],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: getColor().withValues(alpha: isCritical ? 0.5 : 0.3),
          width: isCritical ? 1.5 : 1,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(getIcon(), size: isCritical ? 20 : 18, color: getColor()),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  getMessage(),
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: getColor(),
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
              if (widget.onRefresh != null)
                InkWell(
                  onTap: widget.onRefresh,
                  child: Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: getColor().withValues(alpha: 0.1),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Icon(Icons.refresh, size: 16, color: getColor()),
                  ),
                ),
            ],
          ),
          if (!isExpired) ...[
            const SizedBox(height: 4),
            Row(
              children: [
                const SizedBox(width: 28),
                Expanded(
                  child: Text(
                    getSubMessage(),
                    style: theme.textTheme.labelSmall?.copyWith(
                      color: getColor().withValues(alpha: 0.8),
                    ),
                  ),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}
