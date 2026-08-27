import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// SLICE B1: Content creation header structural contract.
///
/// Source-contract tests verifying that ContentVisibilityHeader is a
/// generic visibility/composer header with no type selector.

String _contentHeaderSource() => File(
  'lib/domains/social/content/presentation/widgets/create_content/content_type_visibility_header.dart',
).readAsStringSync();

void main() {
  group('Content header B1 structural contract', () {
    test('uses the generic visibility header name', () {
      final source = _contentHeaderSource();

      expect(
        source.contains(
          'class ContentVisibilityHeader extends StatelessWidget',
        ),
        isTrue,
        reason: 'Content header must use the generic visibility header.',
      );
    });

    test('no content kind selector remains in the header source', () {
      final source = _contentHeaderSource();

      expect(
        source.contains('ContentType'),
        isFalse,
        reason: 'Type selector authority must not remain in the header.',
      );
      expect(source.contains('onTypeChanged'), isFalse);
      expect(source.contains('postType'), isFalse);
    });
  });
}
