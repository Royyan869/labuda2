import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/widgets/page_error_state.dart';

/// CANONICAL contract for the page-level error foundation.
///
/// Locks four invariants:
/// 1. Rendering: error icon + localized title + safe localized message, and a
///    retry action only when a callback exists.
/// 2. Retry: the callback fires once, the action is disabled while the retry
///    future is in flight, concurrent taps cannot start a second retry, and
///    the action re-enables when the callback settles.
/// 3. Safety: the surface renders ONLY the three localized strings — no raw
///    exception/technical text can appear through it.
/// 4. Accessibility: the state is a semantic container with a localized
///    retry label discoverable by screen readers.

const _title = 'Terjadi Kesalahan';
const _message = 'Data belum bisa dimuat. Silakan coba lagi.';
const _retryLabel = 'Coba Lagi';

Future<void> _pump(
  WidgetTester tester, {
  FutureOr<void> Function()? onRetry,
}) {
  return tester.pumpWidget(
    MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('id'),
      home: Scaffold(body: PageErrorState(onRetry: onRetry)),
    ),
  );
}

void main() {
  testWidgets('renders error icon, localized title and safe message', (
    tester,
  ) async {
    await _pump(tester, onRetry: () async {});

    expect(find.byIcon(Icons.error_outline), findsOneWidget);
    expect(find.text(_title), findsOneWidget);
    expect(find.text(_message), findsOneWidget);
  });

  testWidgets('shows the retry action only when a callback is provided', (
    tester,
  ) async {
    await _pump(tester);
    expect(find.widgetWithText(ElevatedButton, _retryLabel), findsNothing);

    await _pump(tester, onRetry: () async {});
    expect(find.widgetWithText(ElevatedButton, _retryLabel), findsOneWidget);
  });

  testWidgets(
    'retry fires once, stays disabled in flight, blocks concurrent taps',
    (tester) async {
      var calls = 0;
      final completer = Completer<void>();
      Future<void> onRetry() {
        calls++;
        return completer.future;
      }

      await _pump(tester, onRetry: onRetry);
      final button = find.widgetWithText(ElevatedButton, _retryLabel);

      expect(tester.widget<ElevatedButton>(button).onPressed, isNotNull);

      await tester.tap(button);
      await tester.pump();
      expect(calls, 1);

      // In flight: action disabled, repeated taps start no second retry.
      expect(tester.widget<ElevatedButton>(button).onPressed, isNull);
      await tester.tap(button, warnIfMissed: false);
      await tester.pump();
      expect(calls, 1);

      completer.complete();
      await tester.pump();
      await tester.pump();

      // Settled: action enabled again.
      expect(tester.widget<ElevatedButton>(button).onPressed, isNotNull);
      await tester.tap(button);
      await tester.pump();
      expect(calls, 2);
    },
  );

  testWidgets('renders only safe localized copy — no technical error text', (
    tester,
  ) async {
    await _pump(tester, onRetry: () async {});

    final rendered = tester
        .widgetList<Text>(find.byType(Text))
        .map((t) => t.data ?? '')
        .toList();

    expect(rendered, containsAll(<String>[_title, _message, _retryLabel]));
    expect(rendered, hasLength(3), reason: 'foundation renders copy only');

    final forbidden = RegExp(
      r'Exception|Error:|StateError|toString|#\d+|stack|boom',
      caseSensitive: false,
    );
    for (final text in rendered) {
      expect(
        forbidden.hasMatch(text),
        isFalse,
        reason: 'technical error text leaked into UI: $text',
      );
    }
  });

  testWidgets('exposes basic semantics with a localized retry label', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();

    await _pump(tester, onRetry: () async {});

    // The semantic nodes the screen reader actually sees, in traversal order.
    final nodes = tester.semantics.simulatedAccessibilityTraversal().toList();

    // The state announces the localized title and safe message together.
    final stateLabels = nodes
        .map((n) => n.label)
        .where((label) => label.contains(_title) || label.contains(_message))
        .toList();
    expect(stateLabels, hasLength(1));
    expect(stateLabels.single, contains(_title));
    expect(stateLabels.single, contains(_message));

    // The retry action is a semantic button with an accessible, localized
    // label.
    final retryNodes = nodes.where((n) => n.label == _retryLabel).toList();
    expect(retryNodes, hasLength(1));
    expect(
      retryNodes.single.getSemanticsData().flagsCollection.isButton,
      isTrue,
    );

    handle.dispose();
  });
}
