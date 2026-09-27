/// MONEY ENGINE PARITY (audit 2026-09-27).
///
/// `CurrencyUtils` (seller/wallet/order surfaces) and `formatGroupedAmount`
/// (canonical envelope) used to be two independent engines: one ICU, one
/// hand-rolled loop. This test pins the strings the ICU engine produced so
/// the delegation to the single loop cannot change a single character on any
/// screen — and keeps them identical afterwards.
library;

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/utils/app_formatters.dart';
import 'package:labuda/shared/utils/currency_utils.dart';

void main() {
  group('CurrencyUtils display strings', () {
    test('standard rupiah grouping is unchanged', () {
      expect(CurrencyUtils.formatInt(0), 'Rp 0');
      expect(CurrencyUtils.formatInt(999), 'Rp 999');
      expect(CurrencyUtils.formatInt(50000), 'Rp 50.000');
      expect(CurrencyUtils.formatInt(1250000), 'Rp 1.250.000');
      expect(CurrencyUtils.format(1000000.0), 'Rp 1.000.000');
      expect(CurrencyUtils.format(1250000.0), 'Rp 1.250.000');
    });

    test('numbers without the symbol keep the same grouping', () {
      expect(CurrencyUtils.formatNumberInt(50000), '50.000');
      expect(CurrencyUtils.formatNumber(1250000.0), '1.250.000');
    });

    test('fractional amounts round to whole rupiah', () {
      expect(CurrencyUtils.format(1250000.4), 'Rp 1.250.000');
      expect(CurrencyUtils.format(1250000.6), 'Rp 1.250.001');
      expect(CurrencyUtils.formatNumber(1250000.4), '1.250.000');
    });

    test('an exact .5 rounds exactly like the reference engine', () {
      // Pinned against the ICU engine before delegation: whatever rounding
      // it applied at the half is the rounding the loop must reproduce.
      expect(CurrencyUtils.format(2500.5), 'Rp 2.501');
      expect(CurrencyUtils.format(2501.5), 'Rp 2.502');
      expect(CurrencyUtils.format(-2500.5), '-Rp 2.501');
    });

    test('negative amounts keep the minus ahead of the symbol', () {
      expect(CurrencyUtils.formatInt(-1500), '-Rp 1.500');
      expect(CurrencyUtils.format(-1500.0), '-Rp 1.500');
      expect(CurrencyUtils.formatNumberInt(-1500), '-1.500');
    });

    test('compact shorthand is untouched', () {
      expect(CurrencyUtils.formatShorthand(2300000000), 'Rp 2.3M');
      expect(CurrencyUtils.formatShorthand(1500000), 'Rp 1.5Jt');
      expect(CurrencyUtils.formatShorthand(500000), 'Rp 500K');
      expect(CurrencyUtils.formatShorthand(50000), 'Rp 50.000');
    });

    test('parsing still reads grouped strings back', () {
      expect(CurrencyUtils.parse('Rp 1.000.000'), 1000000.0);
      expect(CurrencyUtils.parse('1.250.000'), 1250000.0);
      expect(CurrencyUtils.parseInt('Rp 50.000'), 50000);
      expect(CurrencyUtils.parse('not a number'), isNull);
    });

    test('the facade and the utility never disagree', () {
      // AppFormatters.formatCurrency is a pass-through onto CurrencyUtils;
      // if either grows its own money logic again, this is the first test to
      // notice the second engine.
      expect(AppFormatters.formatCurrency(1250000), 'Rp 1.250.000');
      expect(
        AppFormatters.formatCurrency(1250000),
        CurrencyUtils.formatInt(1250000),
      );
      expect(AppFormatters.formatCurrencyInt(50000), 'Rp 50.000');
    });
  });

  test('the rupiah facade holds no second formatting engine', () {
    // Ratchet: `intl` NumberFormat was the parallel engine for seller/wallet
    // screens. Display grouping now lives in formatGroupedAmount alone; the
    // only tolerated rounding left in this file is the compact shorthand.
    final source = File(
      'lib/shared/utils/currency_utils.dart',
    ).readAsStringSync();
    expect(
      source.contains('NumberFormat'),
      isFalse,
      reason: 'CurrencyUtils must delegate to formatGroupedAmount, not ICU',
    );
    expect(
      source.contains('formatGroupedAmount('),
      isTrue,
      reason: 'the single grouping loop is formatGroupedAmount',
    );
  });
}
