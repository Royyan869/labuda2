/// Dispute Escalation Dialog - Allows buyer to escalate a rejected refund to admin
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/transaction/order/data/order_providers.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/refund_request.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

/// Dialog for escalating a rejected refund to a dispute
class DisputeEscalationDialog extends ConsumerStatefulWidget {
  final String orderId;
  final RefundRequest refund;
  final VoidCallback? onEscalated;

  const DisputeEscalationDialog({
    super.key,
    required this.orderId,
    required this.refund,
    this.onEscalated,
  });

  @override
  ConsumerState<DisputeEscalationDialog> createState() =>
      _DisputeEscalationDialogState();

  /// Show the dispute escalation dialog
  static Future<void> show({
    required BuildContext context,
    required String orderId,
    required RefundRequest refund,
    VoidCallback? onEscalated,
  }) {
    return showDialog(
      context: context,
      builder: (ctx) => DisputeEscalationDialog(
        orderId: orderId,
        refund: refund,
        onEscalated: onEscalated,
      ),
    );
  }
}

class _DisputeEscalationDialogState
    extends ConsumerState<DisputeEscalationDialog> {
  bool _isSubmitting = false;

  Future<void> _submitEscalation() async {
    setState(() {
      _isSubmitting = true;
    });

    try {
      // Use canonical /refunds/:id/escalate endpoint (H2-D1)
      // Backend atomically transitions refund + creates linked dispute,
      // carrying forward all original evidence from the refund record.
      final datasource = ref.read(orderApiDatasourceProvider);
      await datasource.escalateRefund(widget.refund.id);

      if (mounted) {
        Navigator.of(context).pop();
        AppSnackBar.showSuccess(context, 'Sengketa berhasil diajukan ke admin');
        widget.onEscalated?.call();
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isSubmitting = false;
        });
        AppSnackBar.showError(
          context,
          'Gagal mengajukan sengketa: ${e.toString()}',
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final hasOriginalEvidence =
        widget.refund.evidenceUrls != null &&
        widget.refund.evidenceUrls!.isNotEmpty;

    return AlertDialog(
      title: Row(
        children: [
          Icon(
            Icons.gavel_rounded,
            color: core.AppColors.primaryBlue,
            size: 24,
          ),
          const SizedBox(width: 12),
          const Expanded(
            child: Text(
              'Ajukan Sengketa ke Admin',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.w600),
            ),
          ),
        ],
      ),
      content: SizedBox(
        width: double.maxFinite,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Info message
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: core.AppColors.primaryBlue.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(
                    color: core.AppColors.primaryBlue.withValues(alpha: 0.3),
                  ),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      Icons.info_outline_rounded,
                      color: core.AppColors.primaryBlue,
                      size: 18,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Penjual telah menolak refund Anda. Admin akan meninjau kasus ini secara adil.',
                        style: TextStyle(
                          fontSize: 12,
                          color: isDark ? Colors.white70 : Colors.black87,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 16),

              // Original refund info
              _InfoRow(
                label: 'Alasan Refund',
                value: widget.refund.reason.displayName,
                emoji: widget.refund.reason.emoji,
              ),
              const SizedBox(height: 8),
              _InfoRow(
                label: 'Ditolak Karena',
                value:
                    widget.refund.sellerNotes ??
                    'Tidak ada catatan dari penjual',
              ),
              if (hasOriginalEvidence) ...[
                const SizedBox(height: 8),
                _InfoRow(
                  label: 'Bukti Awal',
                  value: '${widget.refund.evidenceUrls!.length} file terlampir',
                ),
              ],
              const SizedBox(height: 16),

              // Warning about escrow freeze
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: core.AppColors.statusWarning.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      Icons.lock_clock,
                      color: core.AppColors.statusWarning,
                      size: 16,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Dana akan dibekukan (escrow freeze) selama proses peninjauan admin.',
                        style: TextStyle(
                          fontSize: 11,
                          color: core.AppColors.statusWarning,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _isSubmitting ? null : () => Navigator.of(context).pop(),
          child: const Text('Batal'),
        ),
        ElevatedButton(
          onPressed: _isSubmitting ? null : _submitEscalation,
          style: ElevatedButton.styleFrom(
            backgroundColor: core.AppColors.primaryBlue,
            foregroundColor: Colors.white,
            disabledBackgroundColor: Colors.grey,
          ),
          child: _isSubmitting
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: Colors.white,
                  ),
                )
              : const Text('Ajukan Sengketa'),
        ),
      ],
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  final String? emoji;

  const _InfoRow({required this.label, required this.value, this.emoji});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 100,
          child: Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(
              color: Colors.grey[600],
              fontSize: 12,
            ),
          ),
        ),
        Expanded(
          child: Text(
            '${emoji ?? ''} $value'.trim(),
            style: theme.textTheme.bodySmall?.copyWith(
              color: isDark ? Colors.white : Colors.black87,
              fontSize: 12,
            ),
          ),
        ),
      ],
    );
  }
}
