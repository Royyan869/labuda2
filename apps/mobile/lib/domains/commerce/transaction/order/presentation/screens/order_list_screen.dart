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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return PopScope(
      canPop: true,
      child: Scaffold(
        backgroundColor: isDark
            ? core.AppColors.darkGray900
            : core.AppColors.neutralGray50,
        appBar: AppBar(
          title: Text(widget.isSeller ? 'Incoming Orders' : 'My Orders'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => Navigator.of(context).pop(),
          ),
          backgroundColor: isDark
              ? core.AppColors.darkGray800
              : core.AppColors.neutralWhite,
          surfaceTintColor: Colors.transparent,
          scrolledUnderElevation: 0,
          bottom: TabBar(
            controller: _tabController,
            isScrollable: true,
            indicatorColor: core.AppColors.primaryRed,
            labelColor: core.AppColors.primaryRed,
            unselectedLabelColor: isDark
                ? core.AppColors.neutralGray400
                : core.AppColors.neutralGray600,
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
            _buildOrderList(null, isDark),
            _buildOrderList(OrderStatus.pending, isDark),
            _buildOrderList(OrderStatus.paid, isDark),
            _buildOrderList(OrderStatus.shipped, isDark),
            _buildOrderList(OrderStatus.completed, isDark),
          ],
        ),
      ),
    );
  }

  Widget _buildOrderList(OrderStatus? status, bool isDark) {
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
          return _buildEmptyState(context, isDark, widget.isSeller);
        }

        return ListView.builder(
          padding: const EdgeInsets.all(16),
          itemCount: orders.length,
          itemBuilder: (context, index) {
            return _buildOrderCard(isDark, orders[index]);
          },
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, stack) => Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.error_outline, size: 48, color: Colors.red),
            const SizedBox(height: 16),
            const Text('Data belum bisa dimuat.'),
          ],
        ),
      ),
    );
  }

  Widget _buildOrderCard(bool isDark, Order order) {
    final firstItem = order.items.first;

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
        margin: const EdgeInsets.only(bottom: 12),
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: isDark
              ? core.AppColors.darkGray800
              : core.AppColors.neutralWhite,
          borderRadius: BorderRadius.circular(12),
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
                      fontSize: 14,
                      fontWeight: FontWeight.bold,
                      color: isDark
                          ? core.AppColors.neutralWhite
                          : core.AppColors.neutralGray900,
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                // Status Badge
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                  decoration: BoxDecoration(
                    color: _getStatusColor(order.status).withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: Text(
                    _getStatusLabel(order.status),
                    style: TextStyle(
                      fontSize: 11,
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
                      horizontal: 6,
                      vertical: 2,
                    ),
                    decoration: BoxDecoration(
                      color: _getOverdueBadgeColor(
                        order.overdueTier,
                      ).withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(4),
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
                            fontSize: 10,
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
                  borderRadius: BorderRadius.circular(8),
                  child: Image.network(
                    firstItem.listingImage,
                    width: 60,
                    height: 60,
                    fit: BoxFit.cover,
                    errorBuilder: (_, _, _) => Container(
                      width: 60,
                      height: 60,
                      color: core.AppColors.neutralGray300,
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
                        firstItem.listingName,
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: isDark
                              ? core.AppColors.neutralWhite
                              : core.AppColors.neutralGray900,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      const SizedBox(height: 4),
                      Text(
                        '${order.items.length} item${order.items.length > 1 ? 's' : ''}',
                        style: TextStyle(
                          fontSize: 12,
                          color: isDark
                              ? core.AppColors.neutralGray400
                              : core.AppColors.neutralGray600,
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
                        fontSize: 12,
                        color: isDark
                            ? core.AppColors.neutralGray400
                            : core.AppColors.neutralGray600,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      CurrencyUtils.format(order.pricing.total),
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                        color: core.AppColors.primaryRed,
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
                    backgroundColor: core.AppColors.primaryRed,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 20,
                      vertical: 10,
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
        return core.AppColors.statusWarning;
      case OrderStatus.paid:
      case OrderStatus.shipped:
        return core.AppColors.statusInfo;
      case OrderStatus.delivered:
      case OrderStatus.completed:
        return core.AppColors.statusSuccess;
      case OrderStatus.cancelled:
      case OrderStatus.cancelledTimeout:
      case OrderStatus.refunded:
        return core.AppColors.statusError;
      case OrderStatus.disputeOpen:
        return core.AppColors.statusWarning;
      case OrderStatus.partiallyRefunded:
        return core.AppColors.statusInfo;
      case OrderStatus.expired:
        return Colors.grey;
    }
  }

  // =============================================================================
  // OVERDUE INDICATOR HELPERS - OVERDUE ENFORCEMENT CLOSURE
  // =============================================================================

  /// Returns the color for the overdue badge based on tier.
  Color _getOverdueBadgeColor(String? overdueTier) {
    switch (overdueTier) {
      case 'overdue': // Tier 1
        return const Color(0xFFFF9800); // Orange
      case 'severely_overdue': // Tier 2
        return const Color(0xFFF44336); // Red
      case 'critical_overdue': // Tier 3
        return const Color(0xFFD32F2F); // Dark Red
      default:
        return const Color(0xFFFF9800); // Default to orange
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

  Widget _buildEmptyState(BuildContext context, bool isDark, bool isSeller) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 48),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            // Icon
            Container(
              width: 80,
              height: 80,
              decoration: BoxDecoration(
                color: isDark
                    ? core.AppColors.neutralGray700.withValues(alpha: 0.3)
                    : core.AppColors.neutralGray200,
                shape: BoxShape.circle,
              ),
              child: Icon(
                isSeller
                    ? Icons.storefront_outlined
                    : Icons.shopping_bag_outlined,
                size: 40,
                color: isDark
                    ? core.AppColors.neutralGray500
                    : core.AppColors.neutralGray400,
              ),
            ),
            const SizedBox(height: 24),

            // Title
            Text(
              isSeller ? 'Belum Ada Pesanan Masuk' : 'Belum Ada Pesanan',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.w600,
                color: isDark
                    ? core.AppColors.neutralWhite
                    : core.AppColors.neutralGray900,
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
                fontSize: 14,
                color: core.AppColors.neutralGray600,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 32),

            // Action button
            SizedBox(
              width: 240,
              child: FilledButton.icon(
                icon: Icon(
                  isSeller ? Icons.add_circle_outline : Icons.explore_outlined,
                  size: 20,
                ),
                label: Text(
                  isSeller ? 'Tambah Listing' : 'Jelajahi Marketplace',
                ),
                onPressed: () => _handleEmptyStateAction(context, isSeller),
                style: FilledButton.styleFrom(
                  backgroundColor: core.AppColors.primaryRed,
                  foregroundColor: Colors.white,
                  padding: const EdgeInsets.symmetric(
                    vertical: 14,
                    horizontal: 24,
                  ),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12),
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
      // Navigate to create listing
      Navigator.pushNamed(context, core.RoutePaths.createForSale);
    } else {
      // Navigate to listings (marketplace browse)
      Navigator.pushReplacementNamed(context, core.RoutePaths.forSales);
    }
  }
}
