/// Auction Detail Handlers
///
/// Utility class for handling auction detail actions
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';

/// Handlers for auction detail actions
class AuctionDetailHandlers {
  final WidgetRef ref;
  final BuildContext context;
  final Auction auction;
  final String auctionId;
  final VoidCallback onEditSuccess;
  final VoidCallback onDeleteSuccess;

  AuctionDetailHandlers({
    required this.ref,
    required this.context,
    required this.auction,
    required this.auctionId,
    required this.onEditSuccess,
    required this.onDeleteSuccess,
  });

  /// Handle delete auction
  Future<void> handleDelete() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Hapus Lelang'),
        content: const Text('Apakah Anda yakin ingin menghapus lelang ini?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Batal'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Hapus'),
          ),
        ],
      ),
    );

    if (confirmed == true) {
      onDeleteSuccess();
    }
  }

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
        onDeleteSuccess();
      }
    }
  }
}
