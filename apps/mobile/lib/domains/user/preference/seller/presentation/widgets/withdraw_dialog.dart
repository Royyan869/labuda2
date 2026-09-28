/// Withdraw Dialog
///
/// Dialog for requesting a withdrawal.
/// Checks seller verification status before allowing withdrawal.
library;

import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/withdraw_notifier.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/withdraw_state.dart';
import 'package:labuda/shared/utils/app_formatters.dart';
import 'package:labuda/domains/user/identity/verification/verification.dart';

bool hasPayoutAuthority(SellerVerificationV2State verificationState) =>
    verificationState.isVerified;

@visibleForTesting
bool hasBackendWithdrawalFee(double withdrawalFeeAmount) =>
    withdrawalFeeAmount > 0;

@visibleForTesting
double normalizeBackendWithdrawalFee(double withdrawalFeeAmount) =>
    hasBackendWithdrawalFee(withdrawalFeeAmount) ? withdrawalFeeAmount : 0;

class WithdrawDialog extends ConsumerStatefulWidget {
  final double availableBalance;
  final double withdrawalFeeAmount;

  const WithdrawDialog({
    super.key,
    required this.availableBalance,
    required this.withdrawalFeeAmount,
  });

  @override
  ConsumerState<WithdrawDialog> createState() => _WithdrawDialogState();
}

class _WithdrawDialogState extends ConsumerState<WithdrawDialog> {
  final TextEditingController _amountController = TextEditingController();
  final FocusNode _amountFocusNode = FocusNode();
  bool _isValid = false;
  String? _errorMessage;

  // Withdrawal limits (must match backend)
  static const double minWithdrawAmount = 10000;
  static const double maxWithdrawAmount = 50000000;

  @override
  void initState() {
    super.initState();
    // Pre-fill with the full available balance. The entered amount IS the
    // requested withdrawal amount debited from the balance (PASS_18H) — the
    // fee is deducted from it at settlement, not added on top, so no
    // reservation is needed here.
    final prefillAmount = math.max(widget.availableBalance, minWithdrawAmount);
    _amountController.text = prefillAmount.toStringAsFixed(0);
    _amountFocusNode.requestFocus();
    _amountController.addListener(_validateAmount);
  }

  @override
  void dispose() {
    _amountController.dispose();
    _amountFocusNode.dispose();
    super.dispose();
  }

  void _validateAmount() {
    final text = _amountController.text.trim();
    // Digits-only policy — reject separator input without stripping.
    // Stripping '.' before parse converts '1.5' → '15' (100x smaller).
    if (text.isNotEmpty && (text.contains('.') || text.contains(','))) {
      setState(() {
        _isValid = false;
        _errorMessage = 'Jumlah harus bilangan bulat tanpa koma atau titik.';
      });
      return;
    }
    final amount = int.tryParse(text);
    // PASS_18H: the entered amount is the full requested withdrawal amount
    // and is debited from the balance as-is — the fee is deducted from it
    // at settlement, not added on top.
    final totalDebit = (amount ?? 0).toDouble();

    setState(() {
      if (text.isEmpty || amount == null) {
        _isValid = false;
        _errorMessage = null;
      } else if (amount < minWithdrawAmount) {
        _isValid = false;
        _errorMessage =
            'Minimum Rp ${formatGroupedAmount(minWithdrawAmount.round())}';
      } else if (totalDebit > widget.availableBalance) {
        _isValid = false;
        _errorMessage = 'Exceeds available balance';
      } else if (amount > maxWithdrawAmount) {
        _isValid = false;
        _errorMessage =
            'Maximum Rp ${formatGroupedAmount(maxWithdrawAmount.round())}';
      } else {
        _isValid = true;
        _errorMessage = null;
      }
    });
  }

  Future<void> _handleWithdraw() async {
    if (!_isValid) return;

    final text = _amountController.text.trim();
    final amount = int.tryParse(text) ?? 0;

    // Close dialog on success
    final success = await ref
        .read(withdrawNotifierProvider.notifier)
        .requestWithdraw(amount.toDouble());

    if (success && mounted) {
      Navigator.of(context).pop(true); // Return true to indicate success
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final withdrawState = ref.watch(withdrawNotifierProvider);
    final verificationState = ref.watch(sellerVerificationV2NotifierProvider);
    final enteredAmount =
        int.tryParse(_amountController.text.trim())?.toDouble() ?? 0.0;
    final withdrawalFeeAmount = normalizeBackendWithdrawalFee(
      widget.withdrawalFeeAmount,
    );
    final showWithdrawalFee = hasBackendWithdrawalFee(withdrawalFeeAmount);
    // PASS_18H money model: requested amount is debited in full; the fee is
    // deducted FROM it (never added on top) to produce the net payout.
    final totalDebitAmount = enteredAmount;
    final netReceivedAmount = math.max(
      enteredAmount - withdrawalFeeAmount,
      0.0,
    );

    // Verification approval is required for payout authority.
    if (!hasPayoutAuthority(verificationState)) {
      return _buildVerificationRequiredDialog(context, verificationState);
    }

    return AlertDialog(
      title: Row(
        children: [
          Icon(Icons.account_balance_wallet, color: scheme.primary),
          const SizedBox(width: 12),
          const Text('Withdraw Funds'),
        ],
      ),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Available Balance
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: scheme.surfaceContainer,
                borderRadius: BorderRadius.circular(AppShape.r8),
              ),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    'Available Balance',
                    style: TextStyle(
                      fontSize: AppType.s14,
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                  Text(
                    AppFormatters.formatCurrency(widget.availableBalance),
                    style: TextStyle(
                      fontSize: AppType.s16,
                      fontWeight: FontWeight.bold,
                      color: scheme.primary,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 20),

            // Amount Input
            Text(
              'Withdrawal Amount',
              style: TextStyle(
                fontSize: AppType.s14,
                fontWeight: FontWeight.w500,
                color: scheme.onSurface,
              ),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _amountController,
              focusNode: _amountFocusNode,
              enabled: withdrawState is! WithdrawProcessing,
              decoration: InputDecoration(
                labelText: 'Amount',
                prefixText: 'Rp ',
                border: const OutlineInputBorder(),
                errorText: _errorMessage,
                suffixIcon: _amountController.text.isNotEmpty
                    ? IconButton(
                        icon: const Icon(Icons.clear),
                        onPressed: () {
                          _amountController.clear();
                        },
                      )
                    : null,
              ),
              keyboardType: const TextInputType.numberWithOptions(
                decimal: false,
              ),
            ),
            const SizedBox(height: 12),

            // Fee breakdown
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: scheme.surfaceContainer,
                borderRadius: BorderRadius.circular(AppShape.r8),
                border: Border.all(color: scheme.outlineVariant, width: 1),
              ),
              child: Column(
                children: [
                  _buildSummaryRow(
                    'Jumlah withdrawal',
                    AppFormatters.formatCurrency(enteredAmount),
                  ),
                  if (showWithdrawalFee) ...[
                    const SizedBox(height: 8),
                    _buildSummaryRow(
                      'Biaya penarikan',
                      AppFormatters.formatCurrency(withdrawalFeeAmount),
                    ),
                    const SizedBox(height: 8),
                    _buildSummaryRow(
                      'Jumlah diterima',
                      AppFormatters.formatCurrency(netReceivedAmount),
                    ),
                  ],
                  const SizedBox(height: 8),
                  _buildSummaryRow(
                    'Total dipotong dari saldo',
                    AppFormatters.formatCurrency(totalDebitAmount),
                  ),
                ],
              ),
            ),

            const SizedBox(height: 10),

            if (showWithdrawalFee)
              Text(
                'Biaya penarikan ${AppFormatters.formatCurrency(withdrawalFeeAmount)} dikenakan setiap penarikan.',
                style: TextStyle(fontSize: AppType.s12, color: scheme.onSurfaceVariant),
              ),

            // Quick Amount Buttons
            Wrap(spacing: 8, children: _buildQuickAmountButtons()),

            const SizedBox(height: 16),

            // Payout Processing Disclosure (TRUTHFUL)
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: scheme.surfaceContainer,
                borderRadius: BorderRadius.circular(AppShape.r8),
                border: Border.all(color: scheme.outlineVariant, width: 1),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        Icons.info_outline,
                        size: 16,
                        color: scheme.onSurfaceVariant,
                      ),
                      const SizedBox(width: 8),
                      Text(
                        'Pencairan Dana',
                        style: TextStyle(
                          fontSize: AppType.s13,
                          fontWeight: FontWeight.w500,
                          color: scheme.onSurface,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Permintaan pencairan akan diproses secara manual. Dana akan ditransfer ke rekening terdaftar dalam 1-3 hari kerja setelah disetujui.',
                    style: TextStyle(
                      fontSize: AppType.s11,
                      color: scheme.onSurfaceVariant,
                      height: 1.4,
                    ),
                  ),
                ],
              ),
            ),

            const SizedBox(height: 12),

            // Minimum Withdrawal Info
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: scheme.primary.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(AppShape.r8),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.info_outline,
                    size: 16,
                    color: scheme.primary,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Minimum pencairan: Rp ${formatGroupedAmount(minWithdrawAmount.round())}',
                      style: TextStyle(
                        fontSize: AppType.s12,
                        color: scheme.primary,
                      ),
                    ),
                  ),
                ],
              ),
            ),

            // Processing indicator
            if (withdrawState is WithdrawProcessing) ...[
              const SizedBox(height: 16),
              const Row(
                children: [
                  SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
                  SizedBox(width: 12),
                  Text('Processing withdrawal...'),
                ],
              ),
            ],

            // Success message (TRUTHFUL - indicates manual processing)
            if (withdrawState is WithdrawSuccess) ...[
              const SizedBox(height: 16),
              Container(
                padding: const EdgeInsets.all(AppMetrics.p12),
                decoration: BoxDecoration(
                  color: context.statusColors.success.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(AppShape.r8),
                ),
                child: Row(
                  children: [
                    Icon(
                      Icons.check_circle,
                      color: context.statusColors.success,
                      size: 20,
                    ),
                    SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Permintaan pencairan berhasil dikirim. Menunggu proses verifikasi.',
                        style: TextStyle(
                          fontSize: AppType.s13,
                          color: context.statusColors.success,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],

            // Error message
            if (withdrawState case WithdrawError(:final message)) ...[
              const SizedBox(height: 16),
              Container(
                padding: const EdgeInsets.all(AppMetrics.p12),
                decoration: BoxDecoration(
                  color: context.statusColors.error.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(AppShape.r8),
                ),
                child: Row(
                  children: [
                    Icon(
                      Icons.error_outline,
                      color: context.statusColors.error,
                      size: 20,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        message,
                        style: TextStyle(
                          fontSize: AppType.s13,
                          color: context.statusColors.error,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: withdrawState is WithdrawProcessing
              ? null
              : () {
                  ref.read(withdrawNotifierProvider.notifier).reset();
                  Navigator.of(context).pop();
                },
          child: const Text('Cancel'),
        ),
        ElevatedButton(
          onPressed: withdrawState is WithdrawProcessing || !_isValid
              ? null
              : _handleWithdraw,
          style: ElevatedButton.styleFrom(
            backgroundColor: scheme.primary,
            foregroundColor: scheme.onPrimary,
          ),
          child: withdrawState is WithdrawProcessing
              ? SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: scheme.onPrimary,
                  ),
                )
              : const Text('Withdraw'),
        ),
      ],
    );
  }

  Widget _buildSummaryRow(String label, String value) {
    final scheme = Theme.of(context).colorScheme;
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          label,
          style: TextStyle(fontSize: AppType.s13, color: scheme.onSurfaceVariant),
        ),
        Text(
          value,
          style: TextStyle(
            fontSize: AppType.s13,
            fontWeight: FontWeight.w600,
            color: scheme.onSurface,
          ),
        ),
      ],
    );
  }

  List<Widget> _buildQuickAmountButtons() {
    final availableBalance = widget.availableBalance;
    List<double> amounts = [];

    // Add common percentages
    if (availableBalance >= minWithdrawAmount) {
      if (availableBalance * 0.25 >= minWithdrawAmount) {
        amounts.add(availableBalance * 0.25);
      }
      if (availableBalance * 0.5 >= minWithdrawAmount) {
        amounts.add(availableBalance * 0.5);
      }
      if (availableBalance * 0.75 >= minWithdrawAmount) {
        amounts.add(availableBalance * 0.75);
      }
      amounts.add(availableBalance); // 100%
    }

    return amounts.map((amount) {
      return OutlinedButton(
        onPressed: () {
          _amountController.text = amount.toStringAsFixed(0);
        },
        style: OutlinedButton.styleFrom(
          padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p8),
          minimumSize: Size.zero,
          tapTargetSize: MaterialTapTargetSize.shrinkWrap,
        ),
        child: Text(
          amount == availableBalance
              ? 'All'
              : '${(amount / availableBalance * 100).toInt()}%',
          style: const TextStyle(fontSize: AppType.s12),
        ),
      );
    }).toList();
  }

  Widget _buildVerificationRequiredDialog(
    BuildContext context,
    SellerVerificationV2State verificationState,
  ) {
    final scheme = Theme.of(context).colorScheme;
    final isRejected =
        verificationState.status == SellerVerificationStatus.rejected;
    final isPendingReview =
        verificationState.status == SellerVerificationStatus.pendingReview;
    final isUnderInvestigation =
        verificationState.status == SellerVerificationStatus.underInvestigation;

    return AlertDialog(
      title: Row(
        children: [
          Icon(
            isRejected
                ? Icons.warning
                : isUnderInvestigation
                ? Icons.policy
                : Icons.verified_user,
            color: isRejected
                ? context.statusColors.error
                : isUnderInvestigation
                ? context.statusColors.warning
                : scheme.primary,
          ),
          const SizedBox(width: 12),
          Text(
            isRejected
                ? 'Verifikasi Ditolak'
                : isUnderInvestigation
                ? 'Akun Sedang Diinvestigasi'
                : isPendingReview
                ? 'Verifikasi Sedang Ditinjau'
                : 'Verifikasi Diperlukan',
          ),
        ],
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (isRejected) ...[
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: context.statusColors.error.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(AppShape.r8),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.info_outline,
                    color: context.statusColors.error,
                    size: 20,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Dokumen verifikasi Anda ditolak. Mohon periksa dan ajukan kembali.',
                      style: TextStyle(fontSize: AppType.s13, color: context.statusColors.error),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),
          ] else if (isUnderInvestigation) ...[
            const Text('Akun Anda sedang dalam proses investigasi.'),
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: context.statusColors.warning.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(AppShape.r8),
              ),
              child: Row(
                children: [
                  Icon(Icons.policy, color: context.statusColors.warning, size: 20),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Verifikasi sedang ditinjau. Penarikan dana sementara tidak tersedia.',
                      style: TextStyle(fontSize: AppType.s12),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),
          ] else if (isPendingReview) ...[
            const Text(
              'Dokumen verifikasi Anda sedang ditinjau. Penarikan dana akan tersedia setelah status disetujui.',
            ),
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: context.statusColors.warning.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(AppShape.r8),
              ),
              child: Row(
                children: [
                  Icon(Icons.pending, color: context.statusColors.warning, size: 20),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Status saat ini: menunggu review admin.',
                      style: TextStyle(fontSize: AppType.s12),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),
          ] else ...[
            const Text(
              'Anda perlu melakukan verifikasi identitas sebelum dapat menarik dana.',
            ),
            const SizedBox(height: 16),
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: scheme.primary.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(AppShape.r8),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.description_outlined,
                    color: scheme.primary,
                    size: 20,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Siapkan KTP dan foto selfie untuk verifikasi',
                      style: TextStyle(fontSize: AppType.s12),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),
          ],
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Nanti'),
        ),
        // underInvestigation: user cannot self-serve; no verification CTA.
        if (!isUnderInvestigation)
          ElevatedButton(
            onPressed: () {
              Navigator.of(context).pop();
              context.push(RoutePaths.sellerVerification);
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: scheme.primary,
              foregroundColor: scheme.onPrimary,
            ),
            child: const Text('Verifikasi Sekarang'),
          ),
      ],
    );
  }
}

/// Show withdraw dialog
///
/// Returns true if withdrawal was successful
Future<bool> showWithdrawDialog(
  BuildContext context,
  double availableBalance, {
  required double withdrawalFeeAmount,
}) {
  return showDialog<bool>(
    context: context,
    builder: (context) => WithdrawDialog(
      availableBalance: availableBalance,
      withdrawalFeeAmount: withdrawalFeeAmount,
    ),
  ).then((value) => value ?? false);
}
