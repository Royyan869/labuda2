import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_bid_history.dart';

Widget _wrap(List<AuctionBid> bids) {
  return MaterialApp(
    home: Scaffold(body: AuctionBidHistory(bids: bids)),
  );
}

AuctionBid _bid({required String username, String? lifecycle}) {
  return AuctionBid(
    id: 'bid-1',
    auctionId: 'auction-1',
    bidderId: 'user-1',
    bidderUsername: username,
    bidderLifecycle: lifecycle,
    amount: 1500000,
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
  );
}

void main() {
  testWidgets('Auction bid history redacts degraded bidder identity', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap([_bid(username: 'alice', lifecycle: 'unavailable')]),
    );

    expect(find.text('@alice'), findsNothing);
    expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
    expect(find.byIcon(Icons.person), findsOneWidget);
  });

  testWidgets('Auction bid history leaves blank active bidder blank', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap([_bid(username: '', lifecycle: 'active')]));

    expect(find.text('@'), findsNothing);
    expect(find.byIcon(Icons.person), findsOneWidget);
  });

  testWidgets('Auction bid history preserves genuine stored user_deadbeef', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap([_bid(username: 'user_deadbeef', lifecycle: 'active')]),
    );

    expect(find.text('@user_deadbeef'), findsOneWidget);
    expect(find.text('user_deadbeef'), findsNothing);
  });
}
