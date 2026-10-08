/// CANONICAL SELLER SHIPPING QUOTE INTENT — the ONLY entry any host uses to
/// offer the seller's manual shipping quote.
///
/// OWNER TRUTH: shipping quotes are owned end-to-end by the Shipping (Commerce)
/// domain:
///   - the form authority is [ShippingQuoteFormSheet];
///   - the wire is `POST /api/v1/chat/:chat_id/shipping-quote`
///     (`CreateShippingQuoteRequestDto` → `ShippingQuoteRepository`);
///   - the backend is the single business authority: seller/buyer eligibility,
///     quote lifecycle, expiry, supersession, destination lock and checkout
///     validation all live server-side.
///
/// A host (chat today) only forwards an EXPLICIT seller intent. It resolves the
/// selected listing's physical product id through the canonical Commerce
/// detail and nothing else — it never computes eligibility, never decides the
/// buyer, and never carries Commerce business rules.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/shipping_quote_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/widgets/shipping_quote_form_sheet.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

/// The seller-selected listing the manual shipping quote applies to.
///
/// Identity + display title only: the physical product id is resolved fresh
/// from Commerce, never trusted from the host.
class SellerShippingQuoteTarget {
  final String? forSaleId;
  final String? auctionId;
  final String title;

  const SellerShippingQuoteTarget.forSale({
    required String this.forSaleId,
    required this.title,
  }) : auctionId = null;

  const SellerShippingQuoteTarget.auction({
    required String this.auctionId,
    required this.title,
  }) : forSaleId = null;
}

/// Canonical create-quote wire assembly — the single request authority. The
/// backend rejects a for_sale quote without a locked kota/kabupaten
/// destination, so the destination is required.
CreateShippingQuoteRequestDto buildShippingQuoteRequest({
  required String productId,
  required String sourceType,
  required String sourceId,
  required int cost,
  required String destinationCityId,
  required String destinationProvinceId,
  String? note,
}) {
  return CreateShippingQuoteRequestDto(
    productId: productId,
    sourceType: sourceType,
    sourceId: sourceId,
    cost: cost,
    destinationCityId: destinationCityId,
    destinationProvinceId: destinationProvinceId,
    note: note,
  );
}

/// Resolves the physical product id for the seller-selected listing through the
/// canonical Commerce detail. Null when the listing has no product (a quote
/// cannot be opened against it).
Future<String?> resolveSellerShippingQuoteProductId(
  WidgetRef ref,
  SellerShippingQuoteTarget target,
) async {
  final forSaleId = target.forSaleId;
  if (forSaleId != null && forSaleId.isNotEmpty) {
    try {
      final forSale = await ref.read(forSaleDetailProvider(forSaleId).future);
      return forSale?.productId;
    } catch (_) {
      return null;
    }
  }
  final auctionId = target.auctionId;
  if (auctionId != null && auctionId.isNotEmpty) {
    try {
      final auction = await ref.read(auctionDetailProvider(auctionId).future);
      return auction?.productId;
    } catch (_) {
      return null;
    }
  }
  return null;
}

/// Opens the seller shipping-quote flow and posts it through the Shipping
/// repository. Returns true when the quote was created (and a conversation
/// message was produced by the backend).
Future<bool> openSellerShippingQuoteSheet({
  required BuildContext context,
  required WidgetRef ref,
  required String chatRoomId,
  required SellerShippingQuoteTarget target,
  Future<void> Function()? onSuccess,
}) async {
  if (chatRoomId.trim().isEmpty) return false;

  final productId = await resolveSellerShippingQuoteProductId(ref, target);
  if (!context.mounted) return false;
  if (productId == null || productId.isEmpty) {
    AppSnackBar.showError(
      context,
      'Produk belum siap untuk penawaran ongkir ini',
    );
    return false;
  }

  final submitted = await ShippingQuoteFormSheet.show(
    context: context,
    productTitle: target.title,
  );
  if (submitted == null || !context.mounted) return false;

  final sourceType = target.auctionId != null ? 'auction' : 'for_sale';
  final sourceId = target.auctionId ?? target.forSaleId ?? '';
  if (sourceId.isEmpty) return false;

  final request = buildShippingQuoteRequest(
    productId: productId,
    sourceType: sourceType,
    sourceId: sourceId,
    cost: submitted.cost,
    destinationCityId: submitted.destinationCityId,
    destinationProvinceId: submitted.destinationProvinceId,
    note: submitted.note,
  );

  try {
    await ref
        .read(shippingQuoteRepositoryProvider)
        .createShippingQuote(chatId: chatRoomId, request: request);
    if (!context.mounted) return false;
    AppSnackBar.showSuccess(context, 'Penawaran ongkir terkirim');
    await onSuccess?.call();
    return true;
  } catch (_) {
    if (!context.mounted) return false;
    AppSnackBar.showError(
      context,
      'Gagal mengirim penawaran ongkir. Coba lagi.',
    );
    return false;
  }
}
