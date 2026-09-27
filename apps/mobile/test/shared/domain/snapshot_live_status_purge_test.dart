/// SNAPSHOT LIVE-STATUS PURGE RATCHET (audit 2026-09-27).
///
/// `lib/domains/commerce/catalog/shared/live_status_provider.dart` was the
/// last client-side path deriving availability from `ShareReference.preview`
/// (a second status authority beside the canonical projection). Audit showed
/// it had ZERO consumers in `lib/` and `test/` — no badge ever watched it —
/// so the owner decision was not "migrate the badge to the projection" but
/// "kill the path". Status now comes from one place only: the canonical
/// `ResourceProjection` envelope (LIVE), rendered as the card caption.
///
/// This ratchet keeps the file dead: any resurrection, of the file itself or
/// of the identifiers it exported, fails here.
library;

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// The public surface the purged file owned. `\b` keeps look-alike members
/// (e.g. `Attachment.supportsLiveStatus`, an unrelated honesty flag) legal.
const _purgedIdentifiers = <String>[
  r'\bliveStatusProvider\b',
  r'\bLiveStatus\b',
  r'\bForSaleLiveStatus\b',
  r'\bAuctionLiveStatus\b',
  r'\bProfileLiveStatus\b',
  r'\bContentLiveStatus\b',
  r'\bForSaleAvailabilityStatus\b',
  r'\bAuctionDisplayStatus\b',
  r'\bforSaleAvailabilityProvider\b',
  r'\bauctionStatusProvider\b',
  r'\bcreateFallbackStatus\b',
  r'\bgetStatusBadgeColor\b',
  r'\bshouldRefreshStatus\b',
];

void main() {
  test('the snapshot live-status provider stays purged', () {
    expect(
      File(
        'lib/domains/commerce/catalog/shared/live_status_provider.dart',
      ).existsSync(),
      isFalse,
      reason: 'availability is projection-owned; the snapshot path stays dead',
    );
  });

  test('no source rebuilds a status authority from the snapshot', () {
    final patterns = _purgedIdentifiers.map(RegExp.new).toList();
    final offenders = <String>[];

    for (final root in ['lib', 'test']) {
      for (final entity in Directory(root).listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        final path = entity.path.replaceAll('\\', '/');
        final lines = entity.readAsLinesSync();
        for (var i = 0; i < lines.length; i++) {
          for (final pattern in patterns) {
            if (!pattern.hasMatch(lines[i])) continue;
            offenders.add('$path:${i + 1}: ${lines[i].trim()}');
          }
        }
      }
    }

    expect(
      offenders,
      isEmpty,
      reason: 'status comes from ResourceProjection, not a snapshot: '
          '$offenders',
    );
  });
}
