import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/utils/app_formatters.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/create_auction_route_contract.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/seller_auctions_pager.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/shared/widgets/app_dialog.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/shared/widgets/loading_indicator.dart';
import 'package:labuda/shared/widgets/page_error_state.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/seller_management_row.dart';

class SellerAuctionsScreen extends ConsumerStatefulWidget {
  const SellerAuctionsScreen({super.key});

  @override
  ConsumerState<SellerAuctionsScreen> createState() =>
      _SellerAuctionsScreenState();
}

class _SellerAuctionsScreenState extends ConsumerState<SellerAuctionsScreen>
    with SingleTickerProviderStateMixin {
  /// Tab selection machinery only. `SellerAuctionsPagerState.activeFilter`
  /// remains the single business filter state; the tab merely selects it.
  late final TabController _tabController;

  @override
  void initState() {
    super.initState();
    // Mirror the existing pager filter (canonical default `null` = Semua).
    final initial = kSellerAuctionFilters.indexOf(
      ref.read(sellerAuctionsPagerProvider).activeFilter,
    );
    _tabController = TabController(
      length: kSellerAuctionFilters.length,
      initialIndex: initial < 0 ? 0 : initial,
      vsync: this,
    );
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  /// Two distinct empty meanings, one canonical renderer:
  /// - the seller has no auction at all → collection empty;
  /// - auctions exist but the active status tab matched none → filter empty
  ///   with a reset back to "Semua".
  Widget _buildEmptyState() {
    final state = ref.read(sellerAuctionsPagerProvider);
    final l10n = context.l10n;

    if (state.auctions.isNotEmpty) {
      return EmptyState(
        icon: Icons.filter_alt_off_outlined,
        title: l10n.emptySearchTitle,
        subtitle: l10n.emptySearchMessage,
        actionLabel: l10n.resetFilterAction,
        onAction: () {
          ref.read(sellerAuctionsPagerProvider.notifier).setFilter(null);
          setState(() => _tabController.index = 0);
        },
      );
    }

    return EmptyState(
      icon: Icons.gavel_outlined,
      title: l10n.emptyAuctionTitle,
      subtitle: l10n.emptySellerAuctionMessage,
    );
  }

  @override
  Widget build(BuildContext context) {
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
    final scheme = Theme.of(context).colorScheme;

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
        bottom: TabBar(
          controller: _tabController,
          isScrollable: true,
          tabs: [
            for (final filter in kSellerAuctionFilters)
              Tab(text: sellerAuctionFilterLabel(filter)),
          ],
          onTap: (index) => pager.setFilter(kSellerAuctionFilters[index]),
        ),
      ),
      // SAFE-AREA-31: the body content owns the bottom system inset —
      // this is a STANDALONE pushed route (auction_module MaterialPage),
      // so no shell bar owns it. FAB positioning stays the Scaffold
      // endFloat authority, measured outside this SafeArea.
      body: SafeArea(
        child: RefreshIndicator(
          onRefresh: pager.refresh,
          child: CustomScrollView(
            physics: const AlwaysScrollableScrollPhysics(),
            slivers: [
              // LOADING FOUNDATION (owner-locked):
              // - No auctions yet → first-load states only: LoadingIndicator,
              //   PageErrorState, or EmptyState.
              // - Auctions present → they stay visible during refresh; the
              //   update indicator and refresh failure render inline, never as
              //   full-page loading/error.
              if (pagerState.isInitialLoading && visibleAuctions.isEmpty)
                const SliverFillRemaining(
                  hasScrollBody: false,
                  child: Center(child: LoadingIndicator()),
                )
              else if (pagerState.initialError != null &&
                  pagerState.auctions.isEmpty)
                SliverFillRemaining(
                  hasScrollBody: false,
                  // CANONICAL page-level load error (PageErrorState): safe
                  // localized copy only — the raw pager initialError never
                  // reaches the screen. Retry re-executes the canonical
                  // initial load through the single retry authority.
                  child: PageErrorState(onRetry: pager.retryInitial),
                )
              else if (visibleAuctions.isEmpty)
                SliverFillRemaining(
                  hasScrollBody: false,
                  child: _buildEmptyState(),
                )
              else ...[
                if (pagerState.isRefreshing)
                  const SliverToBoxAdapter(
                    child: LinearProgressIndicator(minHeight: 2),
                  ),
                if (pagerState.refreshError != null)
                  SliverToBoxAdapter(child: _buildRefreshErrorBanner(pager)),
                SliverPadding(
                  padding: const EdgeInsets.fromLTRB(
                    AppMetrics.p16,
                    AppMetrics.p8,
                    AppMetrics.p16,
                    AppMetrics.p16,
                  ),
                  sliver: SliverList(
                    delegate: SliverChildBuilderDelegate((context, index) {
                      final auction = visibleAuctions[index];
                      // Rhythm lives in SellerManagementRow (bottom p12): the
                      // list adds no separators of its own.
                      return _SellerAuctionCard(
                        auction: auction,
                        currentUserId: currentUser.id,
                        onOpenDetail: () =>
                            context.push(RoutePaths.auctionDetail(auction.id)),
                        onEdit:
                            auction.status == AuctionStatus.scheduled &&
                                auction.sellerId == currentUser.id
                            ? () =>
                                  unawaited(_openEdit(context, pager, auction))
                            : null,
                        // Relist preview gate: ended with no winner. The
                        // backend re-checks bid/winner/order and may refuse.
                        onRelist:
                            auction.isRelistable &&
                                auction.sellerId == currentUser.id
                            ? () => unawaited(
                                _relistAuction(context, ref, pager, auction),
                              )
                            : null,
                        onCancel:
                            (auction.status == AuctionStatus.scheduled ||
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
                      );
                    }, childCount: visibleAuctions.length),
                  ),
                ),
              ],
              SliverToBoxAdapter(child: _buildFooter(pagerState, pager)),
            ],
          ),
        ),
      ),
      // Canonical page-level create entry (mirrors the For Sale management
      // page FAB). `stay` opts out of the global entry's Marketplace landing
      // so My Auctions remains active; the created auction becomes visible via
      // the notifier's canonical pager invalidation (E.3).
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => context.push(
          RoutePaths.createAuction,
          extra: const CreateAuctionRouteArgs.stay(),
        ),
        backgroundColor: scheme.primary,
        icon: Icon(Icons.add, color: scheme.onPrimary),
        label: Text('Buat Lelang', style: TextStyle(color: scheme.onPrimary)),
      ),
    );
  }

  Future<void> _openEdit(
    BuildContext context,
    SellerAuctionsPagerController pager,
    Auction auction,
  ) async {
    // Canonical edit route: the path identifies the auction, the loaded
    // entity travels as extra, and the form still resolves `true` when saved.
    final result = await context.push<bool>(
      RoutePaths.sellerAuctionEditPath(auction.id),
      extra: auction,
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
    final confirmed = await AppDialog.confirm(
      context: context,
      title: 'Batalkan lelang',
      message:
          'Batalkan lelang ini? Backend akan menentukan apakah tindakan ini diizinkan.',
      confirmLabel: 'Batalkan',
      cancelLabel: 'Batal',
      intent: AppDialogIntent.destructive,
    );
    if (!confirmed || !context.mounted) return;

    final success = await ref
        .read(auctionNotifierProvider.notifier)
        .cancelAuction(
          auctionId: auction.id,
          sellerId: currentUserId,
          reason: 'Seller cancelled from inventory',
        );
    if (success && context.mounted) {
      await pager.refresh();
    }
  }

  Future<void> _relistAuction(
    BuildContext context,
    WidgetRef ref,
    SellerAuctionsPagerController pager,
    Auction auction,
  ) async {
    // RELIST = REPUBLISH: the backend requires the full create-form payload
    // (fresh timing/pricing), so relist opens the form screen instead of a
    // body-less call. The form owns submit + error surfacing.
    final relisted = await context.push<bool>(
      RoutePaths.sellerAuctionRelistPath(auction.id),
      extra: auction,
    );
    if (!context.mounted) return;
    if (relisted == true) {
      await pager.refresh();
    }
  }

  /// Minimum bounded refresh-failure indication: persistent inline banner
  /// with safe localized copy ([pageErrorMessage]) and a retry action that
  /// re-executes the canonical pager refresh. Not a new foundation —
  /// composition of canonical tokens for this screen, matching the Home /
  /// Chat / Coin / Search refresh banners.
  Widget _buildRefreshErrorBanner(SellerAuctionsPagerController pager) {
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
            TextButton(onPressed: pager.refresh, child: Text(l10n.retryAction)),
          ],
        ),
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
        padding: EdgeInsets.only(bottom: AppMetrics.p24),
        child: Center(child: LoadingIndicator.small()),
      );
    }

    if (state.loadMoreError != null) {
      return Padding(
        padding: const EdgeInsets.fromLTRB(
          AppMetrics.p16,
          AppMetrics.p8,
          AppMetrics.p16,
          AppMetrics.p24,
        ),
        // Load-more failure keeps its distinct inline-row semantic (the
        // loaded rows stay above; only the next page failed), but the
        // presentation is controlled canonical copy + retry — the raw
        // pager loadMoreError never reaches the screen.
        child: _LoadMoreErrorRow(pager: pager),
      );
    }

    if (!state.hasMore || state.auctions.isEmpty) {
      return const SizedBox.shrink();
    }

    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p8,
        AppMetrics.p16,
        AppMetrics.p24,
      ),
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
  final VoidCallback? onEdit;
  final VoidCallback? onRelist;
  final VoidCallback? onCancel;

  const _SellerAuctionCard({
    required this.auction,
    required this.currentUserId,
    required this.onOpenDetail,
    required this.onEdit,
    required this.onRelist,
    required this.onCancel,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isOwner = auction.sellerId == currentUserId;
    // Single status-label authority: the canonical AuctionStatus getter.
    final displayStatus = auction.status.displayName;
    final needsAction = _needsAction(auction);
    final media = auction.media.isNotEmpty ? auction.media.first : null;
    final imageUrl = media?.posterUrl ?? media?.thumbnailUrl;
    final currentBid = auction.currentBid > 0
        ? auction.currentBid
        : auction.openingBid;

    return SellerManagementRow(
      key: ValueKey('seller-auction-card-${auction.id}'),
      onTap: onOpenDetail,
      leading: SellerManagementThumbnail(imageUrl: imageUrl),
      title: Column(
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
              if (isOwner && auction.status == AuctionStatus.scheduled)
                const _StatusChip(label: 'Bisa diedit'),
              if (isOwner && onRelist != null)
                const _StatusChip(label: 'Bisa direlist'),
            ],
          ),
        ],
      ),
      trailing: PopupMenuButton<String>(
        onSelected: (action) {
          switch (action) {
            case 'detail':
              onOpenDetail();
              break;
            case 'edit':
              onEdit?.call();
              break;
            case 'relist':
              onRelist?.call();
              break;
            case 'cancel':
              onCancel?.call();
              break;
          }
        },
        itemBuilder: (context) {
          final items = <PopupMenuEntry<String>>[
            const PopupMenuItem(value: 'detail', child: Text('Lihat detail')),
          ];
          if (onEdit != null) {
            items.add(const PopupMenuItem(value: 'edit', child: Text('Edit')));
          }
          if (onRelist != null) {
            items.add(
              const PopupMenuItem(value: 'relist', child: Text('Relist')),
            );
          }
          if (onCancel != null) {
            items.add(
              const PopupMenuItem(value: 'cancel', child: Text('Batalkan')),
            );
          }
          return items;
        },
      ),
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Wrap(
            spacing: 16,
            runSpacing: 8,
            children: [
              _MetaChip(
                icon: Icons.payments_outlined,
                label:
                    'Harga awal Rp ${formatGroupedAmount(auction.openingBid)}',
              ),
              _MetaChip(
                icon: Icons.trending_up_outlined,
                label: 'Terkini Rp ${formatGroupedAmount(currentBid)}',
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
                size: AppIconSize.inlineGlyph,
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
              'Menunggu penyelesaian dari pemenang',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ],
      ),
    );
  }

  bool _needsAction(Auction auction) {
    return auction.status == AuctionStatus.lapsed ||
        auction.status == AuctionStatus.scheduled ||
        auction.status == AuctionStatus.waitingSettlement;
  }

  String _timelineLabel(Auction auction) {
    switch (auction.status) {
      case AuctionStatus.lapsed:
        return 'Kadaluarsa — jadwalkan ulang';
      case AuctionStatus.scheduled:
        return 'Dimulai ${AppFormatters.formatShortDate(auction.startTime)}';
      case AuctionStatus.active:
        return 'Berakhir ${AppFormatters.formatShortDate(auction.endTime)}';
      case AuctionStatus.waitingSettlement:
        return 'Selesaikan sebelum ${AppFormatters.formatShortDate(auction.settlementDeadline)}';
      case AuctionStatus.ended:
        return 'Berakhir ${AppFormatters.formatShortDate(auction.endTime)}';
      case AuctionStatus.cancelled:
        return 'Riwayat ${AppFormatters.formatShortDate(auction.endTime)}';
    }
  }
}

class _StatusChip extends StatelessWidget {
  final String label;

  const _StatusChip({required this.label});

  @override
  Widget build(BuildContext context) {
    return Chip(label: Text(label), visualDensity: VisualDensity.compact);
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
        Icon(
          icon,
          size: AppIconSize.inlineGlyph,
          color: theme.colorScheme.onSurfaceVariant,
        ),
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

/// Compact controlled load-more failure row: same inline-row semantic as
/// before (loaded rows stay visible; only the next page failed), but safe
/// localized copy + retry instead of the purged raw-error renderer.
/// Distinct from [PageErrorState] on purpose — that authority owns the
/// full-page first-load failure, never an inline row.
class _LoadMoreErrorRow extends StatelessWidget {
  final SellerAuctionsPagerController pager;

  const _LoadMoreErrorRow({required this.pager});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final l10n = context.l10n;
    return Semantics(
      container: true,
      liveRegion: true,
      child: Container(
        width: double.infinity,
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
              Icons.expand_more,
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
            TextButton(
              onPressed: pager.retryLoadMore,
              child: Text(l10n.retryAction),
            ),
          ],
        ),
      ),
    );
  }
}
