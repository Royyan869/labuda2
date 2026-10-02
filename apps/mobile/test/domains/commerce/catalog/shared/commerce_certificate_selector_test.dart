import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_certificate_selector.dart';

void main() {
  // Negative contract: the owner-locked vocabulary is breeder, contest, import,
  // health. `ownership`/`Kepemilikan` was retired and must not reappear in the
  // picker, because the backend rejects values outside the canonical set.
  test(
    'canonical certificate vocabulary is exactly breeder, contest, import, health',
    () {
      expect(
        commerceCertificateOptions
            .map((option) => option.value)
            .toList(growable: false),
        const ['breeder', 'contest', 'import', 'health'],
      );
      expect(
        commerceCertificateOptions.map((option) => option.value),
        isNot(contains('ownership')),
      );
    },
  );

  testWidgets(
    'CommerceCertificateSelector toggles canonical certificate chips',
    (tester) async {
      var selected = <String>['breeder'];

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: CommerceCertificateSelector(
              selectedCertificates: selected,
              onChanged: (value) {
                selected = value;
              },
            ),
          ),
        ),
      );

      expect(find.text('Breeder'), findsOneWidget);
      expect(find.text('Kontes'), findsOneWidget);
      expect(find.text('Import'), findsOneWidget);
      expect(find.text('Kesehatan'), findsOneWidget);
      expect(find.text('Kepemilikan'), findsNothing);

      await tester.tap(find.text('Kesehatan'));
      await tester.pump();

      expect(selected, contains('breeder'));
      expect(selected, contains('health'));
    },
  );
}
