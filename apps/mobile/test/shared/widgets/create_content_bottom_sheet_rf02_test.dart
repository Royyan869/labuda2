// RF-02 contract: when a seller has a profile but no ACTIVE subscription, the
// create-content sheet must follow the canonical EXPIRY axis for its copy.
//
// Capability `inactive` only means "cannot sell right now". Only an ENDED
// subscription (`sellerSubscriptionStatus == 'expired'`) may say "Langganan
// berakhir" / "Perpanjang Langganan"; a seller who has not activated yet must
// not be told they expired.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';
import 'package:labuda/shared/widgets/create_content_bottom_sheet.dart';

Widget _host({required bool isSubscriptionExpired}) {
  return MaterialApp(
    home: Scaffold(
      body: CreateContentBottomSheet(
        onCreateContent: () {},
        onRenewSubscription: () {},
        sellerIdentityStatus: SellerIdentityStatus.seller,
        sellerCapabilityStatus: SellerCapabilityStatus.inactive,
        isSubscriptionExpired: isSubscriptionExpired,
      ),
    ),
  );
}

void main() {
  testWidgets('not-expired inactive seller gets activation copy only', (
    tester,
  ) async {
    await tester.pumpWidget(_host(isSubscriptionExpired: false));

    expect(find.text('Aktifkan Langganan'), findsOneWidget);
    expect(find.text('Perpanjang Langganan'), findsNothing);
    expect(find.textContaining('Langganan berakhir'), findsNothing);
    expect(find.textContaining('Langganan belum aktif'), findsWidgets);

    // For Sale/auction stay disabled: no market authority.
    expect(find.text('Jual Koi (For Sale)'), findsOneWidget);
    expect(find.text('Lelang (Auction)'), findsOneWidget);
  });

  testWidgets('expired inactive seller gets renewal copy', (tester) async {
    await tester.pumpWidget(_host(isSubscriptionExpired: true));

    expect(find.text('Perpanjang Langganan'), findsOneWidget);
    expect(find.text('Aktifkan Langganan'), findsNothing);
    expect(find.textContaining('Langganan berakhir'), findsWidgets);
    expect(find.textContaining('Langganan belum aktif'), findsNothing);
  });
}
