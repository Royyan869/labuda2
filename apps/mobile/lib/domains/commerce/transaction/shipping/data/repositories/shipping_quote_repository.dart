/// Manual shipping quote transport authority (Shipping domain, data layer).
///
/// Separated from `ShippingRepository` (setup CRUD): the quote is a
/// room-scoped commerce write owned end-to-end by the Shipping domain.
/// Chat never sees this contract — it only forwards the seller's intent to
/// `openSellerShippingQuoteSheet` (Owner rule 2026-10-01: chat must not
/// handle shipping, zero coupling).
library;

import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/shipping_quote_dto.dart';
import 'package:labuda/domains/commerce/transaction/shipping/data/remote/shipping_remote_datasource.dart';

class ShippingQuoteRepository {
  ShippingQuoteRepository(this._datasource);

  final ShippingRemoteDatasource _datasource;

  /// POST /chat/:chat_id/shipping-quote. Throws on transport/contract
  /// failure — the caller surfaces the error to the seller.
  Future<ShippingQuoteResponseDto> createShippingQuote({
    required String chatId,
    required CreateShippingQuoteRequestDto request,
  }) {
    return _datasource.createShippingQuote(chatId, request);
  }
}
