/// Dispute Escalation Dialog - Allows buyer to escalate a rejected refund to admin
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/domains/commerce/transaction/order/data/order_providers.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/refund_request.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/domains/commerce/transaction/order/presentation/widgets/evidence_media_gallery.dart';

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
      debugPrint('dispute.escalate failed: $e');
      if (mounted) {
        setState(() {
          _isSubmitting = false;
        });
        AppSnackBar.showError(context, 'Gagal mengajukan sengketa. Coba lagi.');
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    final hasOriginalEvidence =
        widget.refund.evidenceUrls != null &&
        widget.refund.evidenceUrls!.isNotEmpty;

    return AlertDialog(
      title: Row(
        children: [
          Icon(
            Icons.gavel_rounded,
            color: colorScheme.secondary,
            size: AppIconSize.header,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              'Ajukan Sengketa ke Admin',
              style: context.typeRoles.titleSection.copyWith(
                fontWeight: FontWeight.w600,
              ),
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
                padding: const EdgeInsets.all(core.AppMetrics.p12),
                decoration: BoxDecoration(
                  color: colorScheme.secondary.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(core.AppShape.r8),
                  border: Border.all(
                    color: colorScheme.secondary.withValues(alpha: 0.3),
                  ),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      Icons.info_outline_rounded,
                      color: colorScheme.secondary,
                      size: AppIconSize.action,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Penjual telah menolak refund Anda. Admin akan meninjau kasus ini secara adil.',
                        style: context.typeRoles.bodyDense.copyWith(
                          color: colorScheme.onSurface,
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
                EvidenceMediaGallery(urls: widget.refund.evidenceUrls!),
              ],
              const SizedBox(height: 16),

              // Warning about escrow freeze
              Container(
                padding: const EdgeInsets.all(core.AppMetrics.p12),
                decoration: BoxDecoration(
                  color: context.statusColors.warning.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(core.AppShape.r8),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      Icons.lock_clock,
                      color: context.statusColors.warning,
                      size: AppIconSize.inlineGlyph,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Dana akan dibekukan (escrow freeze) selama proses peninjauan admin.',
                        style: context.typeRoles.labelMicro.copyWith(
                          color: context.statusColors.warning,
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
          child: _isSubmitting
              ? SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: colorScheme.onSurfaceVariant,
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
    final colorScheme = theme.colorScheme;

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: core.AppContentSize.termLabel,
          child: Text(
            label,
            style: context.typeRoles.labelMicro.copyWith(
              color: colorScheme.onSurfaceVariant,
            ),
          ),
        ),
        Expanded(
          child: Text(
            '${emoji ?? ''} $value'.trim(),
            style: context.typeRoles.labelMicro.copyWith(
              color: colorScheme.onSurface,
            ),
          ),
        ),
      ],
    );
  }
}
