// F9(b) PROBE — leading + long CTA bars across the composition matrix.
// Pumps the REAL BottomActionBar primitive configured exactly like each
// production caller, plus faithful reconstructions (labeled, non-production)
// for custom (non-primitive) bars. Records Hurt: exception, clipped label,
// dead action.
//
// Matrix: 320 / 360 / 412 / 500 x text scale 1.0 / 1.3 / 2.0.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/bottom_action_bar.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

Future<void> _pumpBar(
  WidgetTester tester, {
  required double width,
  required double scale,
  required Widget subject,
}) async {
  final Size surface = Size(width, 800);
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
          body: const SingleChildScrollView(
            child: Text('content above the bar'),
          ),
          bottomNavigationBar: subject,
        ),
      ),
    ),
  );
  await tester.pump();
}

void _expectClean(
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

/// Every character of [label] must be painted inside the bar (no clip).
/// Finds the label Text and requires its rect within the bar rect.
void _expectLabelPainted(
  WidgetTester tester, {
  required String label,
  required Type barType,
}) {
  final Rect bar = tester.getRect(find.byType(barType));
  final Finder finder = find.text(label);
  expect(finder, findsWidgets, reason: '"$label" missing entirely');
  for (final Element el in finder.evaluate()) {
    final RenderBox box = el.renderObject! as RenderBox;
    if (!box.hasSize) continue;
    final Offset at = box.localToGlobal(Offset.zero);
    final Rect rect = at & box.size;
    expect(
      rect.left >= bar.left - 0.5 && rect.right <= bar.right + 0.5,
      isTrue,
      reason: 'label "$label" clipped ($rect vs bar $bar)',
    );
  }
}

BottomBarIconAction _affordance(String label) => BottomBarIconAction(
  icon: Icons.chat_bubble_outline,
  label: label,
  onPressed: () {},
);

void main() {
  group('PROBE for-sale max (2 leading + Beli Sekarang)', () {
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('forsale at ${width}dp @scale $scale', (tester) async {
          await _pumpBar(
            tester,
            width: width,
            scale: scale,
            subject: BottomActionBar(
              leading: [_affordance('Chat'), _affordance('Nego')],
              primary: BottomBarAction(
                label: 'Beli Sekarang',
                onPressed: () {},
              ),
            ),
          );
          _expectClean(
            tester,
            label: 'forsale bar',
            width: width,
            scale: scale,
          );
          _expectLabelPainted(
            tester,
            label: 'Beli Sekarang',
            barType: BottomActionBar,
          );
        });
      }
    }
  });

  group('PROBE auction terminal (leading + Tidak Ada Pemenang + Lihat Lelang Lain)', () {
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('terminal at ${width}dp @scale $scale', (tester) async {
          await _pumpBar(
            tester,
            width: width,
            scale: scale,
            subject: BottomActionBar(
              leading: [_affordance('Chat')],
              secondary: BottomBarAction(
                label: 'Tidak Ada Pemenang',
                onPressed: null,
              ),
              primary: BottomBarAction(
                label: 'Lihat Lelang Lain',
                onPressed: () {},
              ),
            ),
          );
          _expectClean(
            tester,
            label: 'terminal pair',
            width: width,
            scale: scale,
          );
          _expectLabelPainted(
            tester,
            label: 'Tidak Ada Pemenang',
            barType: BottomActionBar,
          );
          _expectLabelPainted(
            tester,
            label: 'Lihat Lelang Lain',
            barType: BottomActionBar,
          );
        });
      }
    }
  });

  group('PROBE wizard (Kembali + Bayar Sekarang / solo Lanjut Lengkapi Data)', () {
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('wizard pair at ${width}dp @scale $scale', (tester) async {
          await _pumpBar(
            tester,
            width: width,
            scale: scale,
            subject: BottomActionBar(
              secondary: BottomBarAction(
                label: 'Kembali',
                onPressed: () {},
              ),
              primary: BottomBarAction(
                label: 'Bayar Sekarang',
                onPressed: () {},
              ),
            ),
          );
          _expectClean(
            tester,
            label: 'wizard pair',
            width: width,
            scale: scale,
          );
          _expectLabelPainted(
            tester,
            label: 'Bayar Sekarang',
            barType: BottomActionBar,
          );
        });
        testWidgets('wizard solo long at ${width}dp @scale $scale', (
          tester,
        ) async {
          await _pumpBar(
            tester,
            width: width,
            scale: scale,
            subject: BottomActionBar(
              primary: BottomBarAction(
                label: 'Lanjut Lengkapi Data',
                onPressed: () {},
              ),
            ),
          );
          _expectClean(
            tester,
            label: 'wizard solo',
            width: width,
            scale: scale,
          );
          _expectLabelPainted(
            tester,
            label: 'Lanjut Lengkapi Data',
            barType: BottomActionBar,
          );
        });
      }
    }
  });

  group('PROBE claim (Batal + Klaim & Lanjutkan, loading)', () {
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('claim at ${width}dp @scale $scale', (tester) async {
          await _pumpBar(
            tester,
            width: width,
            scale: scale,
            subject: BottomActionBar(
              secondary: BottomBarAction(
                label: 'Batal',
                onPressed: () {},
              ),
              primary: BottomBarAction(
                label: 'Klaim & Lanjutkan',
                onPressed: () {},
              ),
            ),
          );
          _expectClean(tester, label: 'claim', width: width, scale: scale);
          _expectLabelPainted(
            tester,
            label: 'Klaim & Lanjutkan',
            barType: BottomActionBar,
          );
        });
        testWidgets('claim loading at ${width}dp @scale $scale', (
          tester,
        ) async {
          await _pumpBar(
            tester,
            width: width,
            scale: scale,
            subject: BottomActionBar(
              secondary: BottomBarAction(label: 'Batal', onPressed: null),
              primary: BottomBarAction(
                label: 'Klaim & Lanjutkan',
                onPressed: null,
                isLoading: true,
              ),
            ),
          );
          _expectClean(
            tester,
            label: 'claim loading',
            width: width,
            scale: scale,
          );
        });
      }
    }
  });

  group('PROBE shipping (Tambah Provinsi + icon / Simpan)', () {
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('shipping at ${width}dp @scale $scale', (tester) async {
          await _pumpBar(
            tester,
            width: width,
            scale: scale,
            subject: BottomActionBar(
              secondary: BottomBarAction(
                label: 'Tambah Provinsi',
                icon: Icons.add,
                onPressed: () {},
              ),
              primary: BottomBarAction(label: 'Simpan', onPressed: () {}),
            ),
          );
          _expectClean(
            tester,
            label: 'shipping',
            width: width,
            scale: scale,
          );
          _expectLabelPainted(
            tester,
            label: 'Tambah Provinsi',
            barType: BottomActionBar,
          );
        });
      }
    }
  });

  group('PROBE overdue custom dual CTA (reconstruction, non-production)', () {
    // Pattern-identical copy of order_detail_screen _buildCTAButton pair
    // AFTER the F9(b) fix: Expanded GestureDetector+Container >
    // Row(center)[Icon + Flexible(Text wrap, centered)].
    Widget cta(String label, bool isPrimary) => Expanded(
      child: GestureDetector(
        onTap: () {},
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
          decoration: BoxDecoration(
            color: isPrimary ? Colors.red : Colors.grey.shade200,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Icon(Icons.chat_bubble_outline, size: 16),
              const SizedBox(width: 6),
              Flexible(
                child: Text(
                  label,
                  style: const TextStyle(fontWeight: FontWeight.w600),
                  softWrap: true,
                  textAlign: TextAlign.center,
                ),
              ),
            ],
          ),
        ),
      ),
    );
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('overdue dual at ${width}dp @scale $scale', (tester) async {
          await _pumpBar(
            tester,
            width: width,
            scale: scale,
            subject: Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  cta('Chat Penjual', false),
                  const SizedBox(width: 8),
                  cta('Hubungi Support', true),
                ],
              ),
            ),
          );
          _expectClean(
            tester,
            label: 'overdue dual recon',
            width: width,
            scale: scale,
          );
          // Labels stay complete and inside the viewport (wrap, never clip).
          for (final String label in <String>[
            'Chat Penjual',
            'Hubungi Support',
          ]) {
            final Rect rect = tester.getRect(find.text(label));
            expect(
              rect.left >= -0.5 && rect.right <= width + 0.5,
              isTrue,
              reason: 'overdue label "$label" left the viewport ($rect)',
            );
          }
        });
        testWidgets('overdue solo at ${width}dp @scale $scale', (tester) async {
          await _pumpBar(
            tester,
            width: width,
            scale: scale,
            subject: Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [cta('Ingatkan Penjual', true)],
              ),
            ),
          );
          _expectClean(
            tester,
            label: 'overdue solo recon',
            width: width,
            scale: scale,
          );
          final Rect rect = tester.getRect(find.text('Ingatkan Penjual'));
          expect(
            rect.left >= -0.5 && rect.right <= width + 0.5,
            isTrue,
            reason: 'overdue label "Ingatkan Penjual" left viewport ($rect)',
          );
        });
      }
    }
  });

  group('PROBE negotiation offer sheet pair (reconstruction)', () {
    // negotiation_offer_sheet.dart:136-165 pattern: raw Row, Expanded x2,
    // native button heights, no fixed box.
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('offer pair at ${width}dp @scale $scale', (tester) async {
          await _pumpBar(
            tester,
            width: width,
            scale: scale,
            subject: Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  Expanded(
                    child: OutlinedButton(
                      onPressed: () {},
                      child: const Text('Batal'),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: ElevatedButton(
                      onPressed: () {},
                      child: const Text(
                        'Kirim Penawaran',
                        style: TextStyle(fontWeight: FontWeight.bold),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          );
          _expectClean(
            tester,
            label: 'offer pair recon',
            width: width,
            scale: scale,
          );
        });
      }
    }
  });

  group('PROBE proposal answer row in 240px bubble (reconstruction)', () {
    // negotiation_proposal_card.dart:203-244 pattern inside maxWidth:300,
    // hosted in a chat bubble (~240px usable here).
    Widget answers(bool threeWay) => SizedBox(
      width: 240,
      child: Row(
        children: [
          Expanded(
            child: ElevatedButton(
              onPressed: () {},
              child: const Text('Terima'),
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: OutlinedButton(
              onPressed: () {},
              child: const Text('Counter'),
            ),
          ),
          if (threeWay) ...[
            const SizedBox(width: 8),
            Expanded(
              child: OutlinedButton(
                onPressed: () {},
                child: const Text('Tolak'),
              ),
            ),
          ],
        ],
      ),
    );
    for (final double scale in _scales) {
      testWidgets('proposal 2-way @scale $scale', (tester) async {
        await _pumpBar(
          tester,
          width: 360,
          scale: scale,
          subject: answers(false),
        );
        _expectClean(
          tester,
          label: 'proposal 2-way recon',
          width: 360,
          scale: scale,
        );
      });
      testWidgets('proposal 3-way @scale $scale', (tester) async {
        await _pumpBar(
          tester,
          width: 360,
          scale: scale,
          subject: answers(true),
        );
        _expectClean(
          tester,
          label: 'proposal 3-way recon',
          width: 360,
          scale: scale,
        );
      });
    }
  });

  group('PROBE deal single CTA in fixed-height box (reconstruction)', () {
    // negotiation_proposal_card.dart:481-494 pattern: fixed-height box +
    // 21ch label, ~228px usable here.
    for (final double scale in _scales) {
      testWidgets('deal CTA @scale $scale', (tester) async {
        await _pumpBar(
          tester,
          width: 360,
          scale: scale,
          subject: SizedBox(
            width: 228,
            child: SizedBox(
              height: 36,
              width: double.infinity,
              child: ElevatedButton(
                onPressed: () {},
                child: const Text(
                  'Beli dengan Harga Deal',
                  style: TextStyle(fontWeight: FontWeight.bold),
                ),
              ),
            ),
          ),
        );
        _expectClean(
          tester,
          label: 'deal CTA recon',
          width: 360,
          scale: scale,
        );
        // Business truth: the CTA must remain fully understandable — the
        // painted label must stay inside the tappable box.
        final Rect box = tester.getRect(
          find.widgetWithText(ElevatedButton, 'Beli dengan Harga Deal'),
        );
        final Rect label = tester.getRect(find.text('Beli dengan Harga Deal'));
        expect(
          label.left >= box.left - 0.5 && label.right <= box.right + 0.5,
          isTrue,
          reason: 'deal label clipped ($label vs button $box) @scale $scale',
        );
      });
    }
  });
}
