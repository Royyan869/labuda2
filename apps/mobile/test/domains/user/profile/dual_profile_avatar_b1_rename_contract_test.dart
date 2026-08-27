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
      expect(
        File('lib/shared/widgets/seller_identity_view.dart').existsSync(),
        isTrue,
      );
    });

    test('profile feature exports shared seller identity view', () {
      final contents = File(
        'lib/domains/user/profile/profile_feature.dart',
      ).readAsStringSync();

      expect(
        contents.contains(
          "export 'package:labuda/shared/widgets/seller_identity_view.dart';",
        ),
        isTrue,
      );
      expect(contents.contains('dual_profile_avatar.dart'), isFalse);
    });

    test('profile screen uses shared seller identity view', () {
      final contents = File(
        'lib/domains/user/profile/presentation/screens/profile_screen.dart',
      ).readAsStringSync();

      expect(contents.contains('SellerIdentityView('), isTrue);
      expect(contents.contains('DualProfileAvatar('), isFalse);
    });

    test('drawer header uses shared seller identity view', () {
      final contents = File(
        'lib/features/home/presentation/widgets/main_drawer/drawer_header.dart',
      ).readAsStringSync();

      expect(contents.contains('SellerIdentityView('), isTrue);
      expect(contents.contains('DualProfileAvatar('), isFalse);
    });
  });
}
