/// Order Refund List Section - Displays refund requests for an order
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/transaction/order/domain/entities/refund_request.dart';
import 'package:labuda/shared/utils/app_formatters.dart';
import 'dispute_escalation_dialog.dart';
import 'seller_refund_decision_dialog.dart';

/// Section showing refund requests for an order
/// Displays as a collapsible list or banner depending on content
class OrderRefundListSection extends ConsumerWidget {
  final List<RefundRequest> refunds;
  final bool isDark;
  final String? currentUserId;
  final String? sellerId;
  final VoidCallback? onActionComplete;

  const OrderRefundListSection({
    super.key,
    required this.refunds,
    required this.isDark,
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
      margin: const EdgeInsets.only(bottom: 16),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? const Color(0xFF1E1E1E) : Colors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: _getStatusColor(latestRefund.status).withValues(alpha: 0.3),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header
          Row(
            children: [
              Icon(
                Icons.currency_exchange,
                size: 20,
                color: _getStatusColor(latestRefund.status),
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
          ),
          const SizedBox(height: 12),

          // Latest refund details
          _RefundDetailRow(
            label: 'Alasan',
            value: latestRefund.reason.displayName,
            emoji: latestRefund.reason.emoji,
            isDark: isDark,
          ),
          if (latestRefund.description != null &&
              latestRefund.description!.isNotEmpty) ...[
            const SizedBox(height: 8),
            _RefundDetailRow(
              label: 'Deskripsi',
              value: latestRefund.description!,
              isDark: isDark,
            ),
          ],
          const SizedBox(height: 8),
          _RefundDetailRow(
            label: 'Jumlah',
            value: AppFormatters.formatCurrency(latestRefund.refundAmount),
            isDark: isDark,
            isBold: true,
            valueColor: core.AppColors.primaryRed,
          ),
          const SizedBox(height: 8),
          _RefundDetailRow(
            label: 'Tanggal',
            value: AppFormatters.formatDateTime(latestRefund.createdAt),
            isDark: isDark,
          ),

          // Status-specific message
          const SizedBox(height: 12),
          _StatusMessageBanner(refund: latestRefund, isDark: isDark),

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
                  color: Colors.grey[600],
                  fontStyle: FontStyle.italic,
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }

  Color _getStatusColor(RefundStatus status) {
    switch (status) {
      case RefundStatus.pendingSellerReview:
        return core.AppColors.statusWarning;
      case RefundStatus.sellerApproved:
      case RefundStatus.adminApproved:
        return core.AppColors.statusSuccess;
      case RefundStatus.escalatedToAdmin:
        return core.AppColors.primaryBlue;
      case RefundStatus.sellerRejected:
      case RefundStatus.rejected:
        return core.AppColors.statusError;
      case RefundStatus.refunded:
        return core.AppColors.primaryGreen;
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
          backgroundColor: core.AppColors.primaryBlue,
          foregroundColor: Colors.white,
          padding: const EdgeInsets.symmetric(vertical: 12),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        ),
        icon: const Icon(Icons.gavel_rounded, size: 18),
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
              foregroundColor: core.AppColors.statusError,
              side: const BorderSide(color: core.AppColors.statusError),
              padding: const EdgeInsets.symmetric(vertical: 12),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            icon: const Icon(Icons.cancel_outlined, size: 18),
            label: const Text('Tolak'),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: ElevatedButton.icon(
            onPressed: onApprove,
            style: ElevatedButton.styleFrom(
              backgroundColor: core.AppColors.statusSuccess,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 12),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            icon: const Icon(Icons.check_circle_outline, size: 18),
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
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: _getBadgeColor().withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(status.emoji, style: const TextStyle(fontSize: 12)),
          const SizedBox(width: 4),
          Text(
            status.displayName,
            style: theme.textTheme.bodySmall?.copyWith(
              color: _getBadgeColor(),
              fontWeight: FontWeight.w600,
              fontSize: 11,
            ),
          ),
        ],
      ),
    );
  }

  Color _getBadgeColor() {
    switch (status) {
      case RefundStatus.pendingSellerReview:
        return core.AppColors.statusWarning;
      case RefundStatus.sellerApproved:
      case RefundStatus.adminApproved:
        return core.AppColors.statusSuccess;
      case RefundStatus.escalatedToAdmin:
        return core.AppColors.primaryBlue;
      case RefundStatus.sellerRejected:
      case RefundStatus.rejected:
        return core.AppColors.statusError;
      case RefundStatus.refunded:
        return core.AppColors.primaryGreen;
    }
  }
}

/// Detail row for refund information
class _RefundDetailRow extends StatelessWidget {
  final String label;
  final String value;
  final String? emoji;
  final bool isDark;
  final bool isBold;
  final Color? valueColor;

  const _RefundDetailRow({
    required this.label,
    required this.value,
    this.emoji,
    required this.isDark,
    this.isBold = false,
    this.valueColor,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 80,
          child: Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(color: Colors.grey),
          ),
        ),
        Expanded(
          child: Text(
            '${emoji ?? ''} $value'.trim(),
            style: theme.textTheme.bodySmall?.copyWith(
              color: valueColor ?? (isDark ? Colors.white : Colors.black87),
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
  final bool isDark;

  const _StatusMessageBanner({required this.refund, required this.isDark});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    Color bgColor;
    Color textColor;
    IconData icon;
    String message;

    switch (refund.status) {
      case RefundStatus.pendingSellerReview:
        bgColor = core.AppColors.statusWarning.withValues(alpha: 0.1);
        textColor = core.AppColors.statusWarning;
        icon = Icons.hourglass_empty;
        message = 'Menunggu respon penjual';
        break;

      case RefundStatus.sellerApproved:
        bgColor = core.AppColors.statusSuccess.withValues(alpha: 0.1);
        textColor = core.AppColors.statusSuccess;
        icon = Icons.check_circle_outline;
        message = 'Disetujui oleh penjual';
        break;

      case RefundStatus.sellerRejected:
        bgColor = core.AppColors.statusError.withValues(alpha: 0.1);
        textColor = core.AppColors.statusError;
        icon = Icons.cancel_outlined;
        message = 'Ditolak penjual';
        break;

      case RefundStatus.escalatedToAdmin:
        bgColor = core.AppColors.primaryBlue.withValues(alpha: 0.1);
        textColor = core.AppColors.primaryBlue;
        icon = Icons.admin_panel_settings;
        message = 'Diteruskan ke admin';
        break;

      case RefundStatus.adminApproved:
        bgColor = core.AppColors.statusSuccess.withValues(alpha: 0.1);
        textColor = core.AppColors.statusSuccess;
        icon = Icons.verified;
        message = 'Disetujui oleh admin';
        break;

      case RefundStatus.rejected:
        bgColor = core.AppColors.statusError.withValues(alpha: 0.1);
        textColor = core.AppColors.statusError;
        icon = Icons.cancel_outlined;
        message = 'Permintaan ditolak penjual';
        break;

      case RefundStatus.refunded:
        bgColor = core.AppColors.primaryGreen.withValues(alpha: 0.1);
        textColor = core.AppColors.primaryGreen;
        icon = Icons.currency_exchange;
        message = 'Pengembalian diproses';
        break;
    }

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Icon(icon, color: textColor, size: 16),
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
