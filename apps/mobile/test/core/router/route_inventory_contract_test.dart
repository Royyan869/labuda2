// ROUTE INVENTORY CONTRACT — PASS 2 (Navigation / Routing convergence).
//
// Proves, against the REAL module registration (not a count), that:
//  1. UNIQUENESS  — no duplicate route path and no duplicate route name.
//  2. REACHABILITY — every registered production route has a canonical producer
//     in the app source, OR is an explicitly allow-listed externally
//     addressable surface. A registered-but-orphan route fails this test.
//  3. PURGE — no My Bidding / Contest / dead-constant navigation residue.
//
// Producer detection scans `lib/` (excluding `route_paths.dart` and the router
// module registration files) for a reference to the route's canonical
// `RoutePaths.<const>` / `RouteNames.<name>` or a matching path literal. This
// is deliberately source-based: a route that exists only in the registration
// list is exactly the orphan this contract must catch.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/src/router/router_modules_manager.dart';

/// Routes that are legitimately reachable ONLY from outside the app (deep
/// links / external entry) and therefore may have no in-app producer. Keep this
/// list tiny and justified. Empty today: every registered route has a producer.
const Map<String, String> externallyAddressableSurfaces = <String, String>{};

void main() {
  group('Route inventory — uniqueness', () {
    test('no duplicate route path and no duplicate route name', () async {
      final routes = await _registeredRoutes();

      final paths = <String>[];
      final names = <String>[];
      for (final route in routes) {
        paths.add(route.path);
        final name = route.name;
        if (name != null && name.isNotEmpty) names.add(name);
      }

      final duplicatePaths = _duplicates(paths);
      final duplicateNames = _duplicates(names);

      expect(
        duplicatePaths,
        isEmpty,
        reason: 'duplicate route path(s) registered: $duplicatePaths',
      );
      expect(
        duplicateNames,
        isEmpty,
        reason: 'duplicate route name(s) registered: $duplicateNames',
      );
    });
  });

  group('Route inventory — reachability', () {
    test(
      'every registered route has a producer or is an external surface',
      () async {
        final routes = await _registeredRoutes();
        final producerSource = _producerSource();
        final pathConstByValue = _pathConstByValue();

        final orphans = <String>[];
        for (final route in routes) {
          if (externallyAddressableSurfaces.containsKey(route.path)) continue;
          if (_hasProducer(
            path: route.path,
            name: route.name,
            producerSource: producerSource,
            pathConstByValue: pathConstByValue,
          )) {
            continue;
          }
          orphans.add('${route.path} (name: ${route.name})');
        }

        expect(
          orphans,
          isEmpty,
          reason:
              'registered route(s) with no producer and no external contract:\n'
              '${orphans.join('\n')}',
        );
      },
    );
  });

  group('Route inventory — purge residue', () {
    test('route_paths.dart carries no bidding/organizer/coinsTopup constant',
        () {
      final source = File(
        'lib/core/src/router/route_paths.dart',
      ).readAsStringSync();

      expect(source, isNot(contains('bidding')));
      expect(source, isNot(contains('organizerTeam')));
      expect(source, isNot(contains('coinsTopup')));
      // The RouteNames value must be gone; RoutePaths.createContent (the
      // canonical `/create/content` path) legitimately remains.
      expect(source, isNot(contains("'createContent'")));
    });

    test('no My Bidding route/screen/module/provider residue', () {
      final files = _libDartFiles();
      final forbidden = <String>[
        'BiddingScreen',
        'bidding_screen.dart',
        'bidding_notifier',
        'bidding_state',
        'bidding_repository',
        'bidding_remote_datasource',
        'bidding_dto',
        'bidding_mapper',
        'bidding_item.dart',
        'biddingRepositoryProvider',
        'biddingNotifierProvider',
      ];
      final hits = <String>[];
      for (final file in files) {
        final source = file.readAsStringSync();
        for (final token in forbidden) {
          if (source.contains(token)) {
            hits.add('${file.path}: $token');
          }
        }
      }
      expect(hits, isEmpty, reason: 'My Bidding residue:\n${hits.join('\n')}');
    });

    test('no Contest route/app-link residue', () {
      final manifest = File(
        'android/app/src/main/AndroidManifest.xml',
      ).readAsStringSync();
      expect(manifest, isNot(contains('/contest')));
      expect(manifest, isNot(contains('contest')));

      final routeSource = File(
        'lib/core/src/router/route_paths.dart',
      ).readAsStringSync();
      expect(routeSource, isNot(contains('/contest')));
    });

    test('no dead AppRouter wrapper methods remain', () {
      final source = File(
        'lib/core/src/router/app_router.dart',
      ).readAsStringSync();
      final forbidden = <String>[
        'void navigateTo(String route',
        'void navigateBack(',
        'void navigateToLogin(',
        'void navigateToRegister(',
        'void navigateToSellerForSales(',
        'void navigateToCoinBalance(',
        'void showModalDialog',
      ];
      final hits = forbidden.where(source.contains).toList();
      expect(hits, isEmpty, reason: 'dead AppRouter methods: $hits');
    });

    test('no permission guard scaffolding remains', () {
      expect(
        File('lib/shared/widgets/permission_guard.dart').existsSync(),
        isFalse,
      );
      final barrel = File('lib/shared/shared.dart').readAsStringSync();
      expect(barrel, isNot(contains("widgets/permission_guard.dart'")));
    });
  });
}

// ─────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────

Future<List<GoRoute>> _registeredRoutes() async {
  final manager = RouterModulesManager();
  await manager.initializeModules();
  return manager.buildRoutes();
}

List<File> _libDartFiles() {
  final lib = Directory('lib');
  return lib
      .listSync(recursive: true)
      .whereType<File>()
      .where((f) => f.path.endsWith('.dart'))
      .toList();
}

/// All app source EXCEPT the route constants and the module registration files
/// (which would trivially "produce" every route).
String _producerSource() {
  final buffer = StringBuffer();
  for (final file in _libDartFiles()) {
    final normalized = file.path.replaceAll('\\', '/');
    if (normalized.endsWith('core/src/router/route_paths.dart')) continue;
    if (normalized.contains('core/src/router/modules/')) continue;
    if (normalized.endsWith('checkout_router_module.dart')) continue;
    buffer.writeln(file.readAsStringSync());
  }
  return buffer.toString();
}

/// Map of `RoutePaths` constant value → constant name.
Map<String, String> _pathConstByValue() {
  final source = File('lib/core/src/router/route_paths.dart').readAsStringSync();
  final result = <String, String>{};
  final regex = RegExp(r"static const String (\w+)\s*=\s*'([^']*)'");
  for (final match in regex.allMatches(source)) {
    result[match.group(2)!] = match.group(1)!;
  }
  return result;
}

bool _hasProducer({
  required String path,
  required String? name,
  required String producerSource,
  required Map<String, String> pathConstByValue,
}) {
  // 1. RouteNames.<name>
  if (name != null && name.isNotEmpty) {
    final nameRegex = RegExp('RouteNames\\.${RegExp.escape(name)}(?!\\w)');
    if (nameRegex.hasMatch(producerSource)) return true;
  }

  // 2. RoutePaths.<const> (or a builder whose name extends the const name,
  //    e.g. `RoutePaths.followListPath` for const `followList`).
  final constName = pathConstByValue[path];
  if (constName != null) {
    final constRegex = RegExp(
      'RoutePaths\\.${RegExp.escape(constName)}(?:Path|Location|String)?(?!\\w)',
    );
    if (constRegex.hasMatch(producerSource)) return true;
  }

  // 3. A matching path literal with interpolated params.
  final literalRegex = RegExp(_pathLiteralPattern(path));
  if (literalRegex.hasMatch(producerSource)) return true;

  return false;
}

/// Turns `/orders/:orderId` into a regex that also matches interpolated
/// literals like `'/orders/$orderId'` and `'/orders/${order.id}'`.
String _pathLiteralPattern(String path) {
  final buffer = StringBuffer();
  var i = 0;
  while (i < path.length) {
    final ch = path[i];
    if (ch == ':') {
      var j = i + 1;
      while (j < path.length && RegExp(r'[A-Za-z0-9_]').hasMatch(path[j])) {
        j++;
      }
      buffer.write(r'(?:\$\{?[A-Za-z0-9_]+\}?|:[A-Za-z0-9_]+)');
      i = j;
    } else {
      buffer.write(RegExp.escape(ch));
      i++;
    }
  }
  return buffer.toString();
}

List<String> _duplicates(List<String> values) {
  final seen = <String>{};
  final dupes = <String>{};
  for (final value in values) {
    if (!seen.add(value)) dupes.add(value);
  }
  return dupes.toList();
}
