/// REFERENCE ATTACHMENT LIVE-FETCH PURGE — structural ratchet
///
/// A communication surface displays a resource in exactly one way: through the
/// server-resolved `ResourceProjection` envelope rendered by the per-surface
/// projection card. The parallel "resolve it myself" path — a family provider
/// keyed on a client-built `ObjectReference`, its batch wrapper, and the
/// `ShareReference` ⇄ `ObjectReference` bridge — was the N+1: one
/// detail-endpoint call per shared message, plus a second, client-side status
/// and price vocabulary.
///
/// This ratchet fails if any piece of that path comes back. The purge is
/// structural on purpose: the deleted provider cannot be "fixed", only
/// resurrected, and that must never happen silently — the projection envelope
/// is the only authority the client is allowed to display.
library;

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

const _objectDir = 'lib/shared/object';

void main() {
  test('the client-side resource resolver machinery is gone', () {
    for (final path in const [
      '$_objectDir/object_preview_provider.dart',
      '$_objectDir/object_preview_batch_provider.dart',
      '$_objectDir/object_reference_bridge.dart',
      '$_objectDir/object_reference.dart',
    ]) {
      expect(
        File(path).existsSync(),
        isFalse,
        reason:
            '$path must not exist: resolving a resource per row from the '
            'client is the N+1 the canonical projection replaced',
      );
    }
  });

  test('no display surface references the deleted resolver machinery', () {
    const surfaces = [
      'lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart',
      'lib/domains/chat/chat/presentation/widgets/message_bubble.dart',
      'lib/shared/object/presentation/widgets/object_preview_card.dart',
      'lib/shared/widgets/attachment_widget.dart',
    ];
    const forbidden = [
      'objectPreviewProvider',
      'objectPreviewBatchProvider',
      'ObjectReference',
      'preResolved',
      'batchPreviews',
    ];

    for (final path in surfaces) {
      final source = File(path).readAsStringSync();
      for (final needle in forbidden) {
        expect(
          source.contains(needle),
          isFalse,
          reason: '$path must not reference "$needle"',
        );
      }
    }
  });

  test('the reference shell owns no money formatter', () {
    final source =
        File('$_objectDir/presentation/widgets/object_preview_card.dart')
            .readAsStringSync();

    expect(
      source.contains('Rp '),
      isFalse,
      reason:
          'money has one formatter (resource_projection.dart) and one source '
          'of truth (the projection envelope)',
    );
    expect(source.contains('toStringAsFixed'), isFalse);
  });

  test('the per-card batch fetch API the resolver used stays dead', () {
    // The multi-get for-sale fetch (a POST batch route) was the client batch
    // fetch behind the purged resolver: zero production callers, and the
    // backend never shipped the route, so it could only ever have 404'd. It
    // is dead API surface, not functionality — resurrection means somebody
    // is resolving resource rows on the client again.
    //
    // Adjacent literals keep the needles out of this file (source *and*
    // comment), so the scan needs no self-exemption.
    const needles = [
      'getForSales' 'ByIds',
      '/for-sale/' 'batch',
    ];

    final offenders = <String>[];
    for (final root in ['lib', 'test']) {
      for (final entity in Directory(root).listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        final source = entity.readAsStringSync();
        for (final needle in needles) {
          if (!source.contains(needle)) continue;
          offenders.add('${entity.path.replaceAll('\\', '/')} ($needle)');
        }
      }
    }

    expect(offenders, isEmpty, reason: 'dead batch API: $offenders');
  });
}
