// THE rule for the typography migration. ONE definition, imported by the
// contract test that guards it.
//
// `AppType.s<n>` is a SIZE token, and `AppTheme` already documents that a ROLE
// beats a size: "where a widget can say `Theme.of(context).textTheme.bodySmall`
// … it must". This file counts how many size tokens are still in the app so the
// migration can only ever go DOWN — a new `fontSize: AppType.s14` must fail the
// gate instead of quietly joining the backlog.
//
// The census counts EVERY `AppType.s*` reference, not just the `fontSize:`
// spelling: a site that decides its size inline
// (`fontSize: isActive ? AppType.s12 : AppType.s24`) is exactly as much a
// call site as the direct form, and a regex that only matches the direct form
// reports progress that does not exist.
import 'dart:io';

/// The file that DECLARES the tokens. It names them in prose and in its own
/// `static const` members, so it is not a call site and is excluded from the
/// census.
const typeTokenOwnerFile = 'lib/core/src/theme/app_theme.dart';

/// Direct `fontSize: AppType.s*` spelling — the shape that migrates onto a role.
final RegExp typeCallSite = RegExp(r'fontSize:\s*AppType\.s\w+');

/// Any `AppType.s*` reference, including conditional/inline sizes.
/// `AppType._()` (the private constructor) is deliberately NOT matched.
final RegExp typeReference = RegExp(r'AppType\.s\w+');

/// Floors, not exact counts: a census that reads nothing must not look like a
/// finished migration.
const typeCensusFileFloor = 900;
const typeCensusReferenceFloor = 900;

/// Line comments are stripped before counting so prose about the migration
/// (`/// widgets spell `fontSize: AppType.s14``) is not mistaken for a call
/// site. Block comments are not stripped; the codebase uses `///`.
String stripLineComments(String source) {
  final buffer = StringBuffer();
  for (final line in source.split('\n')) {
    if (line.trimLeft().startsWith('//')) continue;
    final cut = line.indexOf('//');
    buffer.writeln(cut >= 0 ? line.substring(0, cut) : line);
  }
  return buffer.toString();
}

List<File> typeCensusFiles({String dir = 'lib'}) => Directory(dir)
    .listSync(recursive: true)
    .whereType<File>()
    .where((file) => file.path.endsWith('.dart'))
    .where((file) => !file.path.replaceAll(r'\', '/').contains('/generated/'))
    .toList();

/// Token (`s14`, `s8_5`, …) -> reference count, across `lib`.
Map<String, int> typeReferenceCensus({String dir = 'lib'}) {
  final census = <String, int>{};
  for (final file in typeCensusFiles(dir: dir)) {
    final path = file.path.replaceAll(r'\', '/');
    if (path == typeTokenOwnerFile) continue;
    final source = stripLineComments(file.readAsStringSync());
    for (final match in typeReference.allMatches(source)) {
      final token = match.group(0)!.substring('AppType.'.length);
      census[token] = (census[token] ?? 0) + 1;
    }
  }
  return census;
}

/// Token -> reference count for ONE file. A migrated file must read `{}`.
Map<String, int> typeReferencesInFile(String path) {
  final file = File(path);
  if (!file.existsSync()) return const {};
  final census = <String, int>{};
  final source = stripLineComments(file.readAsStringSync());
  for (final match in typeReference.allMatches(source)) {
    final token = match.group(0)!.substring('AppType.'.length);
    census[token] = (census[token] ?? 0) + 1;
  }
  return census;
}

int typeReferenceTotal(Map<String, int> census) =>
    census.values.fold(0, (sum, count) => sum + count);
