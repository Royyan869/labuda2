import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _readSource(String relativePath) =>
    File(relativePath).readAsStringSync();

void main() {
  group('Edit-profile canonical identity contracts', () {
    test('edit-profile uses store naming for seller workspace identity', () {
      final source = _readSource(
        'lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_store_section.dart',
      );

      expect(source.contains('EditProfileStoreSection'), isTrue);
      expect(source.contains('storeNameController'), isTrue);
    });

    test('unified edit-profile screen renders Farm Information for sellers', () {
      final source = _readSource(
        'lib/domains/user/profile/presentation/screens/unified_edit_profile_screen.dart',
      );

      expect(source.contains('Farm Information'), isTrue);
      expect(source.contains('EditProfileFarmSection'), isTrue);
    });
  });
}
