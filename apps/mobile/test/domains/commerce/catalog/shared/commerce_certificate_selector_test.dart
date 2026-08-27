import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_certificate_selector.dart';

void main() {
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
      expect(find.text('Kepemilikan'), findsOneWidget);
      expect(find.text('Kesehatan'), findsOneWidget);

      await tester.tap(find.text('Kesehatan'));
      await tester.pump();

      expect(selected, contains('breeder'));
      expect(selected, contains('health'));
    },
  );
}
