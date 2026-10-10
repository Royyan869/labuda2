import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:hishumi/shared/models/seller_identity_data.dart';
import 'package:hishumi/shared/widgets/app_image.dart';
import 'package:hishumi/shared/widgets/seller_dual_avatar.dart';

Widget _wrap(Widget child) {
  return MaterialApp(
    home: Scaffold(body: Center(child: child)),
  );
}

void main() {
  testWidgets('CommerceMarketplaceCardMedia reloadToken re-keys the image',
      (tester) async {
    const imageUrl = 'https://d358tu61i1wrtt.cloudfront.net/images/shared.jpg';

    await tester.pumpWidget(
      _wrap(
        CommerceMarketplaceCardMedia(
          imageUrl: imageUrl,
          reloadToken: 'cover-v1',
          fallback: const SizedBox(width: 48, height: 48),
        ),
      ),
    );
    await tester.pump();

    final first = tester.widget<AppImage>(find.byType(AppImage));
    final firstKey = first.key as ValueKey<String>;

    await tester.pumpWidget(
      _wrap(
        CommerceMarketplaceCardMedia(
          imageUrl: imageUrl,
          reloadToken: 'cover-v2',
          fallback: const SizedBox(width: 48, height: 48),
        ),
      ),
    );
    await tester.pump();

    final second = tester.widget<AppImage>(find.byType(AppImage));
    final secondKey = second.key as ValueKey<String>;

    expect(secondKey.value, isNot(firstKey.value));
    expect(second.imageUrl, imageUrl);
  });

  testWidgets(
    'SellerDualAvatar reloadToken preserves store and personal identity',
    (tester) async {
      const storeUrl = 'https://d358tu61i1wrtt.cloudfront.net/images/store.jpg';

      await tester.pumpWidget(
        _wrap(
          const SellerDualAvatar(
            identity: SellerIdentityData(
              userId: 'seller-1',
              username: 'qiqijho',
              storeName: 'Qiqi Store',
              avatarUrl: null,
              storeImageUrl: storeUrl,
              isSeller: true,
            ),
            storeImageReloadToken: 'store-v1',
            size: 96,
          ),
        ),
      );
      await tester.pump();

      expect(find.byType(SellerDualAvatar), findsOneWidget);
      final first = tester
          .widgetList<AppImage>(find.byType(AppImage))
          .firstWhere((w) => (w.key as ValueKey<String>).value.contains('store-v1'));

      await tester.pumpWidget(
        _wrap(
          const SellerDualAvatar(
            identity: SellerIdentityData(
              userId: 'seller-1',
              username: 'qiqijho',
              storeName: 'Qiqi Store',
              avatarUrl: null,
              storeImageUrl: storeUrl,
              isSeller: true,
            ),
            storeImageReloadToken: 'store-v2',
            size: 96,
          ),
        ),
      );
      await tester.pump();

      final second = tester
          .widgetList<AppImage>(find.byType(AppImage))
          .firstWhere((w) => (w.key as ValueKey<String>).value.contains('store-v2'));

      expect(first.key, isNot(second.key));
      expect(second.imageUrl, storeUrl);
    },
  );
}
