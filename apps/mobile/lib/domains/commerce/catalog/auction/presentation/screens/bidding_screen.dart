/// Bidding Screen
/// Shows all auctions where the user has placed bids
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';
import 'package:labuda/core/core.dart';
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
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        border: Border(
          bottom: BorderSide(color: Theme.of(context).dividerColor, width: 1),
        ),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceAround,
        children: [
          _StatItem(
            label: 'Active',
            value: result.activeCount.toString(),
            color: AppColors.successGreen,
          ),
          _StatItem(
            label: 'Won',
            value: result.wonCount.toString(),
            color: AppColors.neutralGray500,
          ),
          _StatItem(
            label: 'Lost',
            value: result.lostCount.toString(),
            color: AppColors.statusError,
          ),
        ],
      ),
    );
  }

  Widget _buildError(String error) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(
            Icons.error_outline,
            size: 64,
            color: AppColors.statusError,
          ),
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
            ).textTheme.bodyMedium?.copyWith(color: AppColors.statusError),
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
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.gavel_outlined, size: 64, color: AppColors.neutralGray400),
          const SizedBox(height: 16),
          Text(
            'No Bidding Activity',
            style: Theme.of(
              context,
            ).textTheme.titleLarge?.copyWith(color: AppColors.neutralGray600),
          ),
          const SizedBox(height: 8),
          Text(
            'Start bidding on auctions to track your activity here',
            style: Theme.of(
              context,
            ).textTheme.bodyMedium?.copyWith(color: AppColors.neutralGray500),
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
          style: Theme.of(
            context,
          ).textTheme.bodySmall?.copyWith(color: AppColors.neutralGray500),
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
    final currencyFormat = NumberFormat.currency(
      symbol: 'Rp ',
      decimalDigits: 0,
    );
    final dateFormat = DateFormat('MMM dd, yyyy • HH:mm');

    return InkWell(
      onTap: () => _navigateToAuction(context),
      child: Container(
        margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
        decoration: BoxDecoration(
          color: Theme.of(context).colorScheme.surface,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: _getStatusColor().withValues(alpha: 0.3),
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
                      currencyFormat: currencyFormat,
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
                      currencyFormat: currencyFormat,
                      isHighlight: false,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              // End time row
              Row(
                children: [
                  const Icon(
                    Icons.access_time,
                    size: 14,
                    color: AppColors.neutralGray500,
                  ),
                  const SizedBox(width: 4),
                  Text(
                    'Ends: ${dateFormat.format(item.endAt.toLocal())}',
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: AppColors.neutralGray500,
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
                    color: const Color(0xFFFFF7ED), // Light orange background
                    borderRadius: BorderRadius.circular(6),
                    border: Border.all(
                      color: const Color(0xFFF97316).withValues(alpha: 0.3),
                      width: 1,
                    ),
                  ),
                  child: Row(
                    children: [
                      Container(
                        padding: const EdgeInsets.all(4),
                        decoration: BoxDecoration(
                          color: const Color(
                            0xFFF97316,
                          ).withValues(alpha: 0.15),
                          shape: BoxShape.circle,
                        ),
                        child: const Icon(
                          Icons.warning_amber_rounded,
                          size: 14,
                          color: Color(0xFFF97316),
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
                                    color: const Color(0xFF9A3412),
                                    fontWeight: FontWeight.w600,
                                  ),
                            ),
                            const SizedBox(height: 2),
                            Text(
                              'Keterlambatan dapat memengaruhi kepercayaan akun',
                              style: Theme.of(context).textTheme.bodySmall
                                  ?.copyWith(
                                    color: const Color(0xFF9A3412),
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

  Color _getStatusColor() {
    switch (item.status) {
      case BiddingStatus.leading:
      case BiddingStatus.waitingClaim:
        return AppColors.successGreen;
      case BiddingStatus.outbid:
        return AppColors.statusError;
      case BiddingStatus.won:
      case BiddingStatus.lost:
        return AppColors.neutralGray500;
    }
  }

  void _navigateToAuction(BuildContext context) {
    Navigator.of(context).pushNamed('/auction/${item.auctionId}');
  }
}

/// Bid info widget
class _BidInfo extends StatelessWidget {
  final String label;
  final double amount;
  final NumberFormat currencyFormat;
  final bool isHighlight;

  const _BidInfo({
    required this.label,
    required this.amount,
    required this.currencyFormat,
    required this.isHighlight,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: Theme.of(
            context,
          ).textTheme.bodySmall?.copyWith(color: AppColors.neutralGray500),
        ),
        const SizedBox(height: 2),
        Text(
          currencyFormat.format(amount),
          style: Theme.of(context).textTheme.titleSmall?.copyWith(
            fontWeight: FontWeight.w600,
            color: isHighlight
                ? AppColors.primaryBlue
                : AppColors.neutralGray900,
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
    final info = _getStatusInfo();

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

  _StatusInfo _getStatusInfo() {
    switch (status) {
      case BiddingStatus.leading:
        return _StatusInfo(
          'Leading',
          AppColors.successGreen,
          AppColors.neutralGray50,
        );
      case BiddingStatus.outbid:
        return _StatusInfo(
          'Outbid',
          AppColors.statusError,
          AppColors.neutralGray100,
        );
      case BiddingStatus.waitingClaim:
        return _StatusInfo(
          'Claim',
          AppColors.statusWarning,
          AppColors.neutralGray100,
        );
      case BiddingStatus.won:
        return _StatusInfo(
          'Won',
          AppColors.neutralGray600,
          AppColors.neutralGray100,
        );
      case BiddingStatus.lost:
        return _StatusInfo(
          'Lost',
          AppColors.neutralGray500,
          AppColors.neutralGray100,
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
