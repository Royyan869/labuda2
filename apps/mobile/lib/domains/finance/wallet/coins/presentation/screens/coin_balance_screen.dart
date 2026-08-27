/// Coin Balance Screen
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
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
      appBar: AppBar(title: const Text('Coins'), elevation: 0),
      body: balanceAsync.when(
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
                const SliverToBoxAdapter(
                  child: Padding(
                    padding: EdgeInsets.fromLTRB(16, 24, 16, 12),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Text(
                          'Transaksi Terbaru',
                          style: TextStyle(
                            fontSize: 16,
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
                      padding: const EdgeInsets.symmetric(horizontal: 16),
                      sliver: SliverList(
                        delegate: SliverChildBuilderDelegate((context, index) {
                          if (index == transactions.length) {
                            return Padding(
                              padding: const EdgeInsets.symmetric(vertical: 16),
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
                            padding: const EdgeInsets.only(bottom: 12),
                            child: _buildTransactionItem(transaction),
                          );
                        }, childCount: transactions.length + 1),
                      ),
                    );
                  },
                  loading: () => const SliverToBoxAdapter(
                    child: Center(
                      child: Padding(
                        padding: EdgeInsets.all(32),
                        child: CircularProgressIndicator(),
                      ),
                    ),
                  ),
                  error: (error, _) =>
                      SliverToBoxAdapter(child: _buildError(error.toString())),
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
    );
  }

  Widget _buildTransactionItem(CoinTransaction transaction) {
    return Card(
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(
          color: Theme.of(context).brightness == Brightness.dark
              ? AppColors.neutralGray700
              : AppColors.neutralGray200,
        ),
      ),
      child: ListTile(
        contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        leading: _getTransactionIcon(transaction),
        title: Text(
          transaction.description,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w500),
        ),
        subtitle: Text(
          _formatDate(transaction.createdAt),
          style: const TextStyle(fontSize: 12),
        ),
        trailing: Text(
          '${transaction.amount > 0 ? '+' : ''}${transaction.amount}',
          style: TextStyle(
            fontSize: 15,
            fontWeight: FontWeight.bold,
            color: transaction.amount > 0 ? Colors.green : Colors.red,
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
      child: const Icon(iconData, color: color, size: 20),
    );
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
    } else {
      return '${date.day}/${date.month}/${date.year}';
    }
  }

  Widget _buildAuthRequired() {
    return Scaffold(
      appBar: AppBar(title: const Text('Coins')),
      body: const Center(child: Text('Silakan login untuk melihat Coins Anda')),
    );
  }

  Widget _buildEmptyState() {
    return const Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.stars_outlined, size: 64, color: AppColors.coinPrimary),
          SizedBox(height: 16),
          Text(
            'Belum ada Coins',
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
          ),
          SizedBox(height: 8),
          Text(
            'Dapatkan Coins dari berbagai aktivitas di Labuda',
            style: TextStyle(fontSize: 14),
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
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.error_outline, size: 64, color: Colors.red),
            const SizedBox(height: 16),
            const Text(
              'Terjadi Kesalahan',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),
            Text(
              displayMessage,
              textAlign: TextAlign.center,
              style: const TextStyle(fontSize: 14),
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
