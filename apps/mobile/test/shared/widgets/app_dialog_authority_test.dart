// AppDialog authority contract.
//
// CANONICAL TRUTH: `lib/shared/widgets/app_dialog.dart` is the ONE compositor
// for the app's common dialog grammar — a confirmation decision ([AppDialog.confirm])
// and an acknowledge-only notice ([AppDialog.info]). Flutter's `AlertDialog`
// stays the shell (action geometry, overflow, insets, semantics); AppDialog owns
// only the common contract: the action grammar, the confirming tone and the
// result contract (a non-nullable `bool`, dismissal == cancel).
//
// This file locks three things:
//  1. POSITIVE: the authority resolves the decision, maps a dismissed barrier to
//     `false`, and paints the destructive intent with `scheme.error`.
//  2. NEGATIVE: the two dead competing authorities stay dead, the migrated
//     slice carries no raw dialog, and a body-wide ratchet keeps raw
//     `showDialog<bool>(` from growing.
//  3. Both detectors actually fire (negative proof).
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/widgets/app_dialog.dart';

/// Files migrated onto the authority in slice 1. They must never host a raw
/// dialog again.
const _migratedSlice = <String>[
  'lib/domains/commerce/pricing/discount/presentation/screens/'
      'seller_discount_list_screen.dart',
  'lib/domains/commerce/pricing/discount/presentation/screens/'
      'create_discount_screen.dart',
  'lib/domains/commerce/pricing/discount/presentation/screens/'
      'edit_discount_screen.dart',
  'lib/domains/commerce/pricing/promotion/presentation/screens/'
      'canonical_promotion_list_screen.dart',
  // A local private confirm helper folded into the authority in the same slice.
  'lib/domains/user/identity/authentication/presentation/screens/'
      'login_sessions_screen.dart',
];

/// The authority itself is allowed to call `showDialog<bool>` — that is the ONE
/// implementation. Every other call is a competing raw confirmation.
const _authorityPath = 'lib/shared/widgets/app_dialog.dart';

/// The ratchet count measured after slice 4: 20 raw `showDialog<bool>(` remain
/// outside the authority (28 before slice 1, 24 after slice 1, 22 after the
/// chat confirmations, 20 after the auction cancel confirmations). This cap
/// may only go DOWN; a new raw confirmation turns the gate red.
const _rawConfirmCap = 20;

/// The raw confirmation route the authority replaced.
final _rawConfirm = RegExp(r'showDialog\s*<\s*bool\s*>');

int _rawConfirmCountIn(String source) => _rawConfirm.allMatches(source).length;

List<String> _libDartFiles() => Directory('lib')
    .listSync(recursive: true)
    .whereType<File>()
    .where((f) => f.path.endsWith('.dart'))
    .map((f) => f.path.replaceAll(r'\', '/'))
    .toList();

Widget _host(Future<void> Function(BuildContext) onOpen) => MaterialApp(
  theme: AppTheme.lightTheme,
  home: Builder(
    builder: (context) => Scaffold(
      body: Center(
        child: ElevatedButton(
          onPressed: () => onOpen(context),
          child: const Text('open'),
        ),
      ),
    ),
  ),
);

void main() {
  group('AppDialog positive proof', () {
    testWidgets('confirm resolves true on the confirming action', (
      tester,
    ) async {
      bool? result;
      await tester.pumpWidget(
        _host((context) async {
          result = await AppDialog.confirm(
            context: context,
            title: 'Judul',
            message: 'Pesan',
            confirmLabel: 'Ya',
            cancelLabel: 'Tidak',
          );
        }),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.text('Judul'), findsOneWidget);
      expect(find.text('Pesan'), findsOneWidget);
      expect(find.text('Tidak'), findsOneWidget);
      await tester.tap(find.text('Ya'));
      await tester.pumpAndSettle();
      expect(result, isTrue);
      expect(find.byType(AlertDialog), findsNothing);
    });

    testWidgets('cancel resolves false (never null)', (tester) async {
      bool? result;
      await tester.pumpWidget(
        _host((context) async {
          result = await AppDialog.confirm(
            context: context,
            title: 'Judul',
            message: 'Pesan',
            confirmLabel: 'Ya',
            cancelLabel: 'Tidak',
          );
        }),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Tidak'));
      await tester.pumpAndSettle();
      expect(result, isFalse);
    });

    testWidgets('a dismissed barrier resolves false', (tester) async {
      bool? result;
      await tester.pumpWidget(
        _host((context) async {
          result = await AppDialog.confirm(
            context: context,
            title: 'Judul',
            message: 'Pesan',
          );
        }),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      await tester.tapAt(const Offset(5, 5));
      await tester.pumpAndSettle();
      expect(result, isFalse);
      expect(find.byType(AlertDialog), findsNothing);
    });

    testWidgets('neutral leaves the confirming fill to the button theme', (
      tester,
    ) async {
      await tester.pumpWidget(
        _host((context) async {
          await AppDialog.confirm(
            context: context,
            title: 'Judul',
            message: 'Pesan',
            confirmLabel: 'Ya',
          );
        }),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      final button = tester.widget<ElevatedButton>(
        find.widgetWithText(ElevatedButton, 'Ya'),
      );
      expect(
        button.style,
        isNull,
        reason: 'the neutral confirming action must inherit the button theme',
      );
    });

    testWidgets('destructive paints the confirming action with scheme.error', (
      tester,
    ) async {
      await tester.pumpWidget(
        _host((context) async {
          await AppDialog.confirm(
            context: context,
            title: 'Hapus',
            message: 'Yakin?',
            confirmLabel: 'Hapus',
            cancelLabel: 'Batal',
            intent: AppDialogIntent.destructive,
          );
        }),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      final scheme = Theme.of(
        tester.element(find.byType(AlertDialog)),
      ).colorScheme;
      final button = tester.widget<ElevatedButton>(
        find.widgetWithText(ElevatedButton, 'Hapus'),
      );
      expect(
        button.style?.backgroundColor?.resolve(const <WidgetState>{}),
        scheme.error,
      );
      expect(
        button.style?.foregroundColor?.resolve(const <WidgetState>{}),
        scheme.onError,
      );
    });

    testWidgets('info closes through its single action', (tester) async {
      await tester.pumpWidget(
        _host((context) async {
          await AppDialog.info(
            context: context,
            title: 'Info',
            message: 'Keterangan',
          );
        }),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.text('Info'), findsOneWidget);
      expect(find.text('Keterangan'), findsOneWidget);
      await tester.tap(find.text('OK'));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
    });

    testWidgets('info can be dismissed through the barrier', (tester) async {
      await tester.pumpWidget(
        _host((context) async {
          await AppDialog.info(
            context: context,
            title: 'Info',
            message: 'Keterangan',
          );
        }),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.text('Info'), findsOneWidget);
      await tester.tapAt(const Offset(5, 5));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
    });

    testWidgets('info accepts custom content', (tester) async {
      await tester.pumpWidget(
        _host((context) async {
          await AppDialog.info(
            context: context,
            title: 'Info',
            content: const Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [Text('Bagian A'), Text('Bagian B')],
            ),
          );
        }),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.text('Bagian A'), findsOneWidget);
      expect(find.text('Bagian B'), findsOneWidget);
      await tester.tap(find.text('OK'));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
    });

    testWidgets('info bounds long content and scrolls internally', (
      tester,
    ) async {
      tester.view.physicalSize = const Size(400, 640);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);

      await tester.pumpWidget(
        _host((context) async {
          await AppDialog.info(
            context: context,
            title: 'Long Info',
            content: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: List<Widget>.generate(
                80,
                (i) => Text('Information line $i'),
              ),
            ),
          );
        }),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      expect(find.text('Long Info'), findsOneWidget);
      // No RenderFlex/layout overflow was thrown on a long payload.
      expect(tester.takeException(), isNull);
      // The framework bounded-scroll contract is active inside the dialog.
      expect(
        find.descendant(
          of: find.byType(AlertDialog),
          matching: find.byType(Scrollable),
        ),
        findsOneWidget,
      );
      // Still a single-action, dismissible information surface.
      await tester.tap(find.text('OK'));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
    });

    testWidgets('confirm bounds long content and keeps actions usable', (
      tester,
    ) async {
      tester.view.physicalSize = const Size(400, 640);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);

      bool? result;
      await tester.pumpWidget(
        _host((context) async {
          result = await AppDialog.confirm(
            context: context,
            title: 'Long Confirm',
            content: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: List<Widget>.generate(
                80,
                (i) => Text('Confirmation line $i'),
              ),
            ),
            confirmLabel: 'Ya',
            cancelLabel: 'Tidak',
          );
        }),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      expect(find.text('Long Confirm'), findsOneWidget);
      // No RenderFlex/layout overflow was thrown on a long payload.
      expect(tester.takeException(), isNull);
      // The framework bounded-scroll contract is active inside the dialog.
      expect(
        find.descendant(
          of: find.byType(AlertDialog),
          matching: find.byType(Scrollable),
        ),
        findsOneWidget,
      );
      // Both actions stay pinned and usable on the bounded dialog.
      expect(find.text('Tidak'), findsOneWidget);
      await tester.tap(find.text('Ya'));
      await tester.pumpAndSettle();
      expect(result, isTrue);
      expect(find.byType(AlertDialog), findsNothing);
    });

    testWidgets('the long-content probe detects a non-scrollable overflow', (
      tester,
    ) async {
      tester.view.physicalSize = const Size(400, 640);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);

      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.lightTheme,
          home: Builder(
            builder: (context) => Scaffold(
              body: Center(
                child: ElevatedButton(
                  onPressed: () => showDialog<void>(
                    context: context,
                    builder: (dialogContext) => AlertDialog(
                      title: const Text('Long Info'),
                      content: Column(
                        mainAxisSize: MainAxisSize.min,
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: List<Widget>.generate(
                          80,
                          (i) => Text('Information line $i'),
                        ),
                      ),
                      actions: [
                        TextButton(
                          onPressed: () => Navigator.of(dialogContext).pop(),
                          child: const Text('OK'),
                        ),
                      ],
                    ),
                  ),
                  child: const Text('open'),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      // The probe is sensitive: without `scrollable: true` the same payload
      // overflows. This proves the long-content test above is non-vacuous.
      expect(tester.takeException(), isA<FlutterError>());
    });
  });

  group('AppDialog authority negative gate', () {
    test('the authority owns no colour/type/geometry decision', () {
      final source = File(_authorityPath).readAsStringSync();
      // The shell is Flutter's AlertDialog; surface/shape/inset/padding/type
      // belong to `dialogTheme` + the resolved TextTheme, never to this file.
      for (final forbidden in <String>[
        'AppType',
        'fontSize:',
        'Color(0x',
        'Colors.',
        'Radius.circular(',
        'BorderRadius',
        'contentPadding:',
        'insetPadding:',
        'shape:',
        'elevation:',
      ]) {
        expect(
          source.contains(forbidden),
          isFalse,
          reason: 'the dialog authority must not restate $forbidden',
        );
      }
    });

    test('both info and confirm keep the framework bounded-scroll contract',
        () {
      final source = File(_authorityPath).readAsStringSync();
      // Exactly TWO opt-ins: AppDialog.info and AppDialog.confirm share the
      // same bounded-scroll principle. Removing either one turns this gate
      // red — a dialog content area must not overflow merely because caller
      // content is taller than the viewport.
      final scrollableCount = RegExp(
        r'scrollable\s*:\s*true',
      ).allMatches(source).length;
      expect(
        scrollableCount,
        2,
        reason:
            'AppDialog.info and AppDialog.confirm must both opt into bounded '
            'scrolling; removing either reintroduces dialog overflow',
      );
      // The long-content safety is the framework's, not a local workaround.
      for (final forbidden in <String>[
        'SingleChildScrollView',
        'ConstrainedBox',
        'maxHeight:',
      ]) {
        expect(
          source.contains(forbidden),
          isFalse,
          reason:
              'AppDialog must not host a consumer-side overflow workaround '
              '($forbidden)',
        );
      }
    });

    test('no second Page Info / Help component is introduced', () {
      for (final file in _libDartFiles()) {
        final source = File(file).readAsStringSync();
        for (final name in const [
          'AppInfoDialog',
          'PageInfoDialog',
          'AppInfoSurface',
          'PageInfoSurface',
          'AppHelpDialog',
        ]) {
          expect(
            RegExp('\\bclass\\s+$name\\b').hasMatch(source),
            isFalse,
            reason: '$file introduces a second information authority ($name)',
          );
        }
      }
    });

    test('the dead AppModal authority stays deleted and unexported', () {
      final path = <String>['lib/shared/widgets/app_', 'modal.dart'].join();
      expect(
        File(path).existsSync(),
        isFalse,
        reason: 'AppModal was a zero-consumer competing dialog authority',
      );
      final barrel = File('lib/shared/shared.dart').readAsStringSync();
      expect(
        barrel.contains(<String>['widgets/app_', 'modal.dart'].join()),
        isFalse,
        reason: 'the barrel must not resurrect the dead modal',
      );
      expect(barrel.contains('widgets/app_dialog.dart'), isTrue);
    });

    test('the dead context-extension dialog helpers stay deleted', () {
      final source = File(
        'lib/core/src/utils/extensions/context_extensions.dart',
      ).readAsStringSync();
      for (final dead in <String>[
        'showAlertDialog',
        'showConfirmDialog',
        'showLoadingDialog',
        'hideLoadingDialog',
      ]) {
        expect(
          source.contains(dead),
          isFalse,
          reason: '$dead was a zero-consumer second dialog authority',
        );
      }
    });

    test('the migrated slice carries no raw dialog', () {
      for (final path in _migratedSlice) {
        final source = File(path).readAsStringSync();
        expect(
          source.contains('AlertDialog('),
          isFalse,
          reason: '$path must consume AppDialog, not a raw AlertDialog',
        );
        expect(
          _rawConfirmCountIn(source),
          0,
          reason: '$path must not build a raw confirmation route',
        );
      }
    });

    test('raw showDialog<bool>( never grows (ratchet)', () {
      var total = 0;
      for (final path in _libDartFiles()) {
        if (path == _authorityPath) continue;
        total += _rawConfirmCountIn(File(path).readAsStringSync());
      }
      expect(
        total,
        lessThanOrEqualTo(_rawConfirmCap),
        reason:
            'a new raw confirmation appeared — route it through '
            'AppDialog.confirm instead of calling showDialog<bool> directly',
      );
    });

    test('the ratchet detector can actually fail (negative proof)', () {
      expect(
        _rawConfirmCountIn('final ok = await showDialog<bool>('),
        1,
        reason: 'the detector must catch the raw confirmation route',
      );
      expect(
        _rawConfirmCountIn(
          'final ok = await AppDialog.confirm(context: context, title: x);',
        ),
        0,
        reason: 'the canonical call is not a raw dialog',
      );
    });
  });
}
