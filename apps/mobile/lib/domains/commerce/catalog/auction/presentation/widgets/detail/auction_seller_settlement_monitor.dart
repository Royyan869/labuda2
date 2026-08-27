/// Seller Settlement Monitor
///
/// Shows seller the auction winner info and settlement status after auction ends
library;

import 'package:flutter/material.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';

/// Settlement status for seller display
enum SellerSettlementStatus {
  /// Waiting for winner to complete payment
  waitingSettlement,

  /// Winner has claimed/paid
  claimed,

  /// Winner failed to complete payment
  expiredBnr,
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
    } else if (widget.auction.status == AuctionStatus.expiredBNR) {
      return SellerSettlementStatus.expiredBnr;
    }
    // Default to claimed if auction ended with winner and not in waiting/expired state
    // This assumes backend transitions to a "claimed" state when winner completes payment
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
      margin: const EdgeInsets.fromLTRB(16, 12, 16, 12),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: _getStatusBackgroundColor(),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: _getStatusBorderColor(), width: 1),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Status header
          _buildStatusHeader(),
          const SizedBox(height: 12),

          // Winner info
          _buildWinnerInfo(),
          const SizedBox(height: 12),

          // Status-specific content
          _buildStatusContent(),
        ],
      ),
    );
  }

  Widget _buildStatusHeader() {
    return Row(
      children: [
        Container(
          padding: const EdgeInsets.all(8),
          decoration: BoxDecoration(
            color: _getStatusIconColor().withValues(alpha: 0.15),
            shape: BoxShape.circle,
          ),
          child: Icon(_getStatusIcon(), color: _getStatusIconColor(), size: 20),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Text(
            _getStatusTitle(),
            style: TextStyle(
              fontSize: 15,
              fontWeight: FontWeight.w600,
              color: _getStatusTextColor(),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildWinnerInfo() {
    final winnerUsername = widget.auction.winnerUsername ?? 'Pemenang';
    final winningBid = widget.auction.winningBid ?? widget.auction.currentBid;

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Colors.white.withValues(alpha: 0.6),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Pemenang:',
            style: TextStyle(fontSize: 12, color: Colors.grey.shade600),
          ),
          const SizedBox(height: 4),
          Text(
            winnerUsername,
            style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 8),
          Text(
            'Bid: Rp ${winningBid.toStringAsFixed(0)}',
            style: TextStyle(
              fontSize: 14,
              color: Colors.grey.shade700,
              fontWeight: FontWeight.w500,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildStatusContent() {
    switch (_settlementStatus) {
      case SellerSettlementStatus.waitingSettlement:
        return _buildWaitingSettlementContent();
      case SellerSettlementStatus.claimed:
        return _buildClaimedContent();
      case SellerSettlementStatus.expiredBnr:
        return _buildExpiredBnrContent();
    }
  }

  Widget _buildWaitingSettlementContent() {
    final deadline = widget.auction.settlementDeadline;
    if (deadline == null) {
      return const SizedBox.shrink();
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Menunggu pembayaran dari pemenang',
          style: TextStyle(fontSize: 13, color: _getStatusTextColor()),
        ),
        const SizedBox(height: 8),
        Row(
          children: [
            Icon(Icons.schedule, size: 14, color: _getStatusTextColor()),
            const SizedBox(width: 4),
            Text(
              'Selesaikan sebelum:',
              style: TextStyle(fontSize: 12, color: _getStatusTextColor()),
            ),
          ],
        ),
        const SizedBox(height: 4),
        _buildCountdown(deadline),
      ],
    );
  }

  Widget _buildClaimedContent() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Pembayaran sedang diproses',
          style: TextStyle(fontSize: 13, color: _getStatusTextColor()),
        ),
        const SizedBox(height: 12),
        // TODO: Add link to order when order_id is available
        // This requires backend to return order_id in auction response
        // or a separate API to get order info for auction
        Container(
          width: double.infinity,
          padding: const EdgeInsets.symmetric(vertical: 10),
          decoration: BoxDecoration(
            color: _getStatusTextColor().withValues(alpha: 0.1),
            borderRadius: BorderRadius.circular(6),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(Icons.receipt_long, size: 16, color: _getStatusTextColor()),
              const SizedBox(width: 8),
              Text(
                'Lihat Pesanan',
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w500,
                  color: _getStatusTextColor(),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildExpiredBnrContent() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Pemenang tidak menyelesaikan pembayaran',
          style: TextStyle(fontSize: 13, color: _getStatusTextColor()),
        ),
        const SizedBox(height: 4),
        Text(
          'Auction berakhir tanpa transaksi',
          style: TextStyle(
            fontSize: 12,
            color: _getStatusTextColor().withValues(alpha: 0.8),
          ),
        ),
      ],
    );
  }

  Widget _buildCountdown(DateTime deadline) {
    return StreamBuilder(
      stream: Stream.periodic(const Duration(seconds: 1), (count) => count),
      builder: (context, snapshot) {
        final now = DateTime.now();
        final remaining = deadline.difference(now);

        if (remaining.isNegative) {
          return Text(
            'Waktu habis',
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: Colors.red.shade700,
            ),
          );
        }

        final hours = remaining.inHours;
        final minutes = remaining.inMinutes % 60;

        String timeText;
        Color timeColor;

        if (hours > 0) {
          timeText = '$hours jam $minutes menit tersisa';
          timeColor = _getStatusTextColor();
        } else {
          timeText = '$minutes menit tersisa';
          timeColor = Colors.red.shade700;
        }

        return Row(
          children: [
            Icon(Icons.access_time, size: 14, color: timeColor),
            const SizedBox(width: 4),
            Text(
              timeText,
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: timeColor,
              ),
            ),
          ],
        );
      },
    );
  }

  // Status-based styling helpers

  Color _getStatusBackgroundColor() {
    switch (_settlementStatus) {
      case SellerSettlementStatus.waitingSettlement:
        return const Color(0xFFFFF7ED); // Light orange
      case SellerSettlementStatus.claimed:
        return const Color(0xFFECFDF5); // Light green
      case SellerSettlementStatus.expiredBnr:
        return const Color(0xFFFEF2F2); // Light red
    }
  }

  Color _getStatusBorderColor() {
    switch (_settlementStatus) {
      case SellerSettlementStatus.waitingSettlement:
        return const Color(0xFFF97316).withValues(alpha: 0.3);
      case SellerSettlementStatus.claimed:
        return const Color(0xFF10B981).withValues(alpha: 0.3);
      case SellerSettlementStatus.expiredBnr:
        return const Color(0xFFEF4444).withValues(alpha: 0.3);
    }
  }

  Color _getStatusTextColor() {
    switch (_settlementStatus) {
      case SellerSettlementStatus.waitingSettlement:
        return const Color(0xFF9A3412);
      case SellerSettlementStatus.claimed:
        return const Color(0xFF065F46);
      case SellerSettlementStatus.expiredBnr:
        return const Color(0xFF991B1B);
    }
  }

  Color _getStatusIconColor() {
    switch (_settlementStatus) {
      case SellerSettlementStatus.waitingSettlement:
        return const Color(0xFFF97316);
      case SellerSettlementStatus.claimed:
        return const Color(0xFF10B981);
      case SellerSettlementStatus.expiredBnr:
        return const Color(0xFFEF4444);
    }
  }

  IconData _getStatusIcon() {
    switch (_settlementStatus) {
      case SellerSettlementStatus.waitingSettlement:
        return Icons.schedule;
      case SellerSettlementStatus.claimed:
        return Icons.check_circle;
      case SellerSettlementStatus.expiredBnr:
        return Icons.cancel;
    }
  }

  String _getStatusTitle() {
    switch (_settlementStatus) {
      case SellerSettlementStatus.waitingSettlement:
        return 'Menunggu Pembayaran';
      case SellerSettlementStatus.claimed:
        return 'Pembayaran Diproses';
      case SellerSettlementStatus.expiredBnr:
        return 'Pembayaran Kadaluarsa';
    }
  }
}
