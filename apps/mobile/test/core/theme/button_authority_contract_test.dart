// BUTTON / ACTION FOUNDATION — canonical disabled-state authority.
//
// Owner decision 2026-10-05 (Decision A): when an action is disabled its ONE
// canonical appearance is a neutral fill (`surfaceContainerHighest`) with
// `onSurfaceVariant` content. A faded brand colour is explicitly NOT the
// disabled language, and no call site may restate a disabled treatment.
//
// Two layers of proof:
//  1. configuration — the four theme styles resolve the owned tokens;
//  2. rendered — a pumped widget actually paints the owned fill/ink, so a
//     future override cannot pass on source alone.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/shared/widgets/auth_button.dart';
import 'package:labuda/shared/widgets/bottom_action_bar.dart';

/// Rendered fill of the first `Material` built by [buttonType].
Color? _renderedFill(WidgetTester tester, Type buttonType) => tester
    .widget<Material>(
      find
          .descendant(
            of: find.byType(buttonType),
            matching: find.byType(Material),
          )
          .first,
    )
    .color;

/// Rendered ink of the label [label] — the paragraph's resolved colour, i.e.
/// what the user actually sees after the button's DefaultTextStyle merge.
Color? _renderedInk(WidgetTester tester, String label) =>
    tester.renderObject<RenderParagraph>(find.text(label)).text.style?.color;

void main() {
  for (final (name, theme) in <(String, ThemeData)>[
    ('light', AppTheme.lightTheme),
    ('dark', AppTheme.darkTheme),
  ]) {
    final scheme = theme.colorScheme;

    group('$name — standard buttons own the disabled language', () {
      test('filled families disable to the one neutral pair', () {
        for (final (family, style) in <(String, ButtonStyle?)>[
          ('elevated', theme.elevatedButtonTheme.style),
          ('filled', theme.filledButtonTheme.style),
        ]) {
          expect(
            style?.backgroundColor?.resolve(const <WidgetState>{
              WidgetState.disabled,
            }),
            scheme.surfaceContainerHighest,
            reason: '$name $family disabled fill is the owned neutral surface',
          );
          expect(
            style?.foregroundColor?.resolve(const <WidgetState>{
              WidgetState.disabled,
            }),
            scheme.onSurfaceVariant,
            reason: '$name $family disabled ink is the owned muted role',
          );
          // Negative proof: the disabled language is never a faded brand tone.
          expect(
            style?.backgroundColor?.resolve(const <WidgetState>{
              WidgetState.disabled,
            }),
            isNot(scheme.primary),
            reason: '$name $family disabled fill must not be brand fill',
          );
          expect(
            style?.backgroundColor?.resolve(const <WidgetState>{
              WidgetState.disabled,
            }),
            isNot(scheme.primary.withValues(alpha: 0.5)),
            reason: '$name $family disabled fill must not be faded primary',
          );
        }
      });

      test('unfilled families mute only their ink when disabled', () {
        for (final (family, style) in <(String, ButtonStyle?)>[
          ('outlined', theme.outlinedButtonTheme.style),
          ('text', theme.textButtonTheme.style),
        ]) {
          expect(
            style?.foregroundColor?.resolve(const <WidgetState>{
              WidgetState.disabled,
            }),
            scheme.onSurfaceVariant,
            reason: '$name $family disabled ink is the owned muted role',
          );
          expect(
            style?.backgroundColor?.resolve(const <WidgetState>{
              WidgetState.disabled,
            }),
            isNot(scheme.primary),
            reason: '$name $family stays unfilled when disabled',
          );
        }
      });

      test('enabled CTA authority is unchanged', () {
        expect(
          theme.elevatedButtonTheme.style?.backgroundColor?.resolve(
            const <WidgetState>{},
          ),
          scheme.primary,
        );
        expect(
          theme.elevatedButtonTheme.style?.foregroundColor?.resolve(
            const <WidgetState>{},
          ),
          scheme.onPrimary,
        );
        expect(
          theme.filledButtonTheme.style?.backgroundColor?.resolve(
            const <WidgetState>{},
          ),
          scheme.primary,
        );
        expect(
          theme.textButtonTheme.style?.foregroundColor?.resolve(
            const <WidgetState>{},
          ),
          scheme.primary,
        );
      });

      testWidgets('rendered disabled buttons paint the owned tokens', (
        tester,
      ) async {
        await tester.pumpWidget(
          MaterialApp(
            theme: theme,
            home: Scaffold(
              body: Column(
                children: [
                  ElevatedButton(
                    onPressed: null,
                    child: const Text('elev'),
                  ),
                  FilledButton(onPressed: null, child: const Text('fill')),
                  OutlinedButton(onPressed: null, child: const Text('out')),
                  TextButton(onPressed: null, child: const Text('text')),
                ],
              ),
            ),
          ),
        );

        expect(
          _renderedFill(tester, ElevatedButton),
          scheme.surfaceContainerHighest,
          reason: '$name elevated disabled fill renders the neutral surface',
        );
        expect(
          _renderedFill(tester, FilledButton),
          scheme.surfaceContainerHighest,
          reason: '$name filled disabled fill renders the neutral surface',
        );
        expect(_renderedInk(tester, 'elev'), scheme.onSurfaceVariant);
        expect(_renderedInk(tester, 'fill'), scheme.onSurfaceVariant);
        expect(_renderedInk(tester, 'out'), scheme.onSurfaceVariant);
        expect(_renderedInk(tester, 'text'), scheme.onSurfaceVariant);
      });

      testWidgets('rendered enabled CTA paints primary / onPrimary', (
        tester,
      ) async {
        await tester.pumpWidget(
          MaterialApp(
            theme: theme,
            home: Scaffold(
              body: ElevatedButton(
                onPressed: () {},
                child: const Text('enabled'),
              ),
            ),
          ),
        );
        expect(_renderedFill(tester, ElevatedButton), scheme.primary);
        expect(_renderedInk(tester, 'enabled'), scheme.onPrimary);
      });

      testWidgets('BottomActionBar loading rides the disabled fill', (
        tester,
      ) async {
        await tester.pumpWidget(
          MaterialApp(
            theme: theme,
            home: Scaffold(
              bottomNavigationBar: BottomActionBar(
                primary: BottomBarAction(
                  label: 'Kirim',
                  onPressed: () {},
                  isLoading: true,
                ),
              ),
            ),
          ),
        );
        expect(_renderedFill(tester, ElevatedButton), scheme.surfaceContainerHighest);
        final spinner = tester.widget<CircularProgressIndicator>(
          find.descendant(
            of: find.byType(ElevatedButton),
            matching: find.byType(CircularProgressIndicator),
          ),
        );
        expect(spinner.valueColor?.value, scheme.onSurfaceVariant);
      });

      testWidgets('AuthButton loading rides the disabled fill', (tester) async {
        await tester.pumpWidget(
          MaterialApp(
            theme: theme,
            home: Scaffold(
              body: AuthButton(text: 'Masuk', isLoading: true),
            ),
          ),
        );
        expect(_renderedFill(tester, ElevatedButton), scheme.surfaceContainerHighest);
        final spinner = tester.widget<CircularProgressIndicator>(
          find.descendant(
            of: find.byType(ElevatedButton),
            matching: find.byType(CircularProgressIndicator),
          ),
        );
        expect(spinner.valueColor?.value, scheme.onSurfaceVariant);
      });
    });
  }

  test('FollowButton disabled language is canonical (no opacity hack)', () {
    final src = File('lib/shared/widgets/follow_button.dart').readAsStringSync();
    expect(
      src.contains('Opacity('),
      isFalse,
      reason: 'a local opacity fade is not the disabled language',
    );
    expect(src.contains('surfaceContainerHighest'), isTrue);
    expect(src.contains('onSurfaceVariant'), isTrue);
  });
}
