/// Auction Detail Screen
/// Menampilkan detail auction dengan live countdown dan bidding
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/core/api/api_error_codes.dart' as api_codes;
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/checkout_intent.dart';
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
import 'package:labuda/domains/chat/chat/presentation/utils/commerce_chat_navigation.dart';
import 'package:labuda/domains/chat/chat/presentation/models/pending_commerce_attachment.dart';
import 'package:labuda/domains/social/share/share.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_states.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_saved_item_action_button.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';

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
      return CommerceDetailStates.loading(title: 'Auction Detail');
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

  Widget _buildErrorScaffold(String error) {
    // TRANSACTION CLARITY: actionable, user-facing copy. The raw error never
    // reaches the screen — it stays in the notifier/log.
    String errorTitle = 'Gagal Memuat Lelang';
    String errorMessage = error;
    String actionLabel = 'Coba Lagi';
    VoidCallback? action = _loadAuctionData;

    // Parse common error patterns and provide actionable guidance
    if (error.contains('not found') || error.contains('404')) {
      return _buildNotFoundScaffold();
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
    } else {
      errorMessage = 'Data belum bisa dimuat. Coba lagi nanti.';
    }

    return CommerceDetailStates.error(
      title: 'Auction Detail',
      headline: errorTitle,
      message: errorMessage,
      actionLabel: actionLabel,
      onAction: action,
    );
  }

  Widget _buildNotFoundScaffold() {
    // TRANSACTION CLARITY: no dead-end — one next action.
    return CommerceDetailStates.notFound(
      title: 'Auction Detail',
      headline: 'Lelang Tidak Ditemukan',
      message: 'Lelang ini mungkin telah dihapus atau ID tidak valid.',
      actionLabel: 'Kembali',
      onAction: () => Navigator.pop(context),
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

    final handlers = AuctionDetailHandlers(
      ref: ref,
      context: context,
      auction: auction,
      auctionId: widget.auctionId,
      onEditSuccess: () {
        ref.invalidate(auctionDetailProvider(widget.auctionId));
        ref.invalidate(auctionStreamProvider(widget.auctionId));
      },
      onCancelSuccess: () => Navigator.pop(context),
    );

    return Scaffold(
      appBar: AppBarCustom(
        title: 'Auction Detail',
        showBackButton: true,
        actions: [
          // Save button — non-owners only.
          if (!_isCurrentUserTheCreator(auction) && currentUserId.isNotEmpty)
            CommerceSavedItemActionButton(
              targetType: 'auction',
              targetId: auction.id,
              label: 'Simpan',
              activeLabel: 'Tersimpan',
              icon: Icons.bookmark_border,
              activeIcon: Icons.bookmark,
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
            // No onDelete: the backend exposes no auction DELETE endpoint;
            // a dialog-only fake delete is phantom UI. Cancel is the only
            // owner lifecycle action on this surface.
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
              // CANONICAL DETAIL SKELETON (identical to ForSale):
              // media block → title → channel sections, each a 16-margin card.
              SliverToBoxAdapter(child: AuctionDetailHeader(auction: auction)),
              SliverToBoxAdapter(child: _AuctionDetailTitle(auction: auction)),
              SliverToBoxAdapter(
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(
                    AppMetrics.p16,
                    AppMetrics.p0,
                    AppMetrics.p16,
                    AppMetrics.p16,
                  ),
                  child: AuctionCountdownTimer(
                    auction: auction,
                    currentUserId: currentUserId.isNotEmpty
                        ? currentUserId
                        : null,
                  ),
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
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(
                      AppMetrics.p16,
                      AppMetrics.p0,
                      AppMetrics.p16,
                      AppMetrics.p16,
                    ),
                    child: AuctionSellerSettlementMonitor(
                      auction: auction,
                      currentUserId: currentUserId,
                    ),
                  ),
                ),
              // Bid position indicator - show user's current standing
              if (currentUserId.isNotEmpty)
                SliverToBoxAdapter(
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(
                      AppMetrics.p16,
                      AppMetrics.p0,
                      AppMetrics.p16,
                      AppMetrics.p16,
                    ),
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
                  padding: const EdgeInsets.fromLTRB(
                    AppMetrics.p16,
                    AppMetrics.p0,
                    AppMetrics.p16,
                    AppMetrics.p48,
                  ),
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
        currentUserId: currentUserId,
        currentUserName: currentUserName,
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
        ref.read(navigationHandlerProvider).navigateToSignIn();
      }
      return;
    }

    if (!mounted) return;
    await context.push<bool>(
      RoutePaths.reportLocation(
        targetType: ReportTargetType.auction.name,
        targetId: auction.id,
        targetTitle: auction.title,
      ),
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
      description: 'Current bid: Rp ${formatGroupedAmount(currentBid.round())}',
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

  Future<void> _handleChat(Auction auction) async {
    // Canonical commerce chat flow: opens/creates the buyer-seller room,
    // carries the auction as the canonical pending product attachment (sent
    // only through the composer send icon as a resourceOccurrence), and
    // handles the guest auth boundary internally (guest → canonical sign-in
    // route).
    await openCommerceChat(
      context: context,
      ref: ref,
      attachment: PendingCommerceAttachment.auction(
        auctionId: auction.id,
        title: auction.title,
        imageUrl: auction.media.isNotEmpty
            ? (auction.media.first.thumbnailUrl ??
                  auction.media.first.originalUrl)
            : null,
        price: auction.currentBid.round(),
      ),
      sellerId: auction.sellerId,
    );
  }

  void _showUnifiedActionModal(BuildContext context, Auction auction) {
    // GUEST (Model B parity with the fixed-price detail bar): the bid
    // affordance is presentation-only — route to the canonical sign-in flow
    // BEFORE any email/capability gate so a guest never sees an
    // authenticated-only error.
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      context.push(RoutePaths.signIn);
      return;
    }

    final isEmailVerified = ref.read(isEmailVerifiedProvider);

    if (!isEmailVerified) {
      AppSnackBar.showWarning(
        context,
        'Verifikasi email Anda untuk menawar atau membeli.',
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
    int amount,
  ) async {
    // AUTH-2 (CANONICAL AUTHORITY): read the hydrated current user from the
    // canonical authenticatedUserProvider instead of the legacy
    // authServiceProvider.getCurrentUser() wrapper — this removes the last
    // production consumer keeping the legacy IAuthenticationService alive.
    final currentUser = ref.read(authenticatedUserProvider);

    if (currentUser == null) {
      if (!mounted) return;
      ref.read(navigationHandlerProvider).navigateToSignIn();
      return;
    }

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
        'Bid berhasil! Rp ${formatGroupedAmount(amount)}',
      );
    } else {
      final notifierState = ref.read(auctionNotifierProvider);
      // Backend-rejection handler (defense-in-depth): the backend stays the
      // single authority for EMAIL_VERIFICATION_REQUIRED. The bidding chain
      // propagates the API code via Result.errorCode →
      // AuctionNotifierState.errorCode.
      if (notifierState.errorCode == api_codes.emailVerificationRequired) {
        if (!mounted) return;
        AppSnackBar.showError(
          this.context,
          'Verifikasi email kamu diperlukan sebelum menempatkan bid.',
        );
        return;
      }
      // Commerce restriction family — canonical dispatch by error CODE:
      // COMMERCE_RESTRICTED → restriction snackbar,
      // MARKET_AUTHORITY_REQUIRED → canonical seller renewal.
      // (mounted is already guaranteed by the guard above the pop.)
      if (CommerceRestrictionPresenter.handle(
        this.context,
        errorCode: notifierState.errorCode,
        actionDescription: 'menempatkan bid',
      )) {
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
                ? AppFormatters.formatShortDate(date)
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
        await AppDialog.info(
          context: this.context,
          title: 'Akses Lelang Dibatasi',
          message: message,
          closeLabel: 'Mengerti',
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
    // AUTH-2 (CANONICAL AUTHORITY): hydrated current user from the canonical
    // authenticatedUserProvider instead of legacy authServiceProvider.
    final currentUser = ref.read(authenticatedUserProvider);

    if (currentUser == null) {
      if (!mounted) return;
      ref.read(navigationHandlerProvider).navigateToSignIn();
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

    if (currentUser.id == auction.sellerId) {
      if (!mounted) return;
      AppSnackBar.showError(this.context, 'Anda tidak dapat membeli lelang Anda sendiri');
      return;
    }

    // STATE VALIDATION: Check auction state before proceeding
    // BOUNDARY NORMALIZATION (PHASE 1D): Status-based check only, backend is authoritative
    if (auction.status != AuctionStatus.active) {
      if (!mounted) return;
      if (auction.status == AuctionStatus.ended) {
        AppSnackBar.showError(this.context, 'Lelang sudah berakhir');
      } else {
        AppSnackBar.showError(this.context, 'Lelang tidak aktif');
      }
      return;
    }

    if (!mounted) return;

    // ONE FUNNEL: product id resolution, seller trust gate and the checkout
    // route shape are the commerce intent's job (openAuctionCheckout). The
    // detail screen builds no route and plumbs no product id of its own; the
    // path param slot (:fixedPriceSaleId) carries auction.id while
    // source_type='auction' + source_id=auction.id travel as query params.
    await openAuctionCheckout(
      context,
      ref,
      AuctionCheckoutIntent(auctionId: auction.id),
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

    // AUTH-2 (CANONICAL AUTHORITY): hydrated current user from the canonical
    // authenticatedUserProvider instead of legacy authServiceProvider.
    final currentUser = ref.read(authenticatedUserProvider);

    if (currentUser == null) {
      if (!mounted) return;
      ref.read(navigationHandlerProvider).navigateToSignIn();
      return;
    }

    // Verify user is the winner
    if (!_isUserWinner(auction, currentUser.id)) {
      if (!mounted) return;
      AppSnackBar.showError(
        this.context,
        'Anda bukan pemenang lelang ini',
      );
      return;
    }

    // Auction must have a productId for checkout integration
    if (auction.productId == null || auction.productId!.isEmpty) {
      if (!mounted) return;
      AppSnackBar.showError(
        this.context,
        'Tidak dapat melanjutkan checkout. Lelang ini tidak terhubung ke produk.',
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
    // The order has been created by the claim API, so the winner goes
    // directly to the canonical payment-result destination. The auction
    // detail page is replaced (not stacked) so back never returns to it.
    context.pushReplacement(RoutePaths.paymentResultPath(claimResult));
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
            String? shippingSetupId,
            String? shippingQuoteId,
            String? chatId,
            String? discountCode,
            bool useCoins = false,
          }) async {
            // Call claim API via notifier
            final notifier = ref.read(auctionNotifierProvider.notifier);
            final orderId = await notifier.claimAuction(
              auctionId: auction.id,
              addressId: addressId,
              shippingSetupId: shippingSetupId,
              shippingQuoteId: shippingQuoteId,
              chatId: chatId,
              discountCode: discountCode,
              useCoins: useCoins,
            );

            // Check result
            if (orderId != null) {
              return orderId;
            } else {
              if (!mounted) return null;
              final state = ref.read(auctionNotifierProvider);
              // Commerce restriction family — canonical dispatch by error
              // CODE: COMMERCE_RESTRICTED → restriction snackbar,
              // MARKET_AUTHORITY_REQUIRED → canonical seller renewal.
              // Returning null keeps the callback contract intact (modal stays
              // open, no second presentation) and never double-navigates.
              if (CommerceRestrictionPresenter.handle(
                this.context,
                errorCode: state.errorCode,
                actionDescription: 'mengklaim lelang',
              )) {
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

/// Canonical detail title block — the same slot, style and spacing the
/// ForSale detail uses right under the media gallery.
class _AuctionDetailTitle extends StatelessWidget {
  final Auction auction;

  const _AuctionDetailTitle({required this.auction});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p16,
        AppMetrics.p16,
        AppMetrics.p16,
      ),
      child: Text(
        auction.title,
        style: Theme.of(context).textTheme.headlineSmall,
      ),
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
    final scheme = Theme.of(context).colorScheme;
    return Container(
      margin: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p0,
        AppMetrics.p16,
        AppMetrics.p0,
      ),
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: context.statusColors.warning.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(
          color: context.statusColors.warning.withValues(alpha: 0.3),
          width: 1,
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            padding: const EdgeInsets.all(AppMetrics.p8),
            decoration: BoxDecoration(
              color: context.statusColors.warning.withValues(alpha: 0.15),
              shape: BoxShape.circle,
            ),
            child: Icon(
              Icons.warning_amber_rounded,
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
                  '⚠️ Selesaikan dalam 24 jam',
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.w600,
                    color: scheme.onSurface,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  'Jika tidak, Anda dapat dikenai pembatasan akun',
                  style: context.typeRoles.labelMicro.copyWith(
                    color: scheme.onSurfaceVariant,
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
