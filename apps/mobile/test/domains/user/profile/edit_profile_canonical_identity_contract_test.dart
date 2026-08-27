import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _readSource(String relativePath) =>
    File(relativePath).readAsStringSync();

void main() {
  group('Edit-profile canonical identity contracts', () {
    test('canonical edit-profile files use store naming only', () {
      final sources = <String>[
        _readSource(
          'lib/domains/user/profile/presentation/screens/unified_edit_profile_screen.dart',
        ),
        _readSource(
          'lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_save_handler.dart',
        ),
        _readSource(
          'lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_avatar_section.dart',
        ),
        _readSource(
          'lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_store_section.dart',
        ),
      ].join('\n');

      expect(sources.contains('farmNameController'), isFalse);
      expect(sources.contains('farmPhotoUrl'), isFalse);
      expect(sources.contains('farmInfo'), isFalse);
      expect(sources.contains('updateFarmInfo'), isFalse);
      expect(sources.contains('/users/:id/farm'), isFalse);
      expect(sources.contains('EditProfileStoreSection'), isTrue);
      expect(sources.contains('storeNameController'), isTrue);
      expect(sources.contains('storeImageUrl'), isTrue);
      expect(sources.contains('updateStoreProfile'), isTrue);
    });

    test('unified edit-profile screen no longer renders a farm section', () {
      final source = _readSource(
        'lib/domains/user/profile/presentation/screens/unified_edit_profile_screen.dart',
      );

      expect(source.contains('Farm Information'), isFalse);
      expect(source.contains('EditProfileFarmSection'), isFalse);
      expect(source.contains('Store Information'), isTrue);
    });
  });
}
