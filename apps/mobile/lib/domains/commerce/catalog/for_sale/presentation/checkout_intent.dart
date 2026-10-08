/// Commerce-owned checkout intent — the ONLY canonical way another domain
/// opens checkout for a fixed-price sale.
///
/// OWNER RULE: chat (and any other host) is a display layer that forwards an
/// intent. The commerce authority decides what checkout requires:
/// - resolves the LIVE listing (a transient cache miss must not dead-end the
///   CTA into "ID produk belum tersedia");
/// - enforces the seller trust gate;
/// - resolves the physical product id checkout prices against;
/// - carries the negotiation binding the caller forwards (deal price).
/// No caller may re-implement any of these rules.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

/// What a host carries per CTA: the listing id plus the deal binding it got
/// from the negotiation authority (null → checkout prices at list price).
class CheckoutIntent {
  final String forSaleId;
  final String? negotiationId;

  const CheckoutIntent({required this.forSaleId, this.negotiationId});
}

/// Opens checkout for [intent] from any host screen and returns when done.
Future<void> openForSaleCheckout(
  BuildContext context,
  WidgetRef ref,
  CheckoutIntent intent, {
  String? shippingQuoteId,
  String? chatId,
}) async {
  ForSale? forSale;
  try {
    forSale = await ref.read(forSaleDetailProvider(intent.forSaleId).future);
  } catch (_) {
    forSale = null;
  }
  if (!context.mounted) return;
  if (forSale == null) {
    AppSnackBar.showError(context, 'Produk tidak ditemukan');
    return;
  }

  // SELLER TRUST GATE: fast, cached-data check only. Checkout screen (A3)
  // and backend Guard 6 remain the authoritative checks — that is exactly
  // why this gate lives here and not in the host.
  if (forSale.sellerTrustLifecycle != ContentLifecycle.active) {
    AppSnackBar.showError(
      context,
      'Penjual tidak aktif — transaksi tidak dapat dilanjutkan',
    );
    return;
  }

  final productId = forSale.productId;
  if (productId == null || productId.isEmpty) {
    AppSnackBar.showError(
      context,
      'ID produk belum tersedia untuk checkout ini',
    );
    return;
  }

  final queryParams = <String, String>{
    'product_id': productId,
    'negotiation_id': ?intent.negotiationId,
    'shipping_quote_id': ?shippingQuoteId,
    'chat_id': ?chatId,
  };

  if (!context.mounted) return;
  context.push(
    Uri(
      path: '/checkout/${intent.forSaleId}',
      queryParameters: queryParams,
    ).toString(),
  );
}
