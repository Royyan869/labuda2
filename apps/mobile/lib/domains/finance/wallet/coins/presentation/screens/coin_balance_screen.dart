/// Coin Balance Screen
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/finance/wallet/coins/domain/entities/coin_transaction.dart';
import 'package:labuda/domains/finance/wallet/coins/presentation/providers/coin_providers.dart';
import 'package:labuda/domains/finance/wallet/coins/presentation/widgets/coin_balance_card.dart';

/// Main screen for viewing Coin balance and recent transactions.
///
/// IMPORTANT: Coins are LOYALTY POINTS, NOT money.
///
/// V1 SIMPLIFICATION: Coins do NOT expire.
///
/// Features:
/// - Real-time balance updates via stream
/// - Recent transaction history (last 10)
/// - Navigation to full transaction history
class CoinBalanceScreen extends ConsumerStatefulWidget {
  const CoinBalanceScreen({super.key});

  @override
  ConsumerState<CoinBalanceScreen> createState() => _CoinBalanceScreenState();
}

class _CoinBalanceScreenState extends ConsumerState<CoinBalanceScreen> {
  @override
  Widget build(BuildContext context) {
    final currentUser = ref.watch(authenticatedUserProvider);
    if (currentUser == null) {
      return _buildAuthRequired();
    }

    final userId = currentUser.id;

    // Watch balance stream for real-time updates
    final balanceAsync = ref.watch(coinBalanceStreamProvider(userId));

    // Watch recent transactions stream
    final transactionsAsync = ref.watch(
      coinTransactionsStreamProvider((userId: userId, limit: 10)),
    );

    return Scaffold(
      appBar: AppBar(title: const Text('Coins')),
      // SAFE-AREA-33: the body content owns the bottom system inset —
      // /coins is a STANDALONE pushed route (CoinsModule top-level GoRoute,
      // no shell bar), so no shell owns it. This screen has NO FAB/CTA:
      // this ONE SafeArea is the sole bottom-inset authority for every
      // state branch below (populated / empty / loading / error).
      body: SafeArea(
        child: balanceAsync.when(
          data: (balance) {
            if (balance == null) {
              return _buildEmptyState();
            }

            return RefreshIndicator(
              onRefresh: () async {
                ref.invalidate(coinBalanceStreamProvider(userId));
                ref.invalidate(
                  coinTransactionsStreamProvider((userId: userId, limit: 10)),
                );
              },
              child: CustomScrollView(
                slivers: [
                  // Balance Card
                  SliverToBoxAdapter(
                    child: CoinBalanceCard(
                      balance: balance,
                      onViewHistory: () => _navigateToHistory(),
                    ),
                  ),

                  // Section Header
                  SliverToBoxAdapter(
                    child: Padding(
                      padding: const EdgeInsets.fromLTRB(
                        AppMetrics.p16,
                        AppMetrics.p24,
                        AppMetrics.p16,
                        AppMetrics.p12,
                      ),
                      child: Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          Text(
                            'Transaksi Terbaru',
                            style: context.typeRoles.titleSection.copyWith(
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),

                  // Recent Transactions
                  transactionsAsync.when(
                    data: (transactions) {
                      if (transactions.isEmpty) {
                        return const SliverToBoxAdapter(
                          child: SizedBox(
                            height: 200,
                            child: Center(child: Text('Belum ada transaksi')),
                          ),
                        );
                      }

                      return SliverPadding(
                        padding: const EdgeInsets.symmetric(
                          horizontal: AppMetrics.p16,
                        ),
                        sliver: SliverList(
                          delegate: SliverChildBuilderDelegate((
                            context,
                            index,
                          ) {
                            if (index == transactions.length) {
                              return Padding(
                                padding: const EdgeInsets.symmetric(
                                  vertical: AppMetrics.p16,
                                ),
                                child: Center(
                                  child: TextButton(
                                    onPressed: () => _navigateToHistory(),
                                    child: const Text('Lihat Semua Transaksi'),
                                  ),
                                ),
                              );
                            }

                            final transaction = transactions[index];
                            return Padding(
                              padding: const EdgeInsets.only(
                                bottom: AppMetrics.p12,
                              ),
                              child: _buildTransactionItem(transaction),
                            );
                          }, childCount: transactions.length + 1),
                        ),
                      );
                    },
                    loading: () => const SliverToBoxAdapter(
                      child: Center(
                        child: Padding(
                          padding: EdgeInsets.all(AppMetrics.p32),
                          child: CircularProgressIndicator(),
                        ),
                      ),
                    ),
                    error: (error, _) => SliverToBoxAdapter(
                      child: _buildError(error.toString()),
                    ),
                  ),

                  // Bottom spacing
                  const SliverToBoxAdapter(child: SizedBox(height: 32)),
                ],
              ),
            );
          },
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => _buildError(error.toString()),
        ),
      ),
    );
  }

  Widget _buildTransactionItem(CoinTransaction transaction) {
    return Card(
      elevation: AppElevation.none,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppShape.r12),
        side: BorderSide(color: Theme.of(context).colorScheme.outlineVariant),
      ),
      child: ListTile(
        contentPadding: const EdgeInsets.symmetric(
          horizontal: AppMetrics.p16,
          vertical: AppMetrics.p8,
        ),
        leading: _getTransactionIcon(transaction),
        title: Text(
          transaction.description,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: context.typeRoles.bodyDense.copyWith(
            fontWeight: FontWeight.w500,
          ),
        ),
        subtitle: Text(
          const TimeFormatService().formatTimeAgo(transaction.createdAt),
          style: context.typeRoles.labelMicro,
        ),
        trailing: Text(
          '${transaction.amount > 0 ? '+' : ''}${transaction.amount}',
          style: context.typeRoles.titleCompact.copyWith(
            fontWeight: FontWeight.bold,
            color: transaction.amount > 0
                ? context.statusColors.success
                : context.statusColors.error,
          ),
        ),
      ),
    );
  }

  Widget _getTransactionIcon(CoinTransaction transaction) {
    // V1 SIMPLIFICATION: Use single icon for all coins
    const iconData = Icons.stars;
    const color = AppColors.coinPrimary;

    return CircleAvatar(
      backgroundColor: color.withValues(alpha: 0.15),
      child: const Icon(iconData, color: color, size: AppIconSize.action),
    );
  }

  Widget _buildAuthRequired() {
    return Scaffold(
      appBar: AppBar(title: const Text('Coins')),
      body: const Center(child: Text('Silakan login untuk melihat Coins Anda')),
    );
  }

  Widget _buildEmptyState() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(
            Icons.stars_outlined,
            size: AppIconSize.display,
            color: AppColors.coinPrimary,
          ),
          const SizedBox(height: 16),
          Text(
            'Belum ada Coins',
            style: context.typeRoles.titleProminent.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Dapatkan Coins dari berbagai aktivitas di Labuda',
            style: context.typeRoles.bodyDense,
          ),
        ],
      ),
    );
  }

  Widget _buildError(String message) {
    // Truncate very long error messages
    final displayMessage = message.length > 200
        ? '${message.substring(0, 200)}...'
        : message;

    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(AppMetrics.p32),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.error_outline,
              size: AppIconSize.display,
              color: context.statusColors.error,
            ),
            const SizedBox(height: 16),
            Text(
              'Terjadi Kesalahan',
              style: context.typeRoles.titleProminent.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              displayMessage,
              textAlign: TextAlign.center,
              style: context.typeRoles.bodyDense,
            ),
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: () => setState(() {}),
              child: const Text('Coba Lagi'),
            ),
          ],
        ),
      ),
    );
  }

  void _navigateToHistory() {
    ref.read(navigationHandlerProvider).navigateToCoinHistory();
  }
}
