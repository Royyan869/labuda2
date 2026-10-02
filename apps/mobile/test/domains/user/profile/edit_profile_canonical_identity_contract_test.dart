import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _readSource(String relativePath) =>
    File(relativePath).readAsStringSync();

void main() {
  group('Edit-profile canonical identity contracts', () {
    test('unified edit-profile screen renders Farm Information for sellers', () {
      final source = _readSource(
        'lib/domains/user/profile/presentation/screens/unified_edit_profile_screen.dart',
      );

      expect(source.contains('Farm Information'), isTrue);
      expect(source.contains('EditProfileFarmSection'), isTrue);
    });
  });
}
