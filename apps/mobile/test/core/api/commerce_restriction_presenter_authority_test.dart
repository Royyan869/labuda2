/// Canonical presentation authority tests for CommerceRestrictionPresenter.
///
/// Scope: the converged presenter now owns BOTH restriction semantics:
///   - `COMMERCE_RESTRICTED`      → error snackbar (unchanged contract)
///   - `MARKET_AUTHORITY_REQUIRED` → navigate via NavigationHandler
///                                    .navigateToSellerRenewal()
///                                    → RoutePaths.sellerRenewal
///                                    → SellerRenewalScreen
///
/// Negative contract: generic `FORBIDDEN` is NOT a market-authority denial —
/// the presenter must not consume it and must not route to renewal.
///
/// Navigation proof uses the canonical `navigationHandlerProvider` override
/// seam, which fails if the presenter ever bypasses the NavigationHandler
/// doctrine with a direct context.push/go.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/api/api_error_codes.dart' as codes;
import 'package:labuda/core/api/commerce_restriction_presenter.dart';
import 'package:labuda/core/navigation/navigation_handler.dart';
import 'package:labuda/core/navigation/navigation_provider.dart';

/// Records the canonical navigation + snackbar surface without touching the
/// real router. Only the members under test are overridden; the rest of the
/// interface is intentionally unreachable no-ops via the noSuchMethod trap.
class _RecordingNavigationHandler implements NavigationHandler {
  int renewalCalls = 0;
  int upgradeCalls = 0;

  @override
  void navigateToSellerRenewal() => renewalCalls++;

  @override
  void navigateToSellerUpgrade() => upgradeCalls++;

  @override
  void showSnackBar(String message, {bool isError = false}) {}

  @override
  void noSuchMethod(Invocation invocation) {}
}

void main() {
  late _RecordingNavigationHandler navigation;

  setUp(() {
    navigation = _RecordingNavigationHandler();
  });

  /// Pumps the minimal harness with the canonical provider overridden to the
  /// recording handler and returns a BuildContext inside that scope.
  Future<BuildContext> pumpHarness(WidgetTester tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          navigationHandlerProvider.overrideWithValue(navigation),
        ],
        child: const MaterialApp(
          home: Scaffold(body: SizedBox.shrink()),
        ),
      ),
    );
    await tester.pump();
    return tester.element(find.byType(Scaffold));
  }

  group('Code recognition — single presenter, two restriction semantics', () {
    test('isMarketAuthorityRequired recognizes MARKET_AUTHORITY_REQUIRED', () {
      expect(
        CommerceRestrictionPresenter.isMarketAuthorityRequired(
          codes.marketAuthorityRequired,
        ),
        isTrue,
      );
    });

    test('isMarketAuthorityRequired rejects other codes', () {
      expect(
        CommerceRestrictionPresenter.isMarketAuthorityRequired(
          codes.commerceRestricted,
        ),
        isFalse,
      );
      expect(
        CommerceRestrictionPresenter.isMarketAuthorityRequired('FORBIDDEN'),
        isFalse,
      );
      expect(
        CommerceRestrictionPresenter.isMarketAuthorityRequired(
          codes.emailVerificationRequired,
        ),
        isFalse,
      );
      expect(
        CommerceRestrictionPresenter.isMarketAuthorityRequired(null),
        isFalse,
      );
    });

    test('isCommerceRestricted contract unchanged by convergence', () {
      expect(
        CommerceRestrictionPresenter.isCommerceRestricted(
          codes.commerceRestricted,
        ),
        isTrue,
      );
      // MARKET_AUTHORITY_REQUIRED must NOT be presented as commerce restriction.
      expect(
        CommerceRestrictionPresenter.isCommerceRestricted(
          codes.marketAuthorityRequired,
        ),
        isFalse,
      );
    });

    test('isRestrictionPresented covers exactly the two canonical codes', () {
      expect(
        CommerceRestrictionPresenter.isRestrictionPresented(
          codes.commerceRestricted,
        ),
        isTrue,
      );
      expect(
        CommerceRestrictionPresenter.isRestrictionPresented(
          codes.marketAuthorityRequired,
        ),
        isTrue,
      );
      expect(
        CommerceRestrictionPresenter.isRestrictionPresented('FORBIDDEN'),
        isFalse,
      );
      expect(
        CommerceRestrictionPresenter.isRestrictionPresented(null),
        isFalse,
      );
    });
  });

  group('handle() — MARKET_AUTHORITY_REQUIRED → renewal navigation', () {
    testWidgets(
      'navigates to seller renewal via NavigationHandler, no error snackbar',
      (tester) async {
        final context = await pumpHarness(tester);

        final consumed = CommerceRestrictionPresenter.handle(
          context,
          errorCode: codes.marketAuthorityRequired,
          actionDescription: 'membuat lelang',
        );
        await tester.pump();

        expect(consumed, isTrue);
        expect(
          navigation.renewalCalls,
          1,
          reason: 'canonical behavior = navigateToSellerRenewal() → '
              'RoutePaths.sellerRenewal',
        );
        // Renewal — not the registration upgrade lifecycle.
        expect(navigation.upgradeCalls, 0);
        // Restriction ≠ governance snackbar for this semantic.
        expect(find.textContaining('dibatasi'), findsNothing);
      },
    );

    testWidgets('is consumed exactly once per call', (tester) async {
      final context = await pumpHarness(tester);

      CommerceRestrictionPresenter.handle(
        context,
        errorCode: codes.marketAuthorityRequired,
        actionDescription: 'melakukan checkout',
      );
      CommerceRestrictionPresenter.handle(
        context,
        errorCode: codes.marketAuthorityRequired,
        actionDescription: 'melakukan checkout',
      );
      await tester.pump();

      expect(navigation.renewalCalls, 2);
    });
  });

  group('handle() — COMMERCE_RESTRICTED behavior unchanged', () {
    testWidgets('shows the canonical error snackbar, never navigates',
        (tester) async {
      final context = await pumpHarness(tester);

      final consumed = CommerceRestrictionPresenter.handle(
        context,
        errorCode: codes.commerceRestricted,
        actionDescription: 'melakukan checkout',
      );
      await tester.pump();

      expect(consumed, isTrue);
      expect(find.textContaining('dibatasi'), findsOneWidget);
      expect(find.textContaining('Hubungi dukungan'), findsOneWidget);
      expect(navigation.renewalCalls, 0);
      expect(navigation.upgradeCalls, 0);
    });

    testWidgets('direct show() contract unchanged', (tester) async {
      final context = await pumpHarness(tester);

      CommerceRestrictionPresenter.show(
        context,
        actionDescription: 'menempatkan bid',
      );
      await tester.pump();

      expect(find.textContaining('dibatasi'), findsOneWidget);
      expect(navigation.renewalCalls, 0);
    });
  });

  group('handle() — generic FORBIDDEN is NOT market authority', () {
    testWidgets('returns false, no renewal navigation, no snackbar',
        (tester) async {
      final context = await pumpHarness(tester);

      final consumed = CommerceRestrictionPresenter.handle(
        context,
        errorCode: 'FORBIDDEN',
        actionDescription: 'membuat forSale',
      );
      await tester.pump();

      expect(consumed, isFalse);
      expect(navigation.renewalCalls, 0);
      expect(navigation.upgradeCalls, 0);
      expect(find.textContaining('dibatasi'), findsNothing);
    });
  });
}
