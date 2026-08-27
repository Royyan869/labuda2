import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/debug/idle_nav_trace.dart';

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

void main() {
  test(
    'IDLE_NAV_TRACE compile-time toggle is silent by default and loud when enabled',
    () async {
      final lines = await _capturePrints(() async {
        homeWidgetLifecycle(
          phase: 'INIT',
          instanceHash: 1,
          currentRoute: '/home',
        );
        profileLifecycle(
          phase: 'INIT',
          instanceHash: 2,
          currentRoute: '/user/2',
        );
      });

      if (kIdleNavTraceEnabled) {
        expect(lines, isNotEmpty);
        expect(
          lines.any((line) => line.contains('event=HOME_WIDGET_INIT')),
          isTrue,
        );
        expect(
          lines.any((line) => line.contains('event=PROFILE_INIT')),
          isTrue,
        );
      } else {
        expect(lines, isEmpty);
      }
    },
  );
}
