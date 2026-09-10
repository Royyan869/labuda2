/// Auction Detail Handlers
///
/// Utility class for handling auction detail actions
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';

/// Handlers for auction detail actions
///
/// NOTE: There is intentionally NO delete affordance on this surface. The
/// backend exposes no DELETE endpoint for auctions (lifecycle is
/// draft/scheduled → active → ended/cancelled via dedicated transitions), so
/// a dialog-only "delete" that pops the screen without a backend call would be
/// phantom UI. Only the legitimate Cancel lifecycle path is wired.
class AuctionDetailHandlers {
  final WidgetRef ref;
  final BuildContext context;
  final Auction auction;
  final String auctionId;
  final VoidCallback onEditSuccess;
  final VoidCallback onCancelSuccess;

  AuctionDetailHandlers({
    required this.ref,
    required this.context,
    required this.auction,
    required this.auctionId,
    required this.onEditSuccess,
    required this.onCancelSuccess,
  });

  /// Handle cancel auction
  ///
  /// WIRED to backend via auctionNotifier.cancelAuction()
  /// Business rules enforced by backend:
  /// - Draft/Scheduled: Always cancellable
  /// - Active: Only if no bids (backend enforces this)
  /// - Ended/Cancelled: Never cancellable
  Future<void> handleCancel() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Batalkan Lelang'),
        content: const Text('Apakah Anda yakin ingin membatalkan lelang ini?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Batal'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Batalkan'),
          ),
        ],
      ),
    );

    if (confirmed == true) {
      final notifier = ref.read(auctionNotifierProvider.notifier);
      final success = await notifier.cancelAuction(
        auctionId: auctionId,
        sellerId: auction.sellerId,
        reason: 'Seller cancelled',
      );

      if (success && context.mounted) {
        onCancelSuccess();
      }
    }
  }
}
