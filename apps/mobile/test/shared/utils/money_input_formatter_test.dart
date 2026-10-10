// THE canonical money-input mask — behavior proof.
//
// Owner-locked: money typed by the user displays Indonesian thousands
// grouping while editing (`1000000` → `1.000.000`), while the business value
// stays a plain integer (display punctuation is never persisted).
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';
import 'package:hishumi/shared/utils/money_input_formatter.dart';

/// Simulates an edit that produced [newText] with the caret at [caret]
/// (default: end of text) — the shape a real keystroke/paste hands to the
/// formatter.
TextEditingValue _edit(String newText, {int? caret}) =>
    const MoneyInputFormatter().formatEditUpdate(
      const TextEditingValue(),
      TextEditingValue(
        text: newText,
        selection: TextSelection.collapsed(offset: caret ?? newText.length),
      ),
    );

void main() {
  group('grouping while typing', () {
    test('the Owner-locked examples', () {
      expect(_edit('1').text, '1');
      expect(_edit('10').text, '10');
      expect(_edit('1000').text, '1.000');
      expect(_edit('1000000').text, '1.000.000');
    });

    test('each additional digit regroups at the boundary', () {
      expect(_edit('100').text, '100');
      expect(_edit('10000').text, '10.000');
      expect(_edit('100000').text, '100.000');
      expect(_edit('10000000').text, '10.000.000');
    });

    test('the formatter delegates to the one display engine', () {
      // Compared against the REAL formatGroupedAmount: if the mask ever grows
      // its own grouping loop this parity fails.
      for (final digits in [
        '0',
        '1',
        '999',
        '1000',
        '50000',
        '1250000',
        '1000000000',
      ]) {
        expect(_edit(digits).text, formatGroupedAmount(int.parse(digits)));
      }
    });
  });

  group('input normalization', () {
    test('empty stays empty (a valid editing state)', () {
      expect(_edit('').text, '');
      expect(_edit('', caret: 0).selection.baseOffset, 0);
    });

    test('pasted groupings and symbols normalize to the canonical form', () {
      expect(_edit('1.000.000').text, '1.000.000');
      expect(_edit('Rp 1.000.000').text, '1.000.000');
      expect(_edit('1 000 000').text, '1.000.000');
      expect(_edit('1,000,000').text, '1.000.000');
    });

    test('non-digits are rejected/normalized, never kept', () {
      expect(_edit('12a3').text, '123');
      expect(_edit('abc').text, '');
    });

    test('leading zeros are explicitly defined', () {
      expect(_edit('0').text, '0');
      expect(_edit('007').text, '7');
      expect(_edit('000').text, '0');
      expect(_edit('0500').text, '500');
    });
  });

  group('caret stays usable', () {
    test('typing at the end leaves the caret at the end', () {
      final v = _edit('1000');
      expect(v.text, '1.000');
      expect(v.selection.baseOffset, v.text.length);
    });

    test('a caret in the middle keeps its digit offset', () {
      final v = _edit('1000', caret: 2);
      expect(v.text, '1.000');
      // Two digits precede the caret in the raw text → two digits precede it
      // in the grouped text ("1.0|00").
      expect(v.selection.baseOffset, 3);
    });

    test('deleting the last digit drops its separator naturally', () {
      // "1.000" → backspace → "1.00" → "100".
      final v = const MoneyInputFormatter().formatEditUpdate(
        const TextEditingValue(
          text: '1.000',
          selection: TextSelection.collapsed(offset: 5),
        ),
        const TextEditingValue(
          text: '1.00',
          selection: TextSelection.collapsed(offset: 4),
        ),
      );
      expect(v.text, '100');
      expect(v.selection.baseOffset, 3);
    });

    test('deleting a separator is a derived-value no-op', () {
      // Separators are not data: removing one re-derives it, the digits and
      // the caret digit-offset are unchanged.
      final v = const MoneyInputFormatter().formatEditUpdate(
        const TextEditingValue(
          text: '1.000',
          selection: TextSelection.collapsed(offset: 2),
        ),
        const TextEditingValue(
          text: '1000',
          selection: TextSelection.collapsed(offset: 1),
        ),
      );
      expect(v.text, '1.000');
      expect(v.selection.baseOffset, 1);
    });

    test('appending a digit keeps the caret at the end', () {
      final v = const MoneyInputFormatter().formatEditUpdate(
        const TextEditingValue(
          text: '1.000',
          selection: TextSelection.collapsed(offset: 5),
        ),
        const TextEditingValue(
          text: '1.0005',
          selection: TextSelection.collapsed(offset: 6),
        ),
      );
      expect(v.text, '10.005');
      expect(v.selection.baseOffset, v.text.length);
    });
  });

  group('business value is punctuation-free', () {
    test('parseAmount reads grouped and raw text identically (int only)', () {
      expect(MoneyInputFormatter.parseAmount('1.000.000'), 1000000);
      expect(MoneyInputFormatter.parseAmount('1000000'), 1000000);
      expect(MoneyInputFormatter.parseAmount('Rp 50.000'), 50000);
      expect(MoneyInputFormatter.parseAmount('0'), 0);
      expect(MoneyInputFormatter.parseAmount(''), isNull);
      expect(MoneyInputFormatter.parseAmount('abc'), isNull);
    });

    test('digitsOf is the raw business text', () {
      expect(MoneyInputFormatter.digitsOf('1.000.000'), '1000000');
      expect(MoneyInputFormatter.digitsOf('Rp 1.250.000'), '1250000');
      expect(MoneyInputFormatter.digitsOf(''), '');
    });

    test('a formatted display never survives as a value', () {
      final displayed = _edit('1000000').text;
      expect(displayed, '1.000.000');
      expect(MoneyInputFormatter.parseAmount(displayed), 1000000);
      expect(MoneyInputFormatter.parseAmount(displayed), isA<int>());
    });
  });
}
