import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  group('Presence residue contract', () {
    test(
      'mobile source tree has no legacy Firebase/Firestore presence paths',
      () {
        final root = Directory.current.path;
        final scanRoots = <Directory>[
          Directory('$root/lib'),
          Directory('$root/test'),
        ];

        final deprecatedPatterns = <String>[
          'chat_presence',
          '/users/presence/start',
          '/users/presence/stop',
          'presenceAuthSyncProvider',
          'WebSocketService.updatePresence',
        ];

        final hits = <String, List<String>>{};
        for (final directory in scanRoots) {
          if (!directory.existsSync()) continue;
          for (final entity in directory.listSync(
            recursive: true,
            followLinks: false,
          )) {
            if (entity is! File || !entity.path.endsWith('.dart')) continue;
            final normalizedPath = entity.path.replaceAll('\\', '/');
            if (normalizedPath.endsWith(
              'test/core/presence_residue_contract_test.dart',
            )) {
              continue;
            }

            final contents = entity.readAsStringSync();
            for (final pattern in deprecatedPatterns) {
              if (contents.contains(pattern)) {
                hits.putIfAbsent(pattern, () => <String>[]).add(entity.path);
              }
            }
          }
        }

        expect(
          hits,
          isEmpty,
          reason:
              'Deprecated presence residue must not remain in mobile source.',
        );
      },
    );
  });
}
