import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';
import 'package:labuda/shared/widgets/create_content_bottom_sheet.dart';

void main() {
  testWidgets('universal create content entry is available to active sellers', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: CreateContentBottomSheet(
            sellerIdentityStatus: SellerIdentityStatus.seller,
            sellerCapabilityStatus: SellerCapabilityStatus.active,
            onCreateContent: () {},
            onCreateAuction: () {},
          ),
        ),
      ),
    );

    expect(find.text('Buat Konten'), findsOneWidget);
    expect(find.text('Minta Koi (Request)'), findsNothing);
  });

  testWidgets('active sellers can tap the auction option on mobile', (
    tester,
  ) async {
    var auctionTapped = false;

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: CreateContentBottomSheet(
            sellerIdentityStatus: SellerIdentityStatus.seller,
            sellerCapabilityStatus: SellerCapabilityStatus.active,
            onCreateContent: () {},
            onCreateAuction: () {
              auctionTapped = true;
            },
          ),
        ),
      ),
    );

    expect(find.text('Lelang (Auction)'), findsOneWidget);
    expect(find.text('Buat lelang dari mobile'), findsOneWidget);

    await tester.tap(find.text('Lelang (Auction)'));
    await tester.pump();

    expect(auctionTapped, isTrue);
  });
}
