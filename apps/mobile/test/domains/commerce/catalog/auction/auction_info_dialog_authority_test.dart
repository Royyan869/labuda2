// Dialog authority — Slice #5: auction acknowledgement notice.
//
// The auction restricted-access notice is the one live single-acknowledgement
// information dialog in the auction scope. It now delegates composition to the
// canonical `AppDialog.info`; its business branch (bid rejection dispatch) is
// untouched. This file locks the composition purge and the no-wrapper rule.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

const _screenPath =
    'lib/domains/commerce/catalog/auction/presentation/screens/'
    'auction_detail_screen.dart';

void main() {
  group('auction acknowledgement notice — negative gate', () {
    test('the migrated consumer composes no raw AlertDialog', () {
      final source = File(_screenPath).readAsStringSync();
      expect(
        source.contains('AlertDialog('),
        isFalse,
        reason: 'the restricted-access notice must not build a raw dialog',
      );
      expect(
        source.contains("title: const Text('Akses Lelang Dibatasi')"),
        isFalse,
        reason: 'the hand-built title Text must be gone',
      );
      expect(source.contains('AppType'), isFalse);
      expect(source.contains('fontSize:'), isFalse);
    });

    test('the consumer delegates to the canonical info API', () {
      final source = File(_screenPath).readAsStringSync();
      expect(source.contains('AppDialog.info('), isTrue);
      expect(source.contains("title: 'Akses Lelang Dibatasi'"), isTrue);
      expect(source.contains("closeLabel: 'Mengerti'"), isTrue);
    });

    test('no auction-local info wrapper authority was introduced', () {
      final offenders = <String>[];
      for (final file
          in Directory('lib/domains/commerce/catalog/auction')
              .listSync(recursive: true)
              .whereType<File>()
              .where((f) => f.path.endsWith('.dart'))) {
        final source = file.readAsStringSync();
        for (final name in <String>[
          'class AuctionInfoDialog',
          'class AuctionInfo',
          'class AuctionNotice',
          'class InfoDialog',
        ]) {
          if (source.contains(name)) offenders.add('${file.path}: $name');
        }
      }
      expect(offenders, isEmpty, reason: offenders.join('\n'));
    });
  });
}
