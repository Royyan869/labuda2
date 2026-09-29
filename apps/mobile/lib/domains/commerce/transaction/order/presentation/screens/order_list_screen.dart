import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
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
            icon: const Icon(Icons.arrow_back),
            onPressed: () => Navigator.of(context).pop(),
          ),
          surfaceTintColor: Colors.transparent,
          scrolledUnderElevation: 0,
          bottom: TabBar(
            controller: _tabController,
            isScrollable: true,
            indicatorColor: colorScheme.primary,
            labelColor: colorScheme.primary,
            unselectedLabelColor: colorScheme.onSurfaceVariant,
            tabs: const [
              Tab(text: 'All'),
              Tab(text: 'Pending'),
              Tab(text: 'Paid'),
              Tab(text: 'Shipped'),
              Tab(text: 'Completed'),
            ],
          ),
        ),
        body: TabBarView(
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
            const Icon(Icons.login, size: 48),
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

    // Fetch orders based on role (real-time dengan Stream)
    final ordersAsync = widget.isSeller
        ? ref.watch(
            watchSellerOrdersProvider(sellerId: currentUser.id, status: status),
          )
        : ref.watch(
            watchBuyerOrdersProvider(buyerId: currentUser.id, status: status),
          );

    return ordersAsync.when(
      data: (orders) {
        // Stream langsung return List<Order>, bukan Result

        if (orders.isEmpty) {
          return _buildEmptyState(context, widget.isSeller);
        }

        return ListView.builder(
          padding: const EdgeInsets.all(core.AppMetrics.p16),
          itemCount: orders.length,
          itemBuilder: (context, index) {
            return _buildOrderCard(orders[index]);
          },
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, stack) => Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.error_outline,
              size: 48,
              color: context.statusColors.error,
            ),
            const SizedBox(height: 16),
            const Text('Data belum bisa dimuat.'),
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
      onTap: () {
        Navigator.push(
          context,
          MaterialPageRoute(
            builder: (context) => OrderDetailScreen(orderId: order.id),
          ),
        );
      },
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
                    style: TextStyle(
                      fontSize: core.AppType.s14,
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
                    style: TextStyle(
                      fontSize: core.AppType.s11,
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
                      horizontal: core.AppMetrics.p6,
                      vertical: core.AppMetrics.p2,
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
                          size: 12,
                          color: _getOverdueBadgeColor(order.overdueTier),
                        ),
                        const SizedBox(width: 2),
                        Text(
                          _getOverdueBadgeLabel(order.overdueTier),
                          style: TextStyle(
                            fontSize: core.AppType.s10,
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
                        style: TextStyle(
                          fontSize: core.AppType.s14,
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
                        style: TextStyle(
                          fontSize: core.AppType.s12,
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
                      style: TextStyle(
                        fontSize: core.AppType.s12,
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
                      style: TextStyle(
                        fontSize: core.AppType.s16,
                        fontWeight: FontWeight.bold,
                        color: colorScheme.primary,
                      ),
                    ),
                  ],
                ),
                ElevatedButton(
                  onPressed: () {
                    Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (context) =>
                            OrderDetailScreen(orderId: order.id),
                      ),
                    );
                  },
                  style: ElevatedButton.styleFrom(
                    padding: const EdgeInsets.symmetric(
                      horizontal: core.AppMetrics.p20,
                      vertical: core.AppMetrics.p10,
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
        return 'Menunggu Konfirmasi';
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
    final colorScheme = Theme.of(context).colorScheme;
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: core.AppMetrics.p48),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            // Icon
            Container(
              width: 80,
              height: 80,
              decoration: BoxDecoration(
                color: colorScheme.surfaceContainerHigh,
                shape: BoxShape.circle,
              ),
              child: Icon(
                isSeller
                    ? Icons.storefront_outlined
                    : Icons.shopping_bag_outlined,
                size: 40,
                color: colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 24),

            // Title
            Text(
              isSeller ? 'Belum Ada Pesanan Masuk' : 'Belum Ada Pesanan',
              style: TextStyle(
                fontSize: core.AppType.s18,
                fontWeight: FontWeight.w600,
                color: colorScheme.onSurface,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 12),

            // Subtitle with context-aware guidance
            Text(
              isSeller
                  ? 'Pesanan dari pembeli akan muncul di sini'
                  : 'Mulai berbelanja dari koleksi Koi terbaik',
              style: TextStyle(
                fontSize: core.AppType.s14,
                color: colorScheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 32),

            // Action button
            SizedBox(
              width: 240,
              child: FilledButton.icon(
                icon: Icon(
                  isSeller ? Icons.add_circle_outline : Icons.storefront_outlined,
                  size: 20,
                ),
                label: Text(
                  isSeller ? 'Tambah ForSale' : 'Jelajahi Marketplace',
                ),
                onPressed: () => _handleEmptyStateAction(context, isSeller),
                style: FilledButton.styleFrom(
                  padding: const EdgeInsets.symmetric(
                    vertical: core.AppMetrics.p14,
                    horizontal: core.AppMetrics.p24,
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _handleEmptyStateAction(BuildContext context, bool isSeller) {
    if (isSeller) {
      // Navigate to create forSale
      Navigator.pushNamed(context, core.RoutePaths.createForSale);
    } else {
      // Navigate to forSales (marketplace browse)
      Navigator.pushReplacementNamed(context, core.RoutePaths.forSales);
    }
  }
}
