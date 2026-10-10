// F9(c) PROBE — BankAccountCardWidget action row (2-or-3 across) with
// realistic Indonesian bank data across the composition matrix.
// 320 / 360 / 412 / 500 x text scale 1.0 / 1.3 / 2.0.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/profile/domain/entities/bank_account_entity.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/bank_account_card_widget.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

BankAccountEntity _account({
  required bool isDefault,
  BankAccountStatus status = BankAccountStatus.active,
}) => BankAccountEntity(
  id: 'ba-1',
  bankName: 'Bank Syariah Indonesia',
  bankCode: 'BSI',
  accountNumber: '8820034567890123',
  accountHolderName: 'Siti Nurhaliza Putri Ramadhani',
  isDefault: isDefault,
  status: status,
  createdAt: DateTime(2025, 3, 2),
  updatedAt: DateTime(2026, 1, 9),
);

Future<void> _pump(
  WidgetTester tester, {
  required double width,
  required double scale,
  required BankAccountEntity account,
  required void Function() onEdit,
  required void Function() onDelete,
  required void Function() onSetPrimary,
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
          body: SingleChildScrollView(
            child: BankAccountCardWidget(
              account: account,
              onEdit: onEdit,
              onDelete: onDelete,
              onSetPrimary: onSetPrimary,
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}

void main() {
  group('PROBE bank action row (3-across non-default)', () {
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('3-across at ${width}dp @scale $scale', (tester) async {
          var edit = 0;
          var remove = 0;
          var primary = 0;
          await _pump(
            tester,
            width: width,
            scale: scale,
            account: _account(isDefault: false),
            onEdit: () => edit++,
            onDelete: () => remove++,
            onSetPrimary: () => primary++,
          );
          final Object? exception = tester.takeException();
          expect(
            exception,
            isNull,
            reason: 'layout exception at ${width}dp @scale $scale ($exception)',
          );
          // All three options present, tappable, no overlap.
          for (final String label in <String>[
            'Set as Primary',
            'Edit',
            'Delete',
          ]) {
            expect(find.text(label), findsOneWidget);
          }
          // At large scales the card grows vertically (wrapping, never
          // overflow); scroll each action into view before tapping.
          for (final String label in <String>[
            'Set as Primary',
            'Edit',
            'Delete',
          ]) {
            await tester.scrollUntilVisible(
              find.text(label),
              200,
              scrollable: find.byType(Scrollable).first,
            );
            await tester.tap(find.text(label));
          }
          await tester.pump();
          expect(primary, 1);
          expect(edit, 1);
          expect(remove, 1);
          final List<Rect> rects = <String>[
            'Set as Primary',
            'Edit',
            'Delete',
          ].map((String l) => tester.getRect(find.text(l))).toList();
          for (var i = 0; i < rects.length; i++) {
            for (var j = i + 1; j < rects.length; j++) {
              expect(
                rects[i].overlaps(rects[j]),
                isFalse,
                reason: 'action cells overlap (${rects[i]} vs ${rects[j]})',
              );
            }
          }
          // Business-critical identity fully visible (exact strings).
          expect(find.text('Bank Syariah Indonesia'), findsOneWidget);
          expect(find.text('8820034567890123'), findsOneWidget);
          expect(
            find.text('Siti Nurhaliza Putri Ramadhani'),
            findsOneWidget,
          );
          expect(find.text('BSI'), findsOneWidget);
          expect(find.text('Aktif'), findsOneWidget);
        });
      }
    }
  });

  group('PROBE bank action row (2-across default + deleted)', () {
    for (final double width in _widths) {
      for (final double scale in _scales) {
        testWidgets('2-across default at ${width}dp @scale $scale', (
          tester,
        ) async {
          await _pump(
            tester,
            width: width,
            scale: scale,
            account: _account(isDefault: true),
            onEdit: () {},
            onDelete: () {},
            onSetPrimary: () {},
          );
          expect(tester.takeException(), isNull);
          expect(find.text('Set as Primary'), findsNothing);
          expect(find.text('Edit'), findsOneWidget);
          expect(find.text('Delete'), findsOneWidget);
          expect(find.text('Rekening Utama'), findsOneWidget);
        });
        testWidgets('deleted status at ${width}dp @scale $scale', (
          tester,
        ) async {
          await _pump(
            tester,
            width: width,
            scale: scale,
            account: _account(
              isDefault: false,
              status: BankAccountStatus.deleted,
            ),
            onEdit: () {},
            onDelete: () {},
            onSetPrimary: () {},
          );
          expect(tester.takeException(), isNull);
          expect(find.text('Dihapus'), findsOneWidget);
          expect(find.text('Set as Primary'), findsOneWidget);
        });
      }
    }
  });
}
