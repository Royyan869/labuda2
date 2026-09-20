// Checkout readiness contract — R1.2 proof.
//
// The checkout pricing step has ONE truthful readiness projection
// (presentation/models/checkout_readiness.dart). These tests prove the
// projection contract itself, including the states that a widget test cannot
// reach deterministically (token expiry uses wall-clock time).
//
// INVARIANT UNDER TEST:
//   ready ⟺ product identity + address + shipping selection (when required)
//             + an applied preview that is CURRENT for the inputs
//             + an unexpired, usable pricing token
//
// A local price (`forSale.price`) is not an input: it can never make checkout
// ready.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/checkout/presentation/models/checkout_readiness.dart';

/// Builds inputs with explicit, readable defaults: a satisfied precondition
/// set with a current, unexpired preview carrying a usable token.
CheckoutReadinessInputs _inputs({
  bool hasProductId = true,
  bool hasAddress = true,
  bool requiresShippingSelection = true,
  bool hasShippingSelection = true,
  bool hasPreview = true,
  bool isPreviewCurrent = true,
  bool isPreviewExpired = false,
  bool hasPricingToken = true,
  bool isLoadingPreview = false,
  bool hasPreviewError = false,
}) {
  return CheckoutReadinessInputs(
    hasProductId: hasProductId,
    hasAddress: hasAddress,
    requiresShippingSelection: requiresShippingSelection,
    hasShippingSelection: hasShippingSelection,
    hasPreview: hasPreview,
    isPreviewCurrent: isPreviewCurrent,
    isPreviewExpired: isPreviewExpired,
    hasPricingToken: hasPricingToken,
    isLoadingPreview: isLoadingPreview,
    hasPreviewError: hasPreviewError,
  );
}

void main() {
  group('CheckoutReadiness — ready invariant', () {
    test('ready requires product, address, shipping, current unexpired token',
        () {
      expect(
        evaluateCheckoutReadiness(_inputs()),
        CheckoutReadiness.ready,
      );
    });

    test('a current preview is not enough when the token is missing', () {
      expect(
        evaluateCheckoutReadiness(_inputs(hasPricingToken: false)),
        CheckoutReadiness.error,
      );
    });

    test('an expired token is never ready, even with a current preview', () {
      final readiness = evaluateCheckoutReadiness(
        _inputs(isPreviewExpired: true),
      );
      expect(readiness, CheckoutReadiness.expired);
      expect(readiness.isReady, isFalse);
      expect(readiness.isRefreshable, isTrue);
    });

    test('a preview computed for other inputs is never ready', () {
      expect(
        evaluateCheckoutReadiness(_inputs(isPreviewCurrent: false)),
        CheckoutReadiness.stale,
      );
    });

    test('no preview can make checkout ready', () {
      expect(
        evaluateCheckoutReadiness(_inputs(hasPreview: false)),
        CheckoutReadiness.loading,
      );
    });
  });

  group('CheckoutReadiness — prerequisites come first', () {
    test('each missing prerequisite has its own truthful state', () {
      expect(
        evaluateCheckoutReadiness(_inputs(hasProductId: false)),
        CheckoutReadiness.missingProduct,
      );
      expect(
        evaluateCheckoutReadiness(_inputs(hasAddress: false)),
        CheckoutReadiness.missingAddress,
      );
      expect(
        evaluateCheckoutReadiness(_inputs(hasShippingSelection: false)),
        CheckoutReadiness.missingShipping,
      );
    });

    test('a shipping quote checkout needs no option selection', () {
      expect(
        evaluateCheckoutReadiness(
          _inputs(requiresShippingSelection: false, hasShippingSelection: false),
        ),
        CheckoutReadiness.ready,
      );
    });

    test('a stale error never masks a missing prerequisite', () {
      // A previous failure must not hide the action the buyer has to take.
      expect(
        evaluateCheckoutReadiness(
          _inputs(hasPreview: false, hasPreviewError: true, hasAddress: false),
        ),
        CheckoutReadiness.missingAddress,
      );
    });
  });

  group('CheckoutReadiness — failure is truthful, never a spinner', () {
    test('a failed preview with no applicable result is an error state', () {
      final readiness = evaluateCheckoutReadiness(
        _inputs(hasPreview: false, hasPreviewError: true),
      );
      expect(readiness, CheckoutReadiness.error);
      expect(readiness.isReady, isFalse);
      expect(readiness.isRefreshable, isTrue);
      expect(readiness.title, 'Gagal Memuat Harga');
      expect(readiness.message, isNotEmpty);
    });

    test('refreshing a stale preview reads as loading, not as a stale banner',
        () {
      expect(
        evaluateCheckoutReadiness(
          _inputs(isPreviewCurrent: false, isLoadingPreview: true),
        ),
        CheckoutReadiness.loading,
      );
    });

    test('a failed refresh never invalidates a preview that is still current',
        () {
      // The applied pricing still matches the inputs, so checkout stays ready;
      // the failure is surfaced by the manual-refresh path instead.
      expect(
        evaluateCheckoutReadiness(_inputs(hasPreviewError: true)),
        CheckoutReadiness.ready,
      );
    });
  });

  group('CheckoutReadiness — copy contract', () {
    test('every not-ready state explains itself in the buyer language', () {
      for (final readiness in CheckoutReadiness.values) {
        if (readiness.isReady) continue;
        expect(
          readiness.message,
          isNotEmpty,
          reason: '$readiness must explain why checkout is not ready',
        );
        expect(
          readiness.title,
          isNotEmpty,
          reason: '$readiness must have a title',
        );
      }
    });

    test('ready carries no blocking copy', () {
      expect(CheckoutReadiness.ready.message, isEmpty);
      expect(CheckoutReadiness.ready.isReady, isTrue);
      expect(CheckoutReadiness.ready.isRefreshable, isFalse);
    });

    test('the prerequisite copy names the required action', () {
      expect(
        CheckoutReadiness.missingShipping.message,
        'Pilih opsi pengiriman untuk memuat harga dari server.',
      );
      expect(
        CheckoutReadiness.missingProduct.message,
        'Lengkapi data checkout untuk memuat harga dari server.',
      );
      expect(
        CheckoutReadiness.missingAddress.message,
        'Pilih alamat pengiriman terlebih dahulu',
      );
      expect(
        CheckoutReadiness.stale.message,
        'Harga perlu diperbarui karena detail checkout berubah.',
      );
      expect(
        CheckoutReadiness.loading.message,
        'Memuat harga dari server...',
      );
    });
  });
}
