/// Manual shipping quote transport authority (Shipping domain, data layer).
///
/// Separated from `ShippingRepository` (setup CRUD): the quote is a
/// room-scoped commerce write owned end-to-end by the Shipping domain.
/// Chat never sees this contract. The generic product-card ongkir CTA was
/// purged (Owner decision); the seller's manual quote is forwarded by the
/// Shipping-domain intent (`openSellerShippingQuoteSheet`) only.
library;

import 'package:hishumi/domains/commerce/catalog/for_sale/data/dto/shipping_quote_dto.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/data/remote/shipping_remote_datasource.dart';

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
