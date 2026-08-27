/// Coin History Screen
///
/// Displays coin transaction history from the backend.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/finance/wallet/coins/coins_di.dart';
import 'package:labuda/domains/finance/wallet/coins/domain/entities/coin_transaction.dart';
import 'package:labuda/domains/finance/wallet/coins/presentation/providers/coin_notifier.dart';
import 'package:labuda/domains/finance/wallet/coins/presentation/providers/coin_state.dart';

/// Screen for viewing coin transaction history
class CoinHistoryScreen extends ConsumerStatefulWidget {
  final String userId;

  const CoinHistoryScreen({super.key, required this.userId});

  @override
  ConsumerState<CoinHistoryScreen> createState() => _CoinHistoryScreenState();
}

class _CoinHistoryScreenState extends ConsumerState<CoinHistoryScreen> {
  int _currentPage = 1;

  @override
  void initState() {
    super.initState();
    // Load transactions on init
    Future.microtask(() {
      ref.read(coinProvider.notifier).getTransactions(page: _currentPage);
    });
  }

  @override
  Widget build(BuildContext context) {
    final coinState = ref.watch(coinProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Transaction History')),
      body: coinState.when(
        initial: () =>
            const _EmptyState(message: 'Tap refresh to load your transactions'),
        loading: () => const Center(child: CircularProgressIndicator()),
        balanceLoaded: (_, _) =>
            const _EmptyState(message: 'Tap refresh to load your transactions'),
        transactionsLoaded: (transactions) {
          if (transactions.isEmpty) {
            return const _EmptyState(
              message: 'No transactions yet',
              icon: Icons.receipt_long,
            );
          }
          return _TransactionList(
            transactions: transactions,
            onLoadMore: () {
              _currentPage++;
              ref
                  .read(coinProvider.notifier)
                  .getTransactions(page: _currentPage);
            },
          );
        },
        error: (message) => _ErrorState(
          message: message,
          onRetry: () {
            ref.read(coinProvider.notifier).getTransactions(page: _currentPage);
          },
        ),
      ),
      floatingActionButton: coinState.maybeWhen(
        orElse: () => null,
        error: (_) => FloatingActionButton(
          onPressed: () {
            ref.read(coinProvider.notifier).getTransactions(page: _currentPage);
          },
          child: const Icon(Icons.refresh),
        ),
        initial: () => FloatingActionButton(
          onPressed: () {
            ref.read(coinProvider.notifier).getTransactions(page: _currentPage);
          },
          child: const Icon(Icons.refresh),
        ),
      ),
    );
  }
}

class _TransactionList extends StatelessWidget {
  final List<CoinTransaction> transactions;
  final VoidCallback onLoadMore;

  const _TransactionList({
    required this.transactions,
    required this.onLoadMore,
  });

  @override
  Widget build(BuildContext context) {
    return RefreshIndicator(
      onRefresh: () async {
        onLoadMore();
      },
      child: ListView.separated(
        itemCount: transactions.length + 1,
        separatorBuilder: (_, _) => const Divider(height: 1),
        itemBuilder: (context, index) {
          if (index == transactions.length) {
            // Load more indicator
            return Padding(
              padding: const EdgeInsets.all(16),
              child: Center(
                child: TextButton(
                  onPressed: onLoadMore,
                  child: const Text('Load More'),
                ),
              ),
            );
          }
          final transaction = transactions[index];
          return _TransactionTile(transaction: transaction);
        },
      ),
    );
  }
}

class _TransactionTile extends StatelessWidget {
  final CoinTransaction transaction;

  const _TransactionTile({required this.transaction});

  @override
  Widget build(BuildContext context) {
    final isEarn = transaction.type == CoinTransactionType.earn;
    final icon = isEarn ? Icons.add_circle : Icons.remove_circle;
    final iconColor = isEarn ? AppColors.coinPrimary : AppColors.coinSecondary;

    return ListTile(
      leading: CircleAvatar(
        backgroundColor: iconColor.withValues(alpha: 0.1),
        child: Icon(icon, color: iconColor),
      ),
      title: Text(
        transaction.description,
        style: const TextStyle(fontWeight: FontWeight.w500),
      ),
      subtitle: Text(
        _formatDate(transaction.createdAt),
        style: TextStyle(color: Colors.grey.shade600, fontSize: 12),
      ),
      trailing: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          Text(
            '${isEarn ? '+' : '-'}${transaction.amount}',
            style: TextStyle(
              color: iconColor,
              fontWeight: FontWeight.bold,
              fontSize: 16,
            ),
          ),
          Text(
            'Balance: ${transaction.balanceAfter}',
            style: TextStyle(color: Colors.grey.shade600, fontSize: 11),
          ),
        ],
      ),
    );
  }

  String _formatDate(DateTime date) {
    final now = DateTime.now();
    final difference = now.difference(date);

    if (difference.inDays == 0) {
      if (difference.inHours == 0) {
        if (difference.inMinutes == 0) {
          return 'Just now';
        }
        return '${difference.inMinutes}m ago';
      }
      return '${difference.inHours}h ago';
    } else if (difference.inDays == 1) {
      return 'Yesterday';
    } else if (difference.inDays < 7) {
      return '${difference.inDays} days ago';
    } else {
      return '${date.day}/${date.month}/${date.year}';
    }
  }
}

class _EmptyState extends StatelessWidget {
  final String message;
  final IconData icon;

  const _EmptyState({required this.message, this.icon = Icons.history});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            icon,
            size: 64,
            color: AppColors.coinPrimary.withValues(alpha: 0.5),
          ),
          const SizedBox(height: 16),
          Text(
            'No Transactions',
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.bold,
              color: Colors.grey.shade700,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            message,
            style: TextStyle(fontSize: 14, color: Colors.grey.shade500),
          ),
        ],
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;

  const _ErrorState({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.error_outline,
              size: 64,
              color: AppColors.coinSecondary.withValues(alpha: 0.5),
            ),
            const SizedBox(height: 16),
            Text(
              'Failed to Load',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.bold,
                color: Colors.grey.shade700,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              message,
              style: TextStyle(fontSize: 14, color: Colors.grey.shade500),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            ElevatedButton(onPressed: onRetry, child: const Text('Retry')),
          ],
        ),
      ),
    );
  }
}
