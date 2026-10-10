// H.4.6-C public URL contract: ONE canonical base, fixed paths per
// resource, and an explicit mapping from the public profile path to the
// internal mobile destination.
//
// Canonical truth (Owner decision, FINAL):
//   base     https://hishumi.com
//   profile  /profile/:id  -> mobile /user/:userId (via profile-ingress
//                            normalization; the public URL is NOT renamed)
//   content  /content/:id  -> mobile /content/:contentId
//   for-sale /for-sale/:id -> mobile /for-sale/:forSaleId
//   auction  /auction/:id  -> mobile /auction/:auctionId
//
// Firebase `*.web.app` and `labuda.app` hostnames must never appear in
// newly generated share URLs.

import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/social/share/domain/entities/share_target.dart';

ShareTarget _target(ExternalShareType type, String id) => ShareTarget(
  id: id,
  type: type,
  title: 'title',
  description: 'description',
);

void main() {
  group('H.4.6-C public URL contract', () {
    test('canonical base is https://hishumi.com', () {
      expect(kPublicBaseUrl, equals('https://hishumi.com'));
    });

    test('every resource generates the canonical path on the canonical base', () {
      expect(
        _target(ExternalShareType.profile, 'user-42').publicShareUrl,
        equals('https://hishumi.com/profile/user-42'),
      );
      expect(
        _target(ExternalShareType.post, 'content-1').publicShareUrl,
        equals('https://hishumi.com/content/content-1'),
      );
      expect(
        _target(ExternalShareType.request, 'req-1').publicShareUrl,
        equals('https://hishumi.com/content/req-1'),
      );
      expect(
        _target(ExternalShareType.forSale, 'fs-1').publicShareUrl,
        equals('https://hishumi.com/for-sale/fs-1'),
      );
      expect(
        _target(ExternalShareType.auction, 'auc-1').publicShareUrl,
        equals('https://hishumi.com/auction/auc-1'),
      );
    });

    test('generated URLs never reference retired hostnames', () {
      for (final type in ExternalShareType.values) {
        final url = _target(type, 'some-id').publicShareUrl;
        expect(url, isNot(contains('labuda-79de2')));
        expect(url, isNot(contains('labuda.app')));
        expect(url, isNot(contains('.web.app')));
      }
    });

    test('public profile path maps to the correct internal user route', () {
      // The public contract stays /profile/:id; the router normalizes it
      // to the internal /user/:userId destination (no duplicate route).
      expect(
        normalizeProfileIngressForTest('/profile/user-42'),
        equals('/user/user-42'),
      );
    });

    test('share paths coincide with the registered mobile detail routes', () {
      final contentUrl = Uri.parse(
        _target(ExternalShareType.post, 'c-1').publicShareUrl,
      );
      // Mirrors content_module.dart '/content/:contentId'.
      expect(contentUrl.path, equals('/content/c-1'));

      final forSaleUrl = Uri.parse(
        _target(ExternalShareType.forSale, 'fs-1').publicShareUrl,
      );
      expect(
        forSaleUrl.path,
        equals(RoutePaths.forSaleDetailPath('fs-1')),
      );

      final auctionUrl = Uri.parse(
        _target(ExternalShareType.auction, 'a-1').publicShareUrl,
      );
      expect(auctionUrl.path, equals(RoutePaths.auctionDetail('a-1')));
    });

    test('explicit base override still applies when a caller passes one', () {
      final target = _target(ExternalShareType.auction, 'a-9');
      expect(
        target.generatePublicShareUrl('https://example.test'),
        equals('https://example.test/auction/a-9'),
      );
    });
  });
}
