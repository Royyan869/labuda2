import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// Auction-specific negative contract: the unsupported Auction Delete
/// affordance MUST NOT be present in the Auction Detail screen.
///
/// This test fails if:
/// - the PopupMoreOptionsButton call passes `onDelete`
/// - the handlers file defines `handleDelete`
/// - the handlers file contains the fake-delete dialog copy
/// - the handlers file references `onDeleteSuccess`
void main() {
  group('Auction Detail must not wire Delete affordance', () {
    test('auction_detail_screen.dart does not provide onDelete callback', () {
      final source = File(
        'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
      ).readAsStringSync().replaceAll('\r\n', '\n');

      // Must reference PopupMoreOptionsButton (sanity check — we're
      // looking at the right file).
      expect(source, contains('PopupMoreOptionsButton('));

      // The PopupMoreOptionsButton call must NOT pass onDelete.
      // We extract the call-site block to avoid false positives from
      // unrelated uses of the word "onDelete".
      final popupCallStart = source.indexOf('PopupMoreOptionsButton(');
      final afterPopup = source.substring(popupCallStart);
      // Find the matching closing of this constructor call by counting
      // parentheses — the call ends at the first '),' that closes the
      // constructor at the same depth.
      final callSite = _extractConstructorCall(
        afterPopup,
        'PopupMoreOptionsButton(',
      );
      expect(
        callSite,
        isNotNull,
        reason: 'Could not extract PopupMoreOptionsButton call site',
      );

      expect(
        callSite,
        isNot(contains('onDelete:')),
        reason:
            'auction_detail_screen.dart must not wire onDelete to PopupMoreOptionsButton',
      );

      // handlere.handleDelete() must not be referenced (case-insensitive
      // and without dot to catch both .handleDelete and handleDelete).
      expect(
        source,
        isNot(contains('handleDelete')),
        reason: 'auction_detail_screen.dart must not reference handleDelete',
      );
    });

    test('auction_detail_handlers.dart has no delete handler or residue', () {
      final source = File(
        'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_handlers.dart',
      ).readAsStringSync().replaceAll('\r\n', '\n');

      expect(
        source,
        isNot(contains('handleDelete')),
        reason: 'handleDelete method must not exist',
      );
      expect(
        source,
        isNot(contains('onDeleteSuccess')),
        reason: 'onDeleteSuccess must be renamed to onCancelSuccess',
      );
      expect(
        source,
        isNot(contains('Hapus Lelang')),
        reason: 'fake delete dialog title must not exist',
      );
      expect(
        source,
        isNot(contains('Apakah Anda yakin ingin menghapus lelang ini?')),
        reason: 'fake delete dialog body must not exist',
      );

      // Positive assertions — onCancelSuccess must be wired.
      expect(
        source,
        contains('onCancelSuccess'),
        reason: 'onCancelSuccess must exist for the Cancel lifecycle path',
      );
      expect(
        source,
        contains('handleCancel'),
        reason: 'handleCancel must remain for the legitimate Cancel lifecycle',
      );
    });
  });
}

/// Extracts the first constructor call of [name] from [source], handling
/// nested parentheses so we get the complete call rather than cutting at
/// the first closing paren inside a nested expression.
String? _extractConstructorCall(String source, String name) {
  final start = source.indexOf(name);
  if (start == -1) return null;

  var depth = 0;
  var i = start + name.length;
  while (i < source.length) {
    final ch = source[i];
    if (ch == '(') {
      depth++;
    } else if (ch == ')') {
      depth--;
      if (depth == 0) {
        return source.substring(start, i + 1);
      }
    }
    i++;
  }
  return null;
}
