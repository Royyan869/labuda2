import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  group('Seller identity shared module contract', () {
    test('old domain avatar file does not exist', () {
      final oldFile = File(
        'lib/domains/user/profile/presentation/widgets/dual_profile_avatar.dart',
      );
      expect(oldFile.existsSync(), isFalse);
    });

    test('shared seller identity files exist', () {
      expect(
        File('lib/shared/models/seller_identity_data.dart').existsSync(),
        isTrue,
      );
      expect(
        File('lib/shared/widgets/seller_dual_avatar.dart').existsSync(),
        isTrue,
      );
    });

    test('profile screen uses SellerIdentityStatus for seller checks', () {
      final contents = File(
        'lib/domains/user/profile/presentation/screens/profile_screen.dart',
      ).readAsStringSync();

      expect(contents.contains('SellerIdentityStatus.seller'), isTrue);
      expect(contents.contains('DualProfileAvatar('), isFalse);
    });

    test('drawer header does not use old DualProfileAvatar', () {
      final contents = File(
        'lib/features/home/presentation/widgets/main_drawer/drawer_header.dart',
      ).readAsStringSync();

      expect(contents.contains('DualProfileAvatar('), isFalse);
    });
  });
}
