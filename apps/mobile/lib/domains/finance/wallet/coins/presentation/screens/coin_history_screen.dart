/// Coin History Screen
///
/// Displays coin transaction history from the backend.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';
import 'package:labuda/domains/finance/wallet/coins/coins_di.dart';
import 'package:labuda/domains/finance/wallet/coins/domain/entities/coin_transaction.dart';
import 'package:labuda/domains/finance/wallet/coins/presentation/providers/coin_notifier.dart';
import 'package:labuda/domains/finance/wallet/coins/presentation/providers/coin_state.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/shared/widgets/loading_indicator.dart';
import 'package:labuda/shared/widgets/page_error_state.dart';

/// Screen for viewing coin transaction history
class CoinHistoryScreen extends ConsumerStatefulWidget {
  final String userId;

  const CoinHistoryScreen({super.key, required this.userId});

  @override
  ConsumerState<CoinHistoryScreen> createState() => _CoinHistoryScreenState();
}

class _CoinHistoryScreenState extends ConsumerState<CoinHistoryScreen> {
  int _currentPage = 1;

  /// Last-known-good transactions. Kept screen-local (bounded): the shared
  /// [CoinState] union cannot hold list + loading/error at once and is also
  /// consumed by checkout, so the history screen preserves its own snapshot
  /// to satisfy the Loading Foundation (refresh never wipes visible data).
  List<CoinTransaction> _lastTransactions = const [];
  bool _hasLoadedOnce = false;

  @override
  void initState() {
    super.initState();
    // Load transactions on init
    Future.microtask(() {
      ref.read(coinProvider.notifier).getTransactions(page: _currentPage);
    });
  }

  Future<void> _reload() async {
    _currentPage = 1;
    await ref.read(coinProvider.notifier).getTransactions(page: _currentPage);
  }

  @override
  Widget build(BuildContext context) {
    final coinState = ref.watch(coinProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Transaction History')),
      body: coinState.when(
        // Idle / not-yet-loaded is NEVER a successful empty: first-load
        // loading until the request settles.
        initial: () => _buildFirstLoad(),
        loading: () => _hasLoadedOnce && _lastTransactions.isNotEmpty
            ? _buildCachedList(isRefreshing: true)
            : _buildFirstLoad(),
        // Balance-only state carries no transaction list: same idle rule —
        // loading, never EmptyState.
        balanceLoaded: (_, _) => _hasLoadedOnce && _lastTransactions.isNotEmpty
            ? _buildCachedList(isRefreshing: false)
            : _buildFirstLoad(),
        transactionsLoaded: (transactions) {
          _lastTransactions = transactions;
          _hasLoadedOnce = true;
          if (transactions.isEmpty) {
            return _buildEmpty();
          }
          return _buildCachedList(isRefreshing: false);
        },
        // First-load failure → PageErrorState (safe localized copy; the raw
        // backend message never reaches the screen). Refresh failure with
        // cached data → cached list + inline indication.
        error: (_) => _hasLoadedOnce && _lastTransactions.isNotEmpty
            ? _buildCachedList(isRefreshing: false, refreshFailed: true)
            : _buildFirstLoadError(),
      ),
    );
  }

  /// First-load loading: the canonical [LoadingIndicator] as main content.
  Widget _buildFirstLoad() {
    return const Center(child: LoadingIndicator());
  }

  /// First-load error: the canonical [PageErrorState]. No technical detail.
  Widget _buildFirstLoadError() {
    return PageErrorState(
      onRetry: () {
        ref.read(coinProvider.notifier).getTransactions(page: _currentPage);
      },
    );
  }

  /// Successful zero-result: the ONLY place [EmptyState] may appear here.
  Widget _buildEmpty() {
    final l10n = context.l10n;
    return EmptyState(
      icon: Icons.receipt_long_outlined,
      title: l10n.emptyCollectionTitle,
      subtitle: l10n.emptyCollectionMessage,
    );
  }

  /// Cached data stays visible during reload/refresh failure, with an update
  /// indicator or an inline failure banner on top. Never swapped away.
  Widget _buildCachedList({
    required bool isRefreshing,
    bool refreshFailed = false,
  }) {
    return Column(
      children: [
        if (isRefreshing) const LinearProgressIndicator(minHeight: 2),
        if (refreshFailed) _buildRefreshErrorBanner(),
        Expanded(
          child: _TransactionList(
            transactions: _lastTransactions,
            onRefresh: _reload,
            onLoadMore: () {
              _currentPage++;
              ref
                  .read(coinProvider.notifier)
                  .getTransactions(page: _currentPage);
            },
          ),
        ),
      ],
    );
  }

  /// Minimum bounded refresh-failure indication: persistent inline banner
  /// with safe localized copy and a retry action. Not a new foundation.
  Widget _buildRefreshErrorBanner() {
    final scheme = Theme.of(context).colorScheme;
    final l10n = context.l10n;
    return Semantics(
      container: true,
      liveRegion: true,
      child: Container(
        width: double.infinity,
        margin: const EdgeInsets.fromLTRB(
          AppMetrics.p16,
          AppMetrics.p12,
          AppMetrics.p16,
          AppMetrics.p4,
        ),
        padding: const EdgeInsets.symmetric(
          horizontal: AppMetrics.p12,
          vertical: AppMetrics.p8,
        ),
        decoration: BoxDecoration(
          color: scheme.errorContainer,
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(color: scheme.error),
        ),
        child: Row(
          children: [
            Icon(
              Icons.refresh_outlined,
              size: AppIconSize.action,
              color: scheme.onErrorContainer,
            ),
            const SizedBox(width: AppMetrics.p8),
            Expanded(
              child: Text(
                l10n.pageErrorMessage,
                style: context.typeRoles.bodyDense.copyWith(
                  color: scheme.onErrorContainer,
                ),
              ),
            ),
            TextButton(onPressed: _reload, child: Text(l10n.retryAction)),
          ],
        ),
      ),
    );
  }
}

class _TransactionList extends StatelessWidget {
  final List<CoinTransaction> transactions;
  final Future<void> Function()? onRefresh;
  final VoidCallback onLoadMore;

  const _TransactionList({
    required this.transactions,
    this.onRefresh,
    required this.onLoadMore,
  });

  @override
  Widget build(BuildContext context) {
    return RefreshIndicator(
      onRefresh: () async {
        onRefresh?.call();
      },
      child: ListView.separated(
        itemCount: transactions.length + 1,
        separatorBuilder: (_, _) => const Divider(height: 1),
        itemBuilder: (context, index) {
          if (index == transactions.length) {
            // Load more indicator
            return Padding(
              padding: const EdgeInsets.all(AppMetrics.p16),
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
        const TimeFormatService().formatTimeAgo(transaction.createdAt),
        style: context.typeRoles.labelMicro.copyWith(
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        ),
      ),
      trailing: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          Text(
            '${isEarn ? '+' : '-'}${transaction.amount}',
            style: context.typeRoles.titleCompact.copyWith(
              color: iconColor,
              fontWeight: FontWeight.bold,
            ),
          ),
          Text(
            'Balance: ${transaction.balanceAfter}',
            style: context.typeRoles.labelMicro.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}
