/// Canonical visual vocabulary for Labuda's canonical backend payment methods.
///
/// AUTHORITY BOUNDARY:
/// - The backend `payment_methods` table owns method identity (`method_code`),
///   `display_name`, enabled state, and fees.
/// - This file answers ONE question only: how is a canonical `method_code`
///   visually represented in the mobile UI?
///
/// There is exactly ONE mapping ([PaymentMethodVisuals._visuals]). Do not copy
/// it into a feature widget or screen.
///
/// PROVENANCE: payment-brand PNGs are local copies of the official Midtrans
/// logo repository (`https://github.com/veritrans/logo`), whose README grants
/// use on e-commerce sites. The DANA brand asset is NOT included: no official
/// DANA source with clear provenance was obtainable, so DANA intentionally
/// falls back to a generic (non-brand) icon. See [danaOfficialAssetMissing].
library;

import 'package:flutter/material.dart';

/// A single brand mark that is part of a canonical method's representation
/// (for example Visa inside `credit_card`, or Alfamart inside
/// `convenience_store`). [name] is the accessible/semantic label.
@immutable
class PaymentMethodBrandVisual {
  final String asset;
  final String name;

  const PaymentMethodBrandVisual({required this.asset, required this.name});
}

/// The canonical visual definition of one payment method:
/// a label, a primary visual, and optional secondary brand visuals.
@immutable
class PaymentMethodVisual {
  /// Canonical display label for surfaces that only carry `method_code`.
  /// Selection surfaces render the backend `display_name` instead.
  final String label;

  /// Generic (non-brand) fallback icon. Always present so the UI never breaks
  /// and never renders bare text.
  final IconData fallbackIcon;

  /// The primary local asset. When null, the primary visual is [fallbackIcon].
  ///
  /// `bank_transfer`, `credit_card`, `dana`, `gopay`, `ovo`, and `shopeepay`
  /// intentionally have no primary asset: either no official generic/brand
  /// asset exists (bank transfer / card / DANA) or the method is out of this
  /// task's asset scope.
  final String? primaryAsset;

  /// Optional brand marks displayed alongside the primary visual. Empty for
  /// single-brand or generic methods.
  final List<PaymentMethodBrandVisual> secondaryBrands;

  const PaymentMethodVisual({
    required this.label,
    required this.fallbackIcon,
    this.primaryAsset,
    this.secondaryBrands = const <PaymentMethodBrandVisual>[],
  });
}

/// DANA has no official brand asset with clear provenance in this environment.
/// It must be supplied by the owner; until then DANA uses a generic fallback
/// icon and MUST NOT be represented by any third-party/fabricated logo.
///
/// Value is the report token; the identifier is lowerCamelCase per lint rules.
const String danaOfficialAssetMissing = 'DANA_OFFICIAL_ASSET_MISSING';

/// Local payment asset paths (official Midtrans logo repository).
const String _qrisAsset = 'assets/icons/payment/qris.png';
const String _alfamartAsset = 'assets/icons/payment/alfamart.png';
const String _indomaretAsset = 'assets/icons/payment/indomaret.png';
const String _visaAsset = 'assets/icons/payment/visa.png';
const String _mastercardAsset = 'assets/icons/payment/mastercard.png';
const String _amexAsset = 'assets/icons/payment/american_express.png';
const String _jcbAsset = 'assets/icons/payment/jcb.png';

/// Canonical presentation authority for payment methods.
class PaymentMethodVisuals {
  PaymentMethodVisuals._();

  static const Map<String, PaymentMethodVisual> _visuals =
      <String, PaymentMethodVisual>{
    // OWNER DECISION: generic bank transfer / Virtual Account. A specific bank
    // logo (BCA/BNI/BRI/Permata) must never represent this method.
    'bank_transfer': PaymentMethodVisual(
      label: 'Transfer Bank',
      fallbackIcon: Icons.account_balance_outlined,
    ),
    // OWNER DECISION: official QRIS asset.
    'qris': PaymentMethodVisual(
      label: 'QRIS',
      fallbackIcon: Icons.qr_code_2,
      primaryAsset: _qrisAsset,
    ),
    // OWNER DECISION: generic card visual as the PRIMARY; brand marks are
    // secondary and never the sole representation.
    'credit_card': PaymentMethodVisual(
      label: 'Kartu Kredit/Debit',
      fallbackIcon: Icons.credit_card,
      secondaryBrands: <PaymentMethodBrandVisual>[
        PaymentMethodBrandVisual(asset: _visaAsset, name: 'Visa'),
        PaymentMethodBrandVisual(asset: _mastercardAsset, name: 'Mastercard'),
        PaymentMethodBrandVisual(asset: _amexAsset, name: 'American Express'),
        PaymentMethodBrandVisual(asset: _jcbAsset, name: 'JCB'),
      ],
    ),
    // DANA_OFFICIAL_ASSET_MISSING: generic fallback only, never a fabricated
    // or third-party DANA logo.
    'dana': PaymentMethodVisual(
      label: 'DANA',
      fallbackIcon: Icons.account_balance_wallet_outlined,
    ),
    // Out of this asset task's scope; generic fallback only.
    'gopay': PaymentMethodVisual(
      label: 'GoPay',
      fallbackIcon: Icons.account_balance_wallet_outlined,
    ),
    'ovo': PaymentMethodVisual(
      label: 'OVO',
      fallbackIcon: Icons.account_balance_wallet_outlined,
    ),
    'shopeepay': PaymentMethodVisual(
      label: 'ShopeePay',
      fallbackIcon: Icons.account_balance_wallet_outlined,
    ),
    // OWNER DECISION: convenience_store is represented by Alfamart + Indomaret
    // (DAN+DAN is NOT shown as a separate brand). It stays ONE method.
    'convenience_store': PaymentMethodVisual(
      label: 'Minimarket',
      fallbackIcon: Icons.storefront_outlined,
      secondaryBrands: <PaymentMethodBrandVisual>[
        PaymentMethodBrandVisual(asset: _alfamartAsset, name: 'Alfamart'),
        PaymentMethodBrandVisual(asset: _indomaretAsset, name: 'Indomaret'),
      ],
    ),
  };

  static final PaymentMethodVisual _unknown = PaymentMethodVisual(
    label: '—',
    fallbackIcon: Icons.payments_outlined,
  );

  /// The canonical visual for a method code. Unknown/null codes return a
  /// generic visual (label = raw code, or '—' when empty) so nothing breaks.
  static PaymentMethodVisual visual(String? methodCode) {
    if (methodCode != null) {
      final PaymentMethodVisual? found = _visuals[methodCode];
      if (found != null) return found;
    }
    if (methodCode == null || methodCode.isEmpty) return _unknown;
    return PaymentMethodVisual(
      label: methodCode,
      fallbackIcon: Icons.payments_outlined,
    );
  }

  /// Convenience accessors onto the ONE mapping (never a second mapping).
  static IconData icon(String? methodCode) => visual(methodCode).fallbackIcon;

  static String label(String? methodCode) => visual(methodCode).label;

  static String? primaryAsset(String? methodCode) =>
      visual(methodCode).primaryAsset;

  static bool isCanonical(String? methodCode) =>
      methodCode != null && _visuals.containsKey(methodCode);

  /// Read-only view of every canonical visual, for proofs/tests.
  static Map<String, PaymentMethodVisual> get all =>
      Map<String, PaymentMethodVisual>.unmodifiable(_visuals);
}
