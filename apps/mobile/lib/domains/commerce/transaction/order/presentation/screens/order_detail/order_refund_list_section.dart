/// Order Refund List Section - Displays refund requests for an order
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/refund_request.dart';
import 'package:labuda/shared/utils/app_formatters.dart';
import 'dispute_escalation_dialog.dart';
import 'seller_refund_decision_dialog.dart';

/// F5-local fit measure: whether a single-line title + badge pair fits the
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

/// Section showing refund requests for an order
/// Displays as a collapsible list or banner depending on content
class OrderRefundListSection extends ConsumerWidget {
  final List<RefundRequest> refunds;
  final String? currentUserId;
  final String? sellerId;
  final VoidCallback? onActionComplete;

  const OrderRefundListSection({
    super.key,
    required this.refunds,
    this.currentUserId,
    this.sellerId,
    this.onActionComplete,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (refunds.isEmpty) {
      return const SizedBox.shrink();
    }

    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    final latestRefund = refunds.first; // Assuming sorted by date desc

    // Check if current user is the buyer
    final isBuyer =
        currentUserId != null && currentUserId == latestRefund.buyerId;

    // Check if buyer can escalate to dispute (H2-D1: seller rejected, not admin final)
    final canBuyerEscalate =
        isBuyer && latestRefund.status == RefundStatus.sellerRejected;

    // Check if current user is the seller and refund is pending their review (H2-D2)
    final isSeller =
        currentUserId != null && currentUserId == latestRefund.sellerId;
    final canSellerDecide =
        isSeller && latestRefund.status == RefundStatus.pendingSellerReview;

    return Container(
      margin: const EdgeInsets.only(bottom: core.AppMetrics.p16),
      padding: const EdgeInsets.all(core.AppMetrics.p16),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        borderRadius: BorderRadius.circular(core.AppShape.r12),
        border: Border.all(
          color: _getStatusColor(
            context,
            latestRefund.status,
            colorScheme,
          ).withValues(alpha: 0.3),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header (adaptive): title + badge share one row when the
          // single-line pair fits, else the title stacks over the
          // fully-readable badge. Same fit-measure as the F2 pricing rows.
          LayoutBuilder(
            builder: (BuildContext context, BoxConstraints constraints) {
              final TextStyle? titleStyle = theme.textTheme.titleSmall
                  ?.copyWith(fontWeight: FontWeight.w600);
              final TextStyle badgeStyle = context.typeRoles.labelMicro
                  .copyWith(fontWeight: FontWeight.w600);
              final String badgeText =
                  '${latestRefund.status.emoji} ${latestRefund.status.displayName}';
              final bool fits = _fitsTitleBadgeSingleLine(
                context: context,
                maxWidth: constraints.maxWidth,
                title: 'Permintaan Pengembalian',
                badge: badgeText,
                titleStyle: titleStyle,
                badgeStyle: badgeStyle,
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
                      Icons.currency_exchange,
                      size: AppIconSize.action,
                      color: _getStatusColor(
                        context,
                        latestRefund.status,
                        colorScheme,
                      ),
                    ),
                    const SizedBox(width: 8),
                    Text(
                      'Permintaan Pengembalian',
                      style: theme.textTheme.titleSmall?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const Spacer(),
                    _StatusBadge(status: latestRefund.status),
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
                  _StatusBadge(status: latestRefund.status),
                ],
              );
            },
          ),
          const SizedBox(height: 12),

          // Latest refund details
          _RefundDetailRow(
            label: 'Alasan',
            value: latestRefund.reason.displayName,
            emoji: latestRefund.reason.emoji,
          ),
          if (latestRefund.description != null &&
              latestRefund.description!.isNotEmpty) ...[
            const SizedBox(height: 8),
            _RefundDetailRow(
              label: 'Deskripsi',
              value: latestRefund.description!,
            ),
          ],
          const SizedBox(height: 8),
          _RefundDetailRow(
            label: 'Jumlah',
            value: AppFormatters.formatCurrency(latestRefund.refundAmount),
            isBold: true,
            valueColor: colorScheme.primary,
          ),
          const SizedBox(height: 8),
          _RefundDetailRow(
            label: 'Tanggal',
            value: AppFormatters.formatDateTime(latestRefund.createdAt),
          ),

          // Status-specific message
          const SizedBox(height: 12),
          _StatusMessageBanner(refund: latestRefund),

          // Buyer Escalation Button (when refund is rejected)
          if (canBuyerEscalate) ...[
            const SizedBox(height: 12),
            _BuyerEscalationButton(
              refund: latestRefund,
              onEscalate: () => DisputeEscalationDialog.show(
                context: context,
                orderId: latestRefund.orderId,
                refund: latestRefund,
                onEscalated: () {
                  onActionComplete?.call();
                },
              ),
            ),
          ],

          // Seller Approve/Reject Buttons (H2-D2: pendingSellerReview only)
          if (canSellerDecide) ...[
            const SizedBox(height: 12),
            _SellerDecisionButtons(
              refund: latestRefund,
              onApprove: () => SellerRefundDecisionDialog.showApprove(
                context: context,
                refund: latestRefund,
                onDecisionComplete: () => onActionComplete?.call(),
              ),
              onReject: () => SellerRefundDecisionDialog.showReject(
                context: context,
                refund: latestRefund,
                onDecisionComplete: () => onActionComplete?.call(),
              ),
            ),
          ],

          // Show "View All" if there are multiple refunds
          if (refunds.length > 1) ...[
            const SizedBox(height: 12),
            Center(
              child: Text(
                'Ada ${refunds.length} permintaan pengembalian untuk pesanan ini',
                style: theme.textTheme.bodySmall?.copyWith(
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

  Color _getStatusColor(
    BuildContext context,
    RefundStatus status,
    ColorScheme colorScheme,
  ) {
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

/// Buyer escalation button - shown when refund is rejected
class _BuyerEscalationButton extends StatelessWidget {
  final RefundRequest refund;
  final VoidCallback onEscalate;

  const _BuyerEscalationButton({
    required this.refund,
    required this.onEscalate,
  });

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: double.infinity,
      child: ElevatedButton.icon(
        onPressed: onEscalate,
        style: ElevatedButton.styleFrom(
          padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p12),
        ),
        icon: const Icon(Icons.gavel_rounded, size: AppIconSize.action),
        label: const Text('Ajukan ke Admin (Eskalasi)'),
      ),
    );
  }
}

/// Seller approve/reject buttons — shown when refund is pendingSellerReview (H2-D2)
class _SellerDecisionButtons extends StatelessWidget {
  final RefundRequest refund;
  final VoidCallback onApprove;
  final VoidCallback onReject;

  const _SellerDecisionButtons({
    required this.refund,
    required this.onApprove,
    required this.onReject,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: OutlinedButton.icon(
            onPressed: onReject,
            style: OutlinedButton.styleFrom(
              foregroundColor: context.statusColors.error,
              side: BorderSide(color: context.statusColors.error),
              padding: const EdgeInsets.symmetric(
                vertical: core.AppMetrics.p12,
              ),
            ),
            icon: const Icon(Icons.cancel_outlined, size: AppIconSize.action),
            label: const Text('Tolak'),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: ElevatedButton.icon(
            onPressed: onApprove,
            style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(
                vertical: core.AppMetrics.p12,
              ),
            ),
            icon: const Icon(
              Icons.check_circle_outline,
              size: AppIconSize.action,
            ),
            label: const Text('Setujui'),
          ),
        ),
      ],
    );
  }
}

/// Status badge for refund request
class _StatusBadge extends StatelessWidget {
  final RefundStatus status;

  const _StatusBadge({required this.status});

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

/// Detail row for refund information
class _RefundDetailRow extends StatelessWidget {
  final String label;
  final String value;
  final String? emoji;
  final bool isBold;
  final Color? valueColor;

  const _RefundDetailRow({
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

/// Status message banner based on refund status
class _StatusMessageBanner extends StatelessWidget {
  final RefundRequest refund;

  const _StatusMessageBanner({required this.refund});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    Color bgColor;
    Color textColor;
    IconData icon;
    String message;

    switch (refund.status) {
      case RefundStatus.pendingSellerReview:
        bgColor = context.statusColors.warning.withValues(alpha: 0.1);
        textColor = context.statusColors.warning;
        icon = Icons.hourglass_empty;
        message = 'Menunggu respon penjual';
        break;

      case RefundStatus.sellerApproved:
        bgColor = context.statusColors.success.withValues(alpha: 0.1);
        textColor = context.statusColors.success;
        icon = Icons.check_circle_outline;
        message = 'Disetujui oleh penjual';
        break;

      case RefundStatus.sellerRejected:
        bgColor = context.statusColors.error.withValues(alpha: 0.1);
        textColor = context.statusColors.error;
        icon = Icons.cancel_outlined;
        message = 'Ditolak penjual';
        break;

      case RefundStatus.escalatedToAdmin:
        bgColor = Theme.of(
          context,
        ).colorScheme.secondary.withValues(alpha: 0.1);
        textColor = Theme.of(context).colorScheme.secondary;
        icon = Icons.admin_panel_settings;
        message = 'Diteruskan ke admin';
        break;

      case RefundStatus.adminApproved:
        bgColor = context.statusColors.success.withValues(alpha: 0.1);
        textColor = context.statusColors.success;
        icon = Icons.verified;
        message = 'Disetujui oleh admin';
        break;

      case RefundStatus.rejected:
        bgColor = context.statusColors.error.withValues(alpha: 0.1);
        textColor = context.statusColors.error;
        icon = Icons.cancel_outlined;
        message = 'Permintaan ditolak penjual';
        break;

      case RefundStatus.refunded:
        bgColor = context.statusColors.success.withValues(alpha: 0.1);
        textColor = context.statusColors.success;
        icon = Icons.currency_exchange;
        message = 'Pengembalian diproses';
        break;
    }

    return Container(
      padding: const EdgeInsets.all(core.AppMetrics.p12),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(core.AppShape.r8),
      ),
      child: Row(
        children: [
          Icon(icon, color: textColor, size: AppIconSize.inlineGlyph),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              message,
              style: theme.textTheme.bodySmall?.copyWith(color: textColor),
            ),
          ),
        ],
      ),
    );
  }
}
