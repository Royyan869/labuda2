import 'package:flutter/services.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';

/// THE one money-input mask for Indonesian Rupiah.
///
/// Display while editing: digits are grouped with `.` every three digits
/// (`1000000` → `1.000.000`). The grouping itself is delegated to
/// [formatGroupedAmount] — the canonical display formatter — so the input
/// mask and every read-only money surface can never drift into two grouping
/// engines.
///
/// Business value: [digitsOf] / [parseAmount] expose the same text as a plain
/// digit string / int. Display punctuation is NEVER persisted; there is no
/// floating-point step anywhere on this path.
///
/// Behaviour:
/// - digits are accepted; every other character in a paste is dropped;
/// - empty stays empty (a valid editing state);
/// - leading zeros are dropped (`007` → `7`); a single `0` stays `0`;
/// - the caret keeps its digit offset through the edit, so typing, deleting a
///   digit and pasting all behave naturally. Deleting a `.` is a no-op:
///   separators are derived from the digits, never data of their own.
class MoneyInputFormatter extends TextInputFormatter {
  const MoneyInputFormatter();

  /// The business value of a masked money text: digits only, no punctuation.
  static String digitsOf(String text) => text.replaceAll(RegExp(r'[^0-9]'), '');

  /// The canonical input display of [amount] — for seeding a controller
  /// (`TextEditingController(text: MoneyInputFormatter.display(15000))`),
  /// never for persistence. Delegates to the one display formatter.
  static String display(int amount) => formatGroupedAmount(amount);

  /// Canonical numeric parse of a money field.
  ///
  /// Returns null for empty / non-numeric text. Always an int — money on this
  /// path is integer rupiah, and `"1.000.000"` parses to `1000000` with no
  /// double detour.
  static int? parseAmount(String text) {
    final digits = digitsOf(text);
    if (digits.isEmpty) return null;
    return int.tryParse(digits);
  }

  @override
  TextEditingValue formatEditUpdate(
    TextEditingValue oldValue,
    TextEditingValue newValue,
  ) {
    final raw = newValue.text;
    final caret = newValue.selection.end.clamp(0, raw.length);

    final digits = digitsOf(raw);
    if (digits.isEmpty) {
      return const TextEditingValue(
        selection: TextSelection.collapsed(offset: 0),
      );
    }

    // Count the digits that preceded the caret so it can be re-placed after
    // the same digit in the formatted text (never at the string end blindly).
    final digitsBeforeCaret = digitsOf(raw.substring(0, caret)).length;

    // Leading zeros: `007` -> `7`, `000` -> `0`.
    final canonical = digits.replaceFirst(RegExp(r'^0+(?=\d)'), '');
    final strippedZeros = digits.length - canonical.length;
    final effectiveBefore = (digitsBeforeCaret - strippedZeros).clamp(
      0,
      canonical.length,
    );

    final grouped = formatGroupedAmount(int.parse(canonical));

    var caretOut = grouped.length;
    if (effectiveBefore == 0) {
      caretOut = 0;
    } else if (effectiveBefore < canonical.length) {
      var seen = 0;
      for (var i = 0; i < grouped.length; i++) {
        if (grouped[i] != '.') seen++;
        if (seen == effectiveBefore) {
          caretOut = i + 1;
          break;
        }
      }
    }

    return TextEditingValue(
      text: grouped,
      selection: TextSelection.collapsed(offset: caretOut),
    );
  }
}
