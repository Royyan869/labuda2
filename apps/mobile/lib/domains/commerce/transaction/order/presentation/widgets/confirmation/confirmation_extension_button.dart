import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

/// Confirmation Extension Button (Consumer Widget)
///
/// Button for buyer to extend confirmation period by 3 days (1x only).
/// Only shows when confirmation is expiring soon (≤2 days remaining).
///
/// Features:
/// - Confirmation dialog before extending
/// - Loading state during extension
/// - Success/error snackbar feedback
/// - Disabled if already extended or expired
///
/// This is a "dumb" widget - only handles UI interactions and delegates
/// business logic to the controller.
class ConfirmationExtensionButton extends ConsumerStatefulWidget {
  final String orderId;
  final OrderConfirmation confirmation;
  final String buyerId;
  final Future<RepositoryResult<OrderConfirmation>> Function({
    required String orderId,
    required String buyerId,
  })
  extendConfirmation;

  const ConfirmationExtensionButton({
    super.key,
    required this.orderId,
    required this.confirmation,
    required this.buyerId,
    required this.extendConfirmation,
  });

  @override
  ConsumerState<ConfirmationExtensionButton> createState() =>
      _ConfirmationExtensionButtonState();
}

class _ConfirmationExtensionButtonState
    extends ConsumerState<ConfirmationExtensionButton> {
  bool _isLoading = false;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Don't show if extension not available
    if (!widget.confirmation.shouldShowExtensionButton()) {
      return const SizedBox.shrink();
    }

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: core.AppColors.koiOrange.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: core.AppColors.koiOrange.withValues(alpha: 0.3),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.add_alarm, color: core.AppColors.koiOrange, size: 18),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  'Perpanjang Waktu Pemeriksaan',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? core.AppColors.neutralGray200
                        : core.AppColors.neutralGray800,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            'Perpanjang waktu pemeriksaan 3 hari (hanya sekali)',
            style: TextStyle(
              fontSize: 11,
              color: isDark
                  ? core.AppColors.neutralGray400
                  : core.AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 12),
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: _isLoading || !widget.confirmation.canExtend
                  ? null
                  : () => _showExtensionConfirmation(),
              icon: _isLoading
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.add_alarm, size: 18),
              label: Text(
                widget.confirmation.extensionUsed
                    ? 'Sudah Diperpanjang'
                    : 'Perpanjang 3 Hari',
              ),
              style: OutlinedButton.styleFrom(
                foregroundColor: core.AppColors.koiOrange,
                side: BorderSide(color: core.AppColors.koiOrange),
                padding: const EdgeInsets.symmetric(vertical: 12),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _showExtensionConfirmation() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) =>
          _ExtensionConfirmationDialog(confirmation: widget.confirmation),
    );

    if (confirmed == true && mounted) {
      await _extendConfirmation();
    }
  }

  Future<void> _extendConfirmation() async {
    setState(() => _isLoading = true);

    try {
      final result = await widget.extendConfirmation(
        orderId: widget.confirmation.id,
        buyerId: widget.buyerId,
      );

      if (!mounted) return;

      setState(() => _isLoading = false);

      // Check result
      result.fold(
        (error) {
          AppSnackBar.showError(context, 'Gagal memperpanjang. Coba lagi.');
        },
        (_) {
          // Show success message
          AppSnackBar.showSuccess(
            context,
            'Waktu pemeriksaan berhasil diperpanjang 3 hari',
          );
        },
      );
    } catch (e) {
      if (!mounted) return;
      setState(() => _isLoading = false);
      AppSnackBar.showError(context, 'Terjadi kesalahan. Coba lagi.');
    }
  }
}

/// Extension Confirmation Dialog
class _ExtensionConfirmationDialog extends StatelessWidget {
  final OrderConfirmation confirmation;

  const _ExtensionConfirmationDialog({required this.confirmation});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final newEndDate = confirmation.originalEndDate.add(
      const Duration(days: 3),
    );

    return AlertDialog(
      backgroundColor: isDark
          ? core.AppColors.darkGray800
          : core.AppColors.neutralWhite,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      title: Row(
        children: [
          Icon(Icons.add_alarm, color: core.AppColors.koiOrange, size: 28),
          const SizedBox(width: 12),
          const Expanded(child: Text('Perpanjang Waktu Pemeriksaan?')),
        ],
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Waktu pemeriksaan akan diperpanjang 3 hari.',
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? core.AppColors.neutralGray300
                  : core.AppColors.neutralGray700,
            ),
          ),
          const SizedBox(height: 16),
          Container(
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(
              color: core.AppColors.primaryBlue.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Column(
              children: [
                _InfoRow(
                  label: 'Berakhir saat ini:',
                  value: _formatDate(confirmation.activeEndDate),
                  isDark: isDark,
                ),
                const SizedBox(height: 4),
                _InfoRow(
                  label: 'Berakhir setelah perpanjangan:',
                  value: _formatDate(newEndDate),
                  isDark: isDark,
                  isBold: true,
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
          Text(
            'Perpanjangan hanya bisa dilakukan sekali',
            style: TextStyle(
              fontSize: 11,
              fontStyle: FontStyle.italic,
              color: core.AppColors.koiOrange,
            ),
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(
            'Batal',
            style: TextStyle(
              color: isDark
                  ? core.AppColors.neutralGray400
                  : core.AppColors.neutralGray600,
            ),
          ),
        ),
        FilledButton(
          onPressed: () => Navigator.of(context).pop(true),
          style: FilledButton.styleFrom(
            backgroundColor: core.AppColors.koiOrange,
          ),
          child: const Text('Perpanjang'),
        ),
      ],
    );
  }

  String _formatDate(DateTime date) {
    const months = [
      'Jan',
      'Feb',
      'Mar',
      'Apr',
      'Mei',
      'Jun',
      'Jul',
      'Agu',
      'Sep',
      'Okt',
      'Nov',
      'Des',
    ];
    return '${date.day} ${months[date.month - 1]} ${date.year}, ${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}';
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  final bool isDark;
  final bool isBold;

  const _InfoRow({
    required this.label,
    required this.value,
    required this.isDark,
    this.isBold = false,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Expanded(
          child: Text(
            label,
            style: TextStyle(
              fontSize: 11,
              color: isDark
                  ? core.AppColors.neutralGray400
                  : core.AppColors.neutralGray600,
            ),
          ),
        ),
        const SizedBox(width: 8),
        Flexible(
          child: Text(
            value,
            style: TextStyle(
              fontSize: 11,
              fontWeight: isBold ? FontWeight.w700 : FontWeight.normal,
              color: isDark
                  ? core.AppColors.neutralGray200
                  : core.AppColors.neutralGray800,
            ),
            textAlign: TextAlign.right,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );
  }
}
