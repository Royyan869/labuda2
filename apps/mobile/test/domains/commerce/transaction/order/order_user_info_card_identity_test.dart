import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/order/presentation/widgets/order_widgets.dart';

Widget _wrap({
  required String sellerUsername,
  String? sellerFarmName,
  String? sellerAvatarUrl,
}) {
  return ProviderScope(
    child: MaterialApp(
      home: Scaffold(
        body: OrderUserInfoCard(
          currentUserId: 'buyer-1',
          sellerId: 'seller-1',
          buyerId: 'buyer-1',
          sellerUsername: sellerUsername,
          sellerFarmName: sellerFarmName,
          sellerAvatarUrl: sellerAvatarUrl,
          isDark: false,
        ),
      ),
    ),
  );
}

void main() {
  testWidgets('Order Detail Seller Section renders @username then store_name', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        sellerUsername: 'yayan',
        sellerFarmName: 'Farm Koi Nusantara',
        sellerAvatarUrl: 'https://example.com/avatar.png',
      ),
    );

    expect(find.text('Penjual'), findsOneWidget);
    expect(find.text('@yayan'), findsOneWidget);
    expect(find.text('Farm Koi Nusantara'), findsOneWidget);
    expect(find.byType(Image), findsOneWidget);
  });

  testWidgets('Store missing fallback renders @username only', (tester) async {
    await tester.pumpWidget(
      _wrap(
        sellerUsername: 'yayan',
        sellerFarmName: null,
        sellerAvatarUrl: null,
      ),
    );

    expect(find.text('@yayan'), findsOneWidget);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets('Buyer-side seller section does not render technical id', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          home: Scaffold(
            body: OrderUserInfoCard(
              currentUserId: 'buyer-1',
              sellerId: 'seller-1',
              buyerId: 'buyer-1',
              sellerUsername: 'yayan',
              sellerFarmName: 'Farm Koi Nusantara',
              isDark: false,
            ),
          ),
        ),
      ),
    );

    expect(find.text('seller-1'), findsNothing);
    expect(find.text('@yayan'), findsOneWidget);
  });
}
