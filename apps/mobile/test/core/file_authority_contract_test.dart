// Foundation contract: ONE file per authority.
//
// Labuda's history is that every feature grew its own copy of shared
// foundations (wilayah models, PostLocation, BaseEntity) and convergence left
// the feature-local copy behind. A copied file is a second authority the
// moment it exists — even before anything imports it — so this locks three
// things:
//  1. No two `.dart` files under `lib/` may hold identical bytes: duplicating
//     a file is where the drift starts.
//  2. Each converged concept is declared in exactly one file. A copy that has
//     drifted apart is caught here too, not only a byte-identical one.
//  3. The copies convergence killed stay deleted.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// Every Dart file under `lib/` — the contract's scan surface.
List<File> _libDartFiles() => Directory('lib')
    .listSync(recursive: true)
    .whereType<File>()
    .where((f) => f.path.endsWith('.dart'))
    .toList();

/// Concept -> the single file allowed to declare it.
const _singleFileConcepts = <String, List<String>>{
  'lib/shared/models/wilayah_models.dart': [
    'Province',
    'City',
    'District',
    'Village',
  ],
  'lib/shared/entities/post_location.dart': ['PostLocation', 'LatLng'],
  'lib/core/common/base_entity.dart': ['BaseEntity', 'BaseModel'],
};

/// Copies convergence left behind because nothing imported them any more.
/// Deleted; they must not come back.
const _killedCopies = <String>[
  'lib/domains/commerce/catalog/models/wilayah_models.dart',
  'lib/domains/commerce/catalog/entities/post_location.dart',
  'lib/core/src/domain/base_entity.dart',
];

final _declaredClass = RegExp(
  r'^(?:abstract |sealed |final |base |mixin )?class (\w+)',
);

void main() {
  test('no two files in lib/ hold identical bytes', () {
    final byBody = <String, String>{}; // body -> first path carrying it
    final duplicates = <String>[];
    for (final file in _libDartFiles()) {
      final body = file.readAsStringSync();
      final first = byBody.putIfAbsent(body, () => file.path);
      if (first != file.path) duplicates.add('${file.path} == $first');
    }
    expect(
      duplicates,
      isEmpty,
      reason: 'a copied file is a second authority:\n${duplicates.join('\n')}',
    );
  });

  test('each converged concept is declared in exactly one file', () {
    final homes = <String, Set<String>>{}; // class name -> declaring files
    for (final file in _libDartFiles()) {
      final path = file.path.replaceAll(r'\', '/');
      for (final line in file.readAsLinesSync()) {
        final match = _declaredClass.firstMatch(line);
        if (match == null) continue;
        homes.putIfAbsent(match.group(1)!, () => <String>{}).add(path);
      }
    }
    final problems = <String>[];
    _singleFileConcepts.forEach((path, names) {
      for (final name in names) {
        final declaredIn = homes[name] ?? const <String>{};
        if (declaredIn.length != 1 || declaredIn.single != path) {
          problems.add(
            '$name declared in ${declaredIn.join(', ')} '
            '— expected only $path',
          );
        }
      }
    });
    expect(
      problems,
      isEmpty,
      reason: 'a concept has more than one home:\n${problems.join('\n')}',
    );
  });

  test('the authority files exist and the killed copies stay deleted', () {
    // A floor, not an exact count: the lock must cover the real app instead of
    // passing because the sweep found nothing to read.
    expect(_libDartFiles().length, greaterThan(1000));
    for (final path in _singleFileConcepts.keys) {
      expect(
        File(path).existsSync(),
        isTrue,
        reason: 'authority file missing: $path',
      );
    }
    for (final path in _killedCopies) {
      expect(
        File(path).existsSync(),
        isFalse,
        reason: 'dead duplicate resurrected: $path',
      );
    }
  });
}
