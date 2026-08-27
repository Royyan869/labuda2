/// Coin Balance Card Widget
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/finance/wallet/coins/domain/entities/coin_balance.dart';

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

  void _showCoinInfo() {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Tentang LABUDA Coins'),
        content: const SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                'LABUDA Coins adalah poin loyalitas yang memberikan Anda potongan harga saat checkout.',
                style: TextStyle(
                  fontSize: 14,
                  height: 1.5,
                  fontWeight: FontWeight.w600,
                ),
              ),
              SizedBox(height: 16),
              Text(
                'Cara Dapatkan Coins:\n'
                '• Refund dari pembatalan pesanan\n'
                '• Bonus pendaftaran pengguna baru\n'
                '• Promo dan kampanye khusus\n'
                '• Reward referral dan ulasan',
                style: TextStyle(fontSize: 13, height: 1.6),
              ),
              SizedBox(height: 12),
              Text(
                'Penting:\n'
                '• Coins adalah poin loyalitas, BUKAN uang\n'
                '• Coins hanya untuk potongan harga (max 20%)\n'
                '• Coins tidak dapat ditarik atau ditukar uang\n'
                '• Coins tidak dapat ditransfer ke pengguna lain\n'
                '• Maksimal 1.000.000 coins\n'
                '• Coins tidak pernah kadaluarsa',
                style: TextStyle(
                  fontSize: 12,
                  height: 1.5,
                  fontStyle: FontStyle.italic,
                  color: AppColors.coinPrimary,
                ),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Mengerti'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final isNearMaxBalance = widget.balance.isNearMaxBalance;
    final isAtMaxBalance = widget.balance.isAtMaxBalance;

    return Container(
      margin: const EdgeInsets.fromLTRB(16, 12, 16, 12),
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        gradient: AppColors.coinGradient,
        borderRadius: BorderRadius.circular(16),
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
              const Row(
                children: [
                  Icon(Icons.stars, color: AppColors.neutralWhite, size: 16),
                  SizedBox(width: 6),
                  Text(
                    'Coins',
                    style: TextStyle(
                      fontSize: 13,
                      color: AppColors.neutralWhite,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ],
              ),
              IconButton(
                icon: const Icon(
                  Icons.info_outline,
                  color: AppColors.neutralWhite,
                  size: 18,
                ),
                onPressed: _showCoinInfo,
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
                          ? '${_formatNumber(widget.balance.balance)} Coins'
                          : '******** Coins',
                      style: const TextStyle(
                        fontSize: 28,
                        fontWeight: FontWeight.bold,
                        color: AppColors.neutralWhite,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      _isBalanceVisible
                          ? '~Potongan Rp ${_formatNumber(widget.balance.balance * 10)}'
                          : '~Potongan Rp ********',
                      style: TextStyle(
                        fontSize: 13,
                        color: AppColors.neutralWhite.withValues(alpha: 0.9),
                      ),
                    ),
                  ],
                ),
              ),
              IconButton(
                icon: Icon(
                  _isBalanceVisible ? Icons.visibility_off : Icons.visibility,
                  color: AppColors.neutralWhite,
                  size: 20,
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
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: isAtMaxBalance
                    ? AppColors.statusError.withValues(alpha: 0.3)
                    : AppColors.statusWarning.withValues(alpha: 0.3),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Icon(
                    isAtMaxBalance ? Icons.block : Icons.warning_amber,
                    color: AppColors.neutralWhite,
                    size: 16,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      isAtMaxBalance
                          ? 'Maksimal coins tercapai (1.000.000)'
                          : 'Mendekati batas maksimal coins',
                      style: const TextStyle(
                        fontSize: 11,
                        color: AppColors.neutralWhite,
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
                icon: const Icon(Icons.history, size: 16),
                label: const Text(
                  'Lihat Riwayat',
                  style: TextStyle(fontSize: 13),
                ),
                style: OutlinedButton.styleFrom(
                  foregroundColor: AppColors.neutralWhite,
                  side: const BorderSide(
                    color: AppColors.neutralWhite,
                    width: 1.5,
                  ),
                  padding: const EdgeInsets.symmetric(vertical: 10),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8),
                  ),
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }

  String _formatNumber(int number) {
    return number.toString().replaceAllMapped(
      RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'),
      (Match m) => '${m[1]}.',
    );
  }
}
