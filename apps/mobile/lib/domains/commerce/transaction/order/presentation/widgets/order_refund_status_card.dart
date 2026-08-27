part of 'order_widgets_impl.dart';

class RefundStatusCard extends StatelessWidget {
  final RefundRequest refund;

  const RefundStatusCard({super.key, required this.refund});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(16),
      margin: const EdgeInsets.only(bottom: 12),
      decoration: BoxDecoration(
        color: isDark ? const Color(0xFF1E1E1E) : Colors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: _getRefundStatusColor().withValues(alpha: 0.3),
          width: 1.5,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header with status badge
          Row(
            children: [
              Icon(
                _getRefundStatusIcon(),
                size: 20,
                color: _getRefundStatusColor(),
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
          ),
          const SizedBox(height: 12),
          // Reason
          _RefundInfoRow(
            label: 'Alasan',
            value: refund.reason.displayName,
            emoji: refund.reason.emoji,
            isDark: isDark,
          ),
          // Description
          if (refund.description != null && refund.description!.isNotEmpty) ...[
            const SizedBox(height: 8),
            _RefundInfoRow(
              label: 'Deskripsi',
              value: refund.description!,
              isDark: isDark,
            ),
          ],
          // Amount
          const SizedBox(height: 8),
          _RefundInfoRow(
            label: 'Jumlah',
            value: AppFormatters.formatCurrency(refund.refundAmount),
            isDark: isDark,
            isBold: true,
            valueColor: core.AppColors.primaryRed,
          ),
          // Date
          const SizedBox(height: 8),
          _RefundInfoRow(
            label: 'Tanggal',
            value: AppFormatters.formatDateTime(refund.createdAt),
            isDark: isDark,
          ),
          // Status-specific info
          if (refund.status == RefundStatus.pendingSellerReview)
            _PendingReviewBanner(isDark: isDark),
          if (refund.sellerNotes != null && refund.sellerNotes!.isNotEmpty) ...[
            const SizedBox(height: 8),
            _RefundNoteBanner(
              note: refund.sellerNotes!,
              role: 'Penjual',
              isDark: isDark,
            ),
          ],
          if (refund.adminNotes != null && refund.adminNotes!.isNotEmpty) ...[
            const SizedBox(height: 8),
            _RefundNoteBanner(
              note: refund.adminNotes!,
              role: 'Admin',
              isDark: isDark,
            ),
          ],
        ],
      ),
    );
  }

  Color _getRefundStatusColor() {
    switch (refund.status) {
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
  final bool isDark;
  final bool isBold;
  final Color? valueColor;

  const _RefundInfoRow({
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

class _RefundStatusBadge extends StatelessWidget {
  final RefundStatus status;

  const _RefundStatusBadge({required this.status});

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

class _PendingReviewBanner extends StatelessWidget {
  final bool isDark;

  const _PendingReviewBanner({required this.isDark});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      margin: const EdgeInsets.only(top: 12),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: core.AppColors.statusWarning.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Icon(
            Icons.info_outline,
            color: core.AppColors.statusWarning,
            size: 16,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              'Menunggu respon penjual',
              style: theme.textTheme.bodySmall?.copyWith(
                color: core.AppColors.statusWarning,
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
  final bool isDark;

  const _RefundNoteBanner({
    required this.note,
    required this.role,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isDark
            ? Colors.grey.shade800.withValues(alpha: 0.3)
            : Colors.grey.shade100,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Catatan $role:',
            style: theme.textTheme.bodySmall?.copyWith(
              color: Colors.grey,
              fontWeight: FontWeight.w500,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            note,
            style: theme.textTheme.bodySmall?.copyWith(
              color: isDark ? Colors.white : Colors.black87,
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
