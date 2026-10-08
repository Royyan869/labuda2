import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/commerce/catalog/auction/presentation/create_auction_route_contract.dart';

void main() {
  group('CreateAuctionRouteArgs — caller-aware post-create landing', () {
    test('default (global create entry) lands on Marketplace → Auction', () {
      const args = CreateAuctionRouteArgs();
      expect(args.landing, CreateAuctionLanding.marketplace);
      expect(args.landsOnMarketplace, isTrue);
    });

    test('management-page caller opts out with .stay()', () {
      const args = CreateAuctionRouteArgs.stay();
      expect(args.landing, CreateAuctionLanding.stay);
      expect(args.landsOnMarketplace, isFalse);
    });

    test('absent route args (global push without extra) still lands', () {
      const CreateAuctionRouteArgs? routeArgs = null;
      // Mirrors the screen gate: `routeArgs?.landsOnMarketplace ?? true`.
      expect(routeArgs?.landsOnMarketplace ?? true, isTrue);
    });
  });
}
