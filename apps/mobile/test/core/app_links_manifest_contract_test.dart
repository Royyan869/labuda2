// H.4.6-C Android App Links manifest contract.
//
// The manifest's App Links intent-filter must target the canonical public
// host (https://hishumi.com) and cover every public share path the Owner
// mandates: /profile, /content, /for-sale, /auction.
//
// This test parses the shipped AndroidManifest.xml as text. It proves the
// LOCAL configuration only — it does NOT prove OS verification (that
// additionally requires a hosted assetlinks.json, an external blocker).

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  group('H.4.6-C Android App Links manifest contract', () {
    late String manifest;

    setUpAll(() {
      final file = File(
        '${Directory.current.path}/android/app/src/main/AndroidManifest.xml',
      );
      expect(
        file.existsSync(),
        isTrue,
        reason: 'AndroidManifest.xml must exist at the canonical location',
      );
      manifest = file.readAsStringSync();
    });

    test('App Links filter targets the canonical host hishumi.com', () {
      expect(manifest, contains('android:host="hishumi.com"'));
    });

    test('App Links filter covers every mandated public path', () {
      for (final path in <String>[
        '/auction',
        '/profile',
        '/content',
        '/for-sale',
      ]) {
        expect(
          manifest,
          contains('android:pathPrefix="$path"'),
          reason: 'missing App Links pathPrefix for $path',
        );
      }
    });

    test('retired share hostnames are gone from the manifest', () {
      expect(manifest, isNot(contains('labuda-79de2.web.app')));
      expect(manifest, isNot(contains('labuda.app')));
    });
  });
}
