/// Typed contract for the canonical Create ForSale route.
library;

enum CreateForSaleReturnMode { forSale, forSaleId }

class CreateForSaleRouteArgs {
  final CreateForSaleReturnMode returnMode;

  const CreateForSaleRouteArgs({
    this.returnMode = CreateForSaleReturnMode.forSale,
  });

  const CreateForSaleRouteArgs.chatDirectCommerce()
    : returnMode = CreateForSaleReturnMode.forSaleId;
}

class CreatedForSaleResult {
  final String forSaleId;

  const CreatedForSaleResult({required this.forSaleId});
}
