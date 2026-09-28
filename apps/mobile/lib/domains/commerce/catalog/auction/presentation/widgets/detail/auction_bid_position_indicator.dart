/// Auction Bid Position Indicator
///
/// Shows user's current bid position in the auction
/// - Leading: User's bid is the current highest
/// - Outbid: Someone else has a higher bid (with actionable button)
/// - No bid: User hasn't placed any bid yet
library;

import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';

/// Bid position status
enum BidPosition {
  /// User's bid is the current highest
  leading,

  /// Someone else has a higher bid
  outbid,

  /// User hasn't placed any bid yet
  noBid,

  /// Auction is not active
  notActive,
}

/// Bid position indicator widget
class AuctionBidPositionIndicator extends StatelessWidget {
  final Auction auction;
  final List<AuctionBid> userBids;
  final String currentUserId;
  final VoidCallback? onBidAgain;

  const AuctionBidPositionIndicator({
    super.key,
    required this.auction,
    required this.userBids,
    required this.currentUserId,
    this.onBidAgain,
  });

  /// Determine user's bid position
  BidPosition get _bidPosition {
    // Auction not active
    if (!auction.isActive && auction.status != AuctionStatus.scheduled) {
      return BidPosition.notActive;
    }

    // No bids from user
    if (userBids.isEmpty) {
      return BidPosition.noBid;
    }

    // Backend tracks current winner via winnerId for active auctions
    // If winnerId matches currentUserId, user is leading
    if (auction.winnerId == currentUserId) {
      return BidPosition.leading;
    }

    // Otherwise, user has been outbid
    return BidPosition.outbid;
  }

  /// Get display data for the current position.
  ///
  /// Tones come from the scheme authority (semantic palette only where the
  /// scheme has no role: success/warning). Text on tinted fills uses the
  /// display color derived from those same tokens via alpha — no light-only
  /// hex, no local brightness branch.
  _BidPositionDisplay _display(BuildContext context, ColorScheme scheme) {
    switch (_bidPosition) {
      case BidPosition.leading:
        return _BidPositionDisplay(
          label: 'Anda Memimpin',
          icon: Icons.emoji_events,
          color: context.statusColors.success,
          backgroundColor: context.statusColors.success.withValues(alpha: 0.12),
          message: 'Bid Anda saat ini adalah yang tertinggi',
        );
      case BidPosition.outbid:
        return _BidPositionDisplay(
          label: 'Ter-Lelang',
          icon: Icons.trending_up,
          color: scheme.error,
          backgroundColor: scheme.error.withValues(alpha: 0.12),
          message: 'Kamu telah dikalahkan',
          isOutbid: true,
        );
      case BidPosition.noBid:
        return _BidPositionDisplay(
          label: 'Belum Bid',
          icon: Icons.info_outline,
          color: scheme.onSurfaceVariant,
          backgroundColor: scheme.surfaceContainerHighest,
          message: 'Anda belum memasang bid pada lelang ini',
        );
      case BidPosition.notActive:
        // Winner gets special treatment
        if (auction.isUserWinner(currentUserId)) {
          final winningBid = auction.currentBid;
          return _BidPositionDisplay(
            label: 'Anda Menang! 🎉',
            icon: Icons.emoji_events,
            color: context.statusColors.success,
            backgroundColor: context.statusColors.success.withValues(alpha: 0.12),
            message:
                'Bid Menang: Rp ${formatGroupedAmount(winningBid.round())}',
            deadline: _getClaimDeadline(),
          );
        }
        return _BidPositionDisplay(
          label: _getEndedLabel(),
          icon: Icons.info,
          color: scheme.onSurfaceVariant,
          backgroundColor: scheme.surfaceContainerHighest,
          message: _getEndedMessage(),
        );
    }
  }

  String _getEndedLabel() {
    if (auction.isUserWinner(currentUserId)) {
      return 'Anda Menang! 🎉';
    }
    if (auction.isExpired) {
      return 'Lelang Berakhir';
    }
    if (auction.isSold) {
      return 'Terjual';
    }
    return 'Lelang Berakhir';
  }

  String _getEndedMessage() {
    if (auction.isUserWinner(currentUserId)) {
      final winningBid = auction.currentBid;
      return 'Selamat! Menang di Rp ${formatGroupedAmount(winningBid.round())} - Lanjut klaim kemenangan Anda';
    }
    if (auction.isExpired) {
      return 'Lelang ini berakhir tanpa pemenang';
    }
    if (auction.isSold) {
      return 'Lelang ini telah terjual';
    }
    return 'Lelang ini telah berakhir';
  }

  /// Get the claim deadline for auction winners.
  /// Canonical derivation: end_at + 24h (backend Auction.SettlementDeadline()).
  String? _getClaimDeadline() {
    if (!auction.isUserWinner(currentUserId)) {
      return null;
    }
    final deadline = auction.settlementDeadline;
    final dateFormat = DateFormat('MMM dd, yyyy • HH:mm');
    return dateFormat.format(deadline.toLocal());
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final display = _display(context, scheme);

    // Hide indicator for scheduled auctions (not relevant yet)
    if (auction.status == AuctionStatus.scheduled) {
      return const SizedBox.shrink();
    }

    // OUTBID AWARENESS: Special layout with action button
    if (display.isOutbid) {
      return _buildOutbidIndicator(context, display);
    }

    return Container(
      margin: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: display.backgroundColor,
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(
          color: display.color.withValues(alpha: 0.3),
          width: 1,
        ),
      ),
      child: Row(
        children: [
          Icon(display.icon, color: display.color, size: 20),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  display.label,
                  style: TextStyle(
                    fontWeight: FontWeight.bold,
                    color: display.color,
                    fontSize: AppType.s14,
                  ),
                ),
                if (display.message.isNotEmpty) ...[
                  const SizedBox(height: 2),
                  Text(
                    display.message,
                    style: TextStyle(
                      color: display.color.withValues(alpha: 0.8),
                      fontSize: AppType.s12,
                    ),
                  ),
                ],
                // Show claim deadline for winners
                if (display.deadline != null) ...[
                  const SizedBox(height: 6),
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: AppMetrics.p8,
                      vertical: AppMetrics.p4,
                    ),
                    decoration: BoxDecoration(
                      color: context.statusColors.warning.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(AppShape.r4),
                      border: Border.all(
                        color: context.statusColors.warning.withValues(alpha: 0.3),
                        width: 1,
                      ),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.access_time,
                          size: 12,
                          color: context.statusColors.warning,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          'Selesaikan sebelum: ${display.deadline}',
                          style: TextStyle(
                            color: context.statusColors.warning,
                            fontSize: AppType.s11,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ],
            ),
          ),
          // Show user's highest bid if they have one
          if (userBids.isNotEmpty && _bidPosition != BidPosition.notActive) ...[
            const SizedBox(width: 12),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4),
              decoration: BoxDecoration(
                color: display.color.withValues(alpha: 0.15),
                borderRadius: BorderRadius.circular(AppShape.r4),
              ),
              child: Text(
                'Bid: Rp ${formatGroupedAmount(userBids.map((b) => b.amount).reduce((a, b) => a > b ? a : b).round())}',
                style: TextStyle(
                  color: display.color,
                  fontSize: AppType.s11,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }

  /// Outbid indicator with actionable button
  /// Shows "🔴 Kamu telah dikalahkan" with "Pasang Bid Lagi" button
  Widget _buildOutbidIndicator(
    BuildContext context,
    _BidPositionDisplay display,
  ) {
    final userHighestBid = userBids.isNotEmpty
        ? userBids.map((b) => b.amount).reduce((a, b) => a > b ? a : b)
        : 0.0;

    return Container(
      margin: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: display.backgroundColor,
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(
          color: display.color.withValues(alpha: 0.4),
          width: 1.5,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                padding: const EdgeInsets.all(AppMetrics.p4),
                decoration: BoxDecoration(
                  color: display.color.withValues(alpha: 0.15),
                  shape: BoxShape.circle,
                ),
                child: Icon(display.icon, color: display.color, size: 18),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Text(
                          '🔴 ',
                          style: TextStyle(
                            fontWeight: FontWeight.bold,
                            color: display.color,
                            fontSize: AppType.s14,
                          ),
                        ),
                        Text(
                          display.label,
                          style: TextStyle(
                            fontWeight: FontWeight.bold,
                            color: display.color,
                            fontSize: AppType.s14,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 2),
                    Text(
                      display.message,
                      style: TextStyle(
                        color: display.color.withValues(alpha: 0.9),
                        fontSize: AppType.s13,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
          // Show user's bid
          if (userBids.isNotEmpty) ...[
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p10, vertical: AppMetrics.p6),
              decoration: BoxDecoration(
                color: display.color.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(AppShape.r6),
              ),
              child: Text(
                'Bid kamu: Rp ${formatGroupedAmount(userHighestBid.round())}',
                style: TextStyle(
                  color: display.color.withValues(alpha: 0.8),
                  fontSize: AppType.s12,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ),
          ],
          // Action button
          const SizedBox(height: 10),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: onBidAgain,
              style: ElevatedButton.styleFrom(
                backgroundColor: display.color,
                foregroundColor: Theme.of(context).colorScheme.onPrimary,
                padding: const EdgeInsets.symmetric(vertical: AppMetrics.p10),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(AppShape.r6),
                ),
                elevation: AppElevation.none,
              ),
              child: const Text(
                'Pasang Bid Lagi',
                style: TextStyle(fontWeight: FontWeight.bold, fontSize: AppType.s13),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _BidPositionDisplay {
  final String label;
  final IconData icon;
  final Color color;
  final Color backgroundColor;
  final String message;
  final String? deadline;
  final bool isOutbid;

  const _BidPositionDisplay({
    required this.label,
    required this.icon,
    required this.color,
    required this.backgroundColor,
    required this.message,
    this.deadline,
    this.isOutbid = false,
  });
}
