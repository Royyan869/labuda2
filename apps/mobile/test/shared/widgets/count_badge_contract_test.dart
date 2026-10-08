import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/shared/widgets/count_badge.dart';

// ============================================================================
// COUNT BADGE — SINGLE RENDERER CONTRACT (dedupe cleanup).
//
// The three app-bar badges (chat / notification / saved item) each carried a
// verbatim copy of the same ~40-line Stack. They now delegate layout to ONE
// authority, [CountBadgeOverlay], keeping only their domain count seam.
//
// This locks three things:
// 1. The digit renders the ENTHRINED role `labelMicro` (the badge tail's
//    convergence — `s9` is dead) with the renderer's own weight/height;
// 2. zero renders no badge, and the 99+ cap holds;
// 3. NO badge widget can re-grow a local copy of the renderer (source sweep —
//    the resurrection path for this dedupe is a paste, not an import).
// ============================================================================

void main() {
  testWidgets('the digit is the labelMicro role, not a size token', (
    tester,
  ) async {
    late AppTypeRoles roles;
    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.lightTheme,
        home: Builder(
          builder: (context) {
            roles = context.typeRoles;
            return Scaffold(
              body: Align(
                alignment: Alignment.topLeft,
                child: CountBadgeOverlay(
                  count: 7,
                  child: const SizedBox(width: 24, height: 24),
                ),
              ),
            );
          },
        ),
      ),
    );

    final role = roles.labelMicro;
    expect(role.fontSize, isNotNull, reason: 'geometry must be resolved');

    final digit = tester.widget<Text>(find.text('7')).style!;
    expect(digit.fontSize, role.fontSize, reason: 'size comes from the role');
    expect(digit.height, 1.1, reason: "call site's line height");
    expect(digit.fontWeight, FontWeight.w600, reason: "call site's weight");
  });

  testWidgets('zero renders no badge; counts above 99 render 99+', (
    tester,
  ) async {
    Widget overlay(int count) => MaterialApp(
      theme: AppTheme.lightTheme,
      home: Scaffold(
        body: Align(
          alignment: Alignment.topLeft,
          child: CountBadgeOverlay(
            count: count,
            child: const SizedBox(width: 24, height: 24),
          ),
        ),
      ),
    );

    await tester.pumpWidget(overlay(0));
    expect(find.byType(Text), findsNothing, reason: 'zero = plain child');

    await tester.pumpWidget(overlay(123));
    expect(find.text('99+'), findsOneWidget);
  });

  test('the three badges delegate; a local renderer stays dead', () {
    const badgePaths = [
      'lib/domains/chat/chat/presentation/widgets/chat_badge_widget.dart',
      'lib/domains/system/notification/presentation/widgets/'
          'notification_badge_widget.dart',
      'lib/domains/user/preference/saved_item/widgets/'
          'saved_item_badge_widget.dart',
    ];
    for (final path in badgePaths) {
      final source = File(path).readAsStringSync();
      expect(
        source.contains('CountBadgeOverlay'),
        isTrue,
        reason: '$path must delegate to the one renderer',
      );
      expect(
        source.contains('Positioned('),
        isFalse,
        reason: '$path must not re-grow a local copy of the layout',
      );
      expect(
        source.contains('fontSize:'),
        isFalse,
        reason: '$path must not state a size (the badge tail stays converged)',
      );
    }

    // The one renderer is the only place that knows the digit style.
    final renderer = File(
      'lib/shared/widgets/count_badge.dart',
    ).readAsStringSync();
    expect(renderer.contains('labelMicro'), isTrue);
    expect(
      renderer.contains('fontSize:'),
      isFalse,
      reason:
          'the one renderer reads a ROLE; a raw size literal here is a second '
          'type authority growing back',
    );
  });
}
