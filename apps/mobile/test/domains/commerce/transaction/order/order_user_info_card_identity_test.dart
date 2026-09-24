import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/order/presentation/widgets/order_widgets.dart';

/// Canonical Order fixture: the card derives the buyer/seller identity from the
/// order itself, so identity assertions must travel through `Order`.
Order _order({
  String? sellerUsername,
  String? sellerFarmName,
  String? sellerAvatarUrl,
}) {
  return Order(
    id: 'order-1',
    buyerId: 'buyer-1',
    sellerId: 'seller-1',
    items: const [],
    status: OrderStatus.pending,
    paymentMethod: PaymentMethodType.bankTransfer,
    paymentStatus: PaymentStatus.pending,
    shippingInfo: const ShippingInfo(
      recipientName: 'Buyer',
      phone: '08123',
      address: 'Address',
      method: ShippingMethod.bus,
      shippingCost: 10000,
    ),
    pricing: const OrderPricing(
      subtotal: 100000,
      shippingCost: 10000,
      commissionAmount: 0,
      totalBeforeCoinsAmount: 110000,
      totalPayableAmount: 110000,
    ),
    createdAt: DateTime(2026, 6, 1),
    source: OrderSource.forSale,
    sellerUsername: sellerUsername,
    sellerFarmName: sellerFarmName,
    sellerAvatarUrl: sellerAvatarUrl,
  );
}

Widget _wrap({
  required String sellerUsername,
  String? sellerFarmName,
  String? sellerAvatarUrl,
}) {
  return ProviderScope(
    child: MaterialApp(
      home: Scaffold(
        body: OrderUserInfoCard(
          order: _order(
            sellerUsername: sellerUsername,
            sellerFarmName: sellerFarmName,
            sellerAvatarUrl: sellerAvatarUrl,
          ),
          currentUserId: 'buyer-1',
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
              order: _order(
                sellerUsername: 'yayan',
                sellerFarmName: 'Farm Koi Nusantara',
              ),
              currentUserId: 'buyer-1',
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
