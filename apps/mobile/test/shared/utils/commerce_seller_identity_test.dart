import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/utils/commerce_seller_identity.dart';

void main() {
  group('CommerceSellerIdentity formatting', () {
    test('Listing detail seller card style: @username then store_name', () {
      final identity = buildCommerceSellerIdentity(
        username: 'yayan',
        storeName: 'Farm Koi Nusantara',
      );

      expect(identity?.line1, '@yayan');
      expect(identity?.line2, 'Farm Koi Nusantara');
      expect(identity?.multilineLabel, '@yayan\nFarm Koi Nusantara');
    });

    test('Store missing fallback returns only @username', () {
      final identity = buildCommerceSellerIdentity(
        username: 'yayan',
        storeName: null,
      );

      expect(identity?.line1, '@yayan');
      expect(identity?.line2, isNull);
      expect(identity?.multilineLabel, '@yayan');
    });
  });
}
