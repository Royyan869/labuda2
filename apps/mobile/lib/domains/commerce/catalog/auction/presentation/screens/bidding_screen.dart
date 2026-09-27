/// Bidding Screen
/// Shows all auctions where the user has placed bids
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/bidding_item.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/bidding_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/bidding_state.dart';
import 'package:labuda/shared/shared.dart';

/// Bidding Screen
///
/// Displays all auctions where the authenticated user has placed bids,
/// with status indicators (leading, outbid, won, lost, waiting_claim).
class BiddingScreen extends ConsumerStatefulWidget {
  const BiddingScreen({super.key});

  @override
  ConsumerState<BiddingScreen> createState() => _BiddingScreenState();
}

class _BiddingScreenState extends ConsumerState<BiddingScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadBidding());
  }

  void _loadBidding() {
    ref.read(biddingNotifierProvider.notifier).loadMyBidding();
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(biddingNotifierProvider);

    return Scaffold(
      appBar: AppBarCustom(title: 'My Bidding', showBackButton: true),
      body: RefreshIndicator(
        onRefresh: () => ref.read(biddingNotifierProvider.notifier).refresh(),
        child: _buildBody(state),
      ),
    );
  }

  Widget _buildBody(BiddingState state) {
    if (state is BiddingLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (state is BiddingError) {
      return _buildError(state.error);
    }

    if (state is BiddingData) {
      final result = state.result;

      if (result.items.isEmpty) {
        return _buildEmpty();
      }

      return Column(
        children: [
          // Summary stats
          _buildSummaryStats(result),
          // Bidding list
          Expanded(
            child: ListView.builder(
              padding: const EdgeInsets.symmetric(vertical: 8),
              itemCount: result.items.length,
              itemBuilder: (context, index) {
                final item = result.items[index];
                return BiddingItemCard(item: item);
              },
            ),
          ),
        ],
      );
    }

    return _buildEmpty();
  }

  Widget _buildSummaryStats(BiddingResult result) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: scheme.surface,
        border: Border(
          bottom: BorderSide(color: scheme.outlineVariant, width: 1),
        ),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceAround,
        children: [
          _StatItem(
            label: 'Active',
            value: result.activeCount.toString(),
            color: AppColors.statusSuccess,
          ),
          _StatItem(
            label: 'Won',
            value: result.wonCount.toString(),
            color: scheme.onSurfaceVariant,
          ),
          _StatItem(
            label: 'Lost',
            value: result.lostCount.toString(),
            color: scheme.error,
          ),
        ],
      ),
    );
  }

  Widget _buildError(String error) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.error_outline, size: 64, color: scheme.error),
          const SizedBox(height: 16),
          Text(
            'Error loading bidding data',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text(
            error,
            style: Theme.of(
              context,
            ).textTheme.bodyMedium?.copyWith(color: scheme.error),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 24),
          ElevatedButton(
            onPressed: _loadBidding,
            child: const Text('Try Again'),
          ),
        ],
      ),
    );
  }

  Widget _buildEmpty() {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.gavel_outlined,
            size: 64,
            color: scheme.onSurfaceVariant,
          ),
          const SizedBox(height: 16),
          Text(
            'No Bidding Activity',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text(
            'Start bidding on auctions to track your activity here',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: scheme.onSurfaceVariant,
            ),
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }
}

/// Stat item widget for summary
class _StatItem extends StatelessWidget {
  final String label;
  final String value;
  final Color color;

  const _StatItem({
    required this.label,
    required this.value,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Text(
          value,
          style: TextStyle(
            fontSize: 24,
            fontWeight: FontWeight.bold,
            color: color,
          ),
        ),
        const SizedBox(height: 4),
        Text(
          label,
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}

/// Bidding item card
class BiddingItemCard extends ConsumerWidget {
  final BiddingItem item;

  const BiddingItemCard({super.key, required this.item});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final dateFormat = DateFormat('MMM dd, yyyy • HH:mm');

    return InkWell(
      onTap: () => _navigateToAuction(context),
      child: Container(
        margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
        decoration: BoxDecoration(
          color: Theme.of(context).colorScheme.surface,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: _getStatusColor(
              Theme.of(context).colorScheme,
            ).withValues(alpha: 0.3),
            width: 1,
          ),
        ),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Title and status row
              Row(
                children: [
                  Expanded(
                    child: Text(
                      item.title,
                      style: Theme.of(context).textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  const SizedBox(width: 12),
                  _StatusChip(status: item.status),
                ],
              ),
              const SizedBox(height: 12),
              // Bid info row
              Row(
                children: [
                  Expanded(
                    child: _BidInfo(
                      label: 'Your Bid',
                      amount: item.yourLastBid,
                      isHighlight:
                          item.status == BiddingStatus.leading ||
                          item.status == BiddingStatus.waitingClaim,
                    ),
                  ),
                  const SizedBox(width: 24),
                  Expanded(
                    child: _BidInfo(
                      label: 'Current Bid',
                      amount: item.currentBid,
                      isHighlight: false,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              // End time row
              Row(
                children: [
                  Icon(
                    Icons.access_time,
                    size: 14,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                  const SizedBox(width: 4),
                  Text(
                    'Ends: ${dateFormat.format(item.endAt.toLocal())}',
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
              // STEP 2: WARNING DI BIDDING SCREEN (WAITING CLAIM)
              // BNR WARNING & TRUST SIGNAL - Urgent payment warning with trust impact
              if (item.status == BiddingStatus.waitingClaim) ...[
                const SizedBox(height: 10),
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 10,
                    vertical: 8,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.statusWarning.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(6),
                    border: Border.all(
                      color: AppColors.statusWarning.withValues(alpha: 0.3),
                      width: 1,
                    ),
                  ),
                  child: Row(
                    children: [
                      Container(
                        padding: const EdgeInsets.all(4),
                        decoration: BoxDecoration(
                          color: AppColors.statusWarning.withValues(
                            alpha: 0.15,
                          ),
                          shape: BoxShape.circle,
                        ),
                        child: const Icon(
                          Icons.warning_amber_rounded,
                          size: 14,
                          color: AppColors.statusWarning,
                        ),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              '⚠️ Segera selesaikan pembayaran',
                              style: Theme.of(context).textTheme.bodySmall
                                  ?.copyWith(
                                    color: Theme.of(
                                      context,
                                    ).colorScheme.onSurface,
                                    fontWeight: FontWeight.w600,
                                  ),
                            ),
                            const SizedBox(height: 2),
                            Text(
                              'Keterlambatan dapat memengaruhi kepercayaan akun',
                              style: Theme.of(context).textTheme.bodySmall
                                  ?.copyWith(
                                    color: Theme.of(
                                      context,
                                    ).colorScheme.onSurfaceVariant,
                                    fontSize: 10,
                                  ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  Color _getStatusColor(ColorScheme scheme) {
    switch (item.status) {
      case BiddingStatus.leading:
      case BiddingStatus.waitingClaim:
        return AppColors.statusSuccess;
      case BiddingStatus.outbid:
        return scheme.error;
      case BiddingStatus.won:
      case BiddingStatus.lost:
        return scheme.onSurfaceVariant;
    }
  }

  void _navigateToAuction(BuildContext context) {
    // Samakan dengan For Sale + entry lain: go_router push agar stack
    // terjaga. Navigator.pushNamed bypass go_router dan route tidak
    // terdaftar di Navigator biasa.
    context.push('/auction/${item.auctionId}');
  }
}

/// Bid info widget
class _BidInfo extends StatelessWidget {
  final String label;
  final int amount;
  final bool isHighlight;

  const _BidInfo({
    required this.label,
    required this.amount,
    required this.isHighlight,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 2),
        Text(
          'Rp ${formatGroupedAmount(amount)}',
          style: Theme.of(context).textTheme.titleSmall?.copyWith(
            fontWeight: FontWeight.w600,
            color: isHighlight
                ? Theme.of(context).colorScheme.secondary
                : Theme.of(context).colorScheme.onSurface,
          ),
        ),
      ],
    );
  }
}

/// Status chip widget
class _StatusChip extends StatelessWidget {
  final BiddingStatus status;

  const _StatusChip({required this.status});

  @override
  Widget build(BuildContext context) {
    final info = _getStatusInfo(context);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: info.backgroundColor,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        info.label,
        style: TextStyle(
          fontSize: 12,
          fontWeight: FontWeight.w600,
          color: info.color,
        ),
      ),
    );
  }

  _StatusInfo _getStatusInfo(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    switch (status) {
      case BiddingStatus.leading:
        return _StatusInfo(
          'Leading',
          AppColors.statusSuccess,
          AppColors.statusSuccess.withValues(alpha: 0.12),
        );
      case BiddingStatus.outbid:
        return _StatusInfo(
          'Outbid',
          scheme.error,
          scheme.error.withValues(alpha: 0.12),
        );
      case BiddingStatus.waitingClaim:
        return _StatusInfo(
          'Claim',
          AppColors.statusWarning,
          AppColors.statusWarning.withValues(alpha: 0.12),
        );
      case BiddingStatus.won:
        return _StatusInfo(
          'Won',
          scheme.onSurfaceVariant,
          scheme.surfaceContainerHighest,
        );
      case BiddingStatus.lost:
        return _StatusInfo(
          'Lost',
          scheme.onSurfaceVariant,
          scheme.surfaceContainerHighest,
        );
    }
  }
}

/// Status info helper class
class _StatusInfo {
  final String label;
  final Color color;
  final Color backgroundColor;

  const _StatusInfo(this.label, this.color, this.backgroundColor);
}
