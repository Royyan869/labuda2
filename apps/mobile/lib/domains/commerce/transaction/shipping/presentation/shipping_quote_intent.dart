/// CANONICAL SELLER SHIPPING QUOTE INTENT — the ONLY way any host (chat
/// today) offers the seller's manual shipping quote.
///
/// OWNER RULE (2026-10-01): shipping quotes are owned by the Shipping domain.
/// Chat is a display layer that forwards an intent — it renders the CTA when
/// the server projection says the viewer is the seller (commerce_actions
/// can_manage on a LIVE for_sale projection) and calls this entry. It never
/// builds the request, calls the API, or learns the DTO.
///
/// This entry:
/// - re-resolves the LIVE listing (projection cache is display data);
/// - resolves the physical product id checkout/quote prices against;
/// - opens the single form authority (ShippingQuoteFormSheet);
/// - posts through the Shipping repository.
///
/// The chat room id is passed through as data by the host (the quote
/// endpoint is room-scoped server-side); every business decision stays here.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/shipping_quote_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/widgets/shipping_quote_form_sheet.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

/// What the host forwards: the projected listing identity + display title.
/// `productId` is resolved fresh by this intent, never trusted from a host.
class ShippingQuoteTarget {
  final String forSaleId;
  final String title;

  const ShippingQuoteTarget({required this.forSaleId, required this.title});
}

/// Resolves the seller quote target from a server projection. `null` when
/// the surface is not a LIVE for_sale the viewer manages (buyers, guests,
/// tombstones, auctions — auction settlement quotes are backend/workflow
/// owned, never seller-initiated from chat).
ShippingQuoteTarget? resolveSellerShippingQuoteTarget(
  ResourceProjection? projection,
) {
  if (projection == null || !projection.isLive) return null;
  if (projection.resourceType != ResourceProjectionType.fixedPriceSale) {
    return null;
  }
  final actions = projection.commerceActions;
  if (actions == null || !actions.canManage) return null;
  final payload = projection.payload;
  if (payload is! ForSaleLivePayload) return null;
  final forSaleId = projection.resourceId.trim();
  if (forSaleId.isEmpty) return null;
  return ShippingQuoteTarget(
    forSaleId: forSaleId,
    title: payload.title,
  );
}

/// Opens the seller shipping quote flow from any host screen.
///
/// [chatRoomId] — the room the quote binds to (host data, backend scopes the
/// endpoint per room). [onSuccess] — fired after the quote is accepted so
/// the host can refresh (the quote message is persisted server-side).
Future<void> openSellerShippingQuoteSheet({
  required BuildContext context,
  required WidgetRef ref,
  required String chatRoomId,
  ResourceProjection? projection,
  Future<void> Function()? onSuccess,
}) async {
  final target = resolveSellerShippingQuoteTarget(projection);
  if (target == null) return;
  if (!context.mounted) return;

  ForSale? forSale;
  try {
    forSale = await ref.read(forSaleDetailProvider(target.forSaleId).future);
  } catch (_) {
    forSale = null;
  }
  if (!context.mounted) return;
  if (forSale == null) {
    AppSnackBar.showError(context, 'Produk tidak ditemukan');
    return;
  }

  final productId = forSale.productId;
  if (productId == null || productId.isEmpty) {
    AppSnackBar.showError(
      context,
      'ID produk belum tersedia untuk penawaran ongkir ini',
    );
    return;
  }

  final submitted = await ShippingQuoteFormSheet.show(
    context: context,
    productTitle: forSale.title,
  );
  if (submitted == null || !context.mounted) return;

  final request = buildForSaleShippingQuoteRequest(
    productId: productId,
    forSaleId: forSale.forSaleId,
    cost: submitted.cost,
    note: submitted.note,
    destinationCityId: submitted.destinationCityId,
    destinationProvinceId: submitted.destinationProvinceId,
  );

  try {
    await ref
        .read(shippingQuoteRepositoryProvider)
        .createShippingQuote(chatId: chatRoomId, request: request);
    if (!context.mounted) return;
    AppSnackBar.showSuccess(context, 'Penawaran ongkir terkirim');
    await onSuccess?.call();
  } catch (_) {
    if (!context.mounted) return;
    AppSnackBar.showError(context, 'Gagal mengirim penawaran ongkir. Coba lagi.');
  }
}

/// Canonical request builder for a fixed-price-sale shipping quote — the
/// single assembly point of the create-quote wire (moved here from chat:
/// request building is Shipping's authority, never a host's).
///
/// DESTINATION LOCK (Owner 2026-10-01): [destinationCityId] /
/// [destinationProvinceId] are REQUIRED — the backend rejects a for_sale
/// quote without them, and checkout only accepts the buyer address whose
/// kota/kabupaten matches.
CreateShippingQuoteRequestDto buildForSaleShippingQuoteRequest({
  required String productId,
  required String forSaleId,
  required int cost,
  required String destinationCityId,
  required String destinationProvinceId,
  String? note,
}) {
  return CreateShippingQuoteRequestDto(
    productId: productId,
    sourceType: 'for_sale',
    sourceId: forSaleId,
    cost: cost,
    note: note,
    destinationCityId: destinationCityId,
    destinationProvinceId: destinationProvinceId,
  );
}
