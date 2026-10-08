import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/features/marketplace/marketplace.dart';

// ============================================================================
// TYPOGRAPHY MIGRATION — IRISAN 2 RESOLVER PROOF (plan Tahap 2).
//
// The marketplace tab labels used to state a raw size. They now take their
// metrics from the theme's `labelLarge` role, with only
// the selected/unselected weight decided at the call site. An analyzer cannot
// see the difference between a role and a literal that happens to match today;
// this test pumps the REAL screen with the REAL theme and asserts the RENDERED
// style IS the role — the same proof obligation as irisan 1.
// ============================================================================

void main() {
  testWidgets('the tab label renders the theme role, not a size token', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          theme: AppTheme.lightTheme,
          home: const MarketplaceScreen(initialTab: 0),
        ),
      ),
    );
    // Never pumpAndSettle: the tab grids show indeterminate spinners while
    // their lists load (same rule as the create-entry contract test).
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    final theme = Theme.of(tester.element(find.byType(TabBar)));
    final role = theme.textTheme.labelLarge!;
    // Anti-vacuity floor: compare resolved geometry, never `null == null`.
    expect(role.fontSize, isNotNull, reason: 'resolve via Theme.of, not raw');
    expect(role.height, isNotNull);
    expect(role.letterSpacing, isNotNull);

    TextStyle rendered(String text) => DefaultTextStyle.of(
      tester.element(
        find.descendant(of: find.byType(TabBar), matching: find.text(text)),
      ),
    ).style;

    final selected = rendered('For Sale');
    expect(
      selected.fontSize,
      role.fontSize,
      reason: 'size comes from the role',
    );
    expect(selected.height, role.height, reason: 'line height from the role');
    expect(selected.letterSpacing, role.letterSpacing);
    expect(selected.fontWeight, FontWeight.w600, reason: "call site's weight");

    final unselected = rendered('Auction');
    expect(unselected.fontSize, role.fontSize);
    expect(unselected.height, role.height);
    expect(
      unselected.fontWeight,
      FontWeight.w400,
      reason: "call site's weight",
    );
  });
}
