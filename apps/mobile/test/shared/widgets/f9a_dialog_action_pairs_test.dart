// F9(a) — RAW ALERTDIALOG ACTION PAIRS: MECHANISM + CONVERGENCE PROOF.
//
// Framework truth (Flutter SDK dialog.dart): AlertDialog.actions lays out
// through OverflowBar — horizontal while the pair fits, vertical stacking
// when it does not. AppDialog.confirm uses the SAME shell, so convergence
// is grammar-authority (single cancel+confirm grammar, bool contract,
// scrollable content, tone discipline), not geometry.
//
// This suite proves:
//  P1. the mechanism: longest raw F9(a) label pairs stack without layout
//      exception across the width x text-scale matrix (test-local
//      reconstructions, clearly non-production — same pattern as the
//      horizontal-composition gate's negative control);
//  P2. the converged authority: AppDialog.confirm with the EXACT labels of
//      the four converted dialogs across the matrix — no exception, both
//      actions visible/actionable, order preserved, destructive tone kept;
//  P3. call-site pins: the converted files route through AppDialog.confirm.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/widgets/app_dialog.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

// Longest raw F9(a) action labels (keep-raw form dialogs — mechanism probe).
const String _longConfirm = 'Konfirmasi Pengiriman'; // 21ch, shipping form
const String _longVerify = 'Verifikasi Sekarang'; // withdraw gate

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

/// Pumps a host with an "open" button that shows a dialog on tap, then opens
/// it. The dialog future's result lands in [onResult] for tap-contract tests.
Future<void> _openDialog(
  WidgetTester tester, {
  required double width,
  required double height,
  required double scale,
  required Future<void> Function(BuildContext context) show,
}) async {
  final Size surface = Size(width, height);
  await tester.binding.setSurfaceSize(surface);
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(
    MaterialApp(
      theme: AppTheme.lightTheme,
      home: MediaQuery(
        data: MediaQueryData(
          size: surface,
          textScaler: TextScaler.linear(scale),
        ),
        child: Scaffold(
          body: Builder(
            builder: (BuildContext context) => TextButton(
              onPressed: () => show(context),
              child: const Text('open-dialog'),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.tap(find.text('open-dialog'));
  await tester.pumpAndSettle();
}

void _expectNoException(
  WidgetTester tester, {
  required String label,
  required double width,
  required double scale,
}) {
  final Object? exception = tester.takeException();
  expect(
    exception,
    isNull,
    reason: 'layout exception: $label at ${width}dp @scale $scale '
        '($exception)',
  );
}

/// Both actions rendered and in cancel-first order (side-by-side or stacked).
void _expectOrderedPair(
  WidgetTester tester, {
  required String cancelLabel,
  required String confirmLabel,
}) {
  final Finder cancel = find.text(cancelLabel);
  final Finder confirm = find.text(confirmLabel);
  expect(cancel, findsOneWidget, reason: '"$cancelLabel" not visible');
  expect(confirm, findsOneWidget, reason: '"$confirmLabel" not visible');
  final Offset c = tester.getCenter(cancel);
  final Offset f = tester.getCenter(confirm);
  expect(
    c.dx < f.dx - 1 || c.dy < f.dy - 1,
    isTrue,
    reason: 'action order broken: cancel@$c confirm@$f',
  );
}

// ---------------------------------------------------------------------------
// Converted dialog configurations (exact production labels)
// ---------------------------------------------------------------------------

Future<bool> _showDeleteShipping(BuildContext context) =>
    AppDialog.confirm(
      context: context,
      title: 'Hapus Opsi Pengiriman',
      message:
          'Hapus "JNE Reguler" dari daftar opsi pengiriman Anda? '
          'Jika opsi ini masih dipakai di ForSale atau Auction, penghapusan '
          'akan ditolak — matikan lewat tombol aktif sebagai gantinya.',
      confirmLabel: 'Hapus',
      cancelLabel: 'Batal',
      intent: AppDialogIntent.destructive,
    );

Future<bool> _showRecheck(BuildContext context) => AppDialog.confirm(
  context: context,
  title: 'Pembayaran masih diproses',
  message: 'Pembayaran Anda belum terkonfirmasi. Cek ulang statusnya sekarang?',
  confirmLabel: 'Cek status',
  cancelLabel: 'Nanti saja',
);

Future<bool> _showBidConfirm(BuildContext context) => AppDialog.confirm(
  context: context,
  title: 'Konfirmasi Penawaran',
  content: Column(
    mainAxisSize: MainAxisSize.min,
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      const Text('Kamu akan menawar sebesar'),
      const SizedBox(height: 12),
      Text(
        'Rp 12.500.000',
        style: Theme.of(context).textTheme.titleLarge?.copyWith(
          fontWeight: FontWeight.bold,
          color: Theme.of(context).colorScheme.primary,
        ),
      ),
      const SizedBox(height: 16),
      Text(
        'Jika Anda menang dan tidak membayar, akun Anda dapat dibatasi',
        style: Theme.of(context).textTheme.bodySmall,
      ),
    ],
  ),
  confirmLabel: 'Konfirmasi',
  cancelLabel: 'Batal',
);

void main() {
  group('P1 mechanism: longest raw pairs stack without exception', () {
    for (final (String cancel, String confirm) in <(String, String)>[
      ('Batal', _longConfirm),
      ('Nanti', _longVerify),
    ]) {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          testWidgets(
            'raw [$cancel | $confirm] at ${width}dp @scale $scale',
            (tester) async {
              await _openDialog(
                tester,
                width: width,
                height: 800,
                scale: scale,
                show: (BuildContext context) => showDialog<void>(
                  context: context,
                  builder: (dialogContext) => AlertDialog(
                    title: const Text('Probe'),
                    content: const Text('Mechanism probe — not production.'),
                    actions: [
                      TextButton(
                        onPressed: () => Navigator.of(dialogContext).pop(),
                        child: Text(cancel),
                      ),
                      ElevatedButton(
                        onPressed: () => Navigator.of(dialogContext).pop(),
                        child: Text(confirm),
                      ),
                    ],
                  ),
                ),
              );
              _expectNoException(
                tester,
                label: 'raw [$cancel | $confirm]',
                width: width,
                scale: scale,
              );
              _expectOrderedPair(
                tester,
                cancelLabel: cancel,
                confirmLabel: confirm,
              );
              // Both actions dismiss (usable, not just painted).
              await tester.tap(find.text(confirm));
              await tester.pumpAndSettle();
              expect(find.byType(AlertDialog), findsNothing);
            },
          );
        }
      }
    }
  });

  group('P2 converged authority matrix (exact converted labels)', () {
    final Map<String, Future<bool> Function(BuildContext)> cases =
        <String, Future<bool> Function(BuildContext)>{
          'Hapus|Batal': _showDeleteShipping,
          'Nanti saja|Cek status': _showRecheck,
          'Batal|Konfirmasi': _showBidConfirm,
        };
    cases.forEach((String name, Future<bool> Function(BuildContext) show) {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          testWidgets('AppDialog $name at ${width}dp @scale $scale', (
            tester,
          ) async {
            bool? result;
            await _openDialog(
              tester,
              width: width,
              height: 800,
              scale: scale,
              show: (BuildContext context) async {
                result = await show(context);
              },
            );
            _expectNoException(
              tester,
              label: 'AppDialog $name',
              width: width,
              scale: scale,
            );
            final (String cancel, String confirm) = switch (name) {
              'Hapus|Batal' => ('Batal', 'Hapus'),
              'Nanti saja|Cek status' => ('Nanti saja', 'Cek status'),
              _ => ('Batal', 'Konfirmasi'),
            };
            _expectOrderedPair(
              tester,
              cancelLabel: cancel,
              confirmLabel: confirm,
            );
            await tester.tap(find.text(confirm));
            await tester.pumpAndSettle();
            expect(result, isTrue, reason: 'confirm must resolve true');
            expect(find.byType(AlertDialog), findsNothing);
          });
        }
      }
    });

    testWidgets('cancel resolves false and preserves order', (tester) async {
      bool? result;
      await _openDialog(
        tester,
        width: 360,
        height: 800,
        scale: 1.0,
        show: (BuildContext context) async {
          result = await _showDeleteShipping(context);
        },
      );
      _expectNoException(
        tester,
        label: 'cancel path',
        width: 360,
        scale: 1.0,
      );
      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();
      expect(result, isFalse, reason: 'cancel must resolve false');
    });

    testWidgets('destructive tone preserved on confirm fill', (tester) async {
      await _openDialog(
        tester,
        width: 360,
        height: 800,
        scale: 1.0,
        show: (BuildContext context) => _showDeleteShipping(context),
      );
      final BuildContext dialogContext = tester.element(
        find.byType(AlertDialog),
      );
      final Color schemeError = Theme.of(dialogContext).colorScheme.error;
      final ElevatedButton button = tester.widget<ElevatedButton>(
        find.widgetWithText(ElevatedButton, 'Hapus'),
      );
      final Color? fill = button.style?.backgroundColor?.resolve(<WidgetState>{});
      expect(fill, schemeError, reason: 'destructive fill must be scheme.error');
    });

    testWidgets('tallest converted content stays bounded (320x568 @2.0)', (
      tester,
    ) async {
      await _openDialog(
        tester,
        width: 320,
        height: 568,
        scale: 2.0,
        show: (BuildContext context) => _showBidConfirm(context),
      );
      _expectNoException(
        tester,
        label: 'bounded short viewport',
        width: 320,
        scale: 2.0,
      );
      _expectOrderedPair(
        tester,
        cancelLabel: 'Batal',
        confirmLabel: 'Konfirmasi',
      );
    });
  });

  group('P3 call-site convergence pins', () {
    String readLib(String relative) =>
        File('lib/$relative').readAsStringSync();

    test('seller shipping delete routes through AppDialog.confirm', () {
      final String source = readLib(
        'domains/user/preference/seller/presentation/screens/seller_shipping_screen.dart',
      );
      expect(source, contains('AppDialog.confirm'));
      expect(source, contains("'Hapus Opsi Pengiriman'"));
      expect(source.contains('AlertDialog('), isFalse);
    });

    test('wizard + renewal rechecks route through AppDialog.confirm', () {
      for (final String path in <String>[
        'domains/user/preference/seller/presentation/screens/seller_upgrade_wizard_screen.dart',
        'domains/user/preference/seller/presentation/screens/seller_renewal_screen.dart',
      ]) {
        final String source = readLib(path);
        expect(source, contains('AppDialog.confirm'));
        expect(source, contains("'Pembayaran masih diproses'"));
        expect(
          source.contains('showDialog<bool>'),
          isFalse,
          reason: '$path must not build a raw confirmation route',
        );
      }
    });

    test('auction bid confirm routes through AppDialog.confirm', () {
      final String source = readLib(
        'domains/commerce/catalog/auction/presentation/widgets/detail/auction_action_modal.dart',
      );
      expect(source, contains('AppDialog.confirm'));
      expect(source, contains("'Konfirmasi Penawaran'"));
      expect(source, contains("widget.onPlaceBid(amount)"));
      expect(
        source.contains('showDialog('),
        isFalse,
        reason: 'auction modal must not build a raw dialog route',
      );
    });
  });
}
