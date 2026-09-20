/// Checkout Readiness Projection
///
/// **CHECKOUT OWNS NO PRICING.** This projection answers exactly one question:
/// "may the buyer press 'Buat Pesanan' right now, and if not, why?"
///
/// It is a PURE projection of facts that already exist on the checkout screen
/// (prerequisites + the applied backend preview + its request identity). It
/// duplicates no backend state and creates no second pricing producer.
///
/// READINESS INVARIANT:
///   ready
///     ⟺ product identity present
///       AND address present
///       AND (shipping option selected when the checkout is option-driven)
///       AND a backend preview HAS been applied
///       AND that preview still matches the current preview inputs
///       AND its pricing token is not expired
///       AND that token is usable (non-empty)
///
/// `forSale.price` is NEVER an input to this projection: a local price can
/// never make checkout ready.
library;

/// The truthful readiness of the checkout pricing step.
///
/// The order of [CheckoutReadiness.values] is NOT a precedence order; use
/// [evaluateCheckoutReadiness] for precedence.
enum CheckoutReadiness {
  /// The product authority id is missing, so no backend preview is possible.
  missingProduct,

  /// No saved shipping address has been selected yet.
  missingAddress,

  /// The checkout is shipping-option driven and no option is selected yet.
  missingShipping,

  /// Prerequisites are complete but no current backend preview is available.
  loading,

  /// The preview request failed and no applicable preview exists.
  error,

  /// A preview exists but it was computed for DIFFERENT inputs.
  stale,

  /// The applied preview's pricing token has expired.
  expired,

  /// A current, non-expired backend preview exists. Checkout may proceed.
  ready;

  /// Whether the buyer may proceed to order creation.
  bool get isReady => this == CheckoutReadiness.ready;

  /// Whether a pricing refresh can move this state forward.
  bool get isRefreshable =>
      this == CheckoutReadiness.error ||
      this == CheckoutReadiness.stale ||
      this == CheckoutReadiness.expired;

  /// Short state title for the pricing indicator.
  String get title {
    switch (this) {
      case CheckoutReadiness.missingProduct:
        return 'Lengkapi Data Checkout';
      case CheckoutReadiness.missingAddress:
        return 'Pilih Alamat Pengiriman';
      case CheckoutReadiness.missingShipping:
        return 'Pilih Opsi Pengiriman';
      case CheckoutReadiness.loading:
        return 'Memuat Harga';
      case CheckoutReadiness.error:
        return 'Gagal Memuat Harga';
      case CheckoutReadiness.stale:
        return 'Harga Perlu Diperbarui';
      case CheckoutReadiness.expired:
        return 'Harga Kadaluarsa';
      case CheckoutReadiness.ready:
        return 'Harga Terkunci';
    }
  }

  /// Truthful, actionable explanation for the buyer.
  ///
  /// This is the ONLY place that decides why checkout is not ready, so the
  /// summary and the action bar can never disagree about the reason.
  String get message {
    switch (this) {
      case CheckoutReadiness.missingProduct:
        return 'Lengkapi data checkout untuk memuat harga dari server.';
      case CheckoutReadiness.missingAddress:
        return 'Pilih alamat pengiriman terlebih dahulu';
      case CheckoutReadiness.missingShipping:
        return 'Pilih opsi pengiriman untuk memuat harga dari server.';
      case CheckoutReadiness.loading:
        return 'Memuat harga dari server...';
      case CheckoutReadiness.error:
        return 'Tap refresh untuk mencoba lagi';
      case CheckoutReadiness.stale:
        return 'Harga perlu diperbarui karena detail checkout berubah.';
      case CheckoutReadiness.expired:
        return 'Silakan refresh harga terbaru';
      case CheckoutReadiness.ready:
        return '';
    }
  }
}

/// Facts that determine checkout readiness.
///
/// Every field is derived from the checkout screen's own state or from the
/// applied backend preview. No money is computed here.
class CheckoutReadinessInputs {
  /// Canonical product authority id is available.
  final bool hasProductId;

  /// A saved shipping address is selected.
  final bool hasAddress;

  /// This checkout requires a shipping option selection (standard mode, i.e.
  /// not a seller shipping quote).
  final bool requiresShippingSelection;

  /// A shipping option is selected.
  final bool hasShippingSelection;

  /// A backend preview has been applied to the screen.
  final bool hasPreview;

  /// The applied preview was computed for the CURRENT preview inputs.
  final bool isPreviewCurrent;

  /// The applied preview's pricing token is expired.
  final bool isPreviewExpired;

  /// The applied preview carries a usable (non-empty) pricing token.
  final bool hasPricingToken;

  /// A preview request is currently in flight.
  final bool isLoadingPreview;

  /// The last preview request failed.
  final bool hasPreviewError;

  const CheckoutReadinessInputs({
    required this.hasProductId,
    required this.hasAddress,
    required this.requiresShippingSelection,
    required this.hasShippingSelection,
    required this.hasPreview,
    required this.isPreviewCurrent,
    required this.isPreviewExpired,
    required this.hasPricingToken,
    required this.isLoadingPreview,
    required this.hasPreviewError,
  });
}

/// Resolves the single truthful readiness state.
///
/// PRECEDENCE:
///  1. Prerequisites (most actionable for the buyer) — product, address,
///     shipping selection.
///  2. No applied preview → the last failure is shown only when it is the
///     real blocker; otherwise the state is [CheckoutReadiness.loading].
///  3. An applied preview → a preview computed for other inputs is stale
///     (while it is being refreshed the honest state is
///     [CheckoutReadiness.loading], not a permanently stale banner); then
///     expired; then a preview without a usable token is unusable
///     ([CheckoutReadiness.error]).
CheckoutReadiness evaluateCheckoutReadiness(CheckoutReadinessInputs inputs) {
  if (!inputs.hasProductId) {
    return CheckoutReadiness.missingProduct;
  }
  if (!inputs.hasAddress) {
    return CheckoutReadiness.missingAddress;
  }
  if (inputs.requiresShippingSelection && !inputs.hasShippingSelection) {
    return CheckoutReadiness.missingShipping;
  }
  if (!inputs.hasPreview) {
    return inputs.hasPreviewError
        ? CheckoutReadiness.error
        : CheckoutReadiness.loading;
  }
  if (!inputs.isPreviewCurrent) {
    return inputs.isLoadingPreview
        ? CheckoutReadiness.loading
        : CheckoutReadiness.stale;
  }
  if (inputs.isPreviewExpired) {
    return CheckoutReadiness.expired;
  }
  if (!inputs.hasPricingToken) {
    // The backend preview cannot drive an order without its snapshot token.
    return CheckoutReadiness.error;
  }
  return CheckoutReadiness.ready;
}
