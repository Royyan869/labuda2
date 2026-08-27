import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/seller_auctions_pager.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/seller_auction_draft_edit_screen.dart';

class SellerAuctionsScreen extends ConsumerWidget {
  const SellerAuctionsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);
    final pagerState = ref.watch(sellerAuctionsPagerProvider);
    final pager = ref.read(sellerAuctionsPagerProvider.notifier);

    if (authState is! AuthStateAuthenticated) {
      return Scaffold(
        appBar: AppBar(title: const Text('Lelang Saya')),
        body: const Center(child: Text('Silakan login untuk melanjutkan.')),
      );
    }

    final currentUser = authState.user;
    final visibleAuctions = pagerState.visibleAuctions;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Lelang Saya'),
        actions: [
          IconButton(
            tooltip: 'Segarkan',
            onPressed: pager.refresh,
            icon: const Icon(Icons.refresh_outlined),
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: pager.refresh,
        child: CustomScrollView(
          physics: const AlwaysScrollableScrollPhysics(),
          slivers: [
            SliverToBoxAdapter(
              child: _buildFilterStrip(context, ref, pagerState),
            ),
            if (pagerState.isInitialLoading && visibleAuctions.isEmpty)
              const SliverFillRemaining(
                hasScrollBody: false,
                child: Center(child: CircularProgressIndicator()),
              )
            else if (pagerState.initialError != null &&
                pagerState.auctions.isEmpty)
              SliverFillRemaining(
                hasScrollBody: false,
                child: _ErrorState(
                  title: 'Gagal memuat lelang',
                  message: pagerState.initialError!,
                  onRetry: pager.loadInitial,
                ),
              )
            else if (visibleAuctions.isEmpty)
              const SliverFillRemaining(
                hasScrollBody: false,
                child: _EmptyState(
                  title: 'Belum ada lelang',
                  message:
                      'Seller ini belum memiliki lelang pada filter yang dipilih.',
                  icon: Icons.gavel_outlined,
                ),
              )
            else
              SliverPadding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
                sliver: SliverList(
                  delegate: SliverChildBuilderDelegate(
                    (context, index) {
                      final auction = visibleAuctions[index];
                      return Padding(
                        padding: EdgeInsets.only(
                          bottom: index == visibleAuctions.length - 1 ? 0 : 12,
                        ),
                        child: _SellerAuctionCard(
                          auction: auction,
                          currentUserId: currentUser.id,
                          onOpenDetail: () => context.push(
                            RoutePaths.auctionDetail(auction.id),
                          ),
                          onEditDraft: auction.status == AuctionStatus.draft &&
                                  auction.sellerId == currentUser.id
                              ? () => unawaited(
                                  _openDraftEdit(
                                    context,
                                    pager,
                                    auction,
                                  ),
                                )
                              : null,
                          onCancel: (auction.status == AuctionStatus.draft ||
                                  auction.status == AuctionStatus.scheduled ||
                                  auction.status == AuctionStatus.active) &&
                                  auction.sellerId == currentUser.id
                              ? () => unawaited(
                                  _cancelAuction(
                                    context,
                                    ref,
                                    pager,
                                    auction,
                                    currentUser.id,
                                  ),
                                )
                              : null,
                        ),
                      );
                    },
                    childCount: visibleAuctions.length,
                  ),
                ),
              ),
            SliverToBoxAdapter(
              child: _buildFooter(pagerState, pager),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _openDraftEdit(
    BuildContext context,
    SellerAuctionsPagerController pager,
    Auction auction,
  ) async {
    final result = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (context) => SellerAuctionDraftEditScreen(auction: auction),
      ),
    );
    if (result == true && context.mounted) {
      await pager.refresh();
    }
  }

  Future<void> _cancelAuction(
    BuildContext context,
    WidgetRef ref,
    SellerAuctionsPagerController pager,
    Auction auction,
    String currentUserId,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Batalkan lelang'),
        content: const Text(
          'Batalkan lelang ini? Backend akan menentukan apakah tindakan ini diizinkan.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('Batal'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('Batalkan'),
          ),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) return;

    final success = await ref.read(auctionNotifierProvider.notifier).cancelAuction(
          auctionId: auction.id,
          sellerId: currentUserId,
          reason: 'Seller cancelled from inventory',
        );
    if (success && context.mounted) {
      await pager.refresh();
    }
  }

  Widget _buildFilterStrip(
    BuildContext context,
    WidgetRef ref,
    SellerAuctionsPagerState state,
  ) {
    final pager = ref.read(sellerAuctionsPagerProvider.notifier);
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
      child: Wrap(
        spacing: 8,
        runSpacing: 8,
        children: [
          for (final filter in SellerAuctionFilter.values)
            ChoiceChip(
              label: Text(filter.label),
              selected: state.activeFilter == filter,
              onSelected: (_) => pager.setFilter(filter),
            ),
        ],
      ),
    );
  }

  Widget _buildFooter(
    SellerAuctionsPagerState state,
    SellerAuctionsPagerController pager,
  ) {
    final hasItems = state.visibleAuctions.isNotEmpty;
    if (!hasItems && state.initialError == null && !state.isInitialLoading) {
      return const SizedBox.shrink();
    }

    if (state.isLoadMoreLoading) {
      return const Padding(
        padding: EdgeInsets.only(bottom: 24),
        child: Center(child: CircularProgressIndicator()),
      );
    }

    if (state.loadMoreError != null) {
      return Padding(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
        child: _ErrorState(
          title: 'Gagal memuat halaman berikutnya',
          message: state.loadMoreError!,
          onRetry: pager.retryLoadMore,
          compact: true,
        ),
      );
    }

    if (!state.hasMore || state.auctions.isEmpty) {
      return const SizedBox.shrink();
    }

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      child: OutlinedButton.icon(
        onPressed: pager.loadMore,
        icon: const Icon(Icons.expand_more),
        label: const Text('Muat lebih banyak'),
      ),
    );
  }
}

class _SellerAuctionCard extends StatelessWidget {
  final Auction auction;
  final String currentUserId;
  final VoidCallback onOpenDetail;
  final VoidCallback? onEditDraft;
  final VoidCallback? onCancel;

  const _SellerAuctionCard({
    required this.auction,
    required this.currentUserId,
    required this.onOpenDetail,
    required this.onEditDraft,
    required this.onCancel,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isOwner = auction.sellerId == currentUserId;
    final displayStatus = _statusLabel(auction);
    final needsAction = _needsAction(auction);
    final media = auction.media.isNotEmpty ? auction.media.first : null;
    final imageUrl = media?.posterUrl ?? media?.thumbnailUrl;
    final currentBid = auction.currentBid > 0
        ? auction.currentBid
        : auction.openingBid;

    return Card(
      key: ValueKey('seller-auction-card-${auction.id}'),
      elevation: 0,
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onOpenDetail,
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _AuctionThumbnail(imageUrl: imageUrl),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          auction.title,
                          style: theme.textTheme.titleMedium?.copyWith(
                            fontWeight: FontWeight.w700,
                          ),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                        const SizedBox(height: 8),
                        Wrap(
                          spacing: 8,
                          runSpacing: 8,
                          children: [
                            _StatusChip(label: displayStatus),
                            if (needsAction) const _ActionChip(),
                            if (isOwner &&
                                auction.status == AuctionStatus.draft)
                              const _StatusChip(label: 'Bisa diedit'),
                          ],
                        ),
                      ],
                    ),
                  ),
                  PopupMenuButton<String>(
                    onSelected: (action) {
                      switch (action) {
                        case 'detail':
                          onOpenDetail();
                          break;
                        case 'edit':
                          onEditDraft?.call();
                          break;
                        case 'cancel':
                          onCancel?.call();
                          break;
                      }
                    },
                    itemBuilder: (context) {
                      final items = <PopupMenuEntry<String>>[
                        const PopupMenuItem(
                          value: 'detail',
                          child: Text('Lihat detail'),
                        ),
                      ];
                      if (onEditDraft != null) {
                        items.add(
                          const PopupMenuItem(
                            value: 'edit',
                            child: Text('Edit draft'),
                          ),
                        );
                      }
                      if (onCancel != null) {
                        items.add(
                          const PopupMenuItem(
                            value: 'cancel',
                            child: Text('Batalkan'),
                          ),
                        );
                      }
                      return items;
                    },
                  ),
                ],
              ),
              const SizedBox(height: 12),
              Wrap(
                spacing: 16,
                runSpacing: 8,
                children: [
                  _MetaChip(
                    icon: Icons.payments_outlined,
                    label: 'Harga awal Rp ${auction.openingBid}',
                  ),
                  _MetaChip(
                    icon: Icons.trending_up_outlined,
                    label: 'Terkini Rp $currentBid',
                  ),
                  _MetaChip(
                    icon: Icons.emoji_events_outlined,
                    label: '${auction.totalBidders} bid',
                  ),
                ],
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Icon(
                    auction.status == AuctionStatus.active
                        ? Icons.access_time_outlined
                        : Icons.event_outlined,
                    size: 16,
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      _timelineLabel(auction),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ),
                ],
              ),
              if (auction.status == AuctionStatus.waitingSettlement) ...[
                const SizedBox(height: 8),
                Text(
                  auction.winnerUsername == null
                      ? 'Menunggu penyelesaian dari pemenang'
                      : 'Pemenang: ${auction.winnerUsername}',
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  String _statusLabel(Auction auction) {
    switch (auction.status) {
      case AuctionStatus.draft:
        return 'Draft';
      case AuctionStatus.scheduled:
        return 'Terjadwal';
      case AuctionStatus.active:
        return 'Aktif';
      case AuctionStatus.waitingSettlement:
        return 'Menunggu Penyelesaian';
      case AuctionStatus.ended:
        return auction.winnerId == null ? 'Selesai' : 'Selesai';
      case AuctionStatus.expiredBNR:
        return 'Selesai';
      case AuctionStatus.cancelled:
        return 'Selesai';
    }
  }

  bool _needsAction(Auction auction) {
    return auction.status == AuctionStatus.draft ||
        auction.status == AuctionStatus.scheduled ||
        auction.status == AuctionStatus.waitingSettlement;
  }

  String _timelineLabel(Auction auction) {
    switch (auction.status) {
      case AuctionStatus.draft:
        return 'Draft tersimpan';
      case AuctionStatus.scheduled:
        return 'Dimulai ${_formatDate(auction.startTime)}';
      case AuctionStatus.active:
        return 'Berakhir ${_formatDate(auction.endTime)}';
      case AuctionStatus.waitingSettlement:
        return auction.settlementDeadline == null
            ? 'Menunggu penyelesaian'
            : 'Selesaikan sebelum ${_formatDate(auction.settlementDeadline!)}';
      case AuctionStatus.ended:
        return 'Berakhir ${_formatDate(auction.endTime)}';
      case AuctionStatus.expiredBNR:
      case AuctionStatus.cancelled:
        return 'Riwayat ${_formatDate(auction.endTime)}';
    }
  }

  String _formatDate(DateTime value) {
    return '${value.day.toString().padLeft(2, '0')}/${value.month.toString().padLeft(2, '0')}/${value.year}';
  }
}

class _AuctionThumbnail extends StatelessWidget {
  final String? imageUrl;

  const _AuctionThumbnail({required this.imageUrl});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return ClipRRect(
      borderRadius: BorderRadius.circular(12),
      child: Container(
        width: 88,
        height: 88,
        color: theme.colorScheme.surfaceContainerHighest,
        child: imageUrl == null
            ? Icon(
                Icons.image_outlined,
                color: theme.colorScheme.onSurfaceVariant,
              )
            : Image.network(imageUrl!, fit: BoxFit.cover),
      ),
    );
  }
}

class _StatusChip extends StatelessWidget {
  final String label;

  const _StatusChip({required this.label});

  @override
  Widget build(BuildContext context) {
    return Chip(
      label: Text(label),
      visualDensity: VisualDensity.compact,
    );
  }
}

class _ActionChip extends StatelessWidget {
  const _ActionChip();

  @override
  Widget build(BuildContext context) {
    return Chip(
      label: const Text('Butuh tindakan'),
      visualDensity: VisualDensity.compact,
      backgroundColor: Theme.of(context).colorScheme.tertiaryContainer,
    );
  }
}

class _MetaChip extends StatelessWidget {
  final IconData icon;
  final String label;

  const _MetaChip({required this.icon, required this.label});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 16, color: theme.colorScheme.onSurfaceVariant),
        const SizedBox(width: 4),
        Text(
          label,
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}

class _ErrorState extends StatelessWidget {
  final String title;
  final String message;
  final VoidCallback onRetry;
  final bool compact;

  const _ErrorState({
    required this.title,
    required this.message,
    required this.onRetry,
    this.compact = false,
  });

  @override
  Widget build(BuildContext context) {
    final padding = compact ? 12.0 : 24.0;
    return Padding(
      padding: EdgeInsets.all(padding),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.error_outline,
            size: compact ? 32 : 48,
          ),
          const SizedBox(height: 12),
          Text(title, style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 8),
          Text(
            message,
            textAlign: TextAlign.center,
            style: Theme.of(context).textTheme.bodyMedium,
          ),
          const SizedBox(height: 12),
          OutlinedButton(onPressed: onRetry, child: const Text('Coba lagi')),
        ],
      ),
    );
  }
}

class _EmptyState extends StatelessWidget {
  final String title;
  final String message;
  final IconData icon;

  const _EmptyState({
    required this.title,
    required this.message,
    required this.icon,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, size: 64, color: theme.colorScheme.onSurfaceVariant),
            const SizedBox(height: 16),
            Text(
              title,
              style: theme.textTheme.titleLarge?.copyWith(
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              message,
              textAlign: TextAlign.center,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
