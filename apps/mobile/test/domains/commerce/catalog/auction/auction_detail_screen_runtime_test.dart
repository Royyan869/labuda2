// Auction Detail Screen — runtime tests against the FACTUAL converged screen.
//
// What this file asserts (and why):
//   1. viewer_capabilities (backend EvaluateAuctionViewerCapabilities) is the
//      canonical per-viewer action authority: buyer + can_bid → bid CTA
//      enabled; owner / guest / seller-inactive → disabled; can_chat gates the
//      Chat action.
//   2. Canonical Product content (koi attributes, certificates, preparation,
//      description) is consumed through the shared product detail section —
//      no canonical value is left dead in the read model.
//   3. Seller identity comes from the flat scalars (single authority) — no
//      origin/shipping surface exists on the auction detail.
//
// Removed (PHANTOM expectations from the pre-convergence draft): entity
// fields origin / shippingSetups / shippingSetupIds / sellerIdentity, the
// app-bar watch button ('Pantau'), 'Ajukan Bid' / 'Kelola Lelang' labels,
// and CommerceDetailMediaGallery. The factual screen renders the header
// PageView, bottom-bar watch ('Simpan'/'Tersimpan') + 'Chat' + 'Pasang Bid'.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart'
    show auctionWatchRepositoryProvider;
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_watcher.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/repositories/auction_watch_repository.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_recommendation_providers.dart'
    show ownerOtherAuctionsProvider, similarAuctionsProvider;
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart';
import 'package:labuda/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/core/common/types/preparation_time.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeAuctionNotifier extends AuctionNotifier {
  _FakeAuctionNotifier(this._state);

  final AuctionNotifierState _state;

  @override
  AuctionNotifierState build() => _state;

  @override
  Future<void> loadAuctionDetails(String auctionId) async {}

  @override
  Future<void> loadAuctionBids(String auctionId, {int limit = 50}) async {}
}

class _FakeNavigationHandler extends Fake implements NavigationHandler {}

/// Watch repository fake with mutable watched state for the toggle scenario.
class _FakeAuctionWatchRepository implements AuctionWatchRepository {
  bool watched = false;

  @override
  Future<RepositoryResult<AuctionWatcher>> watchAuction({
    required String auctionId,
    required String userId,
    bool notifyOnBid = true,
    bool notifyOnEndingSoon = true,
    bool notifyOnEnded = true,
  }) async {
    watched = true;
    return RepositoryResult.success(
      AuctionWatcher(
        id: '${auctionId}_$userId',
        auctionId: auctionId,
        userId: userId,
        createdAt: DateTime.utc(2026, 1, 1),
        notifyOnBid: notifyOnBid,
        notifyOnEndingSoon: notifyOnEndingSoon,
        notifyOnEnded: notifyOnEnded,
      ),
    );
  }

  @override
  Future<RepositoryResult<void>> unwatchAuction({
    required String auctionId,
    required String userId,
  }) async {
    watched = false;
    return RepositoryResult.success(null);
  }

  @override
  Future<RepositoryResult<bool>> isWatching({
    required String auctionId,
    required String userId,
  }) async {
    return RepositoryResult.success(watched);
  }

  @override
  Future<RepositoryResult<AuctionWatchStats>> getWatchStats({
    required String auctionId,
    required String currentUserId,
  }) async {
    return RepositoryResult.success(
      AuctionWatchStats(
        auctionId: auctionId,
        totalWatchers: 2,
        isWatchedByCurrentUser: watched,
      ),
    );
  }

  @override
  Future<RepositoryResult<int>> getWatchCount(String auctionId) async {
    return RepositoryResult.success(2);
  }

  @override
  Future<RepositoryResult<List<Auction>>> getWatchedAuctions({
    required String userId,
    int limit = 20,
    String? lastAuctionId,
  }) async {
    return RepositoryResult.success(const []);
  }

  @override
  Future<RepositoryResult<List<AuctionWatcher>>> getAuctionWatchers({
    required String auctionId,
    int limit = 100,
  }) async {
    return RepositoryResult.success(const []);
  }

  @override
  Stream<AuctionWatchStats> watchWatchStats({
    required String auctionId,
    required String currentUserId,
  }) {
    return Stream.value(
      AuctionWatchStats(
        auctionId: auctionId,
        totalWatchers: 2,
        isWatchedByCurrentUser: watched,
      ),
    );
  }

  @override
  Future<RepositoryResult<bool>> toggleWatch({
    required String auctionId,
    required String userId,
  }) async {
    watched = !watched;
    return RepositoryResult.success(watched);
  }
}

AuthUser _authUser({required String id}) {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: id,
    createdAt: now,
    updatedAt: now,
    email: '$id@example.com',
    username: id,
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    lifecycle: ContentLifecycle.active,
  );
}

const _buyerCapabilities = CommerceViewerCapabilities(
  role: 'buyer',
  canManage: false,
  canEdit: false,
  canPromote: false,
  canChat: true,
  canNegotiate: false,
  canBuy: false,
  canBid: true,
  canBuyNow: true,
);

const _ownerCapabilities = CommerceViewerCapabilities(
  role: 'owner',
  canManage: true,
  canEdit: false,
  canPromote: false,
  canChat: false,
  canNegotiate: false,
  canBuy: false,
  canBid: false,
  canBuyNow: false,
);

List<MediaEntity> _detailMedia() {
  final now = DateTime.utc(2026, 1, 1);
  return [
    MediaEntity(
      id: 'auction-media-1',
      originalUrl: 'https://cdn.example.com/gallery/auction-1.jpg',
      type: MediaType.image,
      createdAt: now,
    ),
    MediaEntity(
      id: 'auction-media-2',
      originalUrl: 'https://cdn.example.com/gallery/auction-2.jpg',
      type: MediaType.image,
      createdAt: now,
    ),
  ];
}

Auction _auction({
  required String id,
  required String sellerId,
  CommerceViewerCapabilities? capabilities,
  List<MediaEntity> media = const [],
  String description = 'Live auction',
  KoiDetails? koiDetails,
  PreparationTime? preparationTime,
  String? preparationNote,
  ContentLifecycle sellerTrustLifecycle = ContentLifecycle.active,
  AuctionStatus status = AuctionStatus.active,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return Auction(
    id: id,
    sellerId: sellerId,
    // Single identity authority — flat scalars (seller_username /
    // seller_farm_name / seller_avatar_url). No second identity model.
    sellerUsername: 'seller_user',
    sellerFarmName: 'Acme Farm',
    sellerAvatar: null,
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: sellerTrustLifecycle,
    sellerTier: 'pro',
    viewerCapabilities: capabilities,
    title: 'Sanke Auction',
    description: description,
    koiDetails:
        koiDetails ??
        const KoiDetails(
          variety: 'Kohaku',
          sizeInCm: 30,
          ageInMonths: 12,
          gender: 'male',
          breeder: 'Hiro',
          bloodline: 'Miyabi',
          certificates: ['ownership', 'health'],
        ),
    preparationTime: preparationTime,
    preparationNote: preparationNote,
    openingBid: 1000000,
    currentBid: 1500000,
    bidIncrement: 50000,
    buyNowPrice: 2500000,
    media: media,
    startTime: now,
    endTime: now.add(const Duration(days: 1)),
    status: status,
    totalBidders: 2,
    totalWatchers: 0,
    totalViews: 10,
    createdAt: now,
    updatedAt: now,
    productId: 'product-1',
  );
}

Widget _wrap({
  required Auction auction,
  required AuthState authState,
  AuctionNotifierState? auctionNotifierState,
  Stream<Auction?>? auctionStream,
  Stream<List<AuctionBid>>? auctionBidsStream,
  Stream<AuctionWatchStats>? watchStatsStream,
  AuctionWatchRepository? watchRepository,
}) {
  final notifier =
      auctionNotifierState == null
          ? null
          : _FakeAuctionNotifier(auctionNotifierState);
  final currentUserId = authState is AuthStateAuthenticated
      ? authState.user.id
      : '';
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      auctionNotifierProvider.overrideWith(
        () =>
            notifier ??
            _FakeAuctionNotifier(
              AuctionNotifierState(selectedAuction: auction),
            ),
      ),
      auctionStreamProvider(
        auction.id,
      ).overrideWith((ref) => auctionStream ?? Stream.value(auction)),
      auctionBidsStreamProvider(auction.id).overrideWith(
        (ref) => auctionBidsStream ?? Stream.value(const <AuctionBid>[]),
      ),
      watchStatsStreamProvider(
        (auctionId: auction.id, currentUserId: currentUserId),
      ).overrideWith(
        (ref) =>
            watchStatsStream ??
            Stream.value(
              AuctionWatchStats(
                auctionId: auction.id,
                totalWatchers: 2,
                isWatchedByCurrentUser: false,
              ),
            ),
      ),
      ownerOtherAuctionsProvider(
        auction.id,
      ).overrideWith((ref) async => const <Auction>[]),
      similarAuctionsProvider(
        auction.id,
      ).overrideWith((ref) async => const <Auction>[]),
      navigationHandlerProvider.overrideWithValue(_FakeNavigationHandler()),
      if (watchRepository != null)
        auctionWatchRepositoryProvider.overrideWithValue(watchRepository),
    ],
    child: MaterialApp(home: AuctionDetailScreen(auctionId: auction.id)),
  );
}

ElevatedButton _bidButton(WidgetTester tester) =>
    tester.widget<ElevatedButton>(
      find.widgetWithText(ElevatedButton, 'Pasang Bid'),
    );

void main() {
  testWidgets('buyer detail renders canonical content and enabled bid CTA', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(800, 2400));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final auction = _auction(
      id: 'auction-1',
      sellerId: 'seller-1',
      capabilities: _buyerCapabilities,
      media: _detailMedia(),
      preparationTime: PreparationTime.immediate,
      preparationNote: 'Packing aman sebelum kirim',
    );

    await tester.pumpWidget(
      _wrap(
        auction: auction,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-1'),
          emailVerified: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Screen chrome.
    expect(find.text('Auction Detail'), findsOneWidget);
    expect(find.text('Detail Lelang'), findsOneWidget);
    expect(find.byType(PageView), findsOneWidget);

    // Canonical koi content consumed through the shared detail section.
    expect(find.text('Kelamin'), findsOneWidget);
    expect(find.text('Jantan'), findsOneWidget);
    expect(find.text('Ukuran'), findsOneWidget);
    expect(find.text('30 cm'), findsOneWidget);
    expect(find.text('Usia'), findsOneWidget);
    expect(find.text('12 bulan'), findsOneWidget);
    expect(find.text('Varietas'), findsOneWidget);
    expect(find.text('Kohaku'), findsOneWidget);
    expect(find.text('Breeder'), findsOneWidget);
    expect(find.text('Hiro'), findsOneWidget);
    expect(find.text('Bloodline'), findsOneWidget);
    expect(find.text('Miyabi'), findsOneWidget);
    expect(find.text('Sertifikat'), findsOneWidget);
    expect(find.text('Kepemilikan, Kesehatan'), findsOneWidget);
    expect(find.text('Berdasarkan pernyataan seller'), findsOneWidget);

    // Canonical preparation content.
    expect(find.text('Siap kirim langsung'), findsOneWidget);
    expect(find.textContaining('Penjual siap mengirim'), findsOneWidget);
    expect(find.text('Packing aman sebelum kirim'), findsOneWidget);

    // Description + auction-specific row.
    expect(find.text('Live auction'), findsOneWidget);
    expect(find.text('Bid Increment'), findsOneWidget);
    expect(find.text('Rp 50000'), findsOneWidget);

    // Single seller identity authority — flat scalars.
    expect(find.text('Acme Farm'), findsOneWidget);
    expect(find.text('@seller_user'), findsOneWidget);

    // Bottom actions: buyer can_bid + can_chat → bid enabled, chat visible.
    expect(find.text('Chat'), findsOneWidget);
    expect(find.text('Simpan'), findsOneWidget);
    expect(find.text('Pasang Bid'), findsOneWidget);
    expect(_bidButton(tester).onPressed, isNotNull);

    // No phantom origin/shipping surface.
    expect(find.text('Origin'), findsNothing);
    expect(find.text('Opsi Pengiriman'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('owner capability disables bid and hides chat', (tester) async {
    await tester.binding.setSurfaceSize(const Size(800, 2400));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final auction = _auction(
      id: 'auction-owner',
      sellerId: 'seller-owner',
      capabilities: _ownerCapabilities,
    );

    await tester.pumpWidget(
      _wrap(
        auction: auction,
        authState: AuthState.authenticated(
          _authUser(id: 'seller-owner'),
          emailVerified: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Owner has no bid capability → CTA rendered but inert.
    expect(find.text('Pasang Bid'), findsOneWidget);
    expect(_bidButton(tester).onPressed, isNull);
    // can_chat=false for the owner role → chat hidden.
    expect(find.text('Chat'), findsNothing);
    // Owner promote entry point (canonical promotion management) is shown.
    expect(find.byTooltip('Promote'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('guest capability disables bid and hides chat', (tester) async {
    await tester.binding.setSurfaceSize(const Size(800, 2400));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final auction = _auction(
      id: 'auction-guest',
      sellerId: 'seller-guest',
      capabilities: const CommerceViewerCapabilities.guest(),
    );

    await tester.pumpWidget(
      _wrap(
        auction: auction,
        authState: const AuthStateUnauthenticated(),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Pasang Bid'), findsOneWidget);
    expect(_bidButton(tester).onPressed, isNull);
    expect(find.text('Chat'), findsNothing);
    // Share is authenticated-only.
    expect(find.byTooltip('Bagikan'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'seller-inactive buyer capability disables bid and hides chat',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(800, 2400));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      // Backend EvaluateAuctionViewerCapabilities folds seller-trust into
      // can_chat/can_bid — a buyer of an inactive seller gets canBid=false.
      const inactiveBuyer = CommerceViewerCapabilities(
        role: 'buyer',
        canManage: false,
        canEdit: false,
        canPromote: false,
        canChat: false,
        canNegotiate: false,
        canBuy: false,
        canBid: false,
        canBuyNow: false,
      );
      final auction = _auction(
        id: 'auction-inactive',
        sellerId: 'seller-inactive',
        capabilities: inactiveBuyer,
        sellerTrustLifecycle: ContentLifecycle.unavailable,
      );

      await tester.pumpWidget(
        _wrap(
          auction: auction,
          authState: AuthState.authenticated(
            _authUser(id: 'buyer-inactive'),
            emailVerified: true,
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Pasang Bid'), findsOneWidget);
      expect(_bidButton(tester).onPressed, isNull);
      expect(find.text('Chat'), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('empty product fields stay hidden on the detail screen', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(800, 2400));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final auction = _auction(
      id: 'auction-empty',
      sellerId: 'seller-empty',
      description: '',
      koiDetails: const KoiDetails(
        variety: '',
        sizeInCm: 0,
        ageInMonths: 0,
        gender: 'unknown',
        breeder: '',
        bloodline: '',
        certificates: [],
      ),
      preparationTime: null,
      preparationNote: null,
    );

    await tester.pumpWidget(
      _wrap(
        auction: auction,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-empty'),
          emailVerified: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Detail Lelang'), findsOneWidget);
    expect(find.text('Bid Increment'), findsOneWidget);
    expect(find.text('Rp 50000'), findsOneWidget);
    expect(find.text('Varietas'), findsNothing);
    expect(find.text('Ukuran'), findsNothing);
    expect(find.text('Usia'), findsNothing);
    expect(find.text('Kelamin'), findsNothing);
    expect(find.text('Breeder'), findsNothing);
    expect(find.text('Bloodline'), findsNothing);
    expect(find.text('Sertifikat'), findsNothing);
    expect(find.text('Deskripsi'), findsNothing);
    expect(find.text('Opsi Pengiriman'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('loading state renders a spinner and no bottom actions', (
    tester,
  ) async {
    final auction = _auction(id: 'auction-loading', sellerId: 'seller-loading');
    final auctionController = StreamController<Auction?>();
    final bidsController = StreamController<List<AuctionBid>>();
    addTearDown(() async {
      await auctionController.close();
      await bidsController.close();
    });

    await tester.pumpWidget(
      _wrap(
        auction: auction,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-loading'),
          emailVerified: true,
        ),
        auctionNotifierState: const AuctionNotifierState(isLoading: true),
        auctionStream: auctionController.stream,
        auctionBidsStream: bidsController.stream,
      ),
    );
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.text('Chat'), findsNothing);
    expect(find.text('Pasang Bid'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('watch toggle switches the bottom-bar label', (tester) async {
    await tester.binding.setSurfaceSize(const Size(800, 2400));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final auction = _auction(
      id: 'auction-watch',
      sellerId: 'seller-watch',
      capabilities: _buyerCapabilities,
    );
    final watchRepository = _FakeAuctionWatchRepository();
    final statsController = StreamController<AuctionWatchStats>.broadcast();
    addTearDown(statsController.close);

    await tester.pumpWidget(
      _wrap(
        auction: auction,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-watch'),
          emailVerified: true,
        ),
        watchRepository: watchRepository,
        watchStatsStream: statsController.stream,
      ),
    );
    statsController.add(
      AuctionWatchStats(
        auctionId: auction.id,
        totalWatchers: 2,
        isWatchedByCurrentUser: false,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Simpan'), findsOneWidget);
    expect(find.text('Tersimpan'), findsNothing);

    await tester.tap(find.text('Simpan'));
    await tester.pumpAndSettle();

    statsController.add(
      AuctionWatchStats(
        auctionId: auction.id,
        totalWatchers: 2,
        isWatchedByCurrentUser: true,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Tersimpan'), findsOneWidget);
    expect(find.text('Simpan'), findsNothing);
    expect(tester.takeException(), isNull);
  });
}