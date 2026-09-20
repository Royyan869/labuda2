// Checkout completion — remaining HANDOFF finding (NOT in any checkout scope).
//
// This file used to prove the checkout pricing design with source-text
// assertions (`hasFreshPricing`, `helperMessage`, `!state._hasFreshPreview`,
// `previewResult: hasFreshPricing ? previewResult : null`, colourScheme tokens).
// All of those designs have since been converged and their stale assertions were
// removed:
//
//   * pricing / preview identity / readiness →
//     `presentation/screens/checkout_preview_convergence_test.dart`
//     + `checkout_readiness_contract_test.dart` (behavior proofs)
//   * seller chat → `checkout_chat_canonical_contract_test.dart`
//   * theme authority → `checkout_theme_authority_contract_test.dart`
//     (renders the screen in both themes and asserts the scheme's colours)
//
// What is left is the ONE finding that belongs to another domain.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'HANDOFF (Shipping): shipping coverage keeps addressless, unconfigured, out-of-coverage and covered states separate',
    () {
      final shippingSource = File(
        'lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_shipping_section.dart',
      ).readAsStringSync();

      // Unmet: the shipping section does not distinguish
      // `isNoShippingConfiguration` / `isOutOfCoverage` because the canonical
      // delivery-availability response drops the product-configuration state.
      // Fixing this means changing the Shipping domain, which is out of scope.
      expect(
        shippingSource,
        contains('deliveryAvailability?.isNoShippingConfiguration == true'),
      );
      expect(
        shippingSource,
        contains('deliveryAvailability?.isOutOfCoverage == true'),
      );
    },
    skip:
        'HANDOFF: Shipping domain finding — canonical DeliveryOption drops the '
        'product-configured/out-of-coverage distinction. Out of every checkout '
        'scope.',
  );
}
