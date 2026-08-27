/// Auction Detail Bottom Bar
///
/// Bottom action bar for auction detail
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_watcher.dart';

/// Bottom bar widget for auction detail actions
class AuctionDetailBottomBar extends StatelessWidget {
  final Auction auction;
  final AsyncValue<AuctionWatchStats> watchStatsAsync;
  final String currentUserId;
  final String currentUserName;
  final VoidCallback onWatch;
  final VoidCallback onChat;
  final VoidCallback onAction;
  final VoidCallback? onWinnerCheckout;
  final VoidCallback? onBrowseOtherAuctions;

  const AuctionDetailBottomBar({
    super.key,
    required this.auction,
    required this.watchStatsAsync,
    required this.currentUserId,
    required this.currentUserName,
    required this.onWatch,
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

  /// Check if auction is in a terminal state (no actions available)
  /// BOUNDARY NORMALIZATION (PHASE 1D): Status-based check only
  bool get _isTerminalState {
    return auction.status == AuctionStatus.cancelled ||
        auction.status == AuctionStatus.ended ||
        auction.status == AuctionStatus.expiredBNR;
  }

  /// Expired-seller visibility — true when the auction's seller has lapsed
  /// subscription. Treated as a terminal state for action purposes so the
  /// bid/buy-now trigger does not open the modal. Backend rejects the
  /// underlying calls regardless; this is a UX short-circuit.
  bool get _isSellerInactive =>
      auction.sellerTrustLifecycle != ContentLifecycle.active;

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

    // Expired BNR - winner didn't complete purchase
    if (auction.status == AuctionStatus.expiredBNR) {
      if (_isUserWinner) {
        return 'Waktu Habis';
      }
      return 'Pembayaran Habis';
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

    // Active auction
    // Framed as bidding action (buy now is handled in modal)
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

    // Active auction - allow bidding
    return onAction;
  }

  /// Check if we should show a secondary action button for terminal states
  /// TRANSACTION CLARITY: No dead-end - always provide next action
  bool get _showSecondaryAction {
    return _isTerminalState && onBrowseOtherAuctions != null;
  }

  /// Get the main action button color
  Color get _mainActionColor {
    // Winner checkout - use orange for urgency (waiting settlement)
    if (_shouldShowWinnerCheckout && onWinnerCheckout != null) {
      if (auction.status == AuctionStatus.waitingSettlement) {
        return Colors.orange; // Urgent - deadline approaching
      }
      return const Color(0xFFE53935); // Red for regular ended checkout
    }

    // Terminal states - gray
    if (_isTerminalState) {
      return Colors.grey;
    }

    // Scheduled - blue
    if (auction.status == AuctionStatus.scheduled) {
      return Colors.blue.shade300;
    }

    // Active auction - green
    return Colors.green;
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: Colors.white,
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.05),
            blurRadius: 4,
            offset: const Offset(0, -2),
          ),
        ],
      ),
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: SafeArea(
        top: false,
        child: _showSecondaryAction
            ? _buildTerminalStateLayout()
            : _buildNormalLayout(),
      ),
    );
  }

  /// Build layout for terminal states with secondary action
  /// TRANSACTION CLARITY: No dead-end - provide "Lihat Lelang Lain" button
  Widget _buildTerminalStateLayout() {
    return Row(
      children: [
        // Watch button
        watchStatsAsync.when(
          data: (stats) => _buildActionButton(
            icon: stats.isWatchedByCurrentUser
                ? Icons.visibility
                : Icons.visibility_outlined,
            label: stats.isWatchedByCurrentUser ? 'Tersimpan' : 'Simpan',
            onTap: onWatch,
            color: stats.isWatchedByCurrentUser ? Colors.blue : Colors.grey,
          ),
          loading: () => _buildActionButton(
            icon: Icons.visibility_outlined,
            label: 'Simpan',
            onTap: () {},
          ),
          error: (_, _) => _buildActionButton(
            icon: Icons.visibility_outlined,
            label: 'Simpan',
            onTap: onWatch,
          ),
        ),
        const SizedBox(width: 12),
        // Chat button
        _buildActionButton(
          icon: Icons.chat_bubble_outline,
          label: 'Chat',
          onTap: onChat,
        ),
        const SizedBox(width: 12),
        // Main action button (disabled, shows terminal state)
        Expanded(
          child: ElevatedButton(
            onPressed: null,
            style: ElevatedButton.styleFrom(
              backgroundColor: Colors.grey.shade300,
              foregroundColor: Colors.grey.shade600,
              disabledBackgroundColor: Colors.grey.shade300,
              padding: const EdgeInsets.symmetric(vertical: 14),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            child: Text(
              _mainActionLabel,
              style: const TextStyle(fontWeight: FontWeight.bold),
            ),
          ),
        ),
        const SizedBox(width: 8),
        // Secondary action button - "Lihat Lelang Lain"
        ElevatedButton(
          onPressed: onBrowseOtherAuctions,
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.primaryBlue,
            foregroundColor: Colors.white,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(8),
            ),
          ),
          child: const Text(
            'Lihat Lelang Lain',
            style: TextStyle(fontWeight: FontWeight.bold),
          ),
        ),
      ],
    );
  }

  /// Build normal layout with single action button
  Widget _buildNormalLayout() {
    return Row(
      children: [
        // Watch button
        watchStatsAsync.when(
          data: (stats) => _buildActionButton(
            icon: stats.isWatchedByCurrentUser
                ? Icons.visibility
                : Icons.visibility_outlined,
            label: stats.isWatchedByCurrentUser ? 'Tersimpan' : 'Simpan',
            onTap: onWatch,
            color: stats.isWatchedByCurrentUser ? Colors.blue : Colors.grey,
          ),
          loading: () => _buildActionButton(
            icon: Icons.visibility_outlined,
            label: 'Simpan',
            onTap: () {},
          ),
          error: (_, _) => _buildActionButton(
            icon: Icons.visibility_outlined,
            label: 'Simpan',
            onTap: onWatch,
          ),
        ),
        const SizedBox(width: 12),
        // Chat button
        _buildActionButton(
          icon: Icons.chat_bubble_outline,
          label: 'Chat',
          onTap: onChat,
        ),
        const SizedBox(width: 12),
        // Main action button
        Expanded(
          child: ElevatedButton(
            onPressed: _mainActionCallback,
            style: ElevatedButton.styleFrom(
              backgroundColor: _mainActionColor,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 14),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            child: Text(
              _mainActionLabel,
              style: const TextStyle(fontWeight: FontWeight.bold),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildActionButton({
    required IconData icon,
    required String label,
    required VoidCallback onTap,
    Color? color,
  }) {
    return InkWell(
      onTap: onTap,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, color: color ?? Colors.grey, size: 20),
          const SizedBox(height: 2),
          Text(label, style: const TextStyle(fontSize: 10)),
        ],
      ),
    );
  }
}
