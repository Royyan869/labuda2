/// SNACKBAR AUTHORITY CONTRACT.
///
/// The one canonical Snackbar authority is `AppSnackBar`. These tests prove:
/// - the semantic model is exactly four types;
/// - the visual model consumes the status foundation (background, contrast-safe
///   foreground, shape, elevation, padding, icon, typography);
/// - behaviour (floating, canonical durations, no automatic action, intentional
///   actions still work);
/// - no raw Material Snackbar bypass or obsolete infrastructure survives.
library;

import 'dart:io';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/widgets/app_snackbar.dart';
import 'package:hishumi/shared/widgets/bottom_action_bar.dart';

/// All non-generated Dart sources under `lib/`.
List<File> _libDartFiles() {
  final root = Directory('lib');
  return root
      .listSync(recursive: true)
      .whereType<File>()
      .where((f) => f.path.endsWith('.dart'))
      .where((f) => !f.path.contains('generated'))
      .toList();
}

/// Removes `//` and `/* */` comments so doc comments that NAME a banned symbol
/// do not create a false positive.
String _stripComments(String src) {
  final noBlock = src.replaceAll(RegExp(r'/\*[\s\S]*?\*/'), '');
  return noBlock.replaceAll(RegExp(r'//[^\n]*'), '');
}

/// WCAG relative-luminance contrast ratio.
double _contrast(Color a, Color b) {
  final la = a.computeLuminance();
  final lb = b.computeLuminance();
  final hi = math.max(la, lb);
  final lo = math.min(la, lb);
  return (hi + 0.05) / (lo + 0.05);
}

Future<BuildContext> _pumpHost(
  WidgetTester tester, {
  required ThemeData theme,
}) async {
  late BuildContext captured;
  await tester.pumpWidget(
    MaterialApp(
      theme: theme,
      home: Builder(
        builder: (context) {
          captured = context;
          return const Scaffold(body: SizedBox.expand());
        },
      ),
    ),
  );
  return captured;
}

void main() {
  group('semantic contract', () {
    test('exactly four canonical types exist, and no fifth', () {
      expect(AppSnackBarType.values.map((t) => t.name).toList(), <String>[
        'success',
        'error',
        'warning',
        'info',
      ]);
    });

    test('no obsolete semantic placeholder exists in AppSnackBar', () {
      final source = File(
        'lib/shared/widgets/app_snackbar.dart',
      ).readAsStringSync();
      for (final banned in const [
        'actionRequired',
        'neutralAction',
        'comingSoon',
        'featureUnavailable',
        'notice',
      ]) {
        expect(
          source.contains(banned),
          isFalse,
          reason: 'obsolete Snackbar semantic "$banned" must not exist',
        );
      }
    });
  });

  group('visual contract', () {
    testWidgets('renders one floating status-coloured SnackBar', (
      tester,
    ) async {
      final ctx = await _pumpHost(tester, theme: AppTheme.lightTheme);
      final status = ctx.statusColors;

      AppSnackBar.showError(ctx, 'boom');
      await tester.pump();

      final snack = tester.widget<SnackBar>(find.byType(SnackBar));
      expect(snack.behavior, SnackBarBehavior.floating);
      expect(snack.backgroundColor, status.error);
      expect(snack.elevation, AppElevation.snackBar);
      expect(snack.padding, AppMetrics.inputPadding);
      expect(
        (snack.shape! as RoundedRectangleBorder).borderRadius,
        BorderRadius.circular(AppShape.r12),
      );
      final margin = snack.margin! as EdgeInsets;
      expect(margin.left, AppMetrics.p16);
      expect(margin.right, AppMetrics.p16);

      // Foreground + icon come from the foundation, not a blanket white.
      final icon = tester.widget<Icon>(
        find.descendant(of: find.byType(SnackBar), matching: find.byType(Icon)),
      );
      expect(icon.color, status.onError);
      expect(icon.size, AppIconSize.action);

      final text = tester.widget<Text>(
        find.descendant(of: find.byType(SnackBar), matching: find.byType(Text)),
      );
      expect(text.style?.color, status.onError);
      expect(text.style?.fontSize, ctx.typeRoles.bodyDense.fontSize);
    });

    test('info is NOT painted with the brand/error red', () {
      final status = AppTheme.lightTheme.extension<AppStatusColors>()!;
      expect(status.info, isNot(status.error));
    });
  });

  group('contrast contract', () {
    test('every status foreground/background pair meets WCAG AA (>=4.5:1)', () {
      for (final theme in <ThemeData>[
        AppTheme.lightTheme,
        AppTheme.darkTheme,
      ]) {
        final s = theme.extension<AppStatusColors>()!;
        expect(
          _contrast(s.success, s.onSuccess),
          greaterThanOrEqualTo(4.5),
          reason: 'success pair fails contrast',
        );
        expect(
          _contrast(s.warning, s.onWarning),
          greaterThanOrEqualTo(4.5),
          reason: 'warning pair fails contrast',
        );
        expect(
          _contrast(s.error, s.onError),
          greaterThanOrEqualTo(4.5),
          reason: 'error pair fails contrast',
        );
        expect(
          _contrast(s.info, s.onInfo),
          greaterThanOrEqualTo(4.5),
          reason: 'info pair fails contrast',
        );
      }
    });
  });

  group('behaviour contract', () {
    testWidgets('canonical durations: success/info/warning 4s, error 6s', (
      tester,
    ) async {
      Future<Duration> durationOf(void Function(BuildContext) show) async {
        final ctx = await _pumpHost(tester, theme: AppTheme.lightTheme);
        show(ctx);
        await tester.pump();
        return tester.widget<SnackBar>(find.byType(SnackBar)).duration;
      }

      expect(
        await durationOf((c) => AppSnackBar.showSuccess(c, 's')),
        const Duration(seconds: 4),
      );
      expect(
        await durationOf((c) => AppSnackBar.showInfo(c, 'i')),
        const Duration(seconds: 4),
      );
      expect(
        await durationOf((c) => AppSnackBar.showWarning(c, 'w')),
        const Duration(seconds: 4),
      );
      expect(
        await durationOf((c) => AppSnackBar.showError(c, 'e')),
        const Duration(seconds: 6),
      );
    });

    testWidgets('no automatic Close action is attached', (tester) async {
      final ctx = await _pumpHost(tester, theme: AppTheme.lightTheme);
      AppSnackBar.showError(ctx, 'no auto close');
      await tester.pump();
      final snack = tester.widget<SnackBar>(find.byType(SnackBar));
      expect(snack.action, isNull);
    });

    testWidgets('an explicitly provided action is rendered', (tester) async {
      final ctx = await _pumpHost(tester, theme: AppTheme.lightTheme);
      var pressed = false;
      AppSnackBar.showInfo(
        ctx,
        'with action',
        action: AppSnackBarAction(
          label: 'Undo',
          onPressed: () => pressed = true,
        ),
      );
      await tester.pump();
      final snack = tester.widget<SnackBar>(find.byType(SnackBar));
      expect(snack.action, isNotNull);
      expect(snack.action!.label, 'Undo');
      snack.action!.onPressed();
      expect(pressed, isTrue);
    });

    testWidgets('previous toast is cleared before a new one (one at a time)', (
      tester,
    ) async {
      final ctx = await _pumpHost(tester, theme: AppTheme.lightTheme);
      AppSnackBar.showInfo(ctx, 'first');
      await tester.pump();
      AppSnackBar.showInfo(ctx, 'second');
      await tester.pump();
      expect(find.text('second'), findsOneWidget);
      expect(find.text('first'), findsNothing);
    });
  });

  // ===========================================================================
  // SAFE-AREA-05 — BOTTOM INSET AUTHORITY (Owner decision: 16 px visual gap).
  //
  // The Scaffold/framework owns system inset, bottom-bar clearance and keyboard
  // clearance for the snackbar slot. AppSnackBar contributes exactly ONE
  // constant visual gap; nothing about its margin may come from MediaQuery.
  // ===========================================================================
  group('SAFE-AREA-05 — bottom inset authority', () {
    const EdgeInsets expectedMargin = EdgeInsets.only(
      bottom: AppMetrics.p16,
      left: AppMetrics.p16,
      right: AppMetrics.p16,
    );

    void setWindowInsets(
      WidgetTester tester, {
      double systemBottom = 0,
      double keyboard = 0,
    }) {
      final double dpr = tester.view.devicePixelRatio;
      tester.view.padding = FakeViewPadding(bottom: systemBottom * dpr);
      tester.view.viewPadding = FakeViewPadding(bottom: systemBottom * dpr);
      tester.view.viewInsets = FakeViewPadding(bottom: keyboard * dpr);
    }

    Finder snackFinder() => find.byType(SnackBar);

    /// The visible toast surface (the SnackBar's own Material), i.e. the
    /// widget box EXCLUDING the margin padding around it.
    Finder toastFinder() => find.descendant(
      of: snackFinder(),
      matching: find.byWidgetPredicate(
        (Widget w) => w is Material && w.elevation == AppElevation.snackBar,
      ),
    );

    EdgeInsets marginOf(WidgetTester tester) =>
        tester.widget<SnackBar>(snackFinder()).margin! as EdgeInsets;

    Future<void> pumpHost(WidgetTester tester, Widget home) async {
      await tester.pumpWidget(
        MaterialApp(theme: AppTheme.lightTheme, home: home),
      );
      await tester.pumpAndSettle();
    }

    testWidgets('A — constant 16 px visual gap; no 106', (tester) async {
      addTearDown(tester.view.reset);
      setWindowInsets(tester, systemBottom: 0);

      late BuildContext ctx;
      await pumpHost(
        tester,
        Builder(
          builder: (context) {
            ctx = context;
            return const Scaffold(body: SizedBox.expand());
          },
        ),
      );

      AppSnackBar.showInfo(ctx, 'gap');
      await tester.pumpAndSettle();

      final EdgeInsets margin = marginOf(tester);
      expect(margin, const EdgeInsets.only(bottom: 16, left: 16, right: 16));
      expect(margin.bottom, AppMetrics.p16);
      expect(margin.bottom, isNot(106));
    });

    testWidgets(
      'B — inset 0/24/48 moves the framework base, never the margin',
      (tester) async {
        addTearDown(tester.view.reset);

        late BuildContext ctx;
        await pumpHost(
          tester,
          Builder(
            builder: (context) {
              ctx = context;
              return const Scaffold(body: SizedBox.expand());
            },
          ),
        );

        for (final double inset in const <double>[0, 24, 48]) {
          setWindowInsets(tester, systemBottom: inset);
          AppSnackBar.showInfo(ctx, 'inset $inset');
          await tester.pumpAndSettle();

          expect(
            marginOf(tester),
            expectedMargin,
            reason: 'the snackbar margin followed the system inset ($inset)',
          );

          // The FRAMEWORK geometry may — and must — change with the inset.
          final Rect surface = tester.getRect(find.byType(Scaffold));
          final Rect slot = tester.getRect(snackFinder());
          expect(
            slot.bottom,
            closeTo(surface.bottom - inset, 0.01),
            reason: 'the framework base did not follow inset $inset',
          );
        }
      },
    );

    testWidgets(
      'C — beside the bottom bar: bar owns the boundary, gap stays 16',
      (tester) async {
        addTearDown(tester.view.reset);
        setWindowInsets(tester, systemBottom: 48);

        late BuildContext bodyCtx;
        await pumpHost(
          tester,
          Scaffold(
            body: Builder(
              builder: (context) {
                bodyCtx = context;
                return const SizedBox.expand();
              },
            ),
            bottomNavigationBar: BottomActionBar(
              primary: BottomBarAction(label: 'Kirim', onPressed: () {}),
            ),
          ),
        );

        AppSnackBar.showSuccess(bodyCtx, 'saved');
        await tester.pumpAndSettle();

        expect(marginOf(tester), expectedMargin);

        final Rect bar = tester.getRect(find.byType(BottomActionBar));
        final Rect slot = tester.getRect(snackFinder());
        final Rect toast = tester.getRect(toastFinder());

        // The framework slot sits exactly on the bar's top boundary —
        // no body-side second reservation beside the bar.
        expect(
          slot.bottom,
          closeTo(bar.top, 0.01),
          reason: 'the snackbar slot left the framework boundary',
        );
        // The ONLY space between boundary and toast is the 16 px visual gap.
        // (A resurrected 106 px reservation would read ≈122 here.)
        expect(
          toast.bottom,
          closeTo(bar.top - AppMetrics.p16, 0.01),
          reason:
              'the visible gap above the bar is ${bar.top - toast.bottom} px, '
              'not the 16 px design gap',
        );
      },
    );

    testWidgets('D — keyboard 300: framework lifts once, snackbar adds 16', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      setWindowInsets(tester, systemBottom: 24, keyboard: 300);

      // Context ABOVE the Scaffold: this one SEES viewInsets, so the old
      // mechanism would have stacked a full keyboard height here.
      late BuildContext aboveCtx;
      await pumpHost(
        tester,
        Builder(
          builder: (context) {
            aboveCtx = context;
            return const Scaffold(body: SizedBox.expand());
          },
        ),
      );

      AppSnackBar.showError(aboveCtx, 'form invalid');
      await tester.pumpAndSettle();

      expect(marginOf(tester), expectedMargin);

      final Rect surface = tester.getRect(find.byType(Scaffold));
      final Rect slot = tester.getRect(snackFinder());
      final Rect toast = tester.getRect(toastFinder());

      // The framework moved the slot exactly one keyboard height up…
      expect(
        slot.bottom,
        closeTo(surface.bottom - 300, 0.01),
        reason: 'the framework did not take the keyboard',
      );
      // …and the snackbar did NOT add the keyboard again: the visible gap
      // above the framework boundary is the constant 16 px.
      expect(
        toast.bottom,
        closeTo(slot.bottom - AppMetrics.p16, 0.01),
        reason: 'the keyboard height was added a second time by the snackbar',
      );
    });

    testWidgets('E — hidden system nav (inset 0): no stale 106 region', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      setWindowInsets(tester, systemBottom: 0);

      late BuildContext ctx;
      await pumpHost(
        tester,
        Builder(
          builder: (context) {
            ctx = context;
            return const Scaffold(body: SizedBox.expand());
          },
        ),
      );

      AppSnackBar.showWarning(ctx, 'hidden nav');
      await tester.pumpAndSettle();

      expect(marginOf(tester), expectedMargin);

      final Rect surface = tester.getRect(find.byType(Scaffold));
      final Rect slot = tester.getRect(snackFinder());
      final Rect toast = tester.getRect(toastFinder());

      expect(slot.bottom, closeTo(surface.bottom, 0.01));
      expect(
        toast.bottom,
        closeTo(surface.bottom - AppMetrics.p16, 0.01),
        reason:
            'a stale inset-sized gap '
            '(${surface.bottom - toast.bottom} px) survived the hidden '
            'system bar',
      );
    });

    testWidgets(
      'F — same gap from body, above-Scaffold and keyboard contexts',
      (tester) async {
        addTearDown(tester.view.reset);
        setWindowInsets(tester, systemBottom: 24);

        late BuildContext aboveCtx;
        late BuildContext bodyCtx;
        await pumpHost(
          tester,
          Builder(
            builder: (outer) {
              aboveCtx = outer;
              return Scaffold(
                body: Builder(
                  builder: (inner) {
                    bodyCtx = inner;
                    return const SizedBox.expand();
                  },
                ),
              );
            },
          ),
        );

        Future<({EdgeInsets margin, double slotBottom})> showFrom(
          BuildContext c,
        ) async {
          AppSnackBar.showInfo(c, 'ctx');
          await tester.pumpAndSettle();
          return (
            margin: marginOf(tester),
            slotBottom: tester.getRect(snackFinder()).bottom,
          );
        }

        final fromBody = await showFrom(bodyCtx);
        final fromAbove = await showFrom(aboveCtx);

        expect(fromBody.margin, expectedMargin);
        expect(fromAbove.margin, expectedMargin);
        expect(
          fromAbove.slotBottom,
          closeTo(fromBody.slotBottom, 0.01),
          reason: 'the trigger context changed the snackbar geometry',
        );

        // Keyboard-prone context: the above-Scaffold context sees the raw
        // viewInsets — the margin must still be the constant gap.
        setWindowInsets(tester, systemBottom: 24, keyboard: 300);
        final fromKeyboard = await showFrom(aboveCtx);
        expect(fromKeyboard.margin, expectedMargin);
      },
    );

    test('the snackbar source carries no inset arithmetic', () {
      final String source = File(
        'lib/shared/widgets/app_snackbar.dart',
      ).readAsStringSync().replaceAll('\r\n', '\n');

      for (final String banned in const <String>[
        '106',
        'bottomInset',
        'bottomPadding',
        'viewInsets',
        'viewPadding',
        'MediaQuery',
      ]) {
        expect(
          source,
          isNot(contains(banned)),
          reason:
              '`$banned` re-introduced a snackbar-owned system-inset '
              'calculation — the Scaffold owns positioning, the snackbar owns '
              'only the 16 px visual gap',
        );
      }
      expect(
        source,
        isNot(RegExp(r'padding\s*\.\s*bottom')),
        reason: 'the snackbar must not read system bottom padding',
      );
      // The design gap itself stays token-driven.
      expect(source, contains('AppMetrics.p16'));
    });
  });

  group('delivery authority + negative proof', () {
    test('no raw ScaffoldMessenger/SnackBar exists outside AppSnackBar', () {
      final violations = <String>[];
      for (final file in _libDartFiles()) {
        final path = file.path.replaceAll('\\', '/');
        if (path.endsWith('shared/widgets/app_snackbar.dart')) continue;
        final src = _stripComments(file.readAsStringSync());
        if (src.contains('ScaffoldMessenger')) {
          violations.add('$path: ScaffoldMessenger');
        }
        if (RegExp(r'(^|[^A-Za-z0-9_])SnackBar\s*\(').hasMatch(src)) {
          violations.add('$path: bare SnackBar(');
        }
      }
      expect(violations, isEmpty, reason: violations.join('\n'));
    });

    test(
      'no obsolete delivery-wrapper / navigation stub references remain',
      () {
        final violations = <String>[];
        for (final file in _libDartFiles()) {
          final path = file.path.replaceAll('\\', '/');
          final src = _stripComments(file.readAsStringSync());
          for (final banned in const [
            'ErrorSnackBar',
            'SuccessSnackBar',
            'showErrorSnackBar',
            'showSuccessSnackBar',
          ]) {
            if (src.contains(banned)) violations.add('$path: $banned');
          }
        }
        expect(violations, isEmpty, reason: violations.join('\n'));
      },
    );

    test('navigation authority no longer declares a Snackbar method', () {
      final nav = _stripComments(
        File('lib/core/navigation/navigation_handler.dart').readAsStringSync(),
      );
      expect(nav.contains('showSnackBar'), isFalse);
      final router = _stripComments(
        File('lib/core/src/router/app_router.dart').readAsStringSync(),
      );
      expect(router.contains('void showSnackBar'), isFalse);
    });

    test('no divergent global snackBarTheme and no auto-Close action', () {
      final theme = _stripComments(
        File('lib/core/src/theme/app_theme.dart').readAsStringSync(),
      );
      expect(theme.contains('snackBarTheme'), isFalse);

      final source = _stripComments(
        File('lib/shared/widgets/app_snackbar.dart').readAsStringSync(),
      );
      // The old behaviour auto-attached `SnackBarAction(label: 'Close')` for
      // any duration > 3s. That is gone.
      expect(source.contains("label: 'Close'"), isFalse);
      expect(source.contains("'Close'"), isFalse);
    });

    test('context extension no longer forks a Snackbar surface', () {
      final ext = _stripComments(
        File(
          'lib/core/src/utils/extensions/context_extensions.dart',
        ).readAsStringSync(),
      );
      expect(ext.contains('AppSnackBar'), isFalse);
      expect(ext.contains('showErrorSnackBar'), isFalse);
    });
  });

  group('auth-gate authority (closure)', () {
    /// Every balanced `AppSnackBar.show…( … )` call in [src].
    List<String> showCalls(String src) {
      final blocks = <String>[];
      var from = 0;
      while (true) {
        final idx = src.indexOf('AppSnackBar.show', from);
        if (idx < 0) break;
        final open = src.indexOf('(', idx);
        if (open < 0) break;
        var depth = 0;
        var end = open;
        for (; end < src.length; end++) {
          final ch = src[end];
          if (ch == '(') depth++;
          if (ch == ')') {
            depth--;
            if (depth == 0) break;
          }
        }
        blocks.add(src.substring(idx, end + 1));
        from = idx + 1;
      }
      return blocks;
    }

    const loginPhrases = <String>[
      'Silakan masuk',
      'silakan masuk',
      'Please login',
      'Please log in',
      'login terlebih',
      'harus login',
      'belum masuk',
      'must be logged in',
      'You must login',
      'masuk untuk',
      'sign in again',
    ];

    test('no login-required Snackbar remains anywhere in lib', () {
      final violations = <String>[];
      for (final file in _libDartFiles()) {
        final src = _stripComments(file.readAsStringSync());
        for (final call in showCalls(src)) {
          for (final phrase in loginPhrases) {
            if (call.contains(phrase)) {
              violations.add('${file.path}: $phrase');
            }
          }
        }
      }
      expect(violations, isEmpty, reason: violations.join('\n'));
    });

    test('migrated auth gates use the canonical sign-in navigation', () {
      const migratedGates = <String>[
        'lib/domains/social/comment/presentation/widgets/comment_card.dart',
        'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_saved_item_action_button.dart',
        'lib/domains/user/profile/presentation/widgets/settings_support_section.dart',
        'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
        'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart',
        'lib/domains/social/share/presentation/widgets/share_as_post_dialog.dart',
        'lib/domains/user/preference/seller/presentation/screens/seller_renewal_screen.dart',
        'lib/domains/user/preference/seller/presentation/screens/seller_upgrade_wizard_screen.dart',
        'lib/domains/user/profile/presentation/screens/security_screen.dart',
      ];
      final missing = <String>[];
      for (final path in migratedGates) {
        final src = _stripComments(File(path).readAsStringSync());
        if (!src.contains('navigateToSignIn()')) missing.add(path);
      }
      expect(missing, isEmpty, reason: missing.join('\n'));
    });
  });
}
