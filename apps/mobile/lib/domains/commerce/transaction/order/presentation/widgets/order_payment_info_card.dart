part of 'order_widgets_impl.dart';

class OrderPaymentInfoCard extends StatelessWidget {
  final Order order;

  const OrderPaymentInfoCard({super.key, required this.order});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return OrderSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // F5 header (adaptive): title + status badge share one row when the
          // single-line pair fits the incoming width, else the title stacks
          // over the fully-readable badge. The title is compressible chrome
          // (ellipsis); the badge carries business meaning and is never
          // truncated. Same fit-measure as the F2 pricing rows (same library).
          LayoutBuilder(
            builder: (BuildContext context, BoxConstraints constraints) {
              final String? badgeLabel = order.paymentStatus == null
                  ? null
                  : _PaymentStatusBadge.labelOf(order.paymentStatus!);
              final TextStyle? titleStyle = theme.textTheme.titleMedium
                  ?.copyWith(fontWeight: FontWeight.w600);
              final TextStyle badgeStyle = context.typeRoles.labelMicro
                  .copyWith(fontWeight: FontWeight.w600);
              final bool fits =
                  badgeLabel == null ||
                  _fitsOrderLabelValueSingleLine(
                    context: context,
                    maxWidth: constraints.maxWidth,
                    label: 'Info Pembayaran',
                    value: badgeLabel,
                    labelStyle: titleStyle,
                    valueStyle: badgeStyle,
                    fixedExtrasWidth:
                        AppIconSize.action +
                        8 +
                        8 +
                        core.AppMetrics.p12 * 2,
                  );
              if (badgeLabel == null) {
                // No status, no badge: title alone always owns a bounded
                // slot (a bare title can itself exceed narrow widths at
                // large text scales).
                return Row(
                  children: [
                    Icon(
                      Icons.payment_outlined,
                      size: AppIconSize.action,
                      color: _getPaymentStatusColor(context, colorScheme),
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Info Pembayaran',
                        style: theme.textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  ],
                );
              }
              if (fits) {
                return Row(
                  children: [
                    Icon(
                      Icons.payment_outlined,
                      size: AppIconSize.action,
                      color: _getPaymentStatusColor(context, colorScheme),
                    ),
                    const SizedBox(width: 8),
                    Text(
                      'Info Pembayaran',
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const Spacer(),
                    // No verdict on the payment row: claim no payment state
                    // at all.
                    if (order.paymentStatus != null)
                      _PaymentStatusBadge(
                        status: order.paymentStatus!,
                        colorScheme: colorScheme,
                      ),
                  ],
                );
              }
              return Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Info Pembayaran',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  if (order.paymentStatus != null) ...[
                    const SizedBox(height: 4),
                    _PaymentStatusBadge(
                      status: order.paymentStatus!,
                      colorScheme: colorScheme,
                    ),
                  ],
                ],
              );
            },
          ),
          const SizedBox(height: 16),
          // Payment method — canonical bound code (backend payment_method_code).
          _PaymentInfoRow(
            label: 'Metode',
            value: PaymentMethodVisuals.label(order.paymentMethodCode),
            leading: PaymentMethodLogo(
              visual: PaymentMethodVisuals.visual(order.paymentMethodCode),
              size: 18,
              maxWidth: 96,
            ),
          ),
          const SizedBox(height: 8),
          // Total amount
          _PaymentInfoRow(
            label: 'Total',
            value: order.pricing.totalPayableAmount != null
                ? AppFormatters.formatCurrency(
                    order.pricing.totalPayableAmount!,
                  )
                : '—',
            isBold: true,
            valueColor: colorScheme.primary,
          ),
          // Payment date (if paid)
          if (order.paidAt != null) ...[
            const SizedBox(height: 8),
            _PaymentInfoRow(
              label: 'Tanggal Bayar',
              value: AppFormatters.formatDateTime(order.paidAt!),
            ),
          ],
        ],
      ),
    );
  }

  Color _getPaymentStatusColor(BuildContext context, ColorScheme colorScheme) {
    switch (order.paymentStatus) {
      case null:
        return colorScheme.onSurfaceVariant;
      case PaymentStatus.paid:
        return context.statusColors.success;
      case PaymentStatus.pending:
        return context.statusColors.warning;
      case PaymentStatus.processing:
        return colorScheme.secondary;
      case PaymentStatus.failed:
        return context.statusColors.error;
      case PaymentStatus.expired:
        return colorScheme.onSurfaceVariant;
      case PaymentStatus.refunded:
        return colorScheme.secondary;
    }
  }

}

class _PaymentInfoRow extends StatelessWidget {
  final String label;
  final String value;
  final bool isBold;
  final Color? valueColor;
  final Widget? leading;

  const _PaymentInfoRow({
    required this.label,
    required this.value,
    this.isBold = false,
    this.valueColor,
    this.leading,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    final TextStyle? labelStyle = theme.textTheme.bodyMedium?.copyWith(
      color: colorScheme.onSurfaceVariant,
    );
    final TextStyle? valueStyle = theme.textTheme.bodyMedium?.copyWith(
      color: valueColor ?? colorScheme.onSurface,
      fontWeight: isBold ? FontWeight.w700 : FontWeight.w600,
    );

    // F2 canonical composition (see _PricingRow): horizontal when the
    // single-line content provably fits, vertical label-over-value otherwise.
    // The leading logo is bounded by its own maxWidth (96); its reserve is a
    // property of that visual, not a content width hack.
    const double gapWidth = 12;
    const double leadingGapWidth = 6;
    const double leadingReserveWidth = 96;
    final double fixedExtrasWidth =
        gapWidth + (leading != null ? leadingReserveWidth + leadingGapWidth : 0);

    return LayoutBuilder(
      builder: (BuildContext context, BoxConstraints constraints) {
        final bool fits = _fitsOrderLabelValueSingleLine(
          context: context,
          maxWidth: constraints.maxWidth,
          label: label,
          value: value,
          labelStyle: labelStyle,
          valueStyle: valueStyle,
          fixedExtrasWidth: fixedExtrasWidth,
        );
        if (fits) {
          return Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (leading != null) ...[
                leading!,
                const SizedBox(width: leadingGapWidth),
              ],
              Expanded(
                child: Text(
                  label,
                  style: labelStyle,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const SizedBox(width: gapWidth),
              Flexible(
                child: Text(
                  value,
                  style: valueStyle,
                  textAlign: TextAlign.end,
                  // NEVER ellipsis here: the Total value is monetary and must
                  // remain fully visible. softWrap is a layout backstop only.
                  softWrap: true,
                ),
              ),
            ],
          );
        }
        // Canonical vertical fallback: label (with logo) over value.
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                if (leading != null) ...[
                  leading!,
                  const SizedBox(width: leadingGapWidth),
                ],
                Flexible(
                  child: Text(label, style: labelStyle, softWrap: true),
                ),
              ],
            ),
            const SizedBox(height: 4),
            Text(value, style: valueStyle, softWrap: true),
          ],
        );
      },
    );
  }
}

class _PaymentStatusBadge extends StatelessWidget {
  final PaymentStatus status;
  final ColorScheme colorScheme;

  const _PaymentStatusBadge({required this.status, required this.colorScheme});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: core.AppMetrics.p12,
        vertical: core.AppMetrics.p4,
      ),
      decoration: BoxDecoration(
        color: _getBadgeColor(context).withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(core.AppShape.r12),
        border: Border.all(
          color: _getBadgeColor(context).withValues(alpha: 0.3),
        ),
      ),
      child: Text(
        _getBadgeLabel(),
        style: context.typeRoles.labelMicro.copyWith(
          color: _getBadgeColor(context),
          fontWeight: FontWeight.w600,
        ),
        softWrap: true,
      ),
    );
  }

  Color _getBadgeColor(BuildContext context) {
    switch (status) {
      case PaymentStatus.paid:
        return context.statusColors.success;
      case PaymentStatus.pending:
        return context.statusColors.warning;
      case PaymentStatus.processing:
        return colorScheme.secondary;
      case PaymentStatus.failed:
        return context.statusColors.error;
      case PaymentStatus.expired:
        return colorScheme.onSurfaceVariant;
      case PaymentStatus.refunded:
        return colorScheme.secondary;
    }
  }

  String _getBadgeLabel() => labelOf(status);

  /// Canonical badge copy for one payment status. Static so the F5 header
  /// fit-measure reads the same strings the badge renders (single source).
  static String labelOf(PaymentStatus status) {
    switch (status) {
      case PaymentStatus.paid:
        return 'LUNAS';
      case PaymentStatus.pending:
        return 'BELUM';
      case PaymentStatus.processing:
        return 'DIPROSES';
      case PaymentStatus.failed:
        return 'GAGAL';
      case PaymentStatus.expired:
        return 'KADALUARSA';
      case PaymentStatus.refunded:
        return 'DIKEMBALALIKAN';
    }
  }
}

// =============================================================================
// ORDER PRICING DISPLAY WIDGETS
// =============================================================================
// CLIENT UNFREEZE — PRICING ONLY (FINAL)
//
// ATURAN EMAS (WAJIB):
// ❌ Jangan hitung ulang apa pun di client
// ❌ Jangan infer fee dari field lain
// ❌ Jangan tampilkan serviceFeeAmount sebelum tersedia dari backend
// ✅ Semua angka = read-only dari backend
//
// Canonical pricing display (subtotal = P, shippingCost = S,
// serviceFeeAmount = F, totalPayableAmount = PD + S + F):
// - subtotal, shippingCost, serviceFeeAmount, totalPayableAmount
//
// The legacy fields (baseAmount, shippingFee, platformFee, discountAmount,
// coinDiscount, taxAmount, paymentFee) were never part of the canonical Order
// contract and were purged.
// =============================================================================
