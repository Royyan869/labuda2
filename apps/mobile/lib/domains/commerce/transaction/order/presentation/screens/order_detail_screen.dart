import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'order_detail/order_confirmation_section.dart';
import 'order_detail/order_refund_handler.dart';
import 'order_detail/order_refund_list_section.dart';
import 'order_detail/order_action_handler.dart';
import 'order_detail/direct_dispute_dialog.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart'
    as order_domain;
import 'package:labuda/domains/commerce/transaction/order/order.dart';
import 'package:labuda/domains/social/rating/rating.dart';
import 'package:labuda/domains/system/support/presentation/widgets/pre_chat_form_sheet.dart';
import 'package:labuda/domains/chat/chat/chat.dart';
import 'order_detail/order_detail_handlers.dart' show OrderDetailHandlersMixin;

/// Order Detail Screen - Detail pesanan dengan status tracking
class OrderDetailScreen extends ConsumerStatefulWidget {
  final String orderId;

  const OrderDetailScreen({super.key, required this.orderId});

  @override
  ConsumerState<OrderDetailScreen> createState() => _OrderDetailScreenState();
}

class _OrderDetailScreenState extends ConsumerState<OrderDetailScreen>
    with OrderDetailHandlersMixin {
  String get orderId => widget.orderId;

  /// Refresh order data after refund action
  void _refreshOrder() {
    ref.invalidate(watchOrderProvider(widget.orderId));
    ref.invalidate(refundsByOrderProvider(widget.orderId));
  }

  /// Calculate bottom padding based on action buttons visibility
  /// Different statuses show different buttons with varying heights
  double _calculateBottomPadding({
    required Order order,
    required bool isSeller,
    required bool isBuyer,
  }) {
    // Base padding when no buttons
    const basePadding = 24.0;
    // Container padding (16 top + 16 bottom) + SafeArea estimate
    const containerPadding = 32.0 + 34.0;
    // Single button height with padding
    const singleButtonHeight = 48.0;
    // Small gap between buttons
    const buttonGap = 8.0;

    // Seller action buttons
    if (isSeller) {
      // Seller buttons: single row of 1-2 buttons
      // O1: Removed 'processing' - not a real backend status
      if (order.status == OrderStatus.paid ||
          order.status == OrderStatus.shipped) {
        return containerPadding + singleButtonHeight + basePadding;
      }
      return basePadding; // No seller buttons for other statuses
    }

    // Buyer action buttons
    if (isBuyer && !isSeller) {
      switch (order.status) {
        case OrderStatus.pending:
          // Pay button + row of 2 buttons (Ubah Metode + Cancel)
          return containerPadding +
              singleButtonHeight +
              buttonGap +
              singleButtonHeight +
              basePadding;
        case OrderStatus.shipped:
        case OrderStatus.delivered:
          // Confirm receipt button
          return containerPadding + singleButtonHeight + basePadding;
        case OrderStatus.completed:
          // Rate button (if not rated)
          return containerPadding + singleButtonHeight + basePadding;
        default:
          return basePadding;
      }
    }

    return basePadding;
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final orderStream = ref.watch(watchOrderProvider(widget.orderId));
    final authState = ref.watch(authControllerProvider);
    final currentUserId = authState is AuthStateAuthenticated
        ? authState.user.id
        : null;

    return Scaffold(
      backgroundColor: isDark
          ? core.AppColors.darkGray900
          : core.AppColors.neutralGray50,
      appBar: AppBar(
        title: const Text('Order Details'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () {
            if (Navigator.of(context).canPop()) {
              Navigator.of(context).pop();
            }
          },
        ),
        backgroundColor: isDark
            ? core.AppColors.darkGray800
            : core.AppColors.neutralWhite,
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
      ),
      body: orderStream.when(
        data: (order) {
          final isSeller = currentUserId == order.sellerId;
          final isBuyer = currentUserId == order.buyerId;

          // Cek refund aktif (pending/approved) untuk order ini
          final refundAsync = ref.watch(refundsByOrderProvider(order.id));
          return refundAsync.when(
            data: (refunds) {
              // Calculate bottom padding based on action buttons visibility
              final bottomPadding = _calculateBottomPadding(
                order: order,
                isSeller: isSeller,
                isBuyer: isBuyer,
              );

              return SafeArea(
                child: Stack(
                  children: [
                    ListView(
                      // Dynamic bottom padding for action buttons
                      padding: EdgeInsets.fromLTRB(16, 16, 16, bottomPadding),
                      children: [
                        // Seller Action Required Banner (only for seller with pending action)
                        if (isSeller && order.isSellerActionRequired)
                          SellerActionRequiredBanner(
                            order: order,
                            onTapReview: () {
                              // Scroll to bottom where action buttons are
                              // (optional - buttons already sticky at bottom)
                            },
                          ),

                        OrderStatusTimeline(order: order, isDark: isDark),
                        const SizedBox(height: 16),

                        // ===== OVERDUE AWARENESS (SELLER/BUYER) =====
                        // Show overdue info card for paid overdue orders
                        OrderOverdueInfoCard(order: order),

                        // ===== SHIPPING READINESS CONTEXT (PAID ORDERS) =====
                        // Show preparation context when order is paid but not yet shipped
                        if (order.status == OrderStatus.paid)
                          _OrderPreparationSection(
                            order: order,
                            isDark: isDark,
                            onContactSeller: () =>
                                _handleContactSeller(context, order),
                            onContactSupport: () =>
                                _handleContactSupport(context, order),
                          ),

                        if (order.status == OrderStatus.paid)
                          const SizedBox(height: 16),

                        // ===== CONFIRMATION SYSTEM (SHIPPED/DELIVERED ORDERS) =====
                        if (order.status == OrderStatus.shipped ||
                            order.status == OrderStatus.delivered)
                          OrderConfirmationSection(
                            order: order,
                            isBuyer: isBuyer,
                            currentUserId: currentUserId,
                          ),

                        OrderInfoCard(order: order, isDark: isDark),
                        const SizedBox(height: 16),

                        // Seller/Buyer Info Card
                        if (currentUserId != null)
                          OrderUserInfoCard(
                            currentUserId: currentUserId,
                            sellerId: order.sellerId,
                            buyerId: order.buyerId,
                            sellerUsername: order.sellerUsername,
                            sellerFarmName: order.sellerFarmName,
                            sellerAvatarUrl: order.sellerAvatarUrl,
                            isDark: isDark,
                          ),
                        if (currentUserId != null) const SizedBox(height: 16),

                        OrderItemsCard(order: order, isDark: isDark),
                        const SizedBox(height: 16),
                        OrderShippingInfoCard(order: order, isDark: isDark),
                        const SizedBox(height: 16),
                        OrderPaymentInfoCard(order: order, isDark: isDark),
                        const SizedBox(height: 16),

                        // Rincian Pembayaran (paling bawah sebelum refund)
                        if (isSeller) ...[
                          OrderSellerPricingCard(order: order, isDark: isDark),
                          const SizedBox(height: 16),
                        ],
                        if (!isSeller) ...[
                          OrderBuyerPricingCard(order: order, isDark: isDark),
                          const SizedBox(height: 16),
                        ],

                        // ===== POST-COMPLETION CTA (SELLER ONLY) =====
                        // Show "Lihat Penghasilan" button for seller when order is completed
                        if (isSeller && order.status == OrderStatus.completed)
                          _SellerEarningsCTA(order: order, isDark: isDark),

                        const SizedBox(height: 8),
                        // ===== REFUND STATUS CARD (BUYER) =====
                        if (isBuyer && refunds.isNotEmpty)
                          ...refunds.map((r) => RefundStatusCard(refund: r)),
                        // ===== END REFUND STATUS CARD =====
                        // ===== REFUND LIST (SELLER/BUYER) =====
                        OrderRefundListSection(
                          refunds: refunds,
                          isDark: isDark,
                          currentUserId: currentUserId,
                          sellerId: order.sellerId,
                          onActionComplete: _refreshOrder,
                        ),
                        // ===== END REFUND LIST =====
                        // NOTE: Action buttons rendered by DynamicActionButtons
                        // based on backend Decision V2 contract (primary_action, secondary_actions)
                      ],
                    ),
                    // Dynamic action buttons (Decision V2 Contract from Backend)
                    if (isSeller || isBuyer)
                      Positioned(
                        left: 0,
                        right: 0,
                        bottom: 0,
                        child: Consumer(
                          builder: (context, ref, _) {
                            // Check if buyer has already rated this order
                            final hasRatedAsync = isBuyer
                                ? ref.watch(
                                    hasUserRatedOrderProvider(
                                      orderId: order.id,
                                      buyerId: order.buyerId,
                                      sellerId: order.sellerId,
                                    ),
                                  )
                                : const AsyncValue.data(false);

                            final hasRated = hasRatedAsync.when(
                              data: (rated) => rated,
                              loading: () => false,
                              error: (_, stack) => false,
                            );

                            return _buildDynamicActionButtons(
                              context: context,
                              order: order,
                              isSeller: isSeller,
                              isBuyer: isBuyer,
                              hasRated: hasRated,
                              refunds: refunds,
                              authState: authState,
                            );
                          },
                        ),
                      ),
                  ],
                ), // Stack
              ); // SafeArea
            },
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (error, stack) =>
                const Center(child: Text('Data belum bisa dimuat.')),
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
              const SizedBox(height: 16),
              ElevatedButton(
                onPressed: () => Navigator.pop(context),
                child: const Text('Back'),
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// Build dynamic action buttons based on backend Decision V2 contract
  ///
  /// Backend is the SINGLE SOURCE OF TRUTH for all UI actions.
  /// If decision is null, this is a BUG - fail explicitly.
  Widget _buildDynamicActionButtons({
    required BuildContext context,
    required Order order,
    required bool isSeller,
    required bool isBuyer,
    required bool hasRated,
    required List refunds,
    required AuthState authState,
  }) {
    // Backend MUST provide Decision V2 contract
    // If decision is null, it's a backend BUG - fail explicitly
    if (order.decision == null) {
      return _DecisionMissingWidget(orderStatus: order.status.value);
    }

    // Render buttons from backend Decision V2
    return DynamicActionButtons(
      decision: order.decision!,
      callbacks: ActionCallbacks(
        onAction: (action) => _handleAction(
          action: action,
          order: order,
          refunds: refunds,
          hasRated: hasRated,
          authState: authState,
        ),
        onRequestSupport: () => _handleRequestSupport(order, authState),
        onChatSeller: () => _handleChatSeller(order, authState),
      ),
    );
  }

  /// Handle action based on backend Decision V2 contract
  void _handleAction({
    required order_domain.Action action,
    required Order order,
    required List refunds,
    required bool hasRated,
    required AuthState authState,
  }) {
    final handler = OrderActionHandler(
      order: order,
      context: context,
      onAcceptOrder: handleAcceptOrder,
      onRejectOrder: handleRejectOrder,
      onShipOrder: (orderId, sellerId, proofData) =>
          handleShipOrder(orderId, sellerId, proofData),
      onConfirmDelivery: handleConfirmDelivery,
      onExtendConfirmation: handleExtendConfirmation,
      onRefundRequestRequest:
          ({
            required orderId,
            required orderSubtotal,
            required buyerId,
            required sellerId,
          }) => OrderRefundHandler.showRefundDialog(
            context: context,
            ref: ref,
            orderId: orderId,
            orderSubtotal: orderSubtotal,
            buyerId: buyerId,
            sellerId: sellerId,
          ),
      onRate: handleSubmitRating,
      onPayNow: (_) => handlePayNow(order),
      onChangePaymentMethod: (_) => handleChangePaymentMethod(order),
      onCancelOrder: (orderId, reason) => handleCancelOrder(orderId, reason),
      onOpenDispute: ({required orderId}) => DirectDisputeDialog.show(
        context: context,
        orderId: orderId,
        onDisputeOpened: () {
          // Refresh order detail after dispute opened
          ref.invalidate(orderRefreshTriggerProvider);
        },
      ),
      onRequestSupport: () => _handleRequestSupport(order, authState),
    );

    handler.handleAction(action);
  }

  /// Request support handler
  void _handleRequestSupport(Order order, AuthState authState) {
    if (authState is AuthStateAuthenticated) {
      showPreChatFormRefactored(
        context,
        userId: authState.user.id,
        userName: authState.user.username,
        userAvatar: authState.user.avatarUrl,
        linkedOrderId: order.id,
      );
    }
  }

  /// Chat with seller/buyer handler for commerce continuity
  ///
  /// BATCH 2B - DIRECT ORDER → CHAT CONTINUITY
  /// - Finds or creates canonical direct commerce room between buyer and seller
  /// - Links order to chat (LATEST ACTIVE ORDER RULE)
  /// - Navigates to chat with order context active
  void _handleChatSeller(Order order, AuthState authState) async {
    if (authState is! AuthStateAuthenticated) return;

    final currentUserId = authState.user.id;

    // Determine the other participant (buyer ↔ seller)
    // If current user is buyer, other is seller. If seller, other is buyer.
    final String otherUserId;
    if (order.buyerId == currentUserId) {
      otherUserId = order.sellerId;
    } else if (order.sellerId == currentUserId) {
      otherUserId = order.buyerId;
    } else {
      // User is not a participant in this order (shouldn't happen)
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Anda tidak terlibat dalam pesanan ini'),
          ),
        );
      }
      return;
    }

    // Show loading indicator
    if (mounted) {
      showDialog(
        context: context,
        barrierDismissible: false,
        builder: (context) => const Center(child: CircularProgressIndicator()),
      );
    }

    try {
      // Use usecase to handle commerce chat creation and order linking
      final getOrCreateCommerceChat = ref.read(
        getOrCreateCommerceChatUseCaseProvider,
      );
      final roomResult = await getOrCreateCommerceChat(
        currentUserId: currentUserId,
        otherUserId: otherUserId,
        orderId: order.id,
      );

      // Close loading dialog
      if (mounted) Navigator.of(context).pop();

      if (roomResult.isError || !mounted) {
        if (mounted && roomResult.isError) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('Gagal membuka chat. Coba lagi.')),
          );
        }
        return;
      }

      final room = roomResult.data!;

      // Navigate to chat with order context
      if (mounted) {
        context.go('/chat/${room.id}');
      }
    } catch (e) {
      if (mounted) {
        Navigator.of(context).pop(); // Close loading
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Terjadi kesalahan. Coba lagi.')),
        );
      }
    }
  }

  /// Handle contact seller from order preparation section
  void _handleContactSeller(BuildContext context, Order order) {
    final authState = ref.read(authControllerProvider);
    if (authState is AuthStateAuthenticated) {
      _handleChatSeller(order, authState);
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Silakan login terlebih dahulu')),
      );
    }
  }

  /// Handle contact support from order preparation section
  void _handleContactSupport(BuildContext context, Order order) {
    final authState = ref.read(authControllerProvider);
    if (authState is AuthStateAuthenticated) {
      _handleRequestSupport(order, authState);
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Silakan login terlebih dahulu')),
      );
    }
  }
}

/// Widget shown when backend Decision V2 contract is missing
/// This is a BUG - backend MUST provide decision for all order states
class _DecisionMissingWidget extends StatelessWidget {
  final String orderStatus;

  const _DecisionMissingWidget({required this.orderStatus});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? const Color(0xFF1E1E1E) : Colors.white,
        border: Border(
          top: BorderSide(
            color: isDark ? const Color(0xFF333333) : const Color(0xFFE0E0E0),
          ),
        ),
      ),
      child: SafeArea(
        top: false,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, color: Colors.orange, size: 32),
            const SizedBox(height: 12),
            Text(
              'Action Configuration Missing',
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w600,
                color: isDark ? Colors.white : Colors.black87,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Order status: $orderStatus',
              style: TextStyle(fontSize: 12, color: Colors.grey[600]),
            ),
            const SizedBox(height: 4),
            Text(
              'Please contact support',
              style: TextStyle(fontSize: 12, color: Colors.grey[600]),
            ),
          ],
        ),
      ),
    );
  }
}

/// Post-Completion CTA for Seller
/// Shows "Lihat Penghasilan" button when order is completed
class _SellerEarningsCTA extends StatelessWidget {
  final Order order;
  final bool isDark;

  const _SellerEarningsCTA({required this.order, required this.isDark});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 0),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark
            ? const Color(0xFF1E1E1E).withValues(alpha: 0.5)
            : Colors.green.shade50,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? const Color(0xFF333333) : Colors.green.shade200,
          width: 1,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.check_circle_outline,
                color: isDark ? Colors.green.shade300 : Colors.green.shade700,
                size: 20,
              ),
              const SizedBox(width: 8),
              Text(
                'Pesanan Selesai',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: isDark ? Colors.white : Colors.black87,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            'Pesanan telah selesai dan diproses.',
            style: TextStyle(
              fontSize: 12,
              color: isDark ? Colors.grey.shade400 : Colors.grey.shade700,
            ),
          ),
          const SizedBox(height: 12),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton.icon(
              onPressed: () {
                context.push(RoutePaths.sellerEarnings);
              },
              icon: const Icon(Icons.account_balance_wallet_outlined, size: 18),
              label: const Text('Lihat Penghasilan'),
              style: ElevatedButton.styleFrom(
                backgroundColor: isDark
                    ? Colors.green.shade700
                    : Colors.green.shade600,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 12),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Order Preparation Section - Shows shipping readiness context for paid orders
///
/// This widget displays the frozen preparation snapshot from when the order was created.
/// It helps buyers understand that the seller is preparing the fish for shipping.
///
/// OVERDUE DISPLAY LAYER:
/// - Shows overdue badge and warning when order is past ready_to_ship_by
/// - Displays tier-based warnings and CTAs
class _OrderPreparationSection extends StatelessWidget {
  final Order order;
  final bool isDark;
  final VoidCallback? onContactSeller;
  final VoidCallback? onContactSupport;

  const _OrderPreparationSection({
    required this.order,
    required this.isDark,
    this.onContactSeller,
    this.onContactSupport,
  });

  @override
  Widget build(BuildContext context) {
    final preparationTime = order.preparationTimeSnapshot;
    final preparationNote = order.preparationNoteSnapshot;
    final readyToShipBy = order.readyToShipBy;
    final isOverdue = order.isOverdue == true;
    final overdueTier = order.overdueTier;

    // Determine if we should show overdue UI
    final showOverdueUI =
        isOverdue && order.status == OrderStatus.paid && overdueTier != null;

    // Determine colors based on overdue tier
    Color getOverdueBadgeColor() {
      if (overdueTier == 'critical_overdue') return AppColors.statusError;
      if (overdueTier == 'severely_overdue') return AppColors.statusError;
      return AppColors.warning;
    }

    String getOverdueBadgeLabel() {
      if (overdueTier == 'critical_overdue') return 'Sangat Terlambat';
      if (overdueTier == 'severely_overdue') return 'Terlambat';
      return 'Melewati Estimasi';
    }

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: showOverdueUI
            ? (isDark
                  ? AppColors.statusError.withValues(alpha: 0.08)
                  : AppColors.statusError.withValues(alpha: 0.05))
            : (isDark
                  ? AppColors.darkGray800
                  : AppColors.primaryBlue.withValues(alpha: 0.05)),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: showOverdueUI
              ? (isDark
                    ? AppColors.statusError.withValues(alpha: 0.4)
                    : AppColors.statusError.withValues(alpha: 0.3))
              : (isDark
                    ? AppColors.primaryBlue.withValues(alpha: 0.3)
                    : AppColors.primaryBlue.withValues(alpha: 0.2)),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header
          Row(
            children: [
              Container(
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: showOverdueUI
                      ? AppColors.statusError.withValues(alpha: 0.15)
                      : AppColors.primaryBlue.withValues(alpha: 0.15),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  showOverdueUI
                      ? Icons.warning_amber_rounded
                      : Icons.access_time,
                  size: 18,
                  color: showOverdueUI
                      ? AppColors.statusError
                      : AppColors.primaryBlue,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      showOverdueUI
                          ? 'Pesanan Terlambat'
                          : 'Menunggu Penjual Menyiapkan Ikan',
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      showOverdueUI
                          ? 'Pesanan melewati estimasi siap kirim'
                          : 'Penjual sedang menyiapkan pesanan Anda',
                      style: TextStyle(
                        fontSize: 12,
                        color: isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray600,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),

          // OVERDUE UI: Badge and warning message
          if (showOverdueUI) ...[
            // Overdue badge
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
              decoration: BoxDecoration(
                color: getOverdueBadgeColor().withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(20),
                border: Border.all(
                  color: getOverdueBadgeColor().withValues(alpha: 0.3),
                ),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(
                    Icons.warning_amber_rounded,
                    size: 16,
                    color: AppColors.statusError,
                  ),
                  const SizedBox(width: 6),
                  Text(
                    getOverdueBadgeLabel(),
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: getOverdueBadgeColor(),
                    ),
                  ),
                ],
              ),
            ),

            // Overdue warning message
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: isDark
                    ? AppColors.darkGray700.withValues(alpha: 0.5)
                    : AppColors.neutralGray100,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(
                    Icons.info_outline,
                    size: 16,
                    color: getOverdueBadgeColor(),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      _getOverdueWarningMessage(overdueTier),
                      style: TextStyle(
                        fontSize: 13,
                        color: isDark
                            ? AppColors.neutralGray300
                            : AppColors.neutralGray700,
                      ),
                    ),
                  ),
                ],
              ),
            ),

            // CTA Buttons based on tier
            const SizedBox(height: 12),
            _buildOverdueCTAs(context, overdueTier),
          ],

          // NORMAL UI: Preparation time badge
          if (!showOverdueUI) ...[
            // Preparation time badge
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
              decoration: BoxDecoration(
                color: preparationTime.isImmediate
                    ? AppColors.successGreen.withValues(alpha: 0.1)
                    : AppColors.warning.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(20),
                border: Border.all(
                  color: preparationTime.isImmediate
                      ? AppColors.successGreen.withValues(alpha: 0.3)
                      : AppColors.warning.withValues(alpha: 0.3),
                ),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    preparationTime.isImmediate
                        ? Icons.flash_on
                        : Icons.schedule,
                    size: 16,
                    color: preparationTime.isImmediate
                        ? AppColors.successGreen
                        : AppColors.warning,
                  ),
                  const SizedBox(width: 6),
                  Text(
                    preparationTime.isImmediate
                        ? 'Siap dikirim segera'
                        : 'Estimasi siap kirim: ${preparationTime.displayName.toLowerCase()}',
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: preparationTime.isImmediate
                          ? AppColors.successGreen
                          : AppColors.warning,
                    ),
                  ),
                ],
              ),
            ),

            // Description
            if (!preparationTime.isImmediate) ...[
              const SizedBox(height: 12),
              Text(
                preparationTime.description,
                style: TextStyle(
                  fontSize: 13,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
              ),
            ],
          ],

          // Custom note from seller
          if (preparationNote != null && preparationNote.isNotEmpty) ...[
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: isDark
                    ? AppColors.darkGray700.withValues(alpha: 0.5)
                    : AppColors.neutralGray100,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Icon(
                    Icons.info_outline,
                    size: 14,
                    color: AppColors.primaryBlue,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      preparationNote,
                      style: TextStyle(
                        fontSize: 12,
                        color: isDark
                            ? AppColors.neutralGray300
                            : AppColors.neutralGray700,
                        fontStyle: FontStyle.italic,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],

          // Ready to ship by date (if available and not immediate)
          if (readyToShipBy != null && !preparationTime.isImmediate) ...[
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: isDark
                    ? AppColors.darkGray700.withValues(alpha: 0.5)
                    : AppColors.neutralGray100,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.event,
                    size: 14,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                  const SizedBox(width: 6),
                  Text(
                    'Target siap kirim: ${_formatDate(readyToShipBy)}',
                    style: TextStyle(
                      fontSize: 12,
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildOverdueCTAs(BuildContext context, String tier) {
    // Tier 1: "Ingatkan Penjual" only
    // Tier 2: "Chat Penjual" + "Hubungi Support"
    // Tier 3: "Hubungi Support" (emphasis)

    if (tier == 'overdue') {
      // Tier 1 - Single CTA
      return Row(
        children: [
          Expanded(
            child: _buildCTAButton(
              context,
              label: 'Ingatkan Penjual',
              icon: Icons.chat_bubble_outline,
              isPrimary: true,
              onTap: onContactSeller,
            ),
          ),
        ],
      );
    } else if (tier == 'severely_overdue') {
      // Tier 2 - Two CTAs
      return Row(
        children: [
          Expanded(
            child: _buildCTAButton(
              context,
              label: 'Chat Penjual',
              icon: Icons.chat_bubble_outline,
              isPrimary: false,
              onTap: onContactSeller,
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: _buildCTAButton(
              context,
              label: 'Hubungi Support',
              icon: Icons.support_agent,
              isPrimary: true,
              onTap: onContactSupport,
            ),
          ),
        ],
      );
    } else {
      // Tier 3 - Support emphasis
      return Row(
        children: [
          Expanded(
            child: _buildCTAButton(
              context,
              label: 'Hubungi Support',
              icon: Icons.support_agent,
              isPrimary: true,
              onTap: onContactSupport,
            ),
          ),
        ],
      );
    }
  }

  Widget _buildCTAButton(
    BuildContext context, {
    required String label,
    required IconData icon,
    required bool isPrimary,
    VoidCallback? onTap,
  }) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        decoration: BoxDecoration(
          color: isPrimary
              ? AppColors.statusError
              : (isDark ? AppColors.darkGray700 : AppColors.neutralGray200),
          borderRadius: BorderRadius.circular(8),
          border: isPrimary
              ? null
              : Border.all(
                  color: isDark
                      ? AppColors.darkGray600
                      : AppColors.neutralGray300,
                ),
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              icon,
              size: 16,
              color: isPrimary ? Colors.white : AppColors.neutralGray700,
            ),
            const SizedBox(width: 6),
            Text(
              label,
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: isPrimary ? Colors.white : AppColors.neutralGray700,
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _getOverdueWarningMessage(String tier) {
    if (tier == 'critical_overdue') {
      return 'Pesanan ini sangat terlambat dari estimasi siap kirim. Disarankan menghubungi support bantuan.';
    } else if (tier == 'severely_overdue') {
      return 'Pesanan ini terlambat dari estimasi siap kirim. Anda dapat menghubungi penjual atau support.';
    } else {
      return 'Pesanan ini sudah melewati estimasi siap kirim. Jika perlu, Anda dapat mengingatkan penjual.';
    }
  }

  String _formatDate(DateTime date) {
    return '${date.day}/${date.month}/${date.year}';
  }
}
