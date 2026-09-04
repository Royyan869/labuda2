/// Auction Detail Screen
/// Menampilkan detail auction dengan live countdown dan bidding
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/api/api_error_codes.dart' as api_codes;
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/target_type.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_providers.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_recommendation_providers.dart'
    show ownerOtherAuctionsProvider, similarAuctionsProvider;
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_action_modal.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_bid_history.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_bid_position_indicator.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_bid_section.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_countdown_timer.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_bottom_bar.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_handlers.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_header.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_info.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_recommendations_section.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_seller_card.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_seller_settlement_monitor.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_claim_shipping_modal.dart';
import 'package:labuda/domains/social/share/share.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/blocked_action_gate.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/dialogs/report_submission_dialog.dart';

/// Auction Detail Screen
///
/// Shows auction details with live countdown, bidding, buying, watching
/// Uses providers from auction_refactor
class AuctionDetailScreen extends ConsumerStatefulWidget {
  final String auctionId;

  const AuctionDetailScreen({super.key, required this.auctionId});

  @override
  ConsumerState<AuctionDetailScreen> createState() =>
      _AuctionDetailScreenState();
}

class _AuctionDetailScreenState extends ConsumerState<AuctionDetailScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadAuctionData());
  }

  void _loadAuctionData() {
    ref
        .read(auctionNotifierProvider.notifier)
        .loadAuctionDetails(widget.auctionId);
    ref
        .read(auctionNotifierProvider.notifier)
        .loadAuctionBids(widget.auctionId);
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(auctionNotifierProvider);
    final auctionAsync = ref.watch(auctionStreamProvider(widget.auctionId));
    final auction = auctionAsync.when(
      data: (data) => data ?? state.selectedAuction,
      loading: () => state.selectedAuction,
      error: (_, _) => state.selectedAuction,
    );

    final bidsAsync = ref.watch(auctionBidsStreamProvider(widget.auctionId));
    final liveBids = bidsAsync.when<List<AuctionBid>>(
      data: (data) => data,
      loading: () => state.bids.cast<AuctionBid>(),
      error: (_, _) => state.bids.cast<AuctionBid>(),
    );

    final authState = ref.watch(authControllerProvider);
    final currentUserId = authState is AuthStateAuthenticated
        ? authState.user.id
        : '';
    final currentUserName = authState is AuthStateAuthenticated
        ? authState.user.username
        : '';

    if (state.isLoading && auction == null) {
      return _buildLoadingScaffold();
    }
    if (state.error != null && auction == null) {
      return _buildErrorScaffold(state.error!);
    }
    if (auction == null) {
      return _buildNotFoundScaffold();
    }

    return _buildAuctionDetail(
      context,
      auction,
      liveBids,
      currentUserId,
      currentUserName,
    );
  }

  Widget _buildLoadingScaffold() {
    return Scaffold(
      appBar: AppBar(title: const Text('Detail Auction')),
      body: const Center(child: CircularProgressIndicator()),
    );
  }

  Widget _buildErrorScaffold(String error) {
    // TRANSACTION CLARITY: Provide actionable error messages
    // Instead of generic "Error: ...", give specific guidance
    String errorTitle = 'Gagal Memuat Lelang';
    String errorMessage = error;
    String actionLabel = 'Coba Lagi';
    VoidCallback? action = _loadAuctionData;

    // Parse common error patterns and provide actionable guidance
    if (error.contains('not found') || error.contains('404')) {
      errorTitle = 'Lelang Tidak Ditemukan';
      errorMessage = 'Lelang ini mungkin telah dihapus atau tidak tersedia.';
      actionLabel = 'Lihat Lelang Lain';
      action = () => Navigator.pop(context);
    } else if (error.contains('network') || error.contains('connection')) {
      errorTitle = 'Koneksi Bermasalah';
      errorMessage = 'Periksa koneksi internet Anda dan coba lagi.';
      actionLabel = 'Coba Lagi';
      action = _loadAuctionData;
    } else if (error.contains('permission') || error.contains('403')) {
      errorTitle = 'Akses Ditolak';
      errorMessage = 'Anda tidak memiliki akses ke lelang ini.';
      actionLabel = 'Kembali';
      action = () => Navigator.pop(context);
    } else if (error.contains('expired') || error.contains('ended')) {
      errorTitle = 'Lelang Telah Berakhir';
      errorMessage = 'Lelang ini sudah berakhir dan tidak dapat diakses.';
      actionLabel = 'Lihat Lelang Lain';
      action = () => Navigator.pop(context);
    }

    return Scaffold(
      appBar: AppBar(title: const Text('Detail Auction')),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(Icons.error_outline, size: 64, color: AppColors.statusError),
              const SizedBox(height: 16),
              Text(
                errorTitle,
                style: const TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.bold,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 8),
              Text(
                errorMessage,
                style: TextStyle(fontSize: 14, color: AppColors.neutralGray600),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: action,
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primaryRed,
                  foregroundColor: Colors.white,
                  padding: const EdgeInsets.symmetric(
                    horizontal: 24,
                    vertical: 12,
                  ),
                ),
                child: Text(actionLabel),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildNotFoundScaffold() {
    // TRANSACTION CLARITY: No dead-end - provide next action
    return Scaffold(
      appBar: AppBar(title: const Text('Detail Auction')),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(Icons.search_off, size: 64, color: AppColors.neutralGray400),
              const SizedBox(height: 16),
              const Text(
                'Lelang Tidak Ditemukan',
                style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 8),
              Text(
                'Lelang ini mungkin telah dihapus atau ID tidak valid.',
                style: TextStyle(fontSize: 14, color: AppColors.neutralGray600),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: () => Navigator.pop(context),
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primaryRed,
                  foregroundColor: Colors.white,
                  padding: const EdgeInsets.symmetric(
                    horizontal: 24,
                    vertical: 12,
                  ),
                ),
                child: const Text('Lihat Lelang Lain'),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildAuctionDetail(
    BuildContext context,
    Auction auction,
    List<AuctionBid> liveBids,
    String currentUserId,
    String currentUserName,
  ) {
    final ownerOtherAuctionsAsync = ref.watch(
      ownerOtherAuctionsProvider(widget.auctionId),
    );
    final similarAuctionsAsync = ref.watch(
      similarAuctionsProvider(widget.auctionId),
    );

    final watchStatsAsync = ref.watch(
      watchStatsStreamProvider((
        auctionId: auction.id,
        currentUserId: currentUserId,
      )),
    );

    final handlers = AuctionDetailHandlers(
      ref: ref,
      context: context,
      auction: auction,
      auctionId: widget.auctionId,
      onEditSuccess: () {
        ref.invalidate(auctionDetailProvider(widget.auctionId));
        ref.invalidate(auctionStreamProvider(widget.auctionId));
      },
      onDeleteSuccess: () => Navigator.pop(context),
    );

    return Scaffold(
      appBar: AppBarCustom(
        title: 'Auction Detail',
        showBackButton: true,
        actions: [
          if (_isCurrentUserTheCreator(auction) &&
              (auction.status == AuctionStatus.active ||
                  auction.status == AuctionStatus.scheduled))
            IconButton(
              onPressed: () {
                context.push(
                  RoutePaths.sellerPromotionActivate,
                  extra: {
                    'preselectedTargetType': TargetType.auction,
                    'preselectedTargetId': auction.id,
                    'preselectedTargetTitle': auction.title,
                  },
                );
              },
              icon: const Icon(Icons.campaign_outlined),
              tooltip: 'Promote',
            ),
          // Share button — authenticated users only; anonymous viewers
          // cannot post to feed and have no interaction authority.
          if (currentUserId.isNotEmpty)
            IconButton(
              onPressed: () => _handleShareAuction(context, auction),
              icon: const Icon(Icons.share_outlined),
              tooltip: 'Bagikan',
            ),
          PopupMoreOptionsButton(
            isCreator: _isCurrentUserTheCreator(auction),
            isDeleting: false,
            contentType: PopupMoreOptionsContentType.auction,
            onEdit: null, // Disabled - auction editing is desktop-only
            onDelete: () => handlers.handleDelete(),
            onReport: !_isCurrentUserTheCreator(auction)
                ? () => _handleReportAuction(context, auction)
                : null,
            onCancel: auction.status == AuctionStatus.active
                ? () => handlers.handleCancel()
                : null,
          ),
        ],
      ),
      body: SafeArea(
        bottom: false,
        child: RefreshIndicator(
          onRefresh: () async => _loadAuctionData(),
          child: CustomScrollView(
            slivers: [
              SliverToBoxAdapter(child: AuctionDetailHeader(auction: auction)),
              SliverToBoxAdapter(
                child: AuctionCountdownTimer(
                  auction: auction,
                  currentUserId: currentUserId.isNotEmpty
                      ? currentUserId
                      : null,
                ),
              ),
              // STEP 1: WARNING DI DETAIL (WAITING SETTLEMENT)
              // BNR WARNING & TRUST SIGNAL - Settlement deadline warning
              if (auction.status == AuctionStatus.waitingSettlement &&
                  currentUserId.isNotEmpty &&
                  auction.isUserWinner(currentUserId))
                SliverToBoxAdapter(
                  child: _SettlementWarningBanner(auction: auction),
                ),
              // SELLER SETTLEMENT MONITOR - Show seller the winner info and status
              if (currentUserId.isNotEmpty)
                SliverToBoxAdapter(
                  child: AuctionSellerSettlementMonitor(
                    auction: auction,
                    currentUserId: currentUserId,
                  ),
                ),
              // Bid position indicator - show user's current standing
              if (currentUserId.isNotEmpty)
                SliverToBoxAdapter(
                  child: Padding(
                    padding: const EdgeInsets.symmetric(vertical: 12),
                    child: AuctionBidPositionIndicator(
                      auction: auction,
                      userBids: liveBids
                          .where((bid) => bid.bidderId == currentUserId)
                          .toList(),
                      currentUserId: currentUserId,
                      onBidAgain: () =>
                          _showUnifiedActionModal(context, auction),
                    ),
                  ),
                ),
              SliverToBoxAdapter(child: AuctionBidSection(auction: auction)),
              SliverToBoxAdapter(child: AuctionDetailInfo(auction: auction)),
              SliverToBoxAdapter(child: AuctionSellerCard(auction: auction)),
              SliverToBoxAdapter(child: AuctionBidHistory(bids: liveBids)),
              SliverToBoxAdapter(
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(16, 24, 16, 80),
                  child: AuctionRecommendationsSection(
                    currentAuction: auction,
                    ownerOtherAuctions: ownerOtherAuctionsAsync,
                    similarAuctions: similarAuctionsAsync,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
      bottomNavigationBar: AuctionDetailBottomBar(
        auction: auction,
        watchStatsAsync: watchStatsAsync,
        currentUserId: currentUserId,
        currentUserName: currentUserName,
        onWatch: () => _handleWatch(auction, currentUserId),
        onChat: () => _handleChat(auction),
        onAction: () => _showUnifiedActionModal(context, auction),
        onWinnerCheckout: _shouldShowWinnerCheckout(auction, currentUserId)
            ? () => _handleWinnerCheckout(context, auction)
            : null,
        // TRANSACTION CLARITY: No dead-end - provide next action for terminal states
        onBrowseOtherAuctions: () => Navigator.pop(context),
      ),
    );
  }

  bool _isCurrentUserTheCreator(Auction auction) {
    final authState = ref.watch(authControllerProvider);
    return authState is AuthStateAuthenticated &&
        authState.user.id == auction.sellerId;
  }

  Future<void> _handleReportAuction(
    BuildContext context,
    Auction auction,
  ) async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      if (mounted) {
        AppSnackBar.showError(context, 'Silakan masuk untuk melaporkan lelang');
      }
      return;
    }

    if (!mounted) return;
    await ReportSubmissionDialog.show(
      context,
      targetId: auction.id,
      targetType: ReportTargetType.auction,
      targetTitle: auction.title,
    );
  }

  Future<void> _handleShareAuction(
    BuildContext context,
    Auction auction,
  ) async {
    final currentBid = auction.currentBid;

    // Create ShareTarget for auction
    final shareTarget = ShareTarget(
      id: auction.id,
      type: ExternalShareType.auction,
      title: auction.title,
      description: 'Current bid: Rp ${currentBid.toStringAsFixed(0)}',
      imageUrl: auction.media.isNotEmpty
          ? auction.media.first.originalUrl
          : null,
    );

    // Show share bottom sheet with both internal and external sharing options
    // canSharePost=false because auction is not a Post, but user can still share to Feed
    await ShareBottomSheet.show(
      context: context,
      target: shareTarget,
      canSharePost:
          false, // Auctions share to Feed as new posts, not as reposts
    );
  }

  Future<void> _handleWatch(Auction auction, String currentUserId) async {
    if (currentUserId.isEmpty) {
      AppSnackBar.showError(context, 'You must be logged in to watch auction');
      return;
    }

    try {
      final repository = ref.read(auctionWatchRepositoryProvider);
      final isWatchingResult = await repository.isWatching(
        auctionId: auction.id,
        userId: currentUserId,
      );

      if (isWatchingResult.isError) {
        if (!mounted) return;
        AppSnackBar.showError(
          context,
          isWatchingResult.error ?? 'An error occurred',
        );
        return;
      }

      final isCurrentlyWatching = isWatchingResult.data ?? false;
      if (isCurrentlyWatching) {
        await repository.unwatchAuction(
          auctionId: auction.id,
          userId: currentUserId,
        );
      } else {
        await repository.watchAuction(
          auctionId: auction.id,
          userId: currentUserId,
        );
      }

      ref.invalidate(
        watchStatsStreamProvider((
          auctionId: auction.id,
          currentUserId: currentUserId,
        )),
      );
    } catch (e) {
      if (!mounted) return;
      AppSnackBar.showError(context, 'Terjadi kesalahan. Coba lagi.');
    }
  }

  void _handleChat(Auction auction) {
    // HONEST MINIMAL: Navigate to new chat screen (existing real route)
    // The fake ChatNavigationHelper.navigateToAuctionChat did not exist
    Navigator.of(context).pushNamed(RoutePaths.newChat);
  }

  void _showUnifiedActionModal(BuildContext context, Auction auction) {
    final isEmailVerified = ref.read(isEmailVerifiedProvider);

    if (!isEmailVerified) {
      AppSnackBar.showWarning(
        context,
        'Please verify your email to place bids or buy items.',
      );
      return;
    }

    AuctionActionModal.show(
      context,
      auction: auction,
      onPlaceBid: (amount) => _handlePlaceBid(context, auction, amount),
      onBuyNow: () => _handleBuyNow(context, auction),
    );
  }

  Future<void> _handlePlaceBid(
    BuildContext context,
    Auction auction,
    double amount,
  ) async {
    final authService = ref.read(authServiceProvider);
    final userResult = await authService.getCurrentUser();

    if (userResult.isError || userResult.data == null) {
      if (!mounted) return;
      AppSnackBar.showError(
        this.context,
        'You must be logged in to place a bid',
      );
      return;
    }

    final currentUser = userResult.data!;
    if (!mounted) {
      return;
    }
    showDialog(
      context: this.context,
      barrierDismissible: false,
      builder: (context) => const Center(child: CircularProgressIndicator()),
    );

    final success = await ref
        .read(auctionNotifierProvider.notifier)
        .placeBid(
          auctionId: auction.id,
          bidderId: currentUser.id,
          amount: amount,
        );

    if (!mounted) return;
    Navigator.of(this.context).pop();

    if (success) {
      AppSnackBar.showSuccess(
        this.context,
        'Bid successful! Rp ${amount.toStringAsFixed(0)}',
      );
    } else {
      final notifierState = ref.read(auctionNotifierProvider);
      // Inline gate: backend rejected because the user's email is not
      // verified. The bidding chain now propagates the API code via
      // RepositoryResult.errorCode → AuctionNotifierState.errorCode.
      if (notifierState.errorCode == api_codes.emailVerificationRequired) {
        if (!mounted) return;
        await showBlockedActionGate(
          this.context,
          actionDescription: 'menempatkan bid',
        );
        return;
      }
      if (CommerceRestrictionPresenter.isCommerceRestricted(notifierState.errorCode)) {
        if (!mounted) return;
        CommerceRestrictionPresenter.show(
          this.context,
          actionDescription: 'menempatkan bid',
        );
        return;
      }
      if (notifierState.errorCode == api_codes.bnrAuctionRestricted) {
        if (!mounted) return;
        final details = notifierState.errorDetails;
        final permanentBan = details?['permanent_ban'] == true;
        String message;
        if (permanentBan) {
          message =
              'Anda tidak dapat mengikuti lelang karena beberapa kali '
              'tidak menyelesaikan pembayaran lelang.';
        } else {
          final until = details?['restriction_until'] as String?;
          if (until != null) {
            final date = DateTime.tryParse(until);
            final formatted = date != null
                ? '${date.day}/${date.month}/${date.year}'
                : until;
            message =
                'Anda sementara tidak dapat mengikuti lelang karena '
                'pelanggaran BNR. Coba lagi setelah $formatted.';
          } else {
            message =
                'Anda sementara tidak dapat mengikuti lelang karena '
                'pelanggaran BNR.';
          }
        }
        await showDialog<void>(
          context: this.context,
          builder: (dialogContext) => AlertDialog(
            title: const Text('Akses Lelang Dibatasi'),
            content: Text(message),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(dialogContext).pop(),
                child: const Text('Mengerti'),
              ),
            ],
          ),
        );
        return;
      }
      AppSnackBar.showError(
        this.context,
        notifierState.error ?? 'Gagal memasang bid. Coba lagi.',
      );
    }
  }

  Future<void> _handleBuyNow(BuildContext context, Auction auction) async {
    final authService = ref.read(authServiceProvider);
    final userResult = await authService.getCurrentUser();

    if (userResult.isError || userResult.data == null) {
      if (!mounted) return;
      AppSnackBar.showError(
        this.context,
        'Anda harus login untuk menggunakan Buy Now',
      );
      return;
    }

    // SELLER TRUST GATE: Block BuyNow when seller subscription expired.
    // Auction bottom bar already disables the button, but this is defense-in-depth.
    if (auction.sellerTrustLifecycle != ContentLifecycle.active) {
      if (!mounted) return;
      AppSnackBar.showError(
        this.context,
        'Penjual tidak aktif — transaksi tidak dapat dilanjutkan',
      );
      return;
    }

    final currentUser = userResult.data!;
    if (currentUser.id == auction.sellerId) {
      if (!mounted) return;
      AppSnackBar.showError(this.context, 'You cannot buy your own auction');
      return;
    }

    // STATE VALIDATION: Check auction state before proceeding
    // BOUNDARY NORMALIZATION (PHASE 1D): Status-based check only, backend is authoritative
    if (auction.status != AuctionStatus.active) {
      if (!mounted) return;
      if (auction.status == AuctionStatus.ended) {
        AppSnackBar.showError(this.context, 'This auction has ended');
      } else {
        AppSnackBar.showError(this.context, 'This auction is not active');
      }
      return;
    }

    // CHECKOUT INTEGRATION: Navigate to checkout with auction context
    // Auction must have a productId for checkout integration
    if (auction.productId == null || auction.productId!.isEmpty) {
      if (!mounted) return;
      AppSnackBar.showError(
        this.context,
        'Unable to proceed with checkout. This auction is not linked to a product.',
      );
      return;
    }

    if (!mounted) return;

    // Navigate to checkout with auction context for buy-now flow.
    // Path param slot (:fixedPriceSaleId) carries auction.id since this is
    // an auction surface — source_type='auction' and source_id=auction.id
    // are conveyed via query params so the checkout screen derives correct
    // preview and order context.
    this.context.push(
      '/checkout/${auction.id}?product_id=${auction.productId ?? ''}&auction_id=${auction.id}',
    );
  }

  /// Check if current user is the auction winner
  bool _isUserWinner(Auction auction, String currentUserId) {
    if (currentUserId.isEmpty) return false;
    return auction.winnerId == currentUserId;
  }

  /// Check if winner should see checkout CTA
  bool _shouldShowWinnerCheckout(Auction auction, String currentUserId) {
    // User must be the winner
    if (!_isUserWinner(auction, currentUserId)) return false;

    // Winner can checkout in ended (legacy) or waiting_settlement states
    return auction.status == AuctionStatus.ended ||
        auction.status == AuctionStatus.waitingSettlement;
  }

  // ========== Claim Flow ==========

  /// Handle auction winner claim flow
  ///
  /// NEW FLOW (per spec):
  /// 1. Tap "Klaim Sekarang"
  /// 2. Show shipping selection dialog
  /// 3. CALL /auctions/:id/claim (creates order, returns order_id)
  /// 4. Navigate to payment result with order_id
  ///
  /// SINGLE SOURCE OF TRUTH: claim API creates the order
  /// Checkout is only for payment, NOT order creation for claimed auctions
  Future<void> _handleWinnerCheckout(
    BuildContext context,
    Auction auction,
  ) async {
    // SELLER TRUST GATE: Block winner claim when seller subscription expired.
    // Bottom bar already disables the button, but this is defense-in-depth.
    if (auction.sellerTrustLifecycle != ContentLifecycle.active) {
      if (!mounted) return;
      AppSnackBar.showError(
        this.context,
        'Penjual tidak aktif — transaksi tidak dapat dilanjutkan',
      );
      return;
    }

    final authService = ref.read(authServiceProvider);
    final userResult = await authService.getCurrentUser();

    if (userResult.isError || userResult.data == null) {
      if (!mounted) return;
      AppSnackBar.showError(this.context, 'You must be logged in to proceed');
      return;
    }

    final currentUser = userResult.data!;

    // Verify user is the winner
    if (!_isUserWinner(auction, currentUser.id)) {
      if (!mounted) return;
      AppSnackBar.showError(
        this.context,
        'You are not the winner of this auction',
      );
      return;
    }

    // Auction must have a productId for checkout integration
    if (auction.productId == null || auction.productId!.isEmpty) {
      if (!mounted) return;
      AppSnackBar.showError(
        this.context,
        'Unable to proceed with checkout. This auction is not linked to a product.',
      );
      return;
    }

    if (!mounted) return;

    // STEP 1: Show shipping selection dialog
    // (AuctionClaimShippingModal handles address + delivery option pickers.)
    final claimResult = await _showClaimDialog(this.context, auction);

    if (!mounted) return;

    if (claimResult == null) {
      // User cancelled or error occurred
      return;
    }

    // STEP 2: Claim succeeded - navigate to payment result
    // The order has been created by the claim API
    // Navigate directly to payment result with the order_id
    Navigator.of(
      this.context,
    ).pushReplacementNamed('/payment-result/$claimResult');
  }

  /// Show claim dialog and execute claim API call
  ///
  /// Returns order_id on success, null on failure/cancel
  Future<String?> _showClaimDialog(
    BuildContext context,
    Auction auction,
  ) async {
    // Show shipping selection modal with real address and delivery option selection
    return AuctionClaimShippingModal.show(
      context: context,
      auction: auction,
      onClaim:
          ({
            required addressId,
            required shippingSetupId,
            String? discountCode,
            bool useCoins = false,
          }) async {
            // Call claim API via notifier
            final notifier = ref.read(auctionNotifierProvider.notifier);
            final orderId = await notifier.claimAuction(
              auctionId: auction.id,
              addressId: addressId,
              shippingSetupId: shippingSetupId,
              discountCode: discountCode,
              useCoins: useCoins,
            );

            // Check result
            if (orderId != null) {
              return orderId;
            } else {
              if (!mounted) return null;
              final state = ref.read(auctionNotifierProvider);
              // Commerce restriction — canonical backend rejection.
              if (CommerceRestrictionPresenter.isCommerceRestricted(state.errorCode)) {
                CommerceRestrictionPresenter.show(
                  this.context,
                  actionDescription: 'mengklaim lelang',
                );
                return null;
              }
              // Generic error fallback
              final error = state.error ?? 'Gagal mengklaim lelang';
              AppSnackBar.showError(this.context, error);
              return null;
            }
          },
    );
  }
}

/// STEP 1: WARNING DI DETAIL (WAITING SETTLEMENT)
/// BNR WARNING & TRUST SIGNAL - Settlement deadline warning banner
///
/// Shows urgent warning to winner about 24-hour settlement deadline
/// with trust consequences for non-compliance
class _SettlementWarningBanner extends StatelessWidget {
  final Auction auction;

  const _SettlementWarningBanner({required this.auction});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.fromLTRB(16, 0, 16, 0),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFFFFF7ED), // Light orange background
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: const Color(0xFFF97316).withValues(alpha: 0.3),
          width: 1,
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            padding: const EdgeInsets.all(6),
            decoration: BoxDecoration(
              color: const Color(0xFFF97316).withValues(alpha: 0.15),
              shape: BoxShape.circle,
            ),
            child: const Icon(
              Icons.warning_amber_rounded,
              color: Color(0xFFF97316),
              size: 18,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '⚠️ Selesaikan dalam 24 jam',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: const Color(0xFF9A3412),
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  'Jika tidak, Anda dapat dikenai pembatasan akun',
                  style: TextStyle(
                    fontSize: 12,
                    color: const Color(0xFF9A3412),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
