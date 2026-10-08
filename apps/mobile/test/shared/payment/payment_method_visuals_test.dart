import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/shared.dart';

/// Every enabled canonical backend method (migrations 000006/000007/000088/000089).
const canonicalMethods = <String>[
  'bank_transfer',
  'qris',
  'credit_card',
  'dana',
  'convenience_store',
  'gopay',
  'ovo',
  'shopeepay',
];

/// Methods in this asset task's Owner-locked scope.
const scopedMethods = <String>[
  'bank_transfer',
  'qris',
  'credit_card',
  'dana',
  'convenience_store',
];

Set<String> _assetPathsIn(PaymentMethodVisual v) => <String>{
  if (v.primaryAsset != null) v.primaryAsset!,
  for (final b in v.secondaryBrands) b.asset,
};

void main() {
  group('PaymentMethodVisuals model', () {
    test('every canonical method has a visual definition', () {
      for (final code in canonicalMethods) {
        expect(PaymentMethodVisuals.isCanonical(code), isTrue,
            reason: '$code must have a canonical visual');
        final v = PaymentMethodVisuals.visual(code);
        expect(v.label, isNotEmpty);
        expect(v.fallbackIcon, isNotNull);
      }
    });

    test('unknown/null code returns a generic, non-broken visual', () {
      expect(PaymentMethodVisuals.icon(null), Icons.payments_outlined);
      expect(PaymentMethodVisuals.icon('made_up'), Icons.payments_outlined);
      expect(PaymentMethodVisuals.label(null), '—');
      expect(PaymentMethodVisuals.label(''), '—');
      expect(PaymentMethodVisuals.label('made_up'), 'made_up');
      expect(PaymentMethodVisuals.visual('made_up').primaryAsset, isNull);
      expect(PaymentMethodVisuals.isCanonical('made_up'), isFalse);
    });
  });

  group('Owner-locked semantics', () {
    test('qris uses the official QRIS asset', () {
      expect(
        PaymentMethodVisuals.visual('qris').primaryAsset,
        'assets/icons/payment/qris.png',
      );
    });

    test('bank_transfer is generic: no bank-specific primary visual', () {
      final v = PaymentMethodVisuals.visual('bank_transfer');
      expect(v.primaryAsset, isNull,
          reason: 'bank_transfer must not use a specific bank logo');
      expect(v.secondaryBrands, isEmpty,
          reason: 'no bank brand may represent generic bank transfer');
      // No scoped method may reference a specific bank asset.
      final forbiddenBankAssets = <String>[
        'bca', 'bni', 'bri', 'permata', 'danamon', 'mandiri',
        'cimb', 'maybank', 'bank_mega',
      ];
      for (final code in scopedMethods) {
        for (final asset in _assetPathsIn(PaymentMethodVisuals.visual(code))) {
          for (final bank in forbiddenBankAssets) {
            expect(asset.contains(bank), isFalse,
                reason: '$code must not use bank asset $asset');
          }
        }
      }
    });

    test('credit_card has a generic primary and is not one card brand', () {
      final v = PaymentMethodVisuals.visual('credit_card');
      expect(v.primaryAsset, isNull,
          reason: 'the primary card visual must be generic');
      expect(v.fallbackIcon, Icons.credit_card);
      final names = v.secondaryBrands.map((b) => b.name).toList();
      expect(names, containsAll(<String>['Visa', 'Mastercard']));
      expect(names.length, greaterThan(1),
          reason: 'credit_card must not depend on a single card brand');
    });

    test('convenience_store shows Alfamart + Indomaret as one method', () {
      final v = PaymentMethodVisuals.visual('convenience_store');
      expect(v.primaryAsset, isNull,
          reason: 'primary is generic; both brands are shown as secondary');
      final names = v.secondaryBrands.map((b) => b.name).toSet();
      expect(names, containsAll(<String>['Alfamart', 'Indomaret']));
      // DAN+DAN is intentionally not a separate brand (Owner decision).
      for (final b in v.secondaryBrands) {
        expect(b.asset.contains('dan'), isFalse);
      }
    });

    test('DANA uses no third-party/fabricated asset', () {
      final v = PaymentMethodVisuals.visual('dana');
      expect(v.primaryAsset, isNull, reason: danaOfficialAssetMissing);
      expect(v.secondaryBrands, isEmpty);
      final all = PaymentMethodVisuals.all.values
          .expand(_assetPathsIn)
          .join(' ');
      expect(all.contains('dana'), isFalse,
          reason: 'no DANA asset may be referenced until an official one exists');
    });
  });

  group('asset provenance', () {
    test('every referenced payment asset exists locally', () {
      final referenced = <String>{
        for (final v in PaymentMethodVisuals.all.values) ..._assetPathsIn(v),
      };
      expect(referenced, isNotEmpty);
      for (final path in referenced) {
        expect(path.startsWith('assets/icons/payment/'), isTrue,
            reason: '$path must be a local payment asset');
        expect(File(path).existsSync(), isTrue,
            reason: '$path must exist in the app bundle');
      }
    });

    test('no remote payment-brand URLs are referenced', () {
      final joined = PaymentMethodVisuals.all.values
          .expand(_assetPathsIn)
          .join(' ');
      expect(joined.contains('http'), isFalse);
    });
  });

  group('Presentation authority', () {
    test('all payment-method surfaces use the one mapping', () {
      final files = <String>[
        'lib/domains/finance/transaction/payment/presentation/widgets/payment_method_picker_sheet.dart',
        'lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_payment_method_section.dart',
        'lib/domains/commerce/transaction/order/presentation/widgets/order_payment_info_card.dart',
      ];
      for (final path in files) {
        final src = File(path).readAsStringSync();
        expect(src.contains('PaymentMethodVisuals'), isTrue,
            reason: '$path must consume the canonical authority');
        expect(src.contains('PaymentMethodLogo'), isTrue,
            reason: '$path must render through the canonical logo widget');
      }
    });

    test('no duplicate payment-method asset mapping outside the authority', () {
      final dir = Directory('lib');
      final offenders = <String>[];
      for (final entity in dir.listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        if (entity.path.endsWith('payment_method_visuals.dart')) continue;
        final src = entity.readAsStringSync();
        if (src.contains('assets/icons/payment/')) offenders.add(entity.path);
      }
      expect(offenders, isEmpty,
          reason: 'only PaymentMethodVisuals may map method_code -> payment asset');
    });

    test('the PaymentMethodType presentation authority stays purged', () {
      final src = File(
        'lib/core/common/types/payment_types.dart',
      ).readAsStringSync();
      expect(src.contains('enum PaymentMethodType'), isFalse);
      expect(src.contains('PaymentMethodTypeExtension'), isFalse);
    });
  });

  group('PaymentMethodLogo rendering', () {
    Widget host(PaymentMethodVisual v) => MaterialApp(
          home: Scaffold(
            body: Center(child: PaymentMethodLogo(visual: v, size: 28)),
          ),
        );

    testWidgets('qris renders a local image', (tester) async {
      await tester.pumpWidget(host(PaymentMethodVisuals.visual('qris')));
      expect(find.byType(Image), findsOneWidget);
    });

    testWidgets('convenience_store renders both brand marks', (tester) async {
      await tester.pumpWidget(
        host(PaymentMethodVisuals.visual('convenience_store')),
      );
      // 2 secondary brand marks (Alfamart + Indomaret), no primary asset.
      expect(find.byType(Image), findsNWidgets(2));
    });

    testWidgets('dana renders a generic fallback icon (no image)', (
      tester,
    ) async {
      await tester.pumpWidget(host(PaymentMethodVisuals.visual('dana')));
      expect(find.byType(Image), findsNothing);
      expect(find.byIcon(Icons.account_balance_wallet_outlined), findsOneWidget);
    });
  });
}
