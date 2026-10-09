part of 'order_widgets_impl.dart';

class RefundStatusCard extends StatelessWidget {
  final RefundRequest refund;

  const RefundStatusCard({super.key, required this.refund});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return OrderSectionCard(
      margin: const EdgeInsets.only(bottom: core.AppMetrics.p12),
      borderColor: _getRefundStatusColor(
        context,
        colorScheme,
      ).withValues(alpha: 0.3),
      borderWidth: 1.5,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header with status badge (adaptive): title + badge share one row
          // when the single-line pair fits, else the title stacks over the
          // fully-readable badge. The title is compressible chrome
          // (ellipsis); the badge carries business meaning and wraps instead
          // of truncating. Same fit-measure as the F2 pricing rows.
          LayoutBuilder(
            builder: (BuildContext context, BoxConstraints constraints) {
              final TextStyle? titleStyle = theme.textTheme.titleSmall
                  ?.copyWith(fontWeight: FontWeight.w600);
              final TextStyle badgeStyle = context.typeRoles.labelMicro
                  .copyWith(fontWeight: FontWeight.w600);
              final String badgeText =
                  '${refund.status.emoji} ${refund.status.displayName}';
              final bool fits = _fitsOrderLabelValueSingleLine(
                context: context,
                maxWidth: constraints.maxWidth,
                label: 'Permintaan Pengembalian',
                value: badgeText,
                labelStyle: titleStyle,
                valueStyle: badgeStyle,
                fixedExtrasWidth:
                    AppIconSize.action +
                    8 +
                    8 +
                    core.AppMetrics.p8 * 2,
              );
              if (fits) {
                return Row(
                  children: [
                    Icon(
                      _getRefundStatusIcon(),
                      size: AppIconSize.action,
                      color: _getRefundStatusColor(context, colorScheme),
                    ),
                    const SizedBox(width: 8),
                    Text(
                      'Permintaan Pengembalian',
                      style: theme.textTheme.titleSmall?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const Spacer(),
                    _RefundStatusBadge(status: refund.status),
                  ],
                );
              }
              return Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Permintaan Pengembalian',
                    style: theme.textTheme.titleSmall?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 4),
                  _RefundStatusBadge(status: refund.status),
                ],
              );
            },
          ),
          const SizedBox(height: 12),
          // Reason
          _RefundInfoRow(
            label: 'Alasan',
            value: refund.reason.displayName,
            emoji: refund.reason.emoji,
          ),
          // Description
          if (refund.description != null && refund.description!.isNotEmpty) ...[
            const SizedBox(height: 8),
            _RefundInfoRow(label: 'Deskripsi', value: refund.description!),
          ],
          // Amount
          const SizedBox(height: 8),
          _RefundInfoRow(
            label: 'Jumlah',
            value: AppFormatters.formatCurrency(refund.refundAmount),
            isBold: true,
            valueColor: colorScheme.primary,
          ),
          // Date
          const SizedBox(height: 8),
          _RefundInfoRow(
            label: 'Tanggal',
            value: AppFormatters.formatDateTime(refund.createdAt),
          ),
          if (refund.evidenceUrls != null &&
              refund.evidenceUrls!.isNotEmpty) ...[
            const SizedBox(height: 12),
            EvidenceMediaGallery(urls: refund.evidenceUrls!),
          ],
          // Status-specific info
          if (refund.status == RefundStatus.pendingSellerReview)
            const _PendingReviewBanner(),
          if (refund.sellerNotes != null && refund.sellerNotes!.isNotEmpty) ...[
            const SizedBox(height: 8),
            _RefundNoteBanner(note: refund.sellerNotes!, role: 'Penjual'),
          ],
          if (refund.adminNotes != null && refund.adminNotes!.isNotEmpty) ...[
            const SizedBox(height: 8),
            _RefundNoteBanner(note: refund.adminNotes!, role: 'Admin'),
          ],
        ],
      ),
    );
  }

  Color _getRefundStatusColor(BuildContext context, ColorScheme colorScheme) {
    switch (refund.status) {
      case RefundStatus.pendingSellerReview:
        return context.statusColors.warning;
      case RefundStatus.sellerApproved:
      case RefundStatus.adminApproved:
        return context.statusColors.success;
      case RefundStatus.escalatedToAdmin:
        return colorScheme.secondary;
      case RefundStatus.sellerRejected:
      case RefundStatus.rejected:
        return context.statusColors.error;
      case RefundStatus.refunded:
        return context.statusColors.success;
    }
  }

  IconData _getRefundStatusIcon() {
    switch (refund.status) {
      case RefundStatus.pendingSellerReview:
        return Icons.hourglass_empty;
      case RefundStatus.sellerApproved:
      case RefundStatus.adminApproved:
        return Icons.check_circle_outline;
      case RefundStatus.escalatedToAdmin:
        return Icons.admin_panel_settings;
      case RefundStatus.sellerRejected:
      case RefundStatus.rejected:
        return Icons.cancel_outlined;
      case RefundStatus.refunded:
        return Icons.currency_exchange;
    }
  }
}

class _RefundInfoRow extends StatelessWidget {
  final String label;
  final String value;
  final String? emoji;
  final bool isBold;
  final Color? valueColor;

  const _RefundInfoRow({
    required this.label,
    required this.value,
    this.emoji,
    this.isBold = false,
    this.valueColor,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 80,
          child: Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(
              color: colorScheme.onSurfaceVariant,
            ),
          ),
        ),
        Expanded(
          child: Text(
            '${emoji ?? ''} $value'.trim(),
            style: theme.textTheme.bodySmall?.copyWith(
              color: valueColor ?? colorScheme.onSurface,
              fontWeight: isBold ? FontWeight.w600 : FontWeight.normal,
            ),
          ),
        ),
      ],
    );
  }
}

class _RefundStatusBadge extends StatelessWidget {
  final RefundStatus status;

  const _RefundStatusBadge({required this.status});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: core.AppMetrics.p8,
        vertical: core.AppMetrics.p4,
      ),
      decoration: BoxDecoration(
        color: _getBadgeColor(context, colorScheme).withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(core.AppShape.r8),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(status.emoji, style: context.typeRoles.labelMicro),
          const SizedBox(width: 4),
          // F5: wraps (never truncates) when an ancestor bounds this badge;
          // hugs intrinsic width otherwise. Business-meaningful status copy.
          Flexible(
            child: Text(
              status.displayName,
              style: context.typeRoles.labelMicro.copyWith(
                color: _getBadgeColor(context, colorScheme),
                fontWeight: FontWeight.w600,
              ),
              softWrap: true,
            ),
          ),
        ],
      ),
    );
  }

  Color _getBadgeColor(BuildContext context, ColorScheme colorScheme) {
    switch (status) {
      case RefundStatus.pendingSellerReview:
        return context.statusColors.warning;
      case RefundStatus.sellerApproved:
      case RefundStatus.adminApproved:
        return context.statusColors.success;
      case RefundStatus.escalatedToAdmin:
        return colorScheme.secondary;
      case RefundStatus.sellerRejected:
      case RefundStatus.rejected:
        return context.statusColors.error;
      case RefundStatus.refunded:
        return context.statusColors.success;
    }
  }
}

class _PendingReviewBanner extends StatelessWidget {
  const _PendingReviewBanner();

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      margin: const EdgeInsets.only(top: core.AppMetrics.p12),
      padding: const EdgeInsets.all(core.AppMetrics.p12),
      decoration: BoxDecoration(
        color: context.statusColors.warning.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(core.AppShape.r8),
      ),
      child: Row(
        children: [
          Icon(
            Icons.info_outline,
            color: context.statusColors.warning,
            size: AppIconSize.inlineGlyph,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              'Menunggu respon penjual',
              style: theme.textTheme.bodySmall?.copyWith(
                color: context.statusColors.warning,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _RefundNoteBanner extends StatelessWidget {
  final String note;
  final String role;

  const _RefundNoteBanner({required this.note, required this.role});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Container(
      padding: const EdgeInsets.all(core.AppMetrics.p12),
      decoration: BoxDecoration(
        color: colorScheme.surfaceContainerHigh,
        borderRadius: BorderRadius.circular(core.AppShape.r8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Catatan $role:',
            style: theme.textTheme.bodySmall?.copyWith(
              color: colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w500,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            note,
            style: theme.textTheme.bodySmall?.copyWith(
              color: colorScheme.onSurface,
            ),
          ),
        ],
      ),
    );
  }
}

// =============================================================================
// REMOVED: SellerActionButtons and BuyerActionButtons
// =============================================================================
// These classes have been REMOVED in favor of DynamicActionButtons
// which uses backend Decision V2 contract (primary_action, secondary_actions).
//
// The new approach:
// - Backend Decision V2 provides primary_action + secondary_actions
// - Mobile renders buttons dynamically from backend metadata
// - NO hardcoded business logic in UI
//
// See: lib/domains/commerce/transaction/order/presentation/widgets/dynamic_action_buttons.dart
// =============================================================================
