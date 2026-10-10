/// Auction Detail Bottom Bar
///
/// Bottom action bar for auction detail
library;

import 'package:flutter/material.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:hishumi/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/shared.dart';

/// Bottom bar widget for auction detail actions.
///
/// Business state only (viewer capabilities, status, trust gate). Chrome is
/// owned by [BottomActionBar].
class AuctionDetailBottomBar extends StatelessWidget {
  final Auction auction;
  final String currentUserId;
  final String currentUserName;
  final VoidCallback onChat;
  final VoidCallback onAction;
  final VoidCallback? onWinnerCheckout;
  final VoidCallback? onBrowseOtherAuctions;

  const AuctionDetailBottomBar({
    super.key,
    required this.auction,
    required this.currentUserId,
    required this.currentUserName,
    required this.onChat,
    required this.onAction,
    this.onWinnerCheckout,
    this.onBrowseOtherAuctions,
  });

  /// Check if current user is the auction winner
  bool get _isUserWinner {
    if (currentUserId.isEmpty) return false;
    return auction.winnerId == currentUserId;
  }

  /// Check if winner should see checkout CTA
  bool get _shouldShowWinnerCheckout {
    if (!_isUserWinner) return false;
    // Winner can checkout in ended (legacy) or waiting_settlement states
    return auction.status == AuctionStatus.ended ||
        auction.status == AuctionStatus.waitingSettlement;
  }

  bool get _isTerminalState {
    return auction.status == AuctionStatus.cancelled ||
        auction.status == AuctionStatus.ended ||
        auction.status == AuctionStatus.lapsed;
  }

  /// Expired-seller visibility — true when the auction's seller has lapsed
  /// subscription. Treated as a terminal state for action purposes so the
  /// bid/buy-now trigger does not open the modal. Backend rejects the
  /// underlying calls regardless; this is a UX short-circuit.
  bool get _isSellerInactive =>
      auction.sellerTrustLifecycle != ContentLifecycle.active;

  /// Canonical per-viewer action authority from the detail wire
  /// (`viewer_capabilities`). Null only on non-detail payloads
  /// (list/discovery) that do not carry viewer-scoped capabilities.
  CommerceViewerCapabilities? get _capabilities => auction.viewerCapabilities;

  /// Whether the primary bid CTA may open the action modal.
  ///
  /// Canonical authority when present: the backend evaluator's `can_bid`
  /// for the BUYER role. Absence path (non-detail payload) and the GUEST
  /// path keep the status/trust presentation gate so the shared read model
  /// stays consistent.
  ///
  /// GUEST (Model B parity with the fixed-price detail bar): the affordance
  /// stays visible on raw facts; the tap routes to the canonical sign-in
  /// flow. Owner never bids on their own auction.
  bool get _canPlaceBid {
    final caps = _capabilities;
    if (caps == null) return false;
    return caps.canBid;
  }

  /// Whether the chat action is available for this viewer. Canonical
  /// authority when present (`can_chat`); otherwise keep the current
  /// always-visible behavior for non-detail payloads.
  bool get _showChat {
    final caps = _capabilities;
    return caps == null || caps.canChat;
  }

  /// Get the main action button label
  String get _mainActionLabel {
    // Winner checkout - highest priority
    // Frame as claiming victory rather than generic payment
    if (_shouldShowWinnerCheckout && onWinnerCheckout != null) {
      return 'Klaim Sekarang';
    }

    // Cancelled auction
    if (auction.status == AuctionStatus.cancelled) {
      return 'Lelang Dibatalkan';
    }

    // Scheduled auction
    if (auction.status == AuctionStatus.scheduled) {
      return 'Terjadwal';
    }

    // Ended auction - differentiate between sold and expired
    // BOUNDARY NORMALIZATION (PHASE 1D): Status-based check only
    if (auction.status == AuctionStatus.ended) {
      if (auction.isExpired) {
        return 'Tidak Ada Pemenang';
      }
      if (auction.isSold) {
        // User is not the winner (checked above), so someone else won
        return 'Terjual';
      }
      return 'Lelang Berakhir';
    }

    if (auction.status == AuctionStatus.waitingSettlement) {
      return 'Menunggu Penyelesaian';
    }

    if (auction.status == AuctionStatus.lapsed) {
      return 'Lelang Kedaluarsa';
    }

    // Active auction. Buy Now is handled in modal by can_buy_now.
    return 'Pasang Bid';
  }

  /// Get the main action button callback
  VoidCallback? get _mainActionCallback {
    // SELLER TRUST GATE: Must be checked FIRST — disables ALL transaction
    // actions (winner claim, bid, buy-now) when seller subscription expired.
    // Backend Guard 6 also rejects, but this prevents a wasted round-trip.
    if (_isSellerInactive) {
      return null;
    }

    // Winner checkout
    if (_shouldShowWinnerCheckout && onWinnerCheckout != null) {
      return onWinnerCheckout;
    }

    // No action for terminal states
    if (_isTerminalState) {
      return null;
    }

    // Scheduled auction - no action yet
    if (auction.status == AuctionStatus.scheduled) {
      return null;
    }

    // Active auction — bidding only when the canonical per-viewer capability
    // allows it (buyer + can_bid). Owner/guest/seller-inactive → disabled.
    if (!_canPlaceBid) {
      return null;
    }
    return onAction;
  }

  /// Check if we should show a secondary action button for terminal states
  /// TRANSACTION CLARITY: No dead-end - always provide next action
  bool get _showSecondaryAction {
    return _isTerminalState && onBrowseOtherAuctions != null;
  }

  // CTA fill is NOT decided here. The button theme owns the brand action
  // colour (`scheme.primary`), exactly like the ForSale detail bar's
  // "Beli Sekarang" — one action fill for both sale channels. A per-state
  // status tone on a CTA was a second authority (and green on "Pasang Bid"
  // made the auction bar read as a success state). Urgency keeps its status
  // tone on the countdown/timer surfaces, where it marks a real deadline.

  @override
  Widget build(BuildContext context) {
    // Chat affordance — canonical viewer capability (can_chat) when present.
    final leading = _showChat
        ? [
            BottomBarIconAction(
              icon: Icons.chat_bubble_outline,
              label: 'Chat',
              onPressed: onChat,
            ),
          ]
        : <BottomBarIconAction>[];

    if (_showSecondaryAction) {
      // TRANSACTION CLARITY: No dead-end - provide "Lihat Lelang Lain" button.
      // The terminal state renders as the disabled secondary; the browse CTA
      // is the brand primary (one action fill — the old `scheme.secondary`
      // fill was a second CTA colour authority).
      return BottomActionBar(
        leading: leading,
        secondary: BottomBarAction(
          label: _mainActionLabel,
          onPressed: null,
        ),
        primary: BottomBarAction(
          label: 'Lihat Lelang Lain',
          onPressed: onBrowseOtherAuctions,
        ),
      );
    }

    return BottomActionBar(
      leading: leading,
      primary: BottomBarAction(
        label: _mainActionLabel,
        onPressed: _mainActionCallback,
      ),
    );
  }
}
