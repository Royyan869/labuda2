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
///
/// ONE PURCHASE FUNNEL: auction buy-now AND auction bid-win both open the
/// SAME shared CheckoutScreen. [AuctionCheckoutIntent.bidWin] is the only
/// discriminator: bid-win orders are created WITHOUT a payment method (the
/// winner picks one at Order Detail; the first payment binds it).
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/providers/auction_providers.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/widgets/app_snackbar.dart';

/// What a host carries per CTA. Seller manual shipping quotes (auction
/// settlement context) ride the SAME bid-win intent via [shippingQuoteId] +
/// [chatId] — checkout forwards them to the canonical preview/order contract.
class AuctionCheckoutIntent {
  final String auctionId;

  /// True when the caller is the auction WINNER completing the settlement
  /// window (bid-win). False for buy-now (an active-auction purchase).
  final bool bidWin;

  /// Seller's manual shipping quote (auction settlement context). Required
  /// together with [chatId]; mutually exclusive with the buyer-selected
  /// shipping option inside checkout.
  final String? shippingQuoteId;

  /// Conversation that produced [shippingQuoteId] (quote provenance).
  final String? chatId;

  const AuctionCheckoutIntent({
    required this.auctionId,
    this.bidWin = false,
    this.shippingQuoteId,
    this.chatId,
  });
}

/// Opens checkout for [intent] from any host screen and returns when done.
Future<void> openAuctionCheckout(
  BuildContext context,
  WidgetRef ref,
  AuctionCheckoutIntent intent,
) async {
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
    AppSnackBar.showError(
      context,
      'Penjual tidak aktif — transaksi tidak dapat dilanjutkan',
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
    if (intent.bidWin) 'bid_win': '1',
    if (intent.shippingQuoteId != null &&
        intent.shippingQuoteId!.isNotEmpty) ...{
      'shipping_quote_id': intent.shippingQuoteId!,
      if (intent.chatId != null && intent.chatId!.isNotEmpty)
        'chat_id': intent.chatId!,
    },
  };

  if (!context.mounted) return;
  context.push(
    Uri(
      path: '/checkout/${intent.auctionId}',
      queryParameters: queryParams,
    ).toString(),
  );
}
