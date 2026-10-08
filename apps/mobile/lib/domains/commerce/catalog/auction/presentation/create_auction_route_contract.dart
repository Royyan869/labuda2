/// Typed contract for the canonical Create Auction route.
///
/// The global create entry (MainScreen center sheet) launches the route with no
/// extra and gets the canonical landing: after a successful create the shell
/// switches to Marketplace → Auction (where the new auction is discoverable).
///
/// A management-page caller (My Auctions) opts out with
/// [CreateAuctionRouteArgs.stay] so it is not hijacked to the Marketplace
/// surface and can stay on / refresh its own inventory.
library;

enum CreateAuctionLanding { marketplace, stay }

class CreateAuctionRouteArgs {
  final CreateAuctionLanding landing;

  const CreateAuctionRouteArgs({
    this.landing = CreateAuctionLanding.marketplace,
  });

  /// Management-page caller: do not retarget the shell to Marketplace.
  const CreateAuctionRouteArgs.stay() : landing = CreateAuctionLanding.stay;

  /// Whether a successful create retargets the shell to Marketplace → Auction.
  bool get landsOnMarketplace => landing == CreateAuctionLanding.marketplace;
}
