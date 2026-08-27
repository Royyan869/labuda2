part of '../screens/checkout_screen_impl.dart';

/// Coin Toggle Section
class _CoinToggleSection extends ConsumerWidget {
  final bool useCoins;
  final int coinBalance;
  final ValueChanged<bool> onToggle;

  const _CoinToggleSection({
    required this.useCoins,
    required this.coinBalance,
    required this.onToggle,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final coinState = ref.watch(coinProvider);

    // Update coin balance from state
    final currentBalance = coinState.maybeWhen(
      balanceLoaded: (balance, coinBalanceEntity) => balance,
      orElse: () => coinBalance,
    );

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: AppColors.coinPrimary.withValues(alpha: 0.3),
          width: 1.5,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  gradient: AppColors.coinGradient,
                  shape: BoxShape.circle,
                ),
                child: const Icon(
                  Icons.monetization_on,
                  color: Colors.white,
                  size: 22,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      'Gunakan Koin Labuda',
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      'Koin tersedia: $currentBalance',
                      style: TextStyle(
                        fontSize: 13,
                        color: AppColors.coinSecondary,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                  ],
                ),
              ),
              Switch(
                value: useCoins && currentBalance > 0,
                onChanged: currentBalance > 0
                    ? (value) => onToggle(value)
                    : null,
                activeTrackColor: AppColors.coinPrimary,
              ),
            ],
          ),
          if (useCoins && currentBalance > 0) ...[
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: AppColors.coinPrimary.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.info_outline,
                    size: 16,
                    color: AppColors.coinSecondary,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Anda akan menggunakan $currentBalance koin untuk diskon',
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.coinSecondary,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }
}
