package application

import "errors"

// Phase 0 honesty errors.
//
// These sentinel errors carry the machine-readable codes that the HTTP
// handler boundary surfaces to clients (`SHIPPING_NOT_CONFIGURED`,
// `NO_SHIPPING_OPTIONS`, `SHIPPING_OPTION_UNAVAILABLE`). Internal services
// wrap them with `%w` when adding context so handlers can branch via
// `errors.Is` instead of brittle string matching.
//
// Usage doctrine:
//   - ErrShippingNotConfigured: for_sale has zero linked options at PUBLISH
//     time. Returned by ForSaleService.EnsureShippingConfigured /
//     ForSaleService.Publish.
//   - ErrNoShippingSetups: for_sale has zero linked options at ORDER
//     CREATE time. Returned by OrderCreationService when the buyer is on
//     the for_sale but the seller never linked any options.
//   - ErrShippingSetupUnavailable: the buyer-selected option is not
//     covered for the buyer's province/city, or coverage exists but is
//     marked unavailable. Returned by OrderCreationService.
var (
	ErrShippingNotConfigured                  = errors.New("SHIPPING_NOT_CONFIGURED: no shipping options linked to for_sale")
	ErrNoShippingSetups                      = errors.New("NO_SHIPPING_OPTIONS: for_sale has no shipping options configured")
	ErrShippingSetupUnavailable              = errors.New("SHIPPING_OPTION_UNAVAILABLE: shipping option not available for buyer address")
	ErrInvalidSellableCreateShippingSelection = errors.New("INVALID_SHIPPING_SELECTION: shipping option does not exist or does not belong to seller")
)
