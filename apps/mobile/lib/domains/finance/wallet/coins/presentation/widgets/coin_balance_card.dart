/// Coin Balance Card Widget
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/app_dialog.dart';
import 'package:labuda/domains/finance/wallet/coins/domain/entities/coin_balance.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';

/// Displays user's Coin balance with visibility toggle and actions.
///
/// IMPORTANT: Coins are LOYALTY POINTS, NOT money.
///
/// V1 SIMPLIFICATION: Coins do NOT expire. No promo/regular distinction.
///
/// Shows:
/// - Total coins
/// - Estimated discount value for display purposes only
/// - View history action button
/// - Max balance warning
class CoinBalanceCard extends StatefulWidget {
  final CoinBalance balance;
  final VoidCallback? onViewHistory;

  const CoinBalanceCard({super.key, required this.balance, this.onViewHistory});

  @override
  State<CoinBalanceCard> createState() => _CoinBalanceCardState();
}

class _CoinBalanceCardState extends State<CoinBalanceCard> {
  bool _isBalanceVisible = true;

  /// Content for the canonical Page Info surface. The surface itself is
  /// [AppDialog.info]; this widget owns no dialog authority.
  Widget _buildCoinInfoContent(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          'LABUDA Coins adalah poin loyalitas yang memberikan Anda potongan harga saat checkout.',
          style: context.typeRoles.bodyDense.copyWith(
            height: 1.5,
            fontWeight: FontWeight.w600,
          ),
        ),
        const SizedBox(height: 16),
        Text(
          'Cara Dapatkan Coins:\n'
          '• Refund dari pembatalan pesanan\n'
          '• Bonus pendaftaran pengguna baru\n'
          '• Promo dan kampanye khusus\n'
          '• Reward referral dan ulasan',
          style: context.typeRoles.bodyDense.copyWith(height: 1.6),
        ),
        const SizedBox(height: 12),
        Text(
          'Penting:\n'
          '• Coins adalah poin loyalitas, BUKAN uang\n'
          '• Coins hanya untuk potongan harga (max 20%)\n'
          '• Coins tidak dapat ditarik atau ditukar uang\n'
          '• Coins tidak dapat ditransfer ke pengguna lain\n'
          '• Maksimal 1.000.000 coins\n'
          '• Coins tidak pernah kadaluarsa',
          style: context.typeRoles.labelMicro.copyWith(
            height: 1.5,
            fontStyle: FontStyle.italic,
            color: AppColors.coinPrimary,
          ),
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final isNearMaxBalance = widget.balance.isNearMaxBalance;
    final isAtMaxBalance = widget.balance.isAtMaxBalance;
    final colorScheme = Theme.of(context).colorScheme;

    return Container(
      margin: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p12, AppMetrics.p16, AppMetrics.p12),
      padding: const EdgeInsets.all(AppMetrics.p24),
      decoration: BoxDecoration(
        gradient: AppColors.coinGradient,
        borderRadius: BorderRadius.circular(AppShape.r16),
        boxShadow: [
          BoxShadow(
            color: AppColors.coinPrimary.withValues(alpha: 0.4),
            blurRadius: 12,
            offset: const Offset(0, 4),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header: Label + Info button
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Row(
                children: [
                  Icon(Icons.stars, color: colorScheme.onPrimary, size: AppIconSize.inlineGlyph),
                  const SizedBox(width: 6),
                  Text(
                    'Coins',
                    style: context.typeRoles.bodyDense.copyWith(
                      color: colorScheme.onPrimary,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ],
              ),
              IconButton(
                icon: Icon(
                  Icons.info_outline,
                  color: colorScheme.onPrimary,
                  size: AppIconSize.action,
                ),
                onPressed: () => AppDialog.info(
                  context: context,
                  title: 'Tentang LABUDA Coins',
                  content: _buildCoinInfoContent(context),
                  closeLabel: 'Mengerti',
                ),
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
                tooltip: 'Info tentang Coins',
              ),
            ],
          ),
          const SizedBox(height: 12),

          // Total Coins with visibility toggle
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Flexible(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      _isBalanceVisible
                          ? '${formatGroupedAmount(widget.balance.balance)} Coins'
                          : '******** Coins',
                      style: context.typeRoles.titleProminent.copyWith(
                        fontWeight: FontWeight.bold,
                        color: colorScheme.onPrimary,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      _isBalanceVisible
                          ? '~Potongan Rp ${formatGroupedAmount(widget.balance.balance * 10)}'
                          : '~Potongan Rp ********',
                      style: context.typeRoles.bodyDense.copyWith(
                        color: colorScheme.onPrimary.withValues(alpha: 0.9),
                      ),
                    ),
                  ],
                ),
              ),
              IconButton(
                icon: Icon(
                  _isBalanceVisible ? Icons.visibility_off : Icons.visibility,
                  color: colorScheme.onPrimary,
                  size: AppIconSize.action,
                  semanticLabel: _isBalanceVisible
                      ? 'Sembunyikan saldo'
                      : 'Tampilkan saldo',
                ),
                onPressed: () =>
                    setState(() => _isBalanceVisible = !_isBalanceVisible),
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
              ),
            ],
          ),

          // Max Balance Warning
          if (isNearMaxBalance || isAtMaxBalance) ...[
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: isAtMaxBalance
                    ? context.statusColors.error.withValues(alpha: 0.3)
                    : context.statusColors.warning.withValues(alpha: 0.3),
                borderRadius: BorderRadius.circular(AppShape.r8),
              ),
              child: Row(
                children: [
                  Icon(
                    isAtMaxBalance ? Icons.block : Icons.warning_amber,
                    color: colorScheme.onPrimary,
                    size: AppIconSize.inlineGlyph,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      isAtMaxBalance
                          ? 'Maksimal coins tercapai (1.000.000)'
                          : 'Mendekati batas maksimal coins',
                      style: context.typeRoles.labelMicro.copyWith(
                        color: colorScheme.onPrimary,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],

          // Action Buttons
          if (widget.onViewHistory != null) ...[
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity,
              child: OutlinedButton.icon(
                onPressed: widget.onViewHistory,
                icon: const Icon(Icons.history, size: AppIconSize.inlineGlyph),
                label: Text(
                  'Lihat Riwayat',
                  style: Theme.of(context).textTheme.labelLarge,
                ),
                style: OutlinedButton.styleFrom(
                  foregroundColor: colorScheme.onPrimary,
                  side: BorderSide(
                    color: colorScheme.onPrimary,
                    width: 1.5,
                  ),
                  padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12),
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }
}
