/// Auction Action Modal
///
/// Modal for placing bid or using Buy Now feature
/// Buy Now is an auction feature that ends the auction immediately
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_inactive_badge.dart';

typedef BidCallback = void Function(double amount);
typedef BuyNowCallback = void Function();

/// Action modal for auction detail
class AuctionActionModal extends ConsumerStatefulWidget {
  final Auction auction;
  final BidCallback onPlaceBid;
  final BuyNowCallback onBuyNow;

  const AuctionActionModal({
    super.key,
    required this.auction,
    required this.onPlaceBid,
    required this.onBuyNow,
  });

  /// Show the action modal
  static Future<void> show(
    BuildContext context, {
    required Auction auction,
    required BidCallback onPlaceBid,
    required BuyNowCallback onBuyNow,
  }) {
    return showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (context) => AuctionActionModal(
        auction: auction,
        onPlaceBid: onPlaceBid,
        onBuyNow: onBuyNow,
      ),
    );
  }

  @override
  ConsumerState<AuctionActionModal> createState() => _AuctionActionModalState();
}

class _AuctionActionModalState extends ConsumerState<AuctionActionModal> {
  late TextEditingController _bidController;
  late double _minimumBid;

  @override
  void initState() {
    super.initState();
    _minimumBid = widget.auction.currentBid + widget.auction.bidIncrement;
    _bidController = TextEditingController(
      text: _minimumBid.toStringAsFixed(0),
    );
  }

  @override
  void dispose() {
    _bidController.dispose();
    super.dispose();
  }

  void _handlePlaceBid() {
    final input = _bidController.text.replaceAll(',', '');
    final amount = double.tryParse(input);
    if (amount == null || amount < _minimumBid) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Bid minimum: Rp ${_minimumBid.toStringAsFixed(0)}'),
          backgroundColor: Colors.red,
        ),
      );
      return;
    }

    // Show confirmation dialog before placing bid
    showDialog(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Konfirmasi Penawaran'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('Kamu akan menawar sebesar'),
            const SizedBox(height: 12),
            Text(
              'Rp ${amount.toStringAsFixed(0)}',
              style: const TextStyle(
                fontSize: 24,
                fontWeight: FontWeight.bold,
                color: Colors.green,
              ),
            ),
            const SizedBox(height: 16),
            // TRANSACTION CLARITY: Consequence warning for auction inaction
            Container(
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: AppColors.statusWarning.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color: AppColors.statusWarning.withValues(alpha: 0.3),
                ),
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(
                    Icons.info_outline,
                    size: 16,
                    color: AppColors.statusWarning,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Jika Anda menang dan tidak membayar, akun Anda dapat dibatasi',
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray700,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.of(dialogContext).pop();
              Navigator.of(context).pop();
              widget.onPlaceBid(amount);
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: Colors.green,
              foregroundColor: Colors.white,
            ),
            child: const Text('Konfirmasi'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final currentBid = widget.auction.currentBid;
    // Expired-seller visibility — disable bid & buy-now when the seller's
    // subscription has lapsed. Backend rejects these calls anyway (Guard 6 +
    // PlaceBid/BuyNow gates); the UI signals it up-front so the user is not
    // surprised by a deep error after composing a bid.
    final sellerInactive =
        widget.auction.sellerTrustLifecycle != ContentLifecycle.active;

    return Container(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 16,
        bottom: MediaQuery.of(context).viewInsets.bottom + 16,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Handle
          Center(
            child: Container(
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: Colors.grey[300],
                borderRadius: BorderRadius.circular(2),
              ),
            ),
          ),
          const SizedBox(height: 16),
          // Title
          const Text(
            'Tawar Lelang',
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 16),
          // Current bid info
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: Colors.grey[100],
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                const Text('Bid Saat Ini'),
                Text(
                  'Rp $currentBid',
                  style: const TextStyle(
                    fontWeight: FontWeight.bold,
                    color: Colors.green,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 8),
          // Next bid info
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: Colors.blue[50],
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                const Text('Bid Minimum'),
                Text(
                  'Rp ${_minimumBid.toStringAsFixed(0)}',
                  style: const TextStyle(
                    fontWeight: FontWeight.bold,
                    color: Colors.blue,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),

          // Bid amount input
          const Text('Masukkan Jumlah Bid'),
          const SizedBox(height: 8),
          TextField(
            controller: _bidController,
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            decoration: InputDecoration(
              hintText: 'Rp ${_minimumBid.toStringAsFixed(0)}',
              prefixText: 'Rp ',
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            ),
          ),
          const SizedBox(height: 16),
          // Expired-seller badge — render above CTAs when seller-trust is
          // degraded so the user understands why the buttons are disabled.
          if (sellerInactive) ...[
            const SellerInactiveBadge(
              label: 'Penjual tidak aktif — penawaran tidak tersedia',
            ),
            const SizedBox(height: 12),
          ],
          // Place bid button
          ElevatedButton(
            onPressed: sellerInactive ? null : _handlePlaceBid,
            style: ElevatedButton.styleFrom(
              backgroundColor: Colors.green,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 14),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            child: const Text(
              'Pasang Bid',
              style: TextStyle(fontWeight: FontWeight.bold),
            ),
          ),
          // Buy now button
          if (widget.auction.buyNowPrice != null) ...[
            const SizedBox(height: 12),
            OutlinedButton(
              onPressed: sellerInactive
                  ? null
                  : () {
                      Navigator.of(context).pop();
                      widget.onBuyNow();
                    },
              style: OutlinedButton.styleFrom(
                foregroundColor: Colors.blue,
                padding: const EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
                side: const BorderSide(color: Colors.blue),
              ),
              child: Text(
                'Buy Now - Rp ${widget.auction.buyNowPrice!.toStringAsFixed(0)}',
                style: const TextStyle(fontWeight: FontWeight.bold),
              ),
            ),
          ],
        ],
      ),
    );
  }
}
