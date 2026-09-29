/// Seller Refund Decision Dialog - Approve or reject a buyer's refund request.
///
/// Business truth: seller makes a binary decision (approve/reject).
/// The refund amount is system-computed by policy — seller cannot choose amount.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/transaction/order/data/order_providers.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/refund_request.dart';
import 'package:labuda/shared/utils/app_formatters.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

enum _DecisionMode { approve, reject }

/// Dialog for seller to approve or reject a refund request.
class SellerRefundDecisionDialog extends ConsumerStatefulWidget {
  final RefundRequest refund;
  final _DecisionMode _mode;
  final VoidCallback? onDecisionComplete;

  const SellerRefundDecisionDialog._({
    required this.refund,
    required _DecisionMode mode,
    this.onDecisionComplete,
  }) : _mode = mode;

  @override
  ConsumerState<SellerRefundDecisionDialog> createState() =>
      _SellerRefundDecisionDialogState();

  /// Show the approve confirmation dialog.
  static Future<void> showApprove({
    required BuildContext context,
    required RefundRequest refund,
    VoidCallback? onDecisionComplete,
  }) {
    return showDialog(
      context: context,
      builder: (ctx) => SellerRefundDecisionDialog._(
        refund: refund,
        mode: _DecisionMode.approve,
        onDecisionComplete: onDecisionComplete,
      ),
    );
  }

  /// Show the reject dialog (requires notes).
  static Future<void> showReject({
    required BuildContext context,
    required RefundRequest refund,
    VoidCallback? onDecisionComplete,
  }) {
    return showDialog(
      context: context,
      builder: (ctx) => SellerRefundDecisionDialog._(
        refund: refund,
        mode: _DecisionMode.reject,
        onDecisionComplete: onDecisionComplete,
      ),
    );
  }
}

class _SellerRefundDecisionDialogState
    extends ConsumerState<SellerRefundDecisionDialog> {
  final _notesController = TextEditingController();
  bool _isSubmitting = false;

  bool get _isApprove => widget._mode == _DecisionMode.approve;

  @override
  void dispose() {
    _notesController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final notes = _notesController.text.trim();

    // Reject requires notes
    if (!_isApprove && notes.isEmpty) return;

    setState(() => _isSubmitting = true);

    try {
      final datasource = ref.read(orderApiDatasourceProvider);
      if (_isApprove) {
        await datasource.approveRefund(
          widget.refund.id,
          notes: notes.isNotEmpty ? notes : null,
        );
      } else {
        await datasource.rejectRefund(widget.refund.id, notes: notes);
      }

      if (mounted) {
        Navigator.of(context).pop();
        AppSnackBar.showSuccess(
          context,
          _isApprove ? 'Refund disetujui' : 'Refund ditolak',
        );
        widget.onDecisionComplete?.call();
      }
    } catch (e) {
      if (mounted) {
        setState(() => _isSubmitting = false);
        AppSnackBar.showError(
          context,
          _isApprove
              ? 'Gagal menyetujui refund: ${e.toString()}'
              : 'Gagal menolak refund: ${e.toString()}',
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return AlertDialog(
      title: Row(
        children: [
          Icon(
            _isApprove ? Icons.check_circle_outline : Icons.cancel_outlined,
            color: _isApprove
                ? context.statusColors.success
                : context.statusColors.error,
            size: 24,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              _isApprove ? 'Setujui Refund' : 'Tolak Refund',
              style: const TextStyle(fontSize: core.AppType.s18, fontWeight: FontWeight.w600),
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
              // Refund summary
              _SummaryRow(
                label: 'Alasan',
                value: widget.refund.reason.displayName,
                emoji: widget.refund.reason.emoji,
              ),
              const SizedBox(height: 8),
              _SummaryRow(
                label: 'Jumlah',
                value: AppFormatters.formatCurrency(widget.refund.refundAmount),
                isBold: true,
              ),

              const SizedBox(height: 16),

              // Info banner
              if (_isApprove) ...[
                _InfoBanner(
                  color: context.statusColors.success,
                  icon: Icons.info_outline_rounded,
                  message:
                      'Jumlah refund dihitung otomatis oleh sistem berdasarkan kebijakan yang berlaku. Anda tidak perlu menentukan jumlah.',
                ),
              ] else ...[
                _InfoBanner(
                  color: context.statusColors.error,
                  icon: Icons.warning_amber_rounded,
                  message:
                      'Pembeli dapat mengajukan sengketa ke admin setelah penolakan.',
                ),
              ],

              const SizedBox(height: 16),

              // Notes field
              Text(
                _isApprove ? 'Catatan (opsional)' : 'Alasan penolakan *',
                style: theme.textTheme.bodySmall?.copyWith(
                  fontWeight: FontWeight.w600,
                  color: colorScheme.onSurface,
                ),
              ),
              const SizedBox(height: 8),
              TextField(
                controller: _notesController,
                maxLines: 3,
                maxLength: 500,
                decoration: InputDecoration(
                  hintText: _isApprove
                      ? 'Tambahkan catatan...'
                      : 'Jelaskan alasan penolakan...',
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(core.AppShape.r8),
                  ),
                  contentPadding: const EdgeInsets.all(core.AppMetrics.p12),
                ),
                onChanged: (_) => setState(() {}),
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
          onPressed: _canSubmit ? _submit : null,
          style: ElevatedButton.styleFrom(
            backgroundColor: _isApprove
                ? context.statusColors.success
                : context.statusColors.error,
            disabledBackgroundColor: colorScheme.surfaceContainerHighest,
          ),
          child: _isSubmitting
              ? SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: colorScheme.onPrimary,
                  ),
                )
              : Text(_isApprove ? 'Setujui' : 'Tolak'),
        ),
      ],
    );
  }

  bool get _canSubmit {
    if (_isSubmitting) return false;
    if (!_isApprove && _notesController.text.trim().isEmpty) return false;
    return true;
  }
}

// =============================================================================
// Private helper widgets
// =============================================================================

class _SummaryRow extends StatelessWidget {
  final String label;
  final String value;
  final String? emoji;
  final bool isBold;

  const _SummaryRow({
    required this.label,
    required this.value,
    this.emoji,
    this.isBold = false,
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
              color: colorScheme.onSurface,
              fontWeight: isBold ? FontWeight.w600 : FontWeight.normal,
            ),
          ),
        ),
      ],
    );
  }
}

class _InfoBanner extends StatelessWidget {
  final Color color;
  final IconData icon;
  final String message;

  const _InfoBanner({
    required this.color,
    required this.icon,
    required this.message,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(core.AppMetrics.p12),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(core.AppShape.r8),
        border: Border.all(color: color.withValues(alpha: 0.3)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, color: color, size: 18),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              message,
              style: TextStyle(
                fontSize: core.AppType.s12,
                color: Theme.of(context).colorScheme.onSurface,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
