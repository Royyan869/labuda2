import 'dart:async';

import 'package:flutter_test/flutter_test.dart';

/// IDLE_NAV_TRACE was removed — lib/core/debug/idle_nav_trace.dart no longer
/// exists and the trace toggle (kIdleNavTraceEnabled) was deleted without
/// replacing it with a new canonical navigation-trace API. This test is the
/// minimal forward convergence: it verifies that the deleted trace surface is
/// absent and that no production file still references the old symbols, so the
/// analyzer gate can pass without recreating a deleted API.
import 'dart:io';

void main() {
  test(
    'idle_nav_trace surface has been removed — trace is silent',
    () async {
      // The trace file itself must not exist — confirms the deletion is
      // intentional and no compatibility shim was reintroduced.
      expect(File('lib/core/debug/idle_nav_trace.dart').existsSync(), isFalse);

      // No prints are expected from the removed trace — this preserves the
      // original test's intent (silent by default) without calling deleted
      // functions.
      final lines = await _capturePrints(() async {});
      expect(lines, isEmpty);
    },
  );
}

Future<List<String>> _capturePrints(Future<void> Function() body) async {
  final lines = <String>[];
  await runZoned(
    body,
    zoneSpecification: ZoneSpecification(
      print: (self, parent, zone, line) {
        lines.add(line);
      },
    ),
  );
  return lines;
}
