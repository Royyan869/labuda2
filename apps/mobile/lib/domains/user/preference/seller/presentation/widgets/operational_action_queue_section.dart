import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/config/seller_upgrade_config_provider.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart'
    show shippingNotifierProvider;
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/shipping_state.dart';
import 'package:labuda/domains/user/identity/verification/verification.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';
import 'package:labuda/domains/user/preference/seller/seller_di.dart';

/// 'Antrian Tindakan Operasional' — one queue for every actionable seller
/// task on the dashboard.
///
/// Spec authority:
/// test/domains/user/preference/seller/presentation/screens/
/// seller_dashboard_operational_action_queue_test.dart. Every visibility rule
/// below is derived from the providers that test overrides; this widget holds
/// no local state and invents no data. Routes are pushed through GoRouter so
/// the resulting location stays observable (URL-bearing), never through
/// imperative Navigator routes.
///
/// Sender-address visibility follows shipping readiness (rule agreed in
/// scope discussion): address + shipping setup form one preparation block and
/// appear together when no active shipping setup exists. The test cannot
/// distinguish address-data-driven rules because it does not override the
/// address provider.
class OperationalActionQueueSection extends ConsumerWidget {
  const OperationalActionQueueSection({super.key, required this.sellerId});

  final String sellerId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scheme = Theme.of(context).colorScheme;

    // Canonical expiry axis (shared with _SubscriptionExpiryBanner):
    // sellerSubscriptionStatus == 'expired'.
    final isExpired = ref.watch(isSellerSubscriptionExpiredProvider);
    final verificationState = ref.watch(sellerVerificationV2NotifierProvider);
    final shippingState = ref.watch(shippingNotifierProvider);
    final pendingAsync = ref.watch(
      watchSellerOrdersProvider(
        sellerId: sellerId,
        status: OrderStatus.pending,
      ),
    );
    final paidAsync = ref.watch(
      watchSellerOrdersProvider(sellerId: sellerId, status: OrderStatus.paid),
    );
    final subscriptionAsync = ref.watch(
      sellerSubscriptionFutureProvider(sellerId),
    );
    final upgradeConfigAsync = ref.watch(sellerUpgradeConfigProvider);

    // Disputes are the most urgent seller task and were counted nowhere
    // (stats and the old action card only knew pending/paid).
    final disputeAsync = ref.watch(
      watchSellerOrdersProvider(
        sellerId: sellerId,
        status: OrderStatus.disputeOpen,
      ),
    );

    final pendingCount = pendingAsync.asData?.value.length ?? 0;
    final paidCount = paidAsync.asData?.value.length ?? 0;
    final disputeCount = disputeAsync.asData?.value.length ?? 0;

    // Only a *loaded* empty shipping list triggers the preparation block;
    // loading/error states stay hidden instead of flashing fake actions.
    final hasActiveShipping =
        shippingState is ShippingSetupsListLoaded &&
        shippingState.options.any((setup) => setup.isActive);
    final missingShippingSetup =
        shippingState is ShippingSetupsListLoaded && !hasActiveShipping;

    final items = <_ActionQueueItem>[];

    if (pendingCount > 0) {
      items.add(
        _ActionQueueItem(
          itemKey: const Key('seller-action-queue-pending-orders'),
          icon: Icons.pending_actions_outlined,
          color: scheme.primary,
          title: '$pendingCount pesanan menunggu diproses',
          route: RoutePaths.sellerOrders,
        ),
      );
    }
    if (paidCount > 0) {
      items.add(
        _ActionQueueItem(
          itemKey: const Key('seller-action-queue-paid-orders'),
          icon: Icons.sell_outlined,
          color: scheme.primary,
          title: '$paidCount pesanan sudah dibayar — siap dikirim',
          route: RoutePaths.sellerOrders,
        ),
      );
    }
    if (disputeCount > 0) {
      items.add(
        _ActionQueueItem(
          itemKey: const Key('seller-action-queue-dispute'),
          icon: Icons.report_outlined,
          color: context.statusColors.error,
          title: '$disputeCount dispute menunggu penanganan',
          route: RoutePaths.sellerOrders,
        ),
      );
    }
    // Verification is a queue item only when ACTION is possible.
    // pendingReview is waiting on the backend — a nag with no action.
    if (verificationState.status != SellerVerificationStatus.approved &&
        verificationState.status != SellerVerificationStatus.pendingReview) {
      items.add(
        _ActionQueueItem(
          itemKey: const Key('seller-action-queue-verification'),
          icon: Icons.badge_outlined,
          color: context.statusColors.info,
          title: 'Verifikasi seller menunggu tindakan',
          route: RoutePaths.sellerVerification,
        ),
      );
    }
    if (missingShippingSetup) {
      items.add(
        _ActionQueueItem(
          itemKey: const Key('seller-action-queue-sender-address'),
          icon: Icons.location_on_outlined,
          color: context.statusColors.success,
          title: 'Lengkapi alamat pengirim',
          route: RoutePaths.addresses,
        ),
      );
      items.add(
        _ActionQueueItem(
          itemKey: const Key('seller-action-queue-shipping-option'),
          icon: Icons.local_shipping_outlined,
          color: context.statusColors.success,
          title: 'Atur opsi pengiriman toko',
          route: RoutePaths.sellerShipping,
        ),
      );
    }
    // Expired is owned by the top banner (_SubscriptionExpiryBanner) — the
    // queue never repeats a state another surface already shouts. Only the
    // expiring-soon window lives here, and only while NOT already expired.
    if (!isExpired) {
      // Expiring-soon window: active subscription inside the backend-config
      // renewal reminder window (expiryDate vs renewalReminderDays).
      final subscription = subscriptionAsync.asData?.value;
      final upgradeConfig = upgradeConfigAsync.asData?.value;
      if (subscription != null && upgradeConfig != null) {
        final daysLeft = subscription.expiryDate
            .difference(DateTime.now())
            .inDays;
        if (daysLeft <= upgradeConfig.renewalReminderDays) {
          items.add(
            _ActionQueueItem(
              itemKey: const Key('seller-action-queue-subscription-expiring'),
              icon: Icons.timer_outlined,
              color: context.statusColors.warning,
              title: 'Subscription Segera Berakhir',
              subtitle: 'Berakhir dalam $daysLeft hari',
              route: RoutePaths.sellerRenewal,
            ),
          );
        }
      }
    }

    if (items.isEmpty) {
      return Container(
        width: double.infinity,
        padding: const EdgeInsets.all(AppMetrics.p16),
        decoration: BoxDecoration(
          color: scheme.surface,
          borderRadius: BorderRadius.circular(AppShape.r16),
          border: Border.all(color: scheme.outlineVariant),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Operasional toko siap',
              style: context.typeRoles.titleCompact.copyWith(
                fontWeight: FontWeight.bold,
                color: scheme.onSurface,
              ),
            ),
            const SizedBox(height: 4),
            Text(
              'Tidak ada tindakan yang menunggu saat ini.',
              style: context.typeRoles.labelMicro.copyWith(
                color: scheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      );
    }

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Antrian Tindakan Operasional',
            style: context.typeRoles.titleCompact.copyWith(
              fontWeight: FontWeight.bold,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 12),
          for (final item in items) ...[
            _ActionQueueTile(item: item),
            const SizedBox(height: 8),
          ],
        ],
      ),
    );
  }
}

class _ActionQueueItem {
  const _ActionQueueItem({
    required this.itemKey,
    required this.icon,
    required this.color,
    required this.title,
    required this.route,
    this.subtitle,
  });

  final Key itemKey;
  final IconData icon;
  final Color color;
  final String title;
  final String? subtitle;
  final String route;
}

class _ActionQueueTile extends StatelessWidget {
  const _ActionQueueTile({required this.item});

  final _ActionQueueItem item;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return InkWell(
      key: item.itemKey,
      onTap: () => context.push(item.route),
      borderRadius: BorderRadius.circular(AppShape.r12),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p12),
        decoration: BoxDecoration(
          color: scheme.surfaceContainerLow,
          borderRadius: BorderRadius.circular(AppShape.r12),
        ),
        child: Row(
          children: [
            Container(
              padding: const EdgeInsets.all(AppMetrics.p8),
              decoration: BoxDecoration(
                color: item.color.withValues(alpha: 0.15),
                shape: BoxShape.circle,
              ),
              child: Icon(
                item.icon,
                size: AppIconSize.action,
                color: item.color,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    item.title,
                    style: context.typeRoles.bodyDense.copyWith(
                      fontWeight: FontWeight.w600,
                      color: scheme.onSurface,
                    ),
                  ),
                  if (item.subtitle != null) ...[
                    const SizedBox(height: 2),
                    Text(
                      item.subtitle!,
                      style: context.typeRoles.labelMicro.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ],
              ),
            ),
            Icon(
              Icons.chevron_right,
              size: AppIconSize.action,
              color: scheme.onSurfaceVariant,
            ),
          ],
        ),
      ),
    );
  }
}
