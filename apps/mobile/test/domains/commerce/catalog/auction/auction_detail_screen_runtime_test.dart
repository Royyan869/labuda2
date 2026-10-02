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
// PageView, bottom-bar 'Chat' + 'Pasang Bid'. Save is in AppBar.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_recommendation_providers.dart'
    show ownerOtherAuctionsProvider, similarAuctionsProvider;
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart';
import 'package:labuda/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
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

/// The screen chrome (saved-item action) reads the saved-item repository on
/// init; the fake keeps the harness free of the real API client.
class _FakeSavedItemRepository implements SavedItemRepository {
  @override
  Future<bool> isSaved({
    required String targetType,
    required String targetId,
  }) async => false;

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
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
  ContentLifecycle sellerTrustLifecycle = ContentLifecycle.active,
  AuctionStatus status = AuctionStatus.active,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return Auction(
    id: id,
    sellerId: sellerId,
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
          certificates: ['import', 'health'],
        ),
    preparationTime: preparationTime,
    openingBid: 1000000,
    currentBid: 1500000,
    bidIncrement: 50000,
    buyNowPrice: 2500000,
    media: media,
    startTime: now,
    endTime: now.add(const Duration(days: 1)),
    status: status,
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
}) {
  final notifier =
      auctionNotifierState == null
          ? null
          : _FakeAuctionNotifier(auctionNotifierState);
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
      ownerOtherAuctionsProvider(
        auction.id,
      ).overrideWith((ref) async => const <Auction>[]),
      similarAuctionsProvider(
        auction.id,
      ).overrideWith((ref) async => const <Auction>[]),
      navigationHandlerProvider.overrideWithValue(_FakeNavigationHandler()),
      savedItemRepositoryProvider.overrideWithValue(
        _FakeSavedItemRepository(),
      ),
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
      preparationTime: PreparationTime.days1_3,
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
    // Media renders through AppImage → CachedNetworkImage whose shimmer
    // animates until the cache-manager file IO completes (real async, not
    // drivable by fake-async pumpAndSettle). Bounded pumps are sufficient for
    // layout/assertions — the carousel itself is exercised in
    // auction_detail_header_media_test.dart.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

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
    expect(find.text('Import, Kesehatan'), findsOneWidget);
    expect(find.text('Berdasarkan pernyataan seller'), findsOneWidget);

    // Canonical preparation content.
    expect(find.text('Estimasi siap kirim: 1–3 hari'), findsOneWidget);
    expect(find.textContaining('Penjual perlu 1–3 hari'), findsOneWidget);
    // NO REGRESS: the preparation-note concept is purged end-to-end; no note
    // copy may be rendered from any payload (even a smuggle attempt).
    expect(find.text('Packing aman sebelum kirim'), findsNothing);

    // Description + auction-specific row.
    expect(find.text('Live auction'), findsOneWidget);
    expect(find.text('Bid Increment'), findsOneWidget);
    // Canonical grouping: the same formatter that renders envelope money.
    expect(find.text('Rp 50.000'), findsOneWidget);

    // Single seller identity authority — flat scalars.
    expect(find.text('Acme Farm'), findsOneWidget);
    expect(find.text('@seller_user'), findsOneWidget);

    // Bottom actions: buyer can_bid + can_chat → bid enabled, chat visible.
    expect(find.text('Chat'), findsOneWidget);
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
    // No promote button on detail screens anymore: promotion is created
    // only from the promote page itself.
    expect(find.byTooltip('Promote'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('guest keeps the bid affordance (routes to sign-in)', (
    tester,
  ) async {
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

    // Model B parity with the fixed-price detail bar: the affordance stays
    // visible on raw facts; the tap routes to the canonical sign-in flow.
    expect(find.text('Pasang Bid'), findsOneWidget);
    expect(_bidButton(tester).onPressed, isNotNull);
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
    // Canonical grouping: the same formatter that renders envelope money.
    expect(find.text('Rp 50.000'), findsOneWidget);
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
}
