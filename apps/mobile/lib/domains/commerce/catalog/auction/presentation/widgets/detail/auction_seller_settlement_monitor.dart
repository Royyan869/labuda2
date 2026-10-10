/// Seller Settlement Monitor
///
/// Shows seller the auction winner info and settlement status after auction ends
library;

import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/entities/auction_status.dart';

/// Settlement status for seller display
///
/// PURGED: expiredBnr — never a backend auction status. Settlement failure
/// returns the auction to DRAFT (TransitionToDraftOnSettlementFailure), where
/// the monitor hides itself (only waiting_settlement/ended-with-winner render).
enum SellerSettlementStatus {
  /// Waiting for winner to complete payment
  waitingSettlement,

  /// Winner has claimed/paid
  claimed,
}

/// Widget that displays auction winner information and settlement status for sellers
class AuctionSellerSettlementMonitor extends StatefulWidget {
  final Auction auction;
  final String currentUserId;

  const AuctionSellerSettlementMonitor({
    super.key,
    required this.auction,
    required this.currentUserId,
  });

  @override
  State<AuctionSellerSettlementMonitor> createState() =>
      _AuctionSellerSettlementMonitorState();
}

class _AuctionSellerSettlementMonitorState
    extends State<AuctionSellerSettlementMonitor> {
  late SellerSettlementStatus _settlementStatus;

  @override
  void initState() {
    super.initState();
    _settlementStatus = _determineSettlementStatus();
  }

  @override
  void didUpdateWidget(AuctionSellerSettlementMonitor oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.auction.status != widget.auction.status) {
      setState(() {
        _settlementStatus = _determineSettlementStatus();
      });
    }
  }

  SellerSettlementStatus _determineSettlementStatus() {
    if (widget.auction.status == AuctionStatus.waitingSettlement) {
      return SellerSettlementStatus.waitingSettlement;
    } else if (widget.auction.status == AuctionStatus.ended &&
        widget.auction.winnerId != null) {
      // Ended with winner - waiting for claim
      return SellerSettlementStatus.waitingSettlement;
    }
    // Default to claimed if auction ended with winner and payment succeeded
    // (payment success settles the auction to ENDED — canonical backend path).
    return SellerSettlementStatus.claimed;
  }

  bool get _isSeller =>
      widget.currentUserId.isNotEmpty &&
      widget.currentUserId == widget.auction.sellerId;

  @override
  Widget build(BuildContext context) {
    // Only show to seller when auction has ended with a winner
    if (!_isSeller) return const SizedBox.shrink();
    if (widget.auction.status == AuctionStatus.active ||
        widget.auction.status == AuctionStatus.scheduled) {
      return const SizedBox.shrink();
    }
    if (widget.auction.winnerId == null) return const SizedBox.shrink();

    return Container(
      margin: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p12,
        AppMetrics.p16,
        AppMetrics.p12,
      ),
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: _getStatusBackgroundColor(),
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: _getStatusBorderColor(), width: 1),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Status header
          _buildStatusHeader(context),
          const SizedBox(height: 12),

          // Winner info
          _buildWinnerInfo(context),
          const SizedBox(height: 12),

          // Status-specific content
          _buildStatusContent(context),
        ],
      ),
    );
  }

  Widget _buildStatusHeader(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Row(
      children: [
        Container(
          padding: const EdgeInsets.all(AppMetrics.p8),
          decoration: BoxDecoration(
            color: _getStatusIconColor().withValues(alpha: 0.15),
            shape: BoxShape.circle,
          ),
          child: Icon(
            _getStatusIcon(),
            color: _getStatusIconColor(),
            size: AppIconSize.action,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Text(
            _getStatusTitle(),
            style: context.typeRoles.titleCompact.copyWith(
              fontWeight: FontWeight.w600,
              color: colorScheme.onSurface,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildWinnerInfo(BuildContext context) {
    final winnerUsername = 'Pemenang';
    final winningBid = widget.auction.currentBid;
    final colorScheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Pemenang:',
            style: context.typeRoles.labelMicro.copyWith(
              color: colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            winnerUsername,
            style: context.typeRoles.titleCompact.copyWith(
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Bid: Rp ${formatGroupedAmount(winningBid.round())}',
            style: context.typeRoles.bodyDense.copyWith(
              color: colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w500,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildStatusContent(BuildContext context) {
    switch (_settlementStatus) {
      case SellerSettlementStatus.waitingSettlement:
        return _buildWaitingSettlementContent(context);
      case SellerSettlementStatus.claimed:
        return _buildClaimedContent(context);
    }
  }

  Widget _buildWaitingSettlementContent(BuildContext context) {
    // Canonical deadline derivation: end_at + 24h (backend
    // Auction.SettlementDeadline()). Non-null by definition.
    final deadline = widget.auction.settlementDeadline;
    final colorScheme = Theme.of(context).colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Menunggu pembayaran dari pemenang',
          style: context.typeRoles.bodyDense.copyWith(
            color: colorScheme.onSurface,
          ),
        ),
        const SizedBox(height: 8),
        Row(
          children: [
            Icon(
              Icons.schedule,
              size: AppIconSize.inlineGlyph,
              color: colorScheme.onSurfaceVariant,
            ),
            const SizedBox(width: 4),
            Text(
              'Selesaikan sebelum:',
              style: context.typeRoles.labelMicro.copyWith(
                color: colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
        const SizedBox(height: 4),
        _buildCountdown(deadline),
      ],
    );
  }

  Widget _buildClaimedContent(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Pembayaran sedang diproses',
          style: context.typeRoles.bodyDense.copyWith(
            color: colorScheme.onSurface,
          ),
        ),
        const SizedBox(height: 12),
        // TODO: Add link to order when order_id is available
        // This requires backend to return order_id in auction response
        // or a separate API to get order info for auction
        Container(
          width: double.infinity,
          padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12),
          decoration: BoxDecoration(
            color: _getStatusIconColor().withValues(alpha: 0.12),
            borderRadius: BorderRadius.circular(AppShape.r6),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(
                Icons.receipt_long,
                size: AppIconSize.inlineGlyph,
                color: colorScheme.onSurface,
              ),
              const SizedBox(width: 8),
              Text(
                'Lihat Pesanan',
                style: context.typeRoles.bodyDense.copyWith(
                  fontWeight: FontWeight.w500,
                  color: colorScheme.onSurface,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildCountdown(DateTime deadline) {
    return StreamBuilder(
      stream: Stream.periodic(const Duration(seconds: 1), (count) => count),
      builder: (context, snapshot) {
        final colorScheme = Theme.of(context).colorScheme;
        final now = DateTime.now();
        final remaining = deadline.difference(now);

        if (remaining.isNegative) {
          return Text(
            'Waktu habis',
            style: context.typeRoles.bodyDense.copyWith(
              fontWeight: FontWeight.w600,
              color: colorScheme.error,
            ),
          );
        }

        final hours = remaining.inHours;
        final minutes = remaining.inMinutes % 60;

        String timeText;
        Color timeColor;

        if (hours > 0) {
          timeText = '$hours jam $minutes menit tersisa';
          timeColor = colorScheme.onSurface;
        } else {
          timeText = '$minutes menit tersisa';
          timeColor = colorScheme.error;
        }

        return Row(
          children: [
            Icon(
              Icons.access_time,
              size: AppIconSize.inlineGlyph,
              color: timeColor,
            ),
            const SizedBox(width: 4),
            Text(
              timeText,
              style: context.typeRoles.bodyDense.copyWith(
                fontWeight: FontWeight.w600,
                color: timeColor,
              ),
            ),
          ],
        );
      },
    );
  }

  // Status-based styling helpers.
  //
  // Semantic tones (warning/success) have no scheme role, so they stay
  // palette authority; all body text uses scheme ink so it stays readable
  // in both modes. No light-only hex, no local brightness branch.
  Color _getStatusBackgroundColor() {
    switch (_settlementStatus) {
      case SellerSettlementStatus.waitingSettlement:
        return context.statusColors.warning.withValues(alpha: 0.12);
      case SellerSettlementStatus.claimed:
        return context.statusColors.success.withValues(alpha: 0.12);
    }
  }

  Color _getStatusBorderColor() {
    switch (_settlementStatus) {
      case SellerSettlementStatus.waitingSettlement:
        return context.statusColors.warning.withValues(alpha: 0.3);
      case SellerSettlementStatus.claimed:
        return context.statusColors.success.withValues(alpha: 0.3);
    }
  }

  Color _getStatusIconColor() {
    switch (_settlementStatus) {
      case SellerSettlementStatus.waitingSettlement:
        return context.statusColors.warning;
      case SellerSettlementStatus.claimed:
        return context.statusColors.success;
    }
  }

  IconData _getStatusIcon() {
    switch (_settlementStatus) {
      case SellerSettlementStatus.waitingSettlement:
        return Icons.schedule;
      case SellerSettlementStatus.claimed:
        return Icons.check_circle;
    }
  }

  String _getStatusTitle() {
    switch (_settlementStatus) {
      case SellerSettlementStatus.waitingSettlement:
        return 'Menunggu Pembayaran';
      case SellerSettlementStatus.claimed:
        return 'Pembayaran Diproses';
    }
  }
}
