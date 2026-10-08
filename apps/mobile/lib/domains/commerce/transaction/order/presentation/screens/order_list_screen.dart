import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart';

/// Order List Screen - Daftar pesanan untuk buyer dan seller
class OrderListScreen extends ConsumerStatefulWidget {
  final bool isSeller;

  const OrderListScreen({super.key, this.isSeller = false});

  @override
  ConsumerState<OrderListScreen> createState() => _OrderListScreenState();
}

class _OrderListScreenState extends ConsumerState<OrderListScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 5, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return PopScope(
      canPop: true,
      child: Scaffold(
        // Page canvas — canonical lowest tone in both modes (checkout precedent).
        backgroundColor: colorScheme.surfaceContainerLowest,
        appBar: AppBar(
          title: Text(widget.isSeller ? 'Incoming Orders' : 'My Orders'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back, semanticLabel: 'Kembali'),
            onPressed: () => Navigator.of(context).pop(),
          ),
          bottom: TabBar(
            controller: _tabController,
            isScrollable: true,
            tabs: const [
              Tab(text: 'All'),
              Tab(text: 'Pending'),
              Tab(text: 'Paid'),
              Tab(text: 'Shipped'),
              Tab(text: 'Completed'),
            ],
          ),
        ),
        // SAFE-AREA-28 — the ONE canonical bottom system-window authority
        // on this standalone (shell-less) route: the body SafeArea wraps
        // the TabBarView so every tab's scroll viewport ends exactly at
        // the system-region start at every inset.
        body: SafeArea(
          child: TabBarView(
            controller: _tabController,
            children: [
              _buildOrderList(null),
              _buildOrderList(OrderStatus.pending),
              _buildOrderList(OrderStatus.paid),
              _buildOrderList(OrderStatus.shipped),
              _buildOrderList(OrderStatus.completed),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildOrderList(OrderStatus? status) {
    // Use centralized provider (TANGGUNG_JAWAB_MODUL compliance)
    final currentUser = ref.watch(authenticatedUserProvider);

    if (currentUser == null) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.login, size: AppIconSize.display),
            const SizedBox(height: 16),
            const Text('Please log in first'),
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('Back'),
            ),
          ],
        ),
      );
    }

    // Canonical list authority: the per-role live-stream providers.
    // `value` is the last-known-good collection — present once the first
    // emission settles, including during a resubscribe and after a failed
    // resubscribe, where the previous value is kept.
    final ordersAsync = widget.isSeller
        ? ref.watch(
            watchSellerOrdersProvider(sellerId: currentUser.id, status: status),
          )
        : ref.watch(
            watchBuyerOrdersProvider(buyerId: currentUser.id, status: status),
          );
    final orders = ordersAsync.value ?? const <Order>[];

    // LOADING FOUNDATION (owner-locked):
    // - No orders yet → first-load states only: LoadingIndicator,
    //   PageErrorState, or EmptyState.
    // - Orders present → they stay visible across resubscribe/refresh; the
    //   update indicator and refresh failure render inline, never as
    //   full-page loading/error. The stream otherwise pushes live updates
    //   itself — no duplicate polling layer exists on this surface.
    return RefreshIndicator(
      onRefresh: () => _reload(status),
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          if (ordersAsync.isLoading && orders.isEmpty)
            // First emission pending with no data → LoadingIndicator. Never
            // EmptyState (not yet loaded) and never a raw spinner.
            const SliverFillRemaining(
              hasScrollBody: false,
              child: Center(child: LoadingIndicator()),
            )
          else if (ordersAsync.hasError && orders.isEmpty)
            // CANONICAL page-level load error (PageErrorState): safe
            // localized copy only; the raw provider error never reaches
            // the screen. Retry resubscribes the canonical stream.
            SliverFillRemaining(
              hasScrollBody: false,
              child: PageErrorState(onRetry: () => _reload(status)),
            )
          else if (orders.isEmpty)
            SliverFillRemaining(
              hasScrollBody: false,
              child: _buildEmptyState(context, widget.isSeller),
            )
          else ...[
            // Resubscribe/refresh with existing data: rows stay, update
            // indication on top.
            if (ordersAsync.isLoading)
              const SliverToBoxAdapter(
                child: LinearProgressIndicator(minHeight: 2),
              ),
            // Resubscribe/refresh failure: rows stay, inline banner with
            // retry that re-executes the canonical reload. Never a
            // full-page error here, and never a silent slide into empty.
            if (ordersAsync.hasError)
              SliverToBoxAdapter(child: _buildRefreshErrorBanner(status)),
            SliverPadding(
              padding: const EdgeInsets.all(core.AppMetrics.p16),
              sliver: SliverList(
                delegate: SliverChildBuilderDelegate((context, index) {
                  return _buildOrderCard(orders[index]);
                }, childCount: orders.length),
              ),
            ),
          ],
        ],
      ),
    );
  }

  /// Single canonical reload for this surface: initial-load retry,
  /// pull-to-refresh, and refresh-banner retry all resubscribe the active
  /// tab's stream. Failure is never rethrown or rendered raw — it stays in
  /// the provider state and renders as [PageErrorState] (no data yet) or
  /// the inline refresh banner (existing data preserved).
  Future<void> _reload(OrderStatus? status) async {
    final currentUser = ref.read(authenticatedUserProvider);
    if (currentUser == null) return;
    final pending = widget.isSeller
        ? watchSellerOrdersProvider(
            sellerId: currentUser.id,
            status: status,
          ).future
        : watchBuyerOrdersProvider(
            buyerId: currentUser.id,
            status: status,
          ).future;
    try {
      // The reloaded collection reaches the screen through the provider
      // state, not through this future — `.then((_) {})` adapts it to
      // `Future<void>` so the `unused_result` contract is satisfied while
      // failures still propagate to the `catch` below.
      await ref.refresh(pending).then((_) {});
    } catch (_) {
      // No-op: the failure remains observable via async.hasError with the
      // last-known-good collection preserved in async.value.
    }
  }

  /// Minimum bounded refresh-failure indication: persistent inline banner
  /// with safe localized copy and a retry action. Not a new foundation —
  /// composition of canonical tokens for this screen, matching the
  /// established refresh banners.
  Widget _buildRefreshErrorBanner(OrderStatus? status) {
    final colorScheme = Theme.of(context).colorScheme;
    final l10n = context.l10n;
    return Semantics(
      container: true,
      liveRegion: true,
      child: Container(
        width: double.infinity,
        margin: const EdgeInsets.fromLTRB(
          core.AppMetrics.p16,
          core.AppMetrics.p12,
          core.AppMetrics.p16,
          core.AppMetrics.p4,
        ),
        padding: const EdgeInsets.symmetric(
          horizontal: core.AppMetrics.p12,
          vertical: core.AppMetrics.p8,
        ),
        decoration: BoxDecoration(
          color: colorScheme.errorContainer,
          borderRadius: BorderRadius.circular(core.AppShape.r12),
          border: Border.all(color: colorScheme.error),
        ),
        child: Row(
          children: [
            Icon(
              Icons.refresh_outlined,
              size: AppIconSize.action,
              color: colorScheme.onErrorContainer,
            ),
            const SizedBox(width: core.AppMetrics.p8),
            Expanded(
              child: Text(
                l10n.pageErrorMessage,
                style: context.typeRoles.bodyDense.copyWith(
                  color: colorScheme.onErrorContainer,
                ),
              ),
            ),
            TextButton(
              onPressed: () => _reload(status),
              child: Text(l10n.retryAction),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildOrderCard(Order order) {
    final colorScheme = Theme.of(context).colorScheme;
    // The GET /orders list surface carries no line items (items[] is emitted
    // only by GET /orders/:id), so the tile degrades to a neutral label rather
    // than inventing an item name or crashing on `.first`.
    final firstItem = order.items.isEmpty ? null : order.items.first;

    return GestureDetector(
      onTap: () => context.push(RoutePaths.orderDetailPath(order.id)),
      child: Container(
        margin: const EdgeInsets.only(bottom: core.AppMetrics.p12),
        padding: const EdgeInsets.all(core.AppMetrics.p16),
        decoration: BoxDecoration(
          color: colorScheme.surface,
          borderRadius: BorderRadius.circular(core.AppShape.r12),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Header
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Expanded(
                  child: Text(
                    order.id.substring(0, 8).toUpperCase(),
                    style: context.typeRoles.labelMicro.copyWith(
                      fontWeight: FontWeight.bold,
                      color: colorScheme.onSurface,
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                // Status Badge
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: core.AppMetrics.p8,
                    vertical: core.AppMetrics.p4,
                  ),
                  decoration: BoxDecoration(
                    color: _getStatusColor(order.status).withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(core.AppShape.r4),
                  ),
                  child: Text(
                    _getStatusLabel(order.status),
                    style: context.typeRoles.labelMicro.copyWith(
                      fontWeight: FontWeight.w600,
                      color: _getStatusColor(order.status),
                    ),
                  ),
                ),
                // Overdue Badge - OVERDUE ENFORCEMENT CLOSURE
                if (order.isOverdue == true &&
                    order.status == OrderStatus.paid) ...[
                  const SizedBox(width: 4),
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: core.AppMetrics.p8,
                      vertical: core.AppMetrics.p4,
                    ),
                    decoration: BoxDecoration(
                      color: _getOverdueBadgeColor(
                        order.overdueTier,
                      ).withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(core.AppShape.r4),
                      border: Border.all(
                        color: _getOverdueBadgeColor(order.overdueTier),
                        width: 1,
                      ),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.warning_amber_rounded,
                          size: AppIconSize.inlineGlyph,
                          color: _getOverdueBadgeColor(order.overdueTier),
                        ),
                        const SizedBox(width: 2),
                        Text(
                          _getOverdueBadgeLabel(order.overdueTier),
                          style: context.typeRoles.labelMicro.copyWith(
                            fontWeight: FontWeight.w600,
                            color: _getOverdueBadgeColor(order.overdueTier),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ],
            ),
            const SizedBox(height: 12),

            // Item Info
            Row(
              children: [
                ClipRRect(
                  borderRadius: BorderRadius.circular(core.AppShape.r8),
                  child: AppImage(
                    imageUrl: firstItem?.forSaleImage,
                    width: 60,
                    height: 60,
                    fit: BoxFit.cover,
                    backgroundColor: colorScheme.surfaceContainerHighest,
                    errorWidget: Container(
                      width: 60,
                      height: 60,
                      color: colorScheme.surfaceContainerHighest,
                      child: const Icon(Icons.image_not_supported),
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        firstItem?.forSaleName ?? 'Pesanan',
                        style: context.typeRoles.titleCompact.copyWith(
                          fontWeight: FontWeight.w600,
                          color: colorScheme.onSurface,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      const SizedBox(height: 4),
                      Text(
                        order.items.isEmpty
                            ? 'Lihat detail pesanan'
                            : '${order.items.length} item${order.items.length > 1 ? 's' : ''}',
                        style: context.typeRoles.labelMicro.copyWith(
                          color: colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),

            // Price & Action
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Total Payment',
                      style: context.typeRoles.labelMicro.copyWith(
                        color: colorScheme.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      order.pricing.totalPayableAmount != null
                          ? CurrencyUtils.format(
                              order.pricing.totalPayableAmount!,
                            )
                          : '—',
                      style: context.typeRoles.titleCompact.copyWith(
                        fontWeight: FontWeight.bold,
                        color: colorScheme.primary,
                      ),
                    ),
                  ],
                ),
                ElevatedButton(
                  onPressed: () =>
                      context.push(RoutePaths.orderDetailPath(order.id)),
                  style: ElevatedButton.styleFrom(
                    padding: const EdgeInsets.symmetric(
                      horizontal: core.AppMetrics.p24,
                      vertical: core.AppMetrics.p12,
                    ),
                  ),
                  child: const Text('View Details'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  String _getStatusLabel(OrderStatus status) {
    switch (status) {
      case OrderStatus.pending:
        return 'Menunggu Pembayaran';
      case OrderStatus.paid:
        return 'Pembayaran Berhasil';
      case OrderStatus.shipped:
        return 'Dalam Pengiriman';
      case OrderStatus.delivered:
      case OrderStatus.completed:
        return 'Selesai';
      case OrderStatus.cancelled:
        return 'Dibatalkan';
      case OrderStatus.cancelledTimeout:
        return 'Dibatalkan (Timeout)';
      case OrderStatus.refunded:
        return 'Dikembalikan';
      case OrderStatus.disputeOpen:
        return 'Sedang Dispute';
      case OrderStatus.partiallyRefunded:
        return 'Pengembalian Sebagian';
      case OrderStatus.expired:
        return 'Kedaluwarsa';
    }
  }

  Color _getStatusColor(OrderStatus status) {
    switch (status) {
      case OrderStatus.pending:
        return context.statusColors.warning;
      case OrderStatus.paid:
      case OrderStatus.shipped:
        return context.statusColors.info;
      case OrderStatus.delivered:
      case OrderStatus.completed:
        return context.statusColors.success;
      case OrderStatus.cancelled:
      case OrderStatus.cancelledTimeout:
      case OrderStatus.refunded:
        return context.statusColors.error;
      case OrderStatus.disputeOpen:
        return context.statusColors.warning;
      case OrderStatus.partiallyRefunded:
        return context.statusColors.info;
      case OrderStatus.expired:
        return Theme.of(context).colorScheme.onSurfaceVariant;
    }
  }

  // =============================================================================
  // OVERDUE INDICATOR HELPERS - OVERDUE ENFORCEMENT CLOSURE
  // =============================================================================

  /// Returns the color for the overdue badge based on tier.
  Color _getOverdueBadgeColor(String? overdueTier) {
    switch (overdueTier) {
      case 'overdue': // Tier 1
        return context.statusColors.warning; // Orange
      case 'severely_overdue': // Tier 2
      case 'critical_overdue': // Tier 3
        return context.statusColors.error; // Red
      default:
        return context.statusColors.warning; // Default to orange
    }
  }

  /// Returns the label for the overdue badge based on tier.
  String _getOverdueBadgeLabel(String? overdueTier) {
    switch (overdueTier) {
      case 'overdue':
        return 'Melewati Estimasi';
      case 'severely_overdue':
        return 'Terlambat';
      case 'critical_overdue':
        return 'Sangat Terlambat';
      default:
        return 'Terlambat';
    }
  }

  Widget _buildEmptyState(BuildContext context, bool isSeller) {
    final l10n = context.l10n;
    // Buyer action only: browse the marketplace. Incoming Orders (seller)
    // carries NO create action — For Sale creation lives on the For Sale
    // page, never on the incoming-orders empty state (owner-locked, test
    // enforced). The canonical renderer offers at most ONE primary action.
    return EmptyState(
      icon: isSeller ? Icons.storefront_outlined : Icons.shopping_bag_outlined,
      title: isSeller ? l10n.emptyIncomingOrdersTitle : l10n.emptyOrdersTitle,
      subtitle: isSeller
          ? l10n.emptyIncomingOrdersMessage
          : l10n.emptyOrdersMessage,
      actionLabel: isSeller ? null : l10n.exploreMarketplaceAction,
      onAction: isSeller ? null : () => context.go(RoutePaths.forSales),
    );
  }
}
