import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/providers/core_providers.dart' show loggerServiceProvider;
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart'
    show auctionRepositoryProvider;
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/repositories/auction_repository.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/auction_card.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/repositories/for_sale_repository.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart'
    show forSaleRepositoryProvider;
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_card.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';
import 'package:labuda/features/explore/explore.dart';
import 'package:labuda/shared/services/logger_service.dart';

/// Explore tabs are organic-only in the canonical promotion era.
///
/// NEGATIVE PROOF: the legacy promotion discovery service
/// (/promotions/discover → PromotionDiscoveryService) is purged. Explore must
/// render organic listings/auctions without any promoted section, and no
/// legacy discovery provider may be referenced.
class _FakeForSaleRepository implements ForSaleRepository {
  final List<ForSale> listings;

  _FakeForSaleRepository(this.listings);

  @override
  Future<Result<List<ForSale>>> getForSales(GetForSalesParams params) async {
    return Result.success(listings);
  }

  @override
  Future<Result<ForSale?>> getForSaleById(String forSaleId) async {
    final listing =
        listings
            .where((item) => item.forSaleId == forSaleId)
            .isEmpty
        ? null
        : listings.firstWhere((item) => item.forSaleId == forSaleId);
    return Result.success(listing);
  }

  @override
  Future<Result<List<ForSale>>> getForSalesByIds(
    List<String> forSaleIds,
  ) async {
    return Result.success(
      listings
          .where((listing) => forSaleIds.contains(listing.forSaleId))
          .toList(),
    );
  }

  @override
  Future<Result<List<ForSale>>> getSellerForSales(
    String sellerId, {
    int page = 1,
    int pageSize = 20,
  }) async {
    return Result.success(const []);
  }

  @override
  Future<Result<ForSale>> createForSale(CreateForSaleRequest request) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<ForSale>> updateForSale(
    String forSaleId,
    UpdateForSaleRequest request,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<void>> deleteForSale(String forSaleId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<ForSale>> updateForSaleStatus(
    String fixedPriceSaleId,
    ForSaleStatus status,
  ) async {
    throw UnimplementedError();
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeAuctionRepository implements AuctionRepository {
  final List<Auction> auctions;

  _FakeAuctionRepository(this.auctions);

  @override
  Future<RepositoryResult<List<Auction>>> getActiveAuctions({
    String? variety,
    double? minSize,
    double? maxSize,
    double? maxBid,
    int limit = 20,
    String? lastAuctionId,
  }) async {
    return RepositoryResult.success(auctions);
  }

  @override
  Future<RepositoryResult<List<Auction>>> getUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 20,
    String? lastAuctionId,
  }) async {
    return RepositoryResult.success(const []);
  }

  @override
  Future<RepositoryResult<Auction>> getAuctionById(String auctionId) async {
    return RepositoryResult.success(
      auctions.firstWhere((auction) => auction.id == auctionId),
    );
  }

  @override
  Future<RepositoryResult<List<Auction>>> getAuctionsByIds(
    List<String> auctionIds,
  ) async {
    return RepositoryResult.success(
      auctions.where((auction) => auctionIds.contains(auction.id)).toList(),
    );
  }

  @override
  Stream<List<Auction>> watchActiveAuctions({int limit = 50}) {
    return Stream<List<Auction>>.value(auctions);
  }

  @override
  Stream<List<Auction>> watchUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 100,
  }) {
    return const Stream<List<Auction>>.empty();
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

ForSale _forSale({required String id, required String title}) {
  final now = DateTime.utc(2026, 6, 9);
  return ForSale(
    forSaleId: id,
    title: title,
    description: '$title description',
    price: 150000,
    stock: 10,
    sellerId: 'seller-1',
    sellerUsername: 'seller_user',
    sellerFarmName: 'Farm Name',
    status: ForSaleStatus.active,
    createdAt: now,
    updatedAt: now,
  );
}

Auction _auction({required String id, required String title}) {
  final now = DateTime.now();
  return Auction(
    id: id,
    sellerId: 'seller-1',
    sellerUsername: 'seller_user',
    sellerFarmName: 'Farm Name',
    title: title,
    description: '$title description',
    koiDetails: const KoiDetails(
      variety: 'Kohaku',
      sizeInCm: 30,
      ageInMonths: 12,
      gender: 'male',
    ),
    openingBid: 150000,
    currentBid: 200000,
    bidIncrement: 5000,
    startTime: now,
    endTime: now.add(const Duration(days: 1)),
    status: AuctionStatus.active,
    totalBidders: 2,
    createdAt: now,
  );
}

Widget _wrapExplore({
  required Widget child,
  required List<ForSale> listings,
  required List<Auction> auctions,
}) {
  final router = GoRouter(
    routes: [
      GoRoute(
        path: '/',
        builder: (context, state) => Scaffold(body: child),
      ),
      GoRoute(
        path: '/for-sale/:forSaleId',
        builder: (context, state) => Scaffold(
          body: Text('for-sale detail ${state.pathParameters['forSaleId']}'),
        ),
      ),
      GoRoute(
        path: '/auction/:auctionId',
        builder: (context, state) => Scaffold(
          body: Text('auction detail ${state.pathParameters['auctionId']}'),
        ),
      ),
    ],
    initialLocation: '/',
  );

  return ProviderScope(
    overrides: [
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
      forSaleRepositoryProvider.overrideWithValue(
        _FakeForSaleRepository(listings),
      ),
      auctionRepositoryProvider.overrideWithValue(
        _FakeAuctionRepository(auctions),
      ),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

void main() {
  testWidgets('listing tab renders organic listings without promoted section', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1080, 2400));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      _wrapExplore(
        child: const ExploreScreen(initialTab: 0),
        listings: [
          _forSale(id: 'for-sale-1', title: 'Koi A'),
          _forSale(id: 'for-sale-2', title: 'Koi B'),
        ],
        auctions: const [],
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Koi A'), findsOneWidget);
    expect(find.text('Koi B'), findsOneWidget);
    // Negative proof: no promoted section may exist.
    expect(find.text('Listing Dipromosikan'), findsNothing);
    expect(find.byType(ForSaleCard), findsNWidgets(2));
  });

  testWidgets('listing tab card navigates to for-sale detail', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1080, 2400));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      _wrapExplore(
        child: const ExploreScreen(initialTab: 0),
        listings: [_forSale(id: 'for-sale-1', title: 'Koi A')],
        auctions: const [],
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byType(ForSaleCard).first);
    await tester.pumpAndSettle();

    expect(find.text('for-sale detail for-sale-1'), findsOneWidget);
  });

  testWidgets('auction tab renders organic auctions without promoted section', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1080, 2400));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      _wrapExplore(
        child: const ExploreScreen(initialTab: 1),
        listings: const [],
        auctions: [
          _auction(id: 'auction-1', title: 'Lelang A'),
          _auction(id: 'auction-2', title: 'Lelang B'),
        ],
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Lelang A'), findsOneWidget);
    expect(find.text('Lelang B'), findsOneWidget);
    // Negative proof: no promoted section may exist.
    expect(find.text('Lelang Dipromosikan'), findsNothing);
    expect(find.byType(AuctionCard), findsNWidgets(2));
  });

  testWidgets('auction tab card navigates to auction detail', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1080, 2400));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      _wrapExplore(
        child: const ExploreScreen(initialTab: 1),
        listings: const [],
        auctions: [_auction(id: 'auction-1', title: 'Lelang A')],
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byType(AuctionCard).first);
    await tester.pumpAndSettle();

    expect(find.text('auction detail auction-1'), findsOneWidget);
  });
}