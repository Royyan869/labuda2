import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart';
import 'package:labuda/shared/utils/app_formatters.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:labuda/domains/system/support/presentation/screens/help_center_screen.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/screens/chat_list_screen.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/operational_action_queue_section.dart';

/// Seller Dashboard Screen
///
/// **WORKSPACE ACCESS POLICY (PHASE 1A):**
/// - Requires: hasSellerProfile (workspace identity)
/// - Expired sellers CAN access (read-only workspace, view orders/history)
/// - Market authority (hasMarketAuthority) is NOT required for workspace access
/// - This allows expired sellers to manage their business and renew subscription
///
/// Minimum viable seller dashboard with:
/// - Order statistics (pending, processing, completed)
/// - Quick actions (view orders)
/// - Empty state when no data
class SellerDashboardScreen extends ConsumerStatefulWidget {
  const SellerDashboardScreen({super.key});

  @override
  ConsumerState<SellerDashboardScreen> createState() =>
      _SellerDashboardScreenState();
}

class _SellerDashboardScreenState extends ConsumerState<SellerDashboardScreen> {
  @override
  void initState() {
    super.initState();
    // Refresh order streams after mount so Riverpod is safe to access.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      ref.invalidate(orderListRefreshTriggerProvider);
    });
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    final sellerIdentityStatus = ref.watch(sellerIdentityStatusProvider);
    final sellerCapabilityStatus = ref.watch(sellerCapabilityStatusProvider);
    if (authState is! AuthStateAuthenticated) {
      return _buildAuthRequired(context);
    }

    if (sellerIdentityStatus == SellerIdentityStatus.unknown ||
        sellerCapabilityStatus == SellerCapabilityStatus.unknown) {
      return _buildSellerStatusLoading(context);
    }

    final user = authState.user;
    if (sellerIdentityStatus != SellerIdentityStatus.seller) {
      return _buildSellerProfileRequired(context);
    }

    final sellerId = user.id;

    return Scaffold(
      backgroundColor: Theme.of(context).colorScheme.surface,
      body: CustomScrollView(
        slivers: [
          // App Bar — flat surface chrome from appBarTheme. No collapsing
          // FlexibleSpaceBar: the growing title/gradient was rejected as a
          // second visual dialect.
          const SliverAppBar(
            pinned: true,
            title: Text('Dashboard Penjual'),
          ),

          // Content
          SliverToBoxAdapter(
            child: _buildContent(context, sellerId),
          ),
        ],
      ),
    );
  }

  Widget _buildAuthRequired(BuildContext context) {
    return Scaffold(
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.lock_outline,
              size: AppIconSize.display,
              color: Theme.of(context).colorScheme.primary,
            ),
            const SizedBox(height: 16),
            const Text(
              'Login Diperlukan',
              style: TextStyle(fontSize: AppType.s20, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),
            const Text('Silakan login untuk mengakses dashboard penjual'),
          ],
        ),
      ),
    );
  }

  Widget _buildSellerProfileRequired(BuildContext context) {
    return Scaffold(
      backgroundColor: Theme.of(context).colorScheme.surface,
      appBar: AppBar(
        title: const Text('Seller Profile Required'),
      ),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(AppMetrics.p32),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Container(
                padding: const EdgeInsets.all(AppMetrics.p24),
                decoration: BoxDecoration(
                  color: context.statusColors.warning.withValues(alpha: 0.1),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  Icons.store_outlined,
                  size: AppIconSize.display,
                  color: context.statusColors.warning,
                ),
              ),
              const SizedBox(height: 32),
              Text(
                'Profil Penjual Diperlukan',
                style: TextStyle(
                  fontSize: AppType.s20,
                  fontWeight: FontWeight.bold,
                  color: Theme.of(context).colorScheme.onSurface,
                ),
              ),
              const SizedBox(height: 16),
              Text(
                'Anda perlu membuat profil penjual untuk mulai berjualan di Labuda.',
                style: TextStyle(
                  fontSize: AppType.s14,
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 32),
              // Setup path explanation
              Container(
                padding: const EdgeInsets.all(AppMetrics.p16),
                decoration: BoxDecoration(
                  color: Theme.of(context).colorScheme.surfaceContainerLow,
                  borderRadius: BorderRadius.circular(AppShape.r12),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Icon(
                          Icons.info_outline,
                          size: AppIconSize.inlineGlyph,
                          color: Theme.of(context).colorScheme.secondary,
                        ),
                        const SizedBox(width: 8),
                        Text(
                          'Langkah-langkah:',
                          style: TextStyle(
                            fontSize: AppType.s14,
                            fontWeight: FontWeight.bold,
                            color: Theme.of(context).colorScheme.onSurface,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 12),
                    _buildStepItem(
                      '1',
                      'Pilih paket penjual (Gratis/Berbayar)',
                    ),
                    _buildStepItem('2', 'Lengkapi info farm & alamat'),
                    _buildStepItem(
                      '3',
                      'Verifikasi KTP (untuk penarikan dana)',
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 40),
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  OutlinedButton.icon(
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.arrow_back, size: AppIconSize.action),
                    label: const Text('Kembali'),
                    style: OutlinedButton.styleFrom(
                      padding: const EdgeInsets.symmetric(
                        horizontal: AppMetrics.p24,
                        vertical: AppMetrics.p12,
                      ),
                    ),
                  ),
                  const SizedBox(width: 16),
                  ElevatedButton.icon(
                    onPressed: () {
                      context.push(RoutePaths.sellerUpgrade);
                    },
                    icon: const Icon(Icons.storefront, size: AppIconSize.action),
                    label: const Text('Mulai Jualan'),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildSellerStatusLoading(BuildContext context) {
    return Scaffold(
      backgroundColor: Theme.of(context).colorScheme.surface,
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(AppMetrics.p32),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const SizedBox(
                width: 28,
                height: 28,
                child: CircularProgressIndicator(strokeWidth: 2.5),
              ),
              const SizedBox(height: 16),
              Text(
                'Memuat status seller...',
                style: TextStyle(
                  fontSize: AppType.s16,
                  fontWeight: FontWeight.w600,
                  color: Theme.of(context).colorScheme.onSurface,
                ),
              ),
              const SizedBox(height: 8),
              Text(
                'Menunggu identitas dan kapabilitas dari backend.',
                style: TextStyle(
                  fontSize: AppType.s14,
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildStepItem(String number, String text) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppMetrics.p8),
      child: Row(
        children: [
          Container(
            width: AppContentSize.badge,
            height: AppContentSize.badge,
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.secondary.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(AppShape.r4),
            ),
            child: Center(
              child: Text(
                number,
                style: TextStyle(
                  fontSize: AppType.s12,
                  fontWeight: FontWeight.bold,
                  color: Theme.of(context).colorScheme.secondary,
                ),
              ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              text,
              style: TextStyle(
                fontSize: AppType.s14,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildContent(BuildContext context, String sellerId) {
    // Chat bridge gets a dashboard seat only while it carries unread.
    final totalUnread = ref.watch(totalUnreadCountProvider);

    return Padding(
      padding: const EdgeInsets.all(AppMetrics.p16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // ONE owner per truth: the queue owns every actionable nag. The
          // top verification card, action-required card and getting-started
          // banner were competing copies — deleted, not hidden. Chat gets a
          // seat only while unread > 0 (at zero it was copy filler).
          // The expiry banner owns the expired STATE (queue never repeats it).
          const _SubscriptionExpiryBanner(),

          if (totalUnread > 0) ...[
            const _SellerChatWorkspaceSection(),
            const SizedBox(height: 16),
          ],

          // Statistics: numbers only, no actions.
          _OrderStatsSection(sellerId: sellerId),

          const SizedBox(height: 24),

          // Operational action queue: one queue for every actionable seller
          // task (spec: seller_dashboard_operational_action_queue_test).
          OperationalActionQueueSection(sellerId: sellerId),

          const SizedBox(height: 24),

          // Quick Actions
          _QuickActionsSection(),

          const SizedBox(height: 16),

          // Single help door.
          _SellerHelpSection(),

          const SizedBox(height: 24),

          // Recent Orders Preview
          _RecentOrdersSection(sellerId: sellerId),

        ],
      ),
    );
  }
}

// =============================================================================
// SUBSCRIPTION GRACE PERIOD BANNER
// =============================================================================

/// Grace period warning banner — UX signal only.
///
/// **AUTHORITY CONTRACT (DO NOT BREAK):**
/// - Grace period = full market authority (same as active).
/// - This widget NEVER gates or disables any seller feature.
/// - Reads `sellerSubscriptionStatusProvider` (raw backend string) only.
/// - Does NOT read or modify `hasMarketAuthority`.
/// - Returns SizedBox.shrink() for 'active', 'expired', 'none', or null.
///
/// Shown only when `sellerSubscriptionStatus == 'expired'`.
class _SubscriptionExpiryBanner extends ConsumerWidget {

  const _SubscriptionExpiryBanner();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final subscriptionStatus = ref.watch(sellerSubscriptionStatusProvider);
    if (subscriptionStatus != 'expired') return const SizedBox.shrink();

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          padding: const EdgeInsets.all(AppMetrics.p16),
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
              colors: [
                context.statusColors.warning.withValues(alpha: 0.15),
                context.statusColors.warning.withValues(alpha: 0.05),
              ],
            ),
            borderRadius: BorderRadius.circular(AppShape.r16),
            border: Border.all(
              color: context.statusColors.warning.withValues(alpha: 0.4),
              width: 1,
            ),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Container(
                    padding: const EdgeInsets.all(AppMetrics.p8),
                    decoration: BoxDecoration(
                      color: context.statusColors.warning.withValues(alpha: 0.2),
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      Icons.warning_amber_outlined,
                      color: context.statusColors.warning,
                      size: AppIconSize.action,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Langganan Kedaluwarsa',
                          style: TextStyle(
                            fontSize: AppType.s16,
                            fontWeight: FontWeight.bold,
                            color: Theme.of(context).colorScheme.onSurface,
                          ),
                        ),
                        Text(
                          'Langganan Anda telah berakhir. Perbarui untuk memulihkan akses pasar.',
                          style: TextStyle(
                            fontSize: AppType.s12,
                            color: Theme.of(context).colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              ElevatedButton.icon(
                onPressed: () => context.push(RoutePaths.sellerRenewal),
                icon: const Icon(Icons.refresh_outlined, size: AppIconSize.action),
                label: const Text('Perpanjang Langganan'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: context.statusColors.warning,
                  minimumSize: const Size(double.infinity, 44),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),
      ],
    );
  }
}

// =============================================================================
// SELLER CHAT WORKSPACE BRIDGE (PHASE 2 HARDENING)
// =============================================================================
/// Chat workspace bridge for sellers - connects seller dashboard to chat
///
/// **PHASE 2 HARDENING - PRIORITY 1:**
/// Chat is the primary work tool for sellers in a chat-first commerce app.
/// This section provides:
/// - Unread buyer messages count (awareness)
/// - Quick CTA to chat list (actionability)
/// - Context on why chat matters for seller work
///
/// This bridges the gap between seller workspace (order-centric) and
/// chat (buyer communication), making the workspace more complete.
class _SellerChatWorkspaceSection extends ConsumerWidget {
  const _SellerChatWorkspaceSection();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final totalUnread = ref.watch(totalUnreadCountProvider);

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            Theme.of(context).colorScheme.secondary.withValues(alpha: 0.12),
            Theme.of(context).colorScheme.secondary.withValues(alpha: 0.06),
          ],
        ),
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(
          color: Theme.of(context).colorScheme.secondary.withValues(alpha: 0.3),
          width: 1.2,
        ),
      ),
      child: InkWell(
        onTap: () => _navigateToChatList(context),
        borderRadius: BorderRadius.circular(AppShape.r16),
        child: Row(
          children: [
            // Chat icon with unread indicator
            Stack(
              children: [
                Container(
                  padding: const EdgeInsets.all(AppMetrics.p12),
                  decoration: BoxDecoration(
                    color: Theme.of(context).colorScheme.secondary.withValues(alpha: 0.2),
                    shape: BoxShape.circle,
                  ),
                  child: Icon(
                    Icons.chat_bubble_outline,
                    color: Theme.of(context).colorScheme.secondary,
                    size: AppIconSize.header,
                  ),
                ),
                // Unread badge
                if (totalUnread > 0)
                  Positioned(
                    top: 0,
                    right: 0,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: AppMetrics.p8,
                        vertical: AppMetrics.p4,
                      ),
                      decoration: BoxDecoration(
                        color: Theme.of(context).colorScheme.primary,
                        borderRadius: BorderRadius.circular(AppShape.r10),
                        border: Border.all(
                          color: Theme.of(context).colorScheme.surface,
                          width: 2,
                        ),
                      ),
                      constraints: const BoxConstraints(minWidth: 18),
                      child: Text(
                        totalUnread > 99 ? '99+' : totalUnread.toString(),
                        style: TextStyle(
                          color: Theme.of(context).colorScheme.onPrimary,
                          fontSize: AppType.s12,
                          fontWeight: FontWeight.bold,
                          height: 1.1,
                        ),
                        textAlign: TextAlign.center,
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(width: 16),
            // Text content
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Pesan Pembeli',
                    style: TextStyle(
                      fontSize: AppType.s16,
                      fontWeight: FontWeight.bold,
                      color: Theme.of(context).colorScheme.onSurface,
                    ),
                  ),
                  const SizedBox(height: 3),
                  Text(
                    _getChatMessage(totalUnread),
                    style: TextStyle(
                      fontSize: AppType.s14,
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
            // Arrow indicator
            Icon(
              Icons.chevron_right,
              color: Theme.of(context).colorScheme.onSurfaceVariant,
              size: AppIconSize.header,
            ),
          ],
        ),
      ),
    );
  }

  String _getChatMessage(int unreadCount) {
    if (unreadCount == 0) {
      return 'Balas pesan pembeli untuk tingkatkan penjualan';
    } else if (unreadCount == 1) {
      return '1 pesan belum dibaca';
    } else if (unreadCount <= 5) {
      return '$unreadCount pesan belum dibaca';
    } else {
      return '$unreadCount pesan menunggu balasan';
    }
  }

  void _navigateToChatList(BuildContext context) {
    Navigator.push(
      context,
      MaterialPageRoute(builder: (context) => const ChatListScreen()),
    );
  }
}

// =============================================================================
// ORDER STATS SECTION
// =============================================================================

class _OrderStatsSection extends ConsumerWidget {
  final String sellerId;

  const _OrderStatsSection({required this.sellerId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Watch all order streams concurrently
    final pendingAsync = ref.watch(
      watchSellerOrdersProvider(
        sellerId: sellerId,
        status: OrderStatus.pending,
      ),
    );
    final paidAsync = ref.watch(
      watchSellerOrdersProvider(sellerId: sellerId, status: OrderStatus.paid),
    );
    final shippedAsync = ref.watch(
      watchSellerOrdersProvider(
        sellerId: sellerId,
        status: OrderStatus.shipped,
      ),
    );
    final completedAsync = ref.watch(
      watchSellerOrdersProvider(
        sellerId: sellerId,
        status: OrderStatus.completed,
      ),
    );

    final pendingCount = pendingAsync.value?.length ?? 0;
    final paidCount = paidAsync.value?.length ?? 0;
    final shippedCount = shippedAsync.value?.length ?? 0;
    final completedCount = completedAsync.value?.length ?? 0;
    final totalOrders =
        pendingCount + paidCount + shippedCount + completedCount;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Statistik Pesanan',
          style: Theme.of(
            context,
          ).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.bold),
        ),
        const SizedBox(height: 16),
        Row(
          children: [
            Expanded(
              child: _StatCard(
                label: 'Perlu Tindakan',
                value: pendingCount.toString(),
                icon: Icons.notification_important,
                color: context.statusColors.warning,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: _StatCard(
                label: 'Diproses',
                value: paidCount.toString(),
                icon: Icons.inventory_2_outlined,
                color: Theme.of(context).colorScheme.secondary,
              ),
            ),
          ],
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            Expanded(
              child: _StatCard(
                label: 'Dikirim',
                value: shippedCount.toString(),
                icon: Icons.local_shipping_outlined,
                color: context.statusColors.info,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: _StatCard(
                label: 'Selesai',
                value: completedCount.toString(),
                icon: Icons.check_circle_outline,
                color: context.statusColors.success,
              ),
            ),
          ],
        ),
        if (totalOrders == 0)
          Padding(
            padding: const EdgeInsets.only(top: AppMetrics.p16),
            child: Container(
              padding: const EdgeInsets.all(AppMetrics.p16),
              decoration: BoxDecoration(
                color: Theme.of(context).colorScheme.surface,
                borderRadius: BorderRadius.circular(AppShape.r12),
                border: Border.all(
                  color: Theme.of(context).colorScheme.outlineVariant,
                ),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.inbox_outlined,
                    size: AppIconSize.emphasis,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Belum Ada Pesanan',
                          style: TextStyle(
                            fontWeight: FontWeight.w600,
                            color: Theme.of(context).colorScheme.onSurface,
                          ),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          'Pesanan masuk akan muncul di sini',
                          style: TextStyle(
                            fontSize: AppType.s12,
                            color: Theme.of(context).colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
      ],
    );
  }
}

class _StatCard extends StatelessWidget {
  final String label;
  final String value;
  final IconData icon;
  final Color color;

  const _StatCard({
    required this.label,
    required this.value,
    required this.icon,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(
          color: Theme.of(context).colorScheme.outlineVariant,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(icon, size: AppIconSize.action, color: color),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  label,
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            value,
            style: Theme.of(context).textTheme.headlineSmall?.copyWith(
              fontWeight: FontWeight.bold,
              color: color,
            ),
          ),
        ],
      ),
    );
  }
}

// =============================================================================
// QUICK ACTIONS SECTION
// =============================================================================

class _QuickActionsSection extends ConsumerWidget {

  const _QuickActionsSection();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Aksi Cepat',
          style: Theme.of(
            context,
          ).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.bold),
        ),
        const SizedBox(height: 16),
        Row(
          children: [
            Expanded(
              child: _QuickActionCard(
                icon: Icons.shopping_bag_outlined,
                label: 'Pesanan Masuk',
                color: Theme.of(context).colorScheme.primary,
                onTap: () => _navigateToOrders(context),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: _QuickActionCard(
                icon: Icons.view_list_outlined,
                label: 'ForSale Saya',
                color: context.statusColors.success,
                onTap: () => _navigateToForSales(context),
              ),
            ),
          ],
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            Expanded(
              child: _QuickActionCard(
                icon: Icons.local_shipping_outlined,
                label: 'Atur Pengiriman',
                color: context.statusColors.info,
                onTap: () => _navigateToShipping(context),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: _QuickActionCard(
                icon: Icons.manage_search_outlined,
                label: 'Kelola Promosi',
                color: Theme.of(context).colorScheme.secondary,
                onTap: () => _navigateToCanonicalPromotions(context),
              ),
            ),
          ],
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            Expanded(
              child: _QuickActionCard(
                key: const Key('seller-quick-action-earnings'),
                icon: Icons.account_balance_wallet_outlined,
                label: 'Pendapatan',
                color: context.statusColors.success,
                onTap: () => context.push(RoutePaths.sellerEarnings),
              ),
            ),
          ],
        ),

      ],
    );
  }

  void _navigateToCanonicalPromotions(BuildContext context) {
    context.push(RoutePaths.sellerCanonicalPromotions);
  }

  void _navigateToShipping(BuildContext context) {
    context.push(RoutePaths.sellerShipping);
  }

  void _navigateToOrders(BuildContext context) {
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (context) => const OrderListScreen(isSeller: true),
      ),
    );
  }

  void _navigateToForSales(BuildContext context) {
    // GoRouter authority: push by path so the location stays observable.
    // Navigator.pushNamed here resolved the path as a route NAME and threw
    // under MaterialApp.router (broke seller_dashboard_quick_action routes).
    context.push(RoutePaths.sellerForSales);
  }
}

class _QuickActionCard extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color color;
  final VoidCallback onTap;

  const _QuickActionCard({
    super.key,
    required this.icon,
    required this.label,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r12),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p16),
        decoration: BoxDecoration(
          color: Theme.of(context).colorScheme.surface,
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(
            color: Theme.of(context).colorScheme.outlineVariant,
          ),
        ),
        child: Column(
          children: [
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: color.withValues(alpha: 0.1),
                shape: BoxShape.circle,
              ),
              child: Icon(icon, color: color, size: AppIconSize.header),
            ),
            const SizedBox(height: 12),
            Text(
              label,
              style: Theme.of(
                context,
              ).textTheme.bodyMedium?.copyWith(fontWeight: FontWeight.w600),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}

// =============================================================================
// SELLER HELP SECTION (PHASE 3 HARDENING)
// =============================================================================

class _SellerHelpSection extends ConsumerWidget {
  const _SellerHelpSection();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);
    final userId =
        authState is AuthStateAuthenticated ? authState.user.id : null;
    final userName =
        authState is AuthStateAuthenticated ? authState.user.username : null;
    final userAvatar =
        authState is AuthStateAuthenticated ? authState.user.avatarUrl : null;

    // ONE help door: the old 3-tile gradient block spent prime dashboard
    // space on entries that all land in the same Help Center anyway
    // (contact support lives inside it).
    return _HelpTile(
      icon: Icons.support_agent,
      title: 'Bantuan & Support Penjual',
      description: 'Pusat bantuan dan kontak tim kami',
      onTap: () {
        Navigator.of(context).push(
          MaterialPageRoute(
            builder: (context) => HelpCenterScreen(
              userId: userId,
              userName: userName,
              userAvatar: userAvatar,
            ),
          ),
        );
      },
    );
  }
}

class _HelpTile extends StatelessWidget {
  final IconData icon;
  final String title;
  final String description;
  final VoidCallback? onTap;

  const _HelpTile({
    required this.icon,
    required this.title,
    required this.description,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r8),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p12),
        decoration: BoxDecoration(
          color: Theme.of(context).colorScheme.surface.withValues(alpha: 0.7),
          borderRadius: BorderRadius.circular(AppShape.r8),
        ),
        child: Row(
          children: [
            Container(
              padding: const EdgeInsets.all(AppMetrics.p8),
              decoration: BoxDecoration(
                color: context.statusColors.success.withValues(alpha: 0.15),
                shape: BoxShape.circle,
              ),
              child: Icon(icon, color: context.statusColors.success, size: AppIconSize.inlineGlyph),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    style: TextStyle(
                      fontSize: AppType.s14,
                      fontWeight: FontWeight.w600,
                      color: Theme.of(context).colorScheme.onSurface,
                    ),
                  ),
                  Text(
                    description,
                    style: TextStyle(
                      fontSize: AppType.s12,
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
            Icon(
              Icons.chevron_right,
              size: AppIconSize.inlineGlyph,
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ],
        ),
      ),
    );
  }
}

// =============================================================================
// RECENT ORDERS SECTION
// =============================================================================

class _RecentOrdersSection extends ConsumerWidget {
  final String sellerId;

  const _RecentOrdersSection({required this.sellerId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final recentOrdersAsync = ref.watch(recentSellerOrdersProvider(sellerId));

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              'Pesanan Terbaru',
              style: Theme.of(
                context,
              ).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.bold),
            ),
            TextButton(
              onPressed: () {
                Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (context) => const OrderListScreen(isSeller: true),
                  ),
                );
              },
              child: const Text('Lihat Semua'),
            ),
          ],
        ),
        const SizedBox(height: 12),
        recentOrdersAsync.when(
          data: (orders) {
            if (orders.isEmpty) {
              return _buildEmptyRecentOrdersState(context);
            }

            return Column(
              children: orders
                  .map(
                    (order) => _OrderTile(
                      order: order,
                      onTap: () => _navigateToOrderDetail(context, order.id),
                    ),
                  )
                  .toList(),
            );
          },
          loading: () => const Center(
            child: Padding(
              padding: EdgeInsets.all(AppMetrics.p32),
              child: CircularProgressIndicator(),
            ),
          ),
          error: (error, _) =>
              _buildRecentOrdersErrorState(context, ref, error.toString()),
        ),
      ],
    );
  }

  Widget _buildEmptyRecentOrdersState(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(
          color: Theme.of(context).colorScheme.outlineVariant,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Belum ada pesanan',
            style: TextStyle(fontSize: AppType.s14, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 4),
          Text(
            'Pesanan terbaru akan muncul di sini setelah ada pembelian.',
            style: TextStyle(fontSize: AppType.s12, color: Theme.of(context).colorScheme.onSurfaceVariant),
          ),
        ],
      ),
    );
  }

  Widget _buildRecentOrdersErrorState(
    BuildContext context,
    WidgetRef ref,
    String message,
  ) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: context.statusColors.error.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: context.statusColors.error.withValues(alpha: 0.3)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Gagal memuat pesanan terbaru',
            style: TextStyle(
              fontSize: AppType.s14,
              fontWeight: FontWeight.bold,
              color: context.statusColors.error,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            message,
            style: TextStyle(fontSize: AppType.s12, color: Theme.of(context).colorScheme.onSurfaceVariant),
          ),
          const SizedBox(height: 12),
          TextButton(
            onPressed: () =>
                ref.invalidate(recentSellerOrdersProvider(sellerId)),
            child: const Text('Coba Lagi'),
          ),
        ],
      ),
    );
  }

  void _navigateToOrderDetail(BuildContext context, String orderId) {
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (context) => OrderDetailScreen(orderId: orderId),
      ),
    );
  }
}

class _OrderTile extends StatelessWidget {
  final Order order;
  final VoidCallback onTap;

  const _OrderTile({
    required this.order,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    // The GET /orders list surface carries no line items (items[] is emitted
    // only by GET /orders/:id), so the tile degrades to a neutral label rather
    // than inventing an item name or crashing on `.first`.
    final firstItem = order.items.isEmpty ? null : order.items.first;

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r12),
      child: Container(
        margin: const EdgeInsets.only(bottom: AppMetrics.p8),
        padding: const EdgeInsets.all(AppMetrics.p12),
        decoration: BoxDecoration(
          color: Theme.of(context).colorScheme.surface,
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(
            color: Theme.of(context).colorScheme.outlineVariant,
          ),
        ),
        child: Row(
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(AppShape.r8),
              child: AppImage(
                imageUrl: firstItem?.forSaleImage,
                width: 48,
                height: 48,
                fit: BoxFit.cover,
                backgroundColor: Theme.of(context).colorScheme.surfaceContainerHighest,
                errorWidget: Container(
                  width: AppIconSize.display,
                  height: AppIconSize.display,
                  color: Theme.of(context).colorScheme.surfaceContainerHighest,
                  child: const Icon(Icons.image_not_supported, size: AppIconSize.action),
                ),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    order.id.substring(0, 8).toUpperCase(),
                    style: TextStyle(
                      fontSize: AppType.s12,
                      fontFamily: 'monospace',
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    firstItem?.forSaleName ?? 'Pesanan',
                    style: const TextStyle(fontWeight: FontWeight.w600),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 4),
                  Text(
                    order.pricing.totalPayableAmount != null
                        ? AppFormatters.formatCurrency(
                            order.pricing.totalPayableAmount!,
                          )
                        : '—',
                    style: TextStyle(
                      color: Theme.of(context).colorScheme.primary,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 8),
            _StatusBadge(status: order.status),
          ],
        ),
      ),
    );
  }
}

class _StatusBadge extends StatelessWidget {
  final OrderStatus status;

  const _StatusBadge({required this.status});

  @override
  Widget build(BuildContext context) {
    Color color;
    String label;

    switch (status) {
      case OrderStatus.pending:
        color = context.statusColors.warning;
        label = 'Pending';
        break;
      case OrderStatus.paid:
        color = Theme.of(context).colorScheme.secondary;
        label = 'Diproses';
        break;
      case OrderStatus.shipped:
        color = context.statusColors.info;
        label = 'Dikirim';
        break;
      case OrderStatus.expired:
        color = context.statusColors.error;
        label = 'Kedaluwarsa';
        break;
      case OrderStatus.delivered:
      case OrderStatus.completed:
        color = context.statusColors.success;
        label = 'Selesai';
        break;
      case OrderStatus.cancelled:
      case OrderStatus.cancelledTimeout:
      case OrderStatus.refunded:
        color = context.statusColors.error;
        label = 'Batal';
        break;
      case OrderStatus.disputeOpen:
        color = context.statusColors.warning;
        label = 'Dispute';
        break;
      case OrderStatus.partiallyRefunded:
        color = context.statusColors.info;
        label = 'Refund Sebagian';
        break;
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Text(
        label,
        style: TextStyle(
          color: color,
          fontSize: AppType.s12,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}
