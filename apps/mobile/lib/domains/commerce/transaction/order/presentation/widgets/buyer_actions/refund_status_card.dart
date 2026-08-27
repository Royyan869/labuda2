import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart' hide RefundRequest, RefundStatus;
import 'package:labuda/domains/commerce/transaction/order/domain/entities/refund_request.dart';
import 'package:labuda/shared/utils/app_formatters.dart';

/// Refund Status Card Widget
///
/// Displays refund status for buyer in order detail screen.
/// Shows timeline, amounts, and decision details.
class RefundStatusCard extends StatelessWidget {
  final RefundRequest refund;

  const RefundStatusCard({super.key, required this.refund});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      margin: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: _getStatusColor(refund.status), width: 2),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.05),
            blurRadius: 10,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header with status AND amount
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: _getStatusColor(refund.status).withValues(alpha: 0.1),
              borderRadius: const BorderRadius.only(
                topLeft: Radius.circular(10),
                topRight: Radius.circular(10),
              ),
            ),
            child: Row(
              children: [
                Icon(
                  _getStatusIcon(refund.status),
                  color: _getStatusColor(refund.status),
                  size: 24,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Refund Request',
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: isDark
                              ? AppColors.neutralGray400
                              : AppColors.neutralGray600,
                        ),
                      ),
                      Text(
                        refund.status.displayName,
                        style: TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.bold,
                          color: _getStatusColor(refund.status),
                        ),
                      ),
                    ],
                  ),
                ),
                // REFUND AMOUNT DISPLAY - Critical for trust
                Column(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    Text(
                      'Requested',
                      style: TextStyle(
                        fontSize: 11,
                        color: isDark
                            ? AppColors.neutralGray500
                            : AppColors.neutralGray600,
                      ),
                    ),
                    Text(
                      AppFormatters.formatCurrency(refund.refundAmount),
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                        color: _getStatusColor(refund.status),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),

          // Content
          Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Timeline
                _buildTimelineSection(isDark),

                // Decision Details (if available)
                if (refund.isSellerApproved ||
                    refund.isAdminApproved ||
                    refund.isRejected) ...[
                  const SizedBox(height: 16),
                  _buildDecisionSection(isDark),
                ],

                // Refunded Notice
                if (refund.isRefunded) ...[
                  const SizedBox(height: 16),
                  _buildRefundedNotice(isDark),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTimelineSection(bool isDark) {
    // Determine which item is the last one
    final isLastAdminDecision =
        (refund.isAdminApproved || refund.isRejected) && !refund.isRefunded;
    final isLastSellerDecision =
        (refund.isSellerApproved || refund.isEscalatedToAdmin) &&
        !refund.isAdminApproved &&
        !refund.isRejected &&
        !refund.isRefunded;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Timeline',
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 12),
        _buildTimelineItem(
          title: 'Refund Submitted',
          subtitle: refund.reason.displayName,
          timestamp: refund.createdAt,
          completed: true,
          isDark: isDark,
          isLast: false,
        ),
        if (refund.isSellerApproved || refund.isEscalatedToAdmin)
          _buildTimelineItem(
            title: refund.isSellerApproved
                ? 'Approved by Seller'
                : 'Rejected by Seller',
            subtitle: refund.sellerNotes ?? 'No notes',
            timestamp: refund.sellerReviewedAt,
            completed: true,
            isDark: isDark,
            isLast: isLastSellerDecision,
          ),
        if (refund.isAdminApproved || refund.isRejected)
          _buildTimelineItem(
            title: refund.isAdminApproved
                ? 'Approved by Admin'
                : 'Rejected by Admin (Final)',
            subtitle: refund.adminNotes ?? 'No notes',
            timestamp: refund.adminReviewedAt,
            completed: true,
            isDark: isDark,
            isLast: isLastAdminDecision,
          ),
        if (refund.isRefunded)
          _buildTimelineItem(
            title: 'Refund Processed',
            subtitle: 'Dana sedang dikembalikan ke metode pembayaran Anda',
            timestamp: refund.refundedAt,
            completed: true,
            isDark: isDark,
            isLast: true,
          ),
        if (refund.isPendingSellerReview)
          _buildTimelineItem(
            title: 'Awaiting Seller Review',
            subtitle: 'Seller is reviewing your refund request',
            timestamp: null,
            completed: false,
            isDark: isDark,
            isLast: true,
          ),
        if (refund.isEscalatedToAdmin &&
            !refund.isAdminApproved &&
            !refund.isRejected)
          _buildTimelineItem(
            title: 'Awaiting Admin Review',
            subtitle: 'Admin is reviewing the escalated refund',
            timestamp: null,
            completed: false,
            isDark: isDark,
            isLast: true,
          ),
      ],
    );
  }

  Widget _buildTimelineItem({
    required String title,
    required String subtitle,
    required DateTime? timestamp,
    required bool completed,
    required bool isDark,
    bool isLast = false,
  }) {
    // Choose icon based on completion status
    final icon = completed ? Icons.check_circle : Icons.hourglass_empty;

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Column(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: completed
                    ? AppColors.statusSuccess
                    : (isDark
                          ? AppColors.darkGray700
                          : AppColors.neutralGray200),
                shape: BoxShape.circle,
              ),
              child: Icon(
                icon,
                size: 20,
                color: completed
                    ? Colors.white
                    : (isDark
                          ? AppColors.neutralGray500
                          : AppColors.neutralGray400),
              ),
            ),
            if (!isLast)
              Container(
                width: 2,
                height: 40,
                color: completed
                    ? AppColors.statusSuccess
                    : (isDark
                          ? AppColors.darkGray700
                          : AppColors.neutralGray200),
              ),
          ],
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: completed
                      ? (isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900)
                      : (isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray600),
                ),
              ),
              const SizedBox(height: 2),
              Text(
                subtitle,
                style: TextStyle(
                  fontSize: 12,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
              if (timestamp != null) ...[
                const SizedBox(height: 4),
                Text(
                  _formatDate(timestamp),
                  style: TextStyle(
                    fontSize: 11,
                    color: isDark
                        ? AppColors.neutralGray500
                        : AppColors.neutralGray500,
                  ),
                ),
              ],
              if (!isLast) const SizedBox(height: 16),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildDecisionSection(bool isDark) {
    if (refund.isSellerApproved) {
      final approvedAmount =
          refund.sellerApprovedAmount ?? refund.finalRefundAmount;
      return Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: AppColors.primaryGreen.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: AppColors.primaryGreen.withValues(alpha: 0.3),
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(
                  Icons.check_circle,
                  size: 16,
                  color: AppColors.primaryGreen,
                ),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    'Seller Approved${refund.sellerApprovedPercent != null ? " ${refund.sellerApprovedPercent}%" : ""}',
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: AppColors.primaryGreen,
                    ),
                  ),
                ),
                if (approvedAmount != null)
                  Text(
                    AppFormatters.formatCurrency(approvedAmount),
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.bold,
                      color: AppColors.primaryGreen,
                    ),
                  ),
              ],
            ),
            if (refund.sellerNotes != null) ...[
              const SizedBox(height: 6),
              Text(
                refund.sellerNotes!,
                style: TextStyle(
                  fontSize: 12,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray700,
                ),
              ),
            ],
          ],
        ),
      );
    }

    if (refund.isAdminApproved) {
      final approvedAmount =
          refund.adminApprovedAmount ?? refund.finalRefundAmount;
      return Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: AppColors.primaryGreen.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: AppColors.primaryGreen.withValues(alpha: 0.3),
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(
                  Icons.admin_panel_settings,
                  size: 16,
                  color: AppColors.primaryGreen,
                ),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    'Admin Approved${refund.adminApprovedPercent != null ? " ${refund.adminApprovedPercent}%" : ""} (Final)',
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: AppColors.primaryGreen,
                    ),
                  ),
                ),
                if (approvedAmount != null)
                  Text(
                    AppFormatters.formatCurrency(approvedAmount),
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.bold,
                      color: AppColors.primaryGreen,
                    ),
                  ),
              ],
            ),
            if (refund.adminNotes != null) ...[
              const SizedBox(height: 6),
              Text(
                refund.adminNotes!,
                style: TextStyle(
                  fontSize: 12,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray700,
                ),
              ),
            ],
          ],
        ),
      );
    }

    if (refund.isRejected) {
      return Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: AppColors.statusError.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: AppColors.statusError.withValues(alpha: 0.3),
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.block, size: 16, color: AppColors.statusError),
                const SizedBox(width: 6),
                Text(
                  'Admin Rejected (Final)',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: AppColors.statusError,
                  ),
                ),
              ],
            ),
            if (refund.adminNotes != null) ...[
              const SizedBox(height: 6),
              Text(
                refund.adminNotes!,
                style: TextStyle(
                  fontSize: 12,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray700,
                ),
              ),
            ],
            const SizedBox(height: 8),
            Text(
              'You will not receive a refund. This decision is final.',
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: AppColors.statusError,
              ),
            ),
          ],
        ),
      );
    }

    return const SizedBox.shrink();
  }

  Widget _buildRefundedNotice(bool isDark) {
    final finalAmount =
        refund.finalRefundAmount ??
        refund.sellerApprovedAmount ??
        refund.adminApprovedAmount ??
        refund.refundAmount;

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.statusSuccess.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: AppColors.statusSuccess.withValues(alpha: 0.3),
        ),
      ),
      child: Row(
        children: [
          Icon(Icons.check_circle, color: AppColors.statusSuccess, size: 20),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                      'Funds Returned',
                      style: TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                        color: AppColors.statusSuccess,
                      ),
                    ),
                    Text(
                      AppFormatters.formatCurrency(finalAmount),
                      style: TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.bold,
                        color: AppColors.statusSuccess,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  'Dana sedang diproses pengembaliannya ke metode pembayaran Anda',
                  style: TextStyle(
                    fontSize: 12,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray700,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Color _getStatusColor(RefundStatus status) {
    switch (status) {
      case RefundStatus.pendingSellerReview:
        return AppColors.statusWarning;
      case RefundStatus.sellerApproved:
        return AppColors.primaryGreen;
      case RefundStatus.escalatedToAdmin:
        return AppColors.statusInfo;
      case RefundStatus.adminApproved:
        return AppColors.primaryGreen;
      case RefundStatus.sellerRejected:
      case RefundStatus.rejected:
        return AppColors.statusError;
      case RefundStatus.refunded:
        return AppColors.statusSuccess;
    }
  }

  IconData _getStatusIcon(RefundStatus status) {
    switch (status) {
      case RefundStatus.pendingSellerReview:
        return Icons.hourglass_empty;
      case RefundStatus.sellerApproved:
        return Icons.check_circle;
      case RefundStatus.escalatedToAdmin:
        return Icons.admin_panel_settings;
      case RefundStatus.adminApproved:
        return Icons.verified;
      case RefundStatus.sellerRejected:
      case RefundStatus.rejected:
        return Icons.cancel;
      case RefundStatus.refunded:
        return Icons.payments;
    }
  }

  String _formatDate(DateTime date) {
    final now = DateTime.now();
    final diff = now.difference(date);

    if (diff.inMinutes < 1) return 'Just now';
    if (diff.inHours < 1) return '${diff.inMinutes} minutes ago';
    if (diff.inDays < 1) return '${diff.inHours} hours ago';
    if (diff.inDays < 7) return '${diff.inDays} days ago';

    return '${date.day}/${date.month}/${date.year} ${date.hour}:${date.minute.toString().padLeft(2, '0')}';
  }
}
