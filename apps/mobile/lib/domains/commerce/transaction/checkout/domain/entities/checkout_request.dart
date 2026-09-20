import 'package:equatable/equatable.dart';

/// Checkout Request for creating a direct buy order
/// Carries both the product authority ID and the for-sale surface ID.
///
/// PRICING TOKEN: Must include the pricing_token from preview response
/// to ensure the order uses the exact same pricing snapshot that was shown to the user.
///
/// COINS: Backend determines the coins amount. Client only sends use_coins: true/false.
/// The backend calculates: min(user_balance, 20% of order_value)
///
/// ADDRESS: Backend expects address_id (UUID of a saved address), NOT an inline
/// shipping_address object. Use the address picker to select a saved address.
///
/// AUCTION CHECKOUT: Optional auctionId for auction checkout (winning bid or buy now)
class CheckoutRequest extends Equatable {
  /// Canonical Product authority ID.
  /// Must be provided by the upstream sale/product surface.
  final String? productId;

  /// Canonical for-sale surface ID.
  /// This is the sale surface / source_id authority.
  final String forSaleId;
  final int quantity;
  final bool? useCoins; // Client sends true/false, backend calculates amount
  final String? notes;

  /// Saved address ID — backend resolves the full address from this UUID.
  final String addressId;

  /// PRICING TOKEN: Required snapshot ID from preview response
  /// Backend validates this token and uses the stored pricing snapshot
  /// Order creation will fail without a valid pricing token
  final String pricingToken;

  /// AUCTION ID: Optional auction context for auction checkout (winning bid or buy now)
  final String? auctionId;

  /// CHAT COMMERCE CONTEXT: Optional context for negotiation checkout
  final String? negotiationId;

  /// SHIPPING QUOTE ID: Optional shipping quote ID from seller's manual quote
  /// When provided, the preview and order will use the seller's quoted shipping price
  /// instead of standard forSale shipping options
  final String? shippingQuoteId;

  /// SHIPPING OPTION ID: Standard shipping option selected by buyer.
  /// Required when shippingQuoteId is not provided.
  /// Mutually exclusive with shippingQuoteId.
  final String? shippingOptionId;

  const CheckoutRequest({
    this.productId,
    required this.forSaleId,
    this.quantity = 1,
    this.useCoins,
    this.notes,
    required this.addressId,
    required this.pricingToken,
    this.auctionId,
    this.negotiationId,
    this.shippingQuoteId,
    this.shippingOptionId,
  });

  // NOTE: the unused `copyWith` was purged. A checkout request is built exactly
  // once per submission from the screen state (the pricing token must belong to
  // the applied preview), so a mutator only invited building a request whose
  // token no longer matched its inputs.

  @override
  List<Object?> get props => [
    productId,
    forSaleId,
    quantity,
    useCoins,
    notes,
    addressId,
    pricingToken,
    auctionId,
    negotiationId,
    shippingQuoteId,
    shippingOptionId,
  ];
}
