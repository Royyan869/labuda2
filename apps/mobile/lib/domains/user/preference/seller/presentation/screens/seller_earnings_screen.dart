/// Seller Earnings Screen
///
/// REAL implementation using backend API
/// Shows: available_balance, pending_balance, total_withdrawn, total_earned
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_earnings.dart';
import 'package:labuda/domains/user/preference/seller/seller_di.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/withdraw_notifier.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/withdraw_dialog.dart';
import 'package:labuda/shared/utils/app_formatters.dart';
import 'package:labuda/domains/system/support/presentation/presentation.dart'; // R3.1: Import for showPreChatFormRefactored
import 'package:labuda/domains/user/preference/seller/domain/entities/withdrawal.dart';

/// Seller Earnings Screen
///
/// Displays seller earnings from backend API:
/// - Available Balance: Withdrawable balance (matured funds only)
/// - Pending Balance: Sum of escrow amounts for shipped/delivered orders
/// - Total Withdrawn: Sum of all COMPLETED withdrawal amounts
/// - Total Earned: Total credits ever received to SELLER_PAYABLE account
class SellerEarningsScreen extends ConsumerStatefulWidget {
  const SellerEarningsScreen({super.key});

  @override
  ConsumerState<SellerEarningsScreen> createState() =>
      _SellerEarningsScreenState();
}

class _SellerEarningsScreenState extends ConsumerState<SellerEarningsScreen> {
  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);

    if (authState is! AuthStateAuthenticated) {
      return _buildError('User not authenticated');
    }

    final sellerId = authState.user.id;
    final earningsAsync = ref.watch(sellerEarningsProvider(sellerId));

    return Scaffold(
      appBar: AppBar(
        title: const Text('Earnings'),
        backgroundColor: AppColors.primaryRed,
        foregroundColor: Colors.white,
      ),
      body: earningsAsync.when(
        data: (earnings) => _buildEarningsContent(earnings, sellerId),
        loading: () => _buildLoading(),
        error: (e, _) => _buildError('Failed to load earnings: $e'),
      ),
    );
  }

  Widget _buildEarningsContent(SellerEarnings earnings, String sellerId) {
    // PASS_18H: the fee is deducted FROM the requested amount, never added
    // on top, so the balance only needs to cover the minimum requested
    // amount itself (Rp 10,000) — not minimum + fee.
    const minimumWithdrawable = 10000.0;

    return RefreshIndicator(
      onRefresh: () async {
        ref.invalidate(sellerEarningsProvider(sellerId));
      },
      child: SingleChildScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Available Balance Card (Primary)
            _buildBalanceCard(
              title: 'Available Balance',
              subtitle: 'Ready to withdraw',
              amount: earnings.availableBalance,
              icon: Icons.account_balance_wallet,
              color: AppColors.primaryRed,
              onTap: earnings.availableBalance >= minimumWithdrawable
                  ? () => _showWithdrawDialog(
                      earnings.availableBalance,
                      withdrawalFeeAmount: earnings.withdrawalFeeAmount,
                    )
                  : null,
            ),

            // Balance breakdown: shown when freeze reduces withdrawable.
            if (earnings.hasBalanceBreakdown &&
                (earnings.activeDisputeFreeze ?? 0) > 0)
              Padding(
                padding: const EdgeInsets.only(top: 12, bottom: 4),
                child: _buildBalanceBreakdown(earnings),
              ),

            const SizedBox(height: 16),

            // Total Earned Card
            _buildBalanceCard(
              title: 'Total Earned',
              subtitle: 'Lifetime earnings',
              amount: earnings.totalRevenue,
              icon: Icons.trending_up,
              color: AppColors.successGreen,
            ),

            const SizedBox(height: 16),

            // Pending Balance Card
            _buildBalanceCard(
              title: 'Pending Balance',
              subtitle: 'In escrow (awaiting delivery)',
              amount: earnings.pendingRevenue,
              icon: Icons.hourglass_empty,
              color: Colors.orange,
            ),

            const SizedBox(height: 16),

            // Total Withdrawn Card
            _buildBalanceCard(
              title: 'Total Withdrawn',
              subtitle: 'Successfully withdrawn',
              amount: earnings.totalWithdrawn,
              icon: Icons.download_done,
              color: Colors.blue,
            ),

            const SizedBox(height: 24),

            // Withdrawal History Section
            _buildWithdrawalHistorySection(),

            const SizedBox(height: 24),

            // Info Section
            _buildInfoSection(),

            const SizedBox(height: 16),

            // Bank Account — required for withdrawal.
            _buildManageBankAccountCard(context),

            const SizedBox(height: 16),

            // PHASE 3 HARDENING: Contextual Help for Withdrawal Issues
            _buildWithdrawalHelpSection(
              isDark: Theme.of(context).brightness == Brightness.dark,
            ),

            const SizedBox(height: 24),

            // Withdrawal Button
            if (earnings.availableBalance >= minimumWithdrawable)
              _buildWithdrawButton(
                earnings.availableBalance,
                withdrawalFeeAmount: earnings.withdrawalFeeAmount,
              )
            else
              _buildMinBalanceInfo(earnings.withdrawalFeeAmount),
          ],
        ),
      ),
    );
  }

  Widget _buildBalanceCard({
    required String title,
    required String subtitle,
    required double amount,
    required IconData icon,
    required Color color,
    VoidCallback? onTap,
  }) {
    return Card(
      elevation: 2,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Row(
            children: [
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(
                  color: color.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Icon(icon, color: color, size: 24),
              ),
              const SizedBox(width: 16),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      title,
                      style: const TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray600,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      AppFormatters.formatCurrency(amount),
                      style: TextStyle(
                        fontSize: 20,
                        fontWeight: FontWeight.bold,
                        color: color,
                      ),
                    ),
                    Text(
                      subtitle,
                      style: const TextStyle(
                        fontSize: 10,
                        color: AppColors.neutralGray400,
                      ),
                    ),
                  ],
                ),
              ),
              if (onTap != null)
                Icon(
                  Icons.arrow_forward_ios,
                  size: 16,
                  color: AppColors.neutralGray400,
                ),
            ],
          ),
        ),
      ),
    );
  }

  /// Balance breakdown card: shows why available balance < gross payable.
  /// Only rendered when dispute freeze > 0.
  Widget _buildBalanceBreakdown(SellerEarnings earnings) {
    final gross = earnings.grossPayable ?? 0;
    final freeze = earnings.activeDisputeFreeze ?? 0;

    return Card(
      elevation: 1,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.info_outline, size: 18, color: Colors.orange),
                const SizedBox(width: 8),
                Text(
                  'Rincian Saldo',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: AppColors.neutralGray800,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            _buildBreakdownRow(
              'Saldo Kotor',
              AppFormatters.formatCurrency(gross),
              AppColors.neutralGray800,
            ),
            if (freeze > 0)
              _buildBreakdownRow(
                'Pembekuan Sengketa',
                '- ${AppFormatters.formatCurrency(freeze)}',
                Colors.orange,
              ),
            const Divider(height: 16),
            _buildBreakdownRow(
              'Dapat Ditarik',
              AppFormatters.formatCurrency(earnings.availableBalance),
              AppColors.primaryRed,
              bold: true,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildBreakdownRow(
    String label,
    String value,
    Color valueColor, {
    bool bold = false,
  }) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            label,
            style: TextStyle(
              fontSize: 13,
              color: AppColors.neutralGray600,
              fontWeight: bold ? FontWeight.w600 : FontWeight.normal,
            ),
          ),
          Text(
            value,
            style: TextStyle(
              fontSize: 13,
              color: valueColor,
              fontWeight: bold ? FontWeight.bold : FontWeight.w500,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildInfoSection() {
    return Card(
      elevation: 1,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(
                  Icons.info_outline,
                  size: 20,
                  color: AppColors.neutralGray600,
                ),
                const SizedBox(width: 8),
                Text(
                  'About Earnings',
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.bold,
                    color: AppColors.neutralGray800,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            _buildInfoItem(
              'Available Balance',
              'Funds that have matured and are ready to withdraw',
            ),
            const Divider(height: 16),
            _buildInfoItem(
              'Pending Balance',
              'Funds from shipped orders held in escrow until delivery confirmation',
            ),
            const Divider(height: 16),
            _buildInfoItem(
              'Minimum Withdrawal',
              'Rp 10.000 minimum withdrawal amount',
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildManageBankAccountCard(BuildContext context) {
    return Card(
      elevation: 1,
      child: InkWell(
        onTap: () => context.push(RoutePaths.sellerBankAccounts),
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Row(
            children: [
              Icon(
                Icons.account_balance,
                color: AppColors.primaryRed,
                size: 24,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Bank Account for Withdrawals',
                      style: const TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                        color: AppColors.neutralGray800,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      'Add or manage bank accounts used to receive payouts',
                      style: const TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray600,
                      ),
                    ),
                  ],
                ),
              ),
              Icon(
                Icons.arrow_forward_ios,
                size: 16,
                color: AppColors.neutralGray400,
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildInfoItem(String title, String description) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: const TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w500,
            color: AppColors.neutralGray800,
          ),
        ),
        const SizedBox(height: 4),
        Text(
          description,
          style: const TextStyle(fontSize: 12, color: AppColors.neutralGray600),
        ),
      ],
    );
  }

  Widget _buildWithdrawButton(
    double availableBalance, {
    required double withdrawalFeeAmount,
  }) {
    return SizedBox(
      width: double.infinity,
      child: ElevatedButton(
        onPressed: () => _showWithdrawDialog(
          availableBalance,
          withdrawalFeeAmount: withdrawalFeeAmount,
        ),
        style: ElevatedButton.styleFrom(
          backgroundColor: AppColors.primaryRed,
          foregroundColor: Colors.white,
          padding: const EdgeInsets.symmetric(vertical: 16),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        ),
        child: const Text(
          'Withdraw Funds',
          style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
        ),
      ),
    );
  }

  Widget _buildMinBalanceInfo(double withdrawalFeeAmount) {
    return Card(
      elevation: 1,
      color: AppColors.neutralGray100,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Icon(Icons.info_outline, color: AppColors.neutralGray600),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                'Minimum penarikan Rp 10.000. Biaya penarikan ${AppFormatters.formatCurrency(withdrawalFeeAmount)} dipotong dari jumlah yang diminta.',
                style: TextStyle(fontSize: 14, color: AppColors.neutralGray700),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildLoading() {
    return const Center(child: CircularProgressIndicator());
  }

  Widget _buildError(String message) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(
              Icons.error_outline,
              size: 64,
              color: AppColors.primaryRed,
            ),
            const SizedBox(height: 16),
            const Text(
              'Error Loading Earnings',
              style: TextStyle(
                fontSize: 20,
                fontWeight: FontWeight.bold,
                color: AppColors.neutralGray800,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              message,
              style: const TextStyle(
                fontSize: 14,
                color: AppColors.neutralGray600,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 24),
            ElevatedButton(
              onPressed: () {
                final authState = ref.read(authControllerProvider);
                if (authState is AuthStateAuthenticated) {
                  ref.invalidate(sellerEarningsProvider(authState.user.id));
                }
              },
              child: const Text('Retry'),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _showWithdrawDialog(
    double availableBalance, {
    required double withdrawalFeeAmount,
  }) async {
    final success = await showWithdrawDialog(
      context,
      availableBalance,
      withdrawalFeeAmount: withdrawalFeeAmount,
    );

    if (success && mounted) {
      // Show success message (TRUTHFUL - indicates manual processing)
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text(
            'Permintaan pencairan dikirim. Menunggu verifikasi admin (1-3 hari kerja).',
          ),
          backgroundColor: AppColors.successGreen,
          duration: Duration(seconds: 3),
        ),
      );

      // Refresh earnings data
      final authState = ref.read(authControllerProvider);
      if (authState is AuthStateAuthenticated) {
        ref.invalidate(sellerEarningsProvider(authState.user.id));
      }
    }
  }

  /// PHASE 3 HARDENING: Contextual help section for withdrawal issues
  /// Provides direct access to help articles and support escalation
  Widget _buildWithdrawalHelpSection({required bool isDark}) {
    final authState = ref.read(authControllerProvider);
    final userId = authState is AuthStateAuthenticated
        ? authState.user.id
        : null;
    final userName = authState is AuthStateAuthenticated
        ? authState.user.username
        : null;
    final userAvatar = authState is AuthStateAuthenticated
        ? authState.user.avatarUrl
        : null;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: [
            AppColors.primaryBlue.withValues(alpha: 0.1),
            AppColors.primaryBlue.withValues(alpha: 0.05),
          ],
        ),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.primaryBlue.withValues(alpha: 0.3)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.help_outline, color: AppColors.primaryBlue, size: 20),
              const SizedBox(width: 8),
              Text(
                'Butuh Bantuan Penarikan?',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.neutralGray900,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            'Cara menarik dana, solusi masalah pencairan, dan info batas minimum.',
            style: TextStyle(fontSize: 12, color: AppColors.neutralGray600),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: OutlinedButton.icon(
                  onPressed: () {
                    Navigator.of(context).push(
                      MaterialPageRoute(
                        builder: (context) => HelpCenterScreen(
                          userId: userId,
                          userName: userName,
                          userAvatar: userAvatar,
                        ),
                      ),
                    );
                  },
                  icon: const Icon(Icons.article_outlined, size: 16),
                  label: const Text('Panduan Penarikan'),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: AppColors.primaryBlue,
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    textStyle: const TextStyle(fontSize: 12),
                    side: BorderSide(
                      color: AppColors.primaryBlue.withValues(alpha: 0.5),
                    ),
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: ElevatedButton.icon(
                  onPressed: userId != null
                      ? () {
                          showPreChatFormRefactored(
                            context,
                            userId: userId,
                            userName: userName ?? 'User',
                            userAvatar: userAvatar,
                          );
                        }
                      : null,
                  icon: const Icon(Icons.support_agent, size: 16),
                  label: const Text('Hubungi Support'),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.primaryBlue,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    textStyle: const TextStyle(fontSize: 12),
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  /// Withdrawal History Section
  ///
  /// Shows seller's transaction/withdrawal history with:
  /// - Date
  /// - Amount
  /// - Status (pending, processing, completed, failed)
  /// - Bank info snapshot
  Widget _buildWithdrawalHistorySection() {
    final historyAsync = ref.watch(withdrawalHistoryProvider);
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Card(
      elevation: 1,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Row(
                  children: [
                    Icon(
                      Icons.history,
                      size: 20,
                      color: AppColors.neutralGray700,
                    ),
                    const SizedBox(width: 8),
                    Text(
                      'Riwayat Penarikan',
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900,
                      ),
                    ),
                  ],
                ),
                TextButton(
                  onPressed: () {
                    ref.invalidate(withdrawalHistoryProvider);
                  },
                  child: const Text('Refresh'),
                ),
              ],
            ),
            const SizedBox(height: 12),
            historyAsync.when(
              data: (withdrawals) {
                if (withdrawals.isEmpty) {
                  return Padding(
                    padding: const EdgeInsets.symmetric(vertical: 24),
                    child: Center(
                      child: Column(
                        children: [
                          Icon(
                            Icons.receipt_long_outlined,
                            size: 48,
                            color: AppColors.neutralGray400,
                          ),
                          const SizedBox(height: 12),
                          Text(
                            'Belum ada riwayat penarikan',
                            style: TextStyle(
                              fontSize: 14,
                              color: AppColors.neutralGray600,
                            ),
                          ),
                        ],
                      ),
                    ),
                  );
                }

                return Column(
                  children: withdrawals
                      .map((w) => _buildWithdrawalTile(w, isDark))
                      .toList(),
                );
              },
              loading: () => const Padding(
                padding: EdgeInsets.symmetric(vertical: 24),
                child: Center(child: CircularProgressIndicator()),
              ),
              error: (_, _) => Padding(
                padding: const EdgeInsets.symmetric(vertical: 16),
                child: Text(
                  'Gagal memuat riwayat penarikan',
                  style: TextStyle(
                    fontSize: 14,
                    color: AppColors.neutralGray600,
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildWithdrawalTile(Withdrawal withdrawal, bool isDark) {
    final statusColor = _getStatusColor(withdrawal.status);
    final statusLabel = _getStatusLabel(withdrawal.status);

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray800 : AppColors.neutralGray50,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  AppFormatters.formatCurrency(withdrawal.amount),
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.bold,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                  decoration: BoxDecoration(
                    color: statusColor.withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: Text(
                    statusLabel,
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: statusColor,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Row(
              children: [
                Icon(
                  Icons.calendar_today,
                  size: 14,
                  color: AppColors.neutralGray600,
                ),
                const SizedBox(width: 4),
                Text(
                  _formatDate(withdrawal.createdAt),
                  style: TextStyle(
                    fontSize: 12,
                    color: AppColors.neutralGray600,
                  ),
                ),
                if (withdrawal.bankNameSnapshot != null) ...[
                  const SizedBox(width: 16),
                  Icon(
                    Icons.account_balance,
                    size: 14,
                    color: AppColors.neutralGray600,
                  ),
                  const SizedBox(width: 4),
                  Expanded(
                    child: Text(
                      withdrawal.bankNameSnapshot!,
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray600,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ],
              ],
            ),
            if (withdrawal.accountNumberSnapshot != null)
              Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Row(
                  children: [
                    const SizedBox(width: 18),
                    Text(
                      '**** ${withdrawal.accountNumberSnapshot!.substring(withdrawal.accountNumberSnapshot!.length - 4)}',
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray600,
                      ),
                    ),
                  ],
                ),
              ),
          ],
        ),
      ),
    );
  }

  Color _getStatusColor(WithdrawalStatus status) {
    switch (status) {
      case WithdrawalStatus.settled:
      case WithdrawalStatus.completed:
        return AppColors.successGreen;
      case WithdrawalStatus.requested:
      case WithdrawalStatus.processing:
      case WithdrawalStatus.submitted:
      case WithdrawalStatus.settling:
      case WithdrawalStatus.pilotBlocked:
        return Colors.orange;
      case WithdrawalStatus.failed:
      case WithdrawalStatus.failedRetryable:
      case WithdrawalStatus.failedFinal:
        return AppColors.error;
      case WithdrawalStatus.unknown:
        return Colors.grey;
    }
  }

  String _getStatusLabel(WithdrawalStatus status) {
    switch (status) {
      case WithdrawalStatus.requested:
        return 'Diajukan';
      case WithdrawalStatus.processing:
        return 'Diproses';
      case WithdrawalStatus.submitted:
        return 'Disubmit';
      case WithdrawalStatus.settling:
        return 'Penyelesaian';
      case WithdrawalStatus.settled:
        return 'Selesai';
      case WithdrawalStatus.completed:
        return 'Selesai';
      case WithdrawalStatus.failed:
        return 'Gagal';
      case WithdrawalStatus.failedRetryable:
        return 'Gagal (Coba Lagi)';
      case WithdrawalStatus.failedFinal:
        return 'Gagal Permanen';
      case WithdrawalStatus.pilotBlocked:
        return 'Ditahan';
      case WithdrawalStatus.unknown:
        return 'Tidak diketahui';
    }
  }

  String _formatDate(DateTime date) {
    final now = DateTime.now();
    final diff = now.difference(date);

    if (diff.inDays == 0) {
      return 'Hari ini';
    } else if (diff.inDays == 1) {
      return 'Kemarin';
    } else if (diff.inDays < 7) {
      return '${diff.inDays} hari lalu';
    } else if (diff.inDays < 30) {
      return '${(diff.inDays / 7).floor()} minggu lalu';
    } else {
      return '${date.day}/${date.month}/${date.year}';
    }
  }
}
