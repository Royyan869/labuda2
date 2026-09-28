import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart';
import 'package:labuda/shared/utils/app_formatters.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:labuda/domains/user/identity/verification/verification.dart';
import 'package:labuda/domains/system/support/presentation/screens/help_center_screen.dart';
import 'package:labuda/domains/system/support/presentation/widgets/pre_chat_form_sheet.dart';
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
    // Canonical expiry axis. Capability `inactive` is NOT expiry: a freshly
    // onboarded seller without a settled payment is capability-inactive with
    // subscription status 'none' and must not see renewal copy (RF-02).
    final isSubscriptionExpired = ref.watch(isSellerSubscriptionExpiredProvider);

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
          // App Bar
          SliverAppBar(
            expandedHeight: 120,
            floating: false,
            pinned: true,
            backgroundColor: Theme.of(context).colorScheme.surface,
            flexibleSpace: FlexibleSpaceBar(
              title: const Text(
                'Dashboard Penjual',
                style: TextStyle(fontWeight: FontWeight.bold),
              ),
              background: Container(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                    colors: [
                      Theme.of(context).colorScheme.primary,
                      Theme.of(context).colorScheme.primary.withValues(alpha: 0.8),
                    ],
                  ),
                ),
              ),
            ),
          ),

          // Content
          SliverToBoxAdapter(
            child: _buildContent(
              context,
              sellerId,
              isSubscriptionExpired,
            ),
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
              size: 64,
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
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
        backgroundColor: Theme.of(context).colorScheme.surface,
        foregroundColor: Theme.of(context).colorScheme.onSurface,
        elevation: AppElevation.none,
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
                  size: 64,
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
                          size: 16,
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
                    icon: const Icon(Icons.arrow_back, size: 18),
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
                    icon: const Icon(Icons.storefront, size: 18),
                    label: const Text('Mulai Jualan'),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: Theme.of(context).colorScheme.primary,
                      foregroundColor:
                          Theme.of(context).colorScheme.onPrimary,
                      padding: const EdgeInsets.symmetric(
                        horizontal: AppMetrics.p24,
                        vertical: AppMetrics.p12,
                      ),
                    ),
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
                  fontSize: AppType.s13,
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
            width: 24,
            height: 24,
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
                fontSize: AppType.s13,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildContent(
    BuildContext context,
    String sellerId,
    bool isSubscriptionExpired,
  ) {
    return Padding(
      padding: const EdgeInsets.all(AppMetrics.p16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Verification Status Section
          const _VerificationStatusSection(),

          const SizedBox(height: 16),

          // Subscription expiry banner - UX signal only, authority unchanged.
          // Expired sellers can still use workspace routes; this is a renewal prompt.
          _SubscriptionExpiryBanner(),

          // PHASE 2 HARDENING: Seller Action Required Card
          // Shows prominently when seller has pending/paid orders needing action
          _SellerActionRequiredCard(sellerId: sellerId),

          const SizedBox(height: 16),

          // PHASE 2 HARDENING: Seller Chat Workspace Bridge
          // Connects seller workspace to chat - chat is primary work tool for sellers
          const _SellerChatWorkspaceSection(),

          const SizedBox(height: 16),

          // Getting Started Section (shown only for new sellers)
          _GettingStartedSection(
            sellerId: sellerId,
            isSubscriptionExpired: isSubscriptionExpired,
          ),

          const SizedBox(height: 16),

          // Statistics Cards
          _OrderStatsSection(sellerId: sellerId),

          const SizedBox(height: 24),

          // Operational action queue: one queue for every actionable seller
          // task (spec: seller_dashboard_operational_action_queue_test).
          OperationalActionQueueSection(sellerId: sellerId),

          const SizedBox(height: 24),

          // Quick Actions
          _QuickActionsSection(),

          const SizedBox(height: 16),

          // PHASE 3 HARDENING: Seller Help Section
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
// VERIFICATION STATUS SECTION
// =============================================================================

class _VerificationStatusSection extends ConsumerStatefulWidget {
  const _VerificationStatusSection();

  @override
  ConsumerState<_VerificationStatusSection> createState() =>
      _VerificationStatusSectionState();
}

class _VerificationStatusSectionState
    extends ConsumerState<_VerificationStatusSection> {
  bool _requestedInitialLoad = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || _requestedInitialLoad) return;
      _requestedInitialLoad = true;
      ref.read(sellerVerificationV2NotifierProvider.notifier).loadStatus();
    });
  }

  @override
  Widget build(BuildContext context) {
    final verificationState = ref.watch(sellerVerificationV2NotifierProvider);

    if (verificationState.isLoading &&
        verificationState.status == SellerVerificationStatus.notSubmitted) {
      return _buildLoadingCard();
    }

    if (verificationState.errorMessage != null &&
        verificationState.status == SellerVerificationStatus.notSubmitted) {
      return _buildErrorCard(verificationState.errorMessage!);
    }

    Color statusColor;
    String statusText;
    String statusDescription;
    IconData statusIcon;
    Color buttonColor;

    switch (verificationState.status) {
      case SellerVerificationStatus.approved:
        statusColor = context.statusColors.success;
        statusText = 'Terverifikasi';
        statusDescription = 'Akun penjual Anda telah diverifikasi';
        statusIcon = Icons.verified;
        buttonColor = context.statusColors.success;
        break;
      case SellerVerificationStatus.pendingReview:
        statusColor = context.statusColors.warning;
        statusText = 'Menunggu Verifikasi';
        statusDescription = 'Dokumen sedang ditinjau (1-2 hari kerja)';
        statusIcon = Icons.pending;
        buttonColor = context.statusColors.warning;
        break;
      case SellerVerificationStatus.rejected:
        statusColor = context.statusColors.error;
        statusText = 'Verifikasi Ditolak';
        statusDescription = 'Mohon periksa dokumen dan ajukan kembali';
        statusIcon = Icons.cancel;
        buttonColor = Theme.of(context).colorScheme.primary;
        break;
      default:
        statusColor = Theme.of(context).colorScheme.outline;
        statusText = 'Belum Diverifikasi';
        statusDescription = 'Verifikasi diperlukan untuk menarik dana';
        statusIcon = Icons.info_outline;
        buttonColor = Theme.of(context).colorScheme.primary;
    }

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: statusColor.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: statusColor.withValues(alpha: 0.3)),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(AppMetrics.p10),
            decoration: BoxDecoration(
              color: statusColor.withValues(alpha: 0.2),
              shape: BoxShape.circle,
            ),
            child: Icon(statusIcon, color: statusColor, size: 24),
          ),
          const SizedBox(width: 16),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  statusText,
                  style: TextStyle(
                    fontSize: AppType.s16,
                    fontWeight: FontWeight.bold,
                    color: statusColor,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  statusDescription,
                  style: TextStyle(
                    fontSize: AppType.s12,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
          if (!verificationState.isVerified)
            TextButton(
              onPressed: () => context.push(RoutePaths.sellerVerification),
              style: TextButton.styleFrom(
                foregroundColor: buttonColor,
                padding: const EdgeInsets.symmetric(
                  horizontal: AppMetrics.p16,
                  vertical: AppMetrics.p8,
                ),
              ),
              child: const Text('Verifikasi'),
            )
          else
            Icon(Icons.check_circle, color: statusColor, size: 24),
        ],
      ),
    );
  }

  Widget _buildLoadingCard() {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(
          color: Theme.of(context).colorScheme.outlineVariant,
        ),
      ),
      child: const Row(
        children: [
          SizedBox(
            width: 20,
            height: 20,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
          SizedBox(width: 16),
          Expanded(
            child: Text(
              'Memuat status verifikasi...',
              style: TextStyle(fontSize: AppType.s14),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildErrorCard(String message) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: context.statusColors.error.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: context.statusColors.error.withValues(alpha: 0.3)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.error_outline,
                color: context.statusColors.error,
                size: 24,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  'Gagal memuat status verifikasi',
                  style: TextStyle(
                    fontSize: AppType.s16,
                    fontWeight: FontWeight.bold,
                    color: context.statusColors.error,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            message,
            style: TextStyle(fontSize: AppType.s12, color: Theme.of(context).colorScheme.onSurfaceVariant),
          ),
          const SizedBox(height: 12),
          TextButton(
            onPressed: () => ref
                .read(sellerVerificationV2NotifierProvider.notifier)
                .loadStatus(),
            child: const Text('Coba Lagi'),
          ),
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
                      size: 20,
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
                icon: const Icon(Icons.refresh_outlined, size: 18),
                label: const Text('Perpanjang Langganan'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: context.statusColors.warning,
                  foregroundColor: Theme.of(context).colorScheme.onPrimary,
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
                    size: 24,
                  ),
                ),
                // Unread badge
                if (totalUnread > 0)
                  Positioned(
                    top: 0,
                    right: 0,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: AppMetrics.p6,
                        vertical: AppMetrics.p2,
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
                          fontSize: AppType.s10,
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
                      fontSize: AppType.s13,
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
              size: 22,
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
// SELLER ACTION REQUIRED CARD (PHASE 2 HARDENING)
// =============================================================================
/// Prominent card shown when seller has orders that need immediate action
/// This addresses the PRIORITY 1 gap: seller awareness of new/pending orders
class _SellerActionRequiredCard extends ConsumerWidget {
  final String sellerId;

  const _SellerActionRequiredCard({
    required this.sellerId,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Watch pending and paid orders (action-required statuses)
    final pendingAsync = ref.watch(
      watchSellerOrdersProvider(
        sellerId: sellerId,
        status: OrderStatus.pending,
      ),
    );
    final paidAsync = ref.watch(
      watchSellerOrdersProvider(sellerId: sellerId, status: OrderStatus.paid),
    );

    return pendingAsync.when(
      data: (pendingOrders) {
        return paidAsync.when(
          data: (paidOrders) {
            final actionRequiredCount =
                pendingOrders.length + paidOrders.length;

            // Hide card if no action-required orders
            if (actionRequiredCount == 0) {
              return const SizedBox.shrink();
            }

            return Container(
              padding: const EdgeInsets.all(AppMetrics.p16),
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                  colors: [
                    Theme.of(context).colorScheme.primary.withValues(alpha: 0.15),
                    Theme.of(context).colorScheme.primary.withValues(alpha: 0.08),
                  ],
                ),
                borderRadius: BorderRadius.circular(AppShape.r16),
                border: Border.all(
                  color: Theme.of(context).colorScheme.primary.withValues(alpha: 0.4),
                  width: 1.5,
                ),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      // Pulsing notification icon
                      Stack(
                        children: [
                          Container(
                            padding: const EdgeInsets.all(AppMetrics.p10),
                            decoration: BoxDecoration(
                              color: Theme.of(context).colorScheme.primary.withValues(
                                alpha: 0.2,
                              ),
                              shape: BoxShape.circle,
                            ),
                            child: Icon(
                              Icons.notifications_active,
                              color: Theme.of(context).colorScheme.primary,
                              size: 24,
                            ),
                          ),
                          // Pulsing dot for urgency
                          Positioned(
                            top: 0,
                            right: 0,
                            child: Container(
                              width: 12,
                              height: 12,
                              decoration: BoxDecoration(
                                color: Theme.of(context).colorScheme.primary,
                                shape: BoxShape.circle,
                                border: Border.all(
                                  color: Theme.of(context).colorScheme.surface,
                                  width: 2,
                                ),
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(width: 16),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              'Pesanan Perlu Tindakan',
                              style: TextStyle(
                                fontSize: AppType.s16,
                                fontWeight: FontWeight.bold,
                                color: Theme.of(context).colorScheme.onSurface,
                              ),
                            ),
                            const SizedBox(height: 4),
                            Text(
                              _getActionMessage(
                                pendingCount: pendingOrders.length,
                                paidCount: paidOrders.length,
                              ),
                              style: TextStyle(
                                fontSize: AppType.s13,
                                color: Theme.of(context).colorScheme.onSurfaceVariant,
                              ),
                            ),
                          ],
                        ),
                      ),
                      // Badge count
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: AppMetrics.p12,
                          vertical: AppMetrics.p6,
                        ),
                        decoration: BoxDecoration(
                          color: Theme.of(context).colorScheme.primary,
                          borderRadius: BorderRadius.circular(AppShape.r20),
                        ),
                        child: Text(
                          actionRequiredCount.toString(),
                          style: TextStyle(
                            color: Theme.of(context).colorScheme.onPrimary,
                            fontSize: AppType.s14,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 16),

                  // Action buttons row
                  Row(
                    children: [
                      Expanded(
                        child: _ActionChip(
                          icon: Icons.pending_actions,
                          label: 'Pending',
                          count: pendingOrders.length,
                          color: context.statusColors.warning,
                          onTap: () => _navigateToOrderList(
                            context,
                            OrderStatus.pending,
                          ),
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: _ActionChip(
                          icon: Icons.local_shipping,
                          label: 'Siap Kirim',
                          count: paidOrders.length,
                          color: Theme.of(context).colorScheme.secondary,
                          onTap: () =>
                              _navigateToOrderList(context, OrderStatus.paid),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            );
          },
          loading: () => const SizedBox.shrink(),
          error: (_, _) => const SizedBox.shrink(),
        );
      },
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
    );
  }

  String _getActionMessage({
    required int pendingCount,
    required int paidCount,
  }) {
    if (pendingCount > 0 && paidCount > 0) {
      return '$pendingCount perlu konfirmasi, $paidCount siap dikirim';
    } else if (pendingCount > 0) {
      return '$pendingCount pesanan menunggu konfirmasi';
    } else if (paidCount > 0) {
      return '$paidCount pesanan siap untuk dikirim';
    }
    return 'Tidak ada pesanan yang perlu tindakan';
  }

  void _navigateToOrderList(BuildContext context, OrderStatus status) {
    Navigator.push(
      context,
      MaterialPageRoute(builder: (context) => OrderListScreen(isSeller: true)),
    );
  }
}

/// Action chip for the seller action required card
class _ActionChip extends StatelessWidget {
  final IconData icon;
  final String label;
  final int count;
  final Color color;
  final VoidCallback onTap;

  const _ActionChip({
    required this.icon,
    required this.label,
    required this.count,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    if (count == 0) {
      return const SizedBox.shrink();
    }

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r12),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p10),
        decoration: BoxDecoration(
          color: Theme.of(context).colorScheme.surface.withValues(alpha: 0.8),
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(color: color.withValues(alpha: 0.3)),
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, size: 16, color: color),
            const SizedBox(width: 6),
            Text(
              label,
              style: TextStyle(
                fontSize: AppType.s13,
                fontWeight: FontWeight.w600,
                color: Theme.of(context).colorScheme.onSurface,
              ),
            ),
            const SizedBox(width: 4),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p6, vertical: AppMetrics.p2),
              decoration: BoxDecoration(
                color: color.withValues(alpha: 0.15),
                borderRadius: BorderRadius.circular(AppShape.r10),
              ),
              child: Text(
                count.toString(),
                style: TextStyle(
                  fontSize: AppType.s11,
                  fontWeight: FontWeight.bold,
                  color: color,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// =============================================================================
// GETTING STARTED SECTION
// =============================================================================

class _GettingStartedSection extends ConsumerWidget {
  final String sellerId;
  final bool isSubscriptionExpired;

  const _GettingStartedSection({
    required this.sellerId,
    required this.isSubscriptionExpired,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Check if seller has any orders or forSales
    final allOrdersAsync = ref.watch(
      watchSellerOrdersProvider(sellerId: sellerId, status: null),
    );

    return allOrdersAsync.when(
      data: (orders) {
        // Only show getting started if seller has no orders yet
        if (orders.isNotEmpty) return const SizedBox.shrink();

        // Only an ENDED subscription replaces getting started with expiry copy.
        // A capability-inactive seller with status 'none' is not expired.
        if (isSubscriptionExpired) {
          return _buildExpiredSellerMessage(context, ref);
        }

        return Container(
          padding: const EdgeInsets.all(AppMetrics.p16),
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
              colors: [
                Theme.of(context).colorScheme.primary.withValues(alpha: 0.1),
                Theme.of(context).colorScheme.primary.withValues(alpha: 0.05),
              ],
            ),
            borderRadius: BorderRadius.circular(AppShape.r16),
            border: Border.all(
              color: Theme.of(context).colorScheme.primary.withValues(alpha: 0.3),
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
                      color: Theme.of(context).colorScheme.primary.withValues(alpha: 0.2),
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      Icons.rocket_launch_outlined,
                      color: Theme.of(context).colorScheme.primary,
                      size: 20,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Mulai Jualan',
                          style: TextStyle(
                            fontSize: AppType.s16,
                            fontWeight: FontWeight.bold,
                            color: Theme.of(context).colorScheme.onSurface,
                          ),
                        ),
                        Text(
                          '3 langkah untuk mulai mendapatkan pesanan',
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
              const SizedBox(height: 20),
              _StepItem(
                number: 1,
                title: 'Atur Pengiriman',
                description: 'Wajib sebelum publish forSale pertama Anda',
                isCompleted: false,
                onTap: () => _navigateToShipping(context),
              ),
              const SizedBox(height: 12),
              _StepItem(
                number: 2,
                title: 'Buat ForSale',
                description: 'Tambahkan produk yang ingin Anda jual',
                isCompleted: false,
                onTap: () => _navigateToCreateForSale(context),
              ),
              const SizedBox(height: 12),
              _StepItem(
                number: 3,
                title: 'Verifikasi Akun',
                description: 'Syarat untuk menarik dana penjualan',
                isCompleted: false,
                onTap: () => _navigateToVerification(context),
              ),
            ],
          ),
        );
      },
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
    );
  }

  Widget _buildExpiredSellerMessage(
    BuildContext context,
    WidgetRef ref,
  ) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            context.statusColors.error.withValues(alpha: 0.1),
            context.statusColors.error.withValues(alpha: 0.05),
          ],
        ),
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(
          color: context.statusColors.error.withValues(alpha: 0.3),
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
                  color: context.statusColors.error.withValues(alpha: 0.2),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  Icons.error_outline,
                  color: context.statusColors.error,
                  size: 20,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Langganan Berakhir',
                      style: TextStyle(
                        fontSize: AppType.s16,
                        fontWeight: FontWeight.bold,
                        color: Theme.of(context).colorScheme.onSurface,
                      ),
                    ),
                    Text(
                      'Perbarui langganan untuk mulai jual kembali',
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
          const SizedBox(height: 16),
          ElevatedButton.icon(
            onPressed: () => context.push(RoutePaths.sellerRenewal),
            icon: const Icon(Icons.refresh_outlined, size: 18),
            label: const Text('Perpanjang Langganan'),
            style: ElevatedButton.styleFrom(
              backgroundColor: context.statusColors.error,
              foregroundColor: Theme.of(context).colorScheme.onError,
              minimumSize: const Size(double.infinity, 44),
            ),
          ),
        ],
      ),
    );
  }

  void _navigateToCreateForSale(BuildContext context) {
    Navigator.pushNamed(context, RoutePaths.createForSale);
  }

  void _navigateToVerification(BuildContext context) {
    context.push(RoutePaths.sellerVerification);
  }

  void _navigateToShipping(BuildContext context) {
    context.push(RoutePaths.sellerShipping);
  }
}

class _StepItem extends StatelessWidget {
  final int number;
  final String title;
  final String description;
  final bool isCompleted;
  final VoidCallback? onTap;

  const _StepItem({
    required this.number,
    required this.title,
    required this.description,
    required this.isCompleted,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r12),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p12),
        decoration: BoxDecoration(
          color: Theme.of(context).colorScheme.surface.withValues(alpha: 0.7),
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(
            color: Theme.of(context).colorScheme.outlineVariant.withValues(alpha: 0.5),
          ),
        ),
        child: Row(
          children: [
            // Step number circle
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(
                color: isCompleted
                    ? context.statusColors.success
                    : Theme.of(context).colorScheme.primary.withValues(alpha: 0.2),
                shape: BoxShape.circle,
              ),
              child: Center(
                child: isCompleted
                    ? Icon(Icons.check, color: Theme.of(context).colorScheme.onPrimary, size: 18)
                    : Text(
                        number.toString(),
                        style: TextStyle(
                          color: Theme.of(context).colorScheme.primary,
                          fontWeight: FontWeight.bold,
                          fontSize: AppType.s14,
                        ),
                      ),
              ),
            ),
            const SizedBox(width: 12),
            // Step content
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    style: TextStyle(
                      fontWeight: FontWeight.w600,
                      fontSize: AppType.s14,
                      color: Theme.of(context).colorScheme.onSurface,
                    ),
                  ),
                  const SizedBox(height: 2),
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
            // Arrow button if actionable
            if (onTap != null)
              Icon(
                Icons.chevron_right,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
                size: 20,
              ),
          ],
        ),
      ),
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
                    size: 32,
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
              Icon(icon, size: 18, color: color),
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
                key: const Key('seller-quick-action-auctions'),
                icon: Icons.gavel_outlined,
                label: 'Lelang Saya',
                color: Theme.of(context).colorScheme.secondary,
                // Spec pins the auctions quick action onto the canonical
                // seller inventory surface: there is no /seller/auctions
                // route anywhere in the router.
                onTap: () => context.push(RoutePaths.sellerForSales),
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
              child: Icon(icon, color: color, size: 24),
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
    final userId = authState is AuthStateAuthenticated
        ? authState.user.id
        : null;
    final userName = authState is AuthStateAuthenticated
        ? authState.user.username
        : null;
    final userAvatar = authState is AuthStateAuthenticated
        ? authState.user.avatarUrl
        : null;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: [
            context.statusColors.success.withValues(alpha: 0.1),
            context.statusColors.success.withValues(alpha: 0.05),
          ],
        ),
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(
          color: context.statusColors.success.withValues(alpha: 0.3),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.lightbulb_outline,
                color: context.statusColors.success,
                size: 20,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  'Tips & Bantuan Penjual',
                  style: TextStyle(
                    fontSize: AppType.s14,
                    fontWeight: FontWeight.w600,
                    color: Theme.of(context).colorScheme.onSurface,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          _HelpTile(
            icon: Icons.visibility_outlined,
            title: 'ForSale tidak terlihat?',
            description: 'Pelajari cara membuat forSale yang lebih menarik',
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
          ),
          const SizedBox(height: 8),
          _HelpTile(
            icon: Icons.payments_outlined,
            title: 'Info pembayaran & pendapatan',
            description: 'Cek kapan dana masuk ke saldo',
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
          ),
          const SizedBox(height: 8),
          _HelpTile(
            icon: Icons.support_agent,
            title: 'Hubungi Support Penjual',
            description: 'Dapatkan bantuan langsung dari tim kami',
            onTap: userId != null
                ? () {
                    showPreChatFormRefactored(
                      context,
                      userId: userId,
                      userName: userName ?? 'User',
                      userAvatar: userAvatar,
                    );
                  }
                : null,
          ),
        ],
      ),
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
              child: Icon(icon, color: context.statusColors.success, size: 16),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    style: TextStyle(
                      fontSize: AppType.s13,
                      fontWeight: FontWeight.w600,
                      color: Theme.of(context).colorScheme.onSurface,
                    ),
                  ),
                  Text(
                    description,
                    style: TextStyle(
                      fontSize: AppType.s11,
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
            Icon(
              Icons.chevron_right,
              size: 16,
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
                  width: 48,
                  height: 48,
                  color: Theme.of(context).colorScheme.surfaceContainerHighest,
                  child: const Icon(Icons.image_not_supported, size: 20),
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
          fontSize: AppType.s11,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}
