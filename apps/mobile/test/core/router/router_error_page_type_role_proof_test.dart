import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/src/router/router_error_page.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

// ============================================================================
// TYPOGRAPHY MIGRATION — IRISAN 3 RESOLVER PROOF (plan Tahap 2).
//
// The router error page once stated four raw sizes. It now reads roles from the
// theme: `headlineSmall` for the headline, `bodyLarge`
// for the error detail, `bodySmall` for the debug line, and `labelLarge` for
// BOTH button labels (canonical button text — this page alone was inflating
// it to 16). Weights and colours remain the call site's decisions.
//
// The page is pumped through the REAL GoRouter error path (unknown location →
// errorBuilder), so the proof runs the widget exactly as production does. An
// analyzer cannot tell a role from a literal that matches today; this asserts
// the RENDERED style IS the role, with isNotNull floors so nothing passes as
// `null == null`.
// ============================================================================

void main() {
  testWidgets('the error page renders theme roles, not size tokens', (
    tester,
  ) async {
    final router = GoRouter(
      initialLocation: '/does-not-exist',
      routes: [
        GoRoute(path: '/', builder: (context, state) => const SizedBox()),
      ],
      errorBuilder: (context, state) => RouterErrorPage(state: state),
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(
      MaterialApp.router(theme: AppTheme.lightTheme, routerConfig: router),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    final theme = Theme.of(tester.element(find.byType(RouterErrorPage)));
    final headline = theme.textTheme.headlineSmall!;
    final detail = theme.textTheme.bodyLarge!;
    final dense = theme.textTheme.bodySmall!;
    final button = theme.textTheme.labelLarge!;
    // Anti-vacuity: resolve geometry first, never compare raw nulls.
    expect(headline.fontSize, isNotNull, reason: 'resolve via Theme.of');
    expect(detail.fontSize, isNotNull);
    expect(dense.fontSize, isNotNull);
    expect(button.fontSize, isNotNull);

    final title = tester.widget<Text>(find.text('Page Not Found')).style!;
    expect(title.fontSize, headline.fontSize, reason: 'size from the role');
    expect(title.height, headline.height, reason: 'line height from the role');
    expect(title.letterSpacing, headline.letterSpacing);
    expect(title.fontWeight, FontWeight.bold, reason: "call site's weight");

    final detailStyle = tester
        .widget<Text>(find.textContaining('could not be found'))
        .style!;
    expect(detailStyle.fontSize, detail.fontSize);
    expect(detailStyle.height, detail.height);
    expect(detailStyle.letterSpacing, detail.letterSpacing);

    final denseStyle = tester
        .widget<Text>(find.textContaining('Full URI:'))
        .style!;
    expect(denseStyle.fontSize, dense.fontSize);
    expect(denseStyle.height, dense.height);

    final home = tester.widget<Text>(find.text('Go to Home')).style!;
    expect(home.fontSize, button.fontSize, reason: 'button text = labelLarge');
    expect(home.height, button.height);
    expect(home.fontWeight, FontWeight.w600, reason: "call site's weight");

    final back = tester.widget<Text>(find.text('Go Back')).style!;
    expect(back.fontSize, button.fontSize, reason: 'button text = labelLarge');
    expect(back.fontWeight, FontWeight.w600, reason: "call site's weight");
  });
}
