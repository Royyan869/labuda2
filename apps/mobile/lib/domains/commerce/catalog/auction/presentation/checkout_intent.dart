/// Commerce-owned checkout intent — the ONLY canonical way another domain
/// opens checkout for an auction.
///
/// OWNER RULE: chat (and any other host) is a display layer that forwards an
/// intent. The commerce authority decides what checkout requires:
/// - resolves the LIVE auction (a transient cache miss must not dead-end the
///   CTA into "ID produk belum tersedia");
/// - enforces the seller trust gate;
/// - resolves the physical product id checkout prices against (distinct from
///   the auction id — never conflated);
/// - constructs the canonical checkout route shape.
/// No caller may re-implement any of these rules.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_providers.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

/// What a host carries per CTA: the auction id plus the shipping quote that
/// triggered the purchase (null → no quote attached).
class AuctionCheckoutIntent {
  final String auctionId;
  final String? shippingQuoteId;

  const AuctionCheckoutIntent({
    required this.auctionId,
    this.shippingQuoteId,
  });
}

/// Opens checkout for [intent] from any host screen and returns when done.
Future<void> openAuctionCheckout(
  BuildContext context,
  WidgetRef ref,
  AuctionCheckoutIntent intent, {
  String? returnToChatRoomId,
}) async {
  Auction? auction;
  try {
    auction = await ref.read(auctionDetailProvider(intent.auctionId).future);
  } catch (_) {
    auction = null;
  }
  if (!context.mounted) return;
  if (auction == null) {
    AppSnackBar.showError(context, 'Produk tidak ditemukan');
    return;
  }

  // SELLER TRUST GATE: fast, cached-data check only. Checkout screen and
  // backend guards remain the authoritative checks — that is exactly why
  // this gate lives here and not in the host.
  if (auction.sellerTrustLifecycle != ContentLifecycle.active) {
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text(
          'Penjual tidak aktif — transaksi tidak dapat dilanjutkan',
        ),
      ),
    );
    return;
  }

  // IDENTITY DISCIPLINE: product_id is the physical product; the :id path slot
  // and auction_id query carry the AUCTION id. They are never interchangeable.
  final productId = auction.productId;
  if (productId == null || productId.isEmpty) {
    AppSnackBar.showError(
      context,
      'ID produk belum tersedia untuk checkout ini',
    );
    return;
  }

  final queryParams = <String, String>{
    'product_id': productId,
    'auction_id': intent.auctionId,
    'shipping_quote_id': ?intent.shippingQuoteId,
    'return_to_chat': ?returnToChatRoomId,
  };

  if (!context.mounted) return;
  context.push(
    Uri(
      path: '/checkout/${intent.auctionId}',
      queryParameters: queryParams,
    ).toString(),
  );
}
