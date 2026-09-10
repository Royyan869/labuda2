import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/common/types/preparation_time.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart'
    show forSaleDetailProvider;
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart';
import 'package:labuda/domains/user/profile/presentation/providers/user_data_provider.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:labuda/domains/user/preference/saved_item/models/saved_item_model.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeSavedItemRepository extends SavedItemRepository {
  _FakeSavedItemRepository() : super(dio: Dio(BaseOptions(baseUrl: 'http://localhost')));

  bool initialSaved = false;
  int isSavedCalls = 0;
  int addCalls = 0;
  int removeCalls = 0;

  @override
  Future<bool> isSaved({
    required String targetType,
    required String targetId,
  }) async {
    isSavedCalls += 1;
    return initialSaved;
  }

  @override
  Future<SavedItemModel> addSavedItem({
    required String targetType,
    required String targetId,
  }) async {
    addCalls += 1;
    initialSaved = true;
    return SavedItemModel(
      id: 'saved-$targetType-$targetId',
      userId: 'user-1',
      targetType: targetType == 'for_sale'
          ? TargetType.forSale
          : TargetType.auction,
      targetId: targetId,
      intentType: targetType == 'for_sale'
          ? IntentType.bookmark
          : IntentType.watch,
      createdAt: DateTime.utc(2026, 1, 1),
    );
  }

  @override
  Future<void> removeSavedItem({
    required String targetType,
    required String targetId,
  }) async {
    removeCalls += 1;
    initialSaved = false;
  }
}

class _FakeNavigationHandler extends Fake implements NavigationHandler {
  String? lastUserId;

  @override
  void navigateToUserProfile(String userId) {
    lastUserId = userId;
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

ForSale _listing({
  required String id,
  required String sellerId,
  CommerceViewerCapabilities? capabilities,
  List<MediaEntity> media = const [],
  bool isNegotiable = true,
  bool stockAvailable = true,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return ForSale(
    forSaleId: id,
    productId: 'product-1',
    title: 'Showa Koi 30cm',
    description: 'Premium showa',
    price: 1500000,
    stock: stockAvailable ? 1 : 0,
    sellerId: sellerId,
    sellerUsername: 'seller_user',
    sellerFarmName: 'Acme Farm',
    sellerAvatar: null,
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
    sellerTier: 'pro',
    viewerCapabilities: capabilities,
    media: media,
    status: ForSaleStatus.active,
    visibility: ForSaleVisibility.public,
    isNegotiable: isNegotiable,
    createdAt: now,
    updatedAt: now,
    variety: 'Kohaku',
    sizeCm: 30,
    ageMonths: 12,
    gender: 'male',
    breeder: 'Hiro',
    bloodline: 'Miyabi',
    preparationTime: PreparationTime.immediate,
    preparationNote: 'Packing aman sebelum kirim',
  );
}

const _buyerCaps = CommerceViewerCapabilities(
  role: 'buyer',
  canManage: false,
  canEdit: false,
  canPromote: false,
  canChat: true,
  canNegotiate: true,
  canBuy: true,
  canBid: false,
  canBuyNow: false,
);

const _buyerNoNegotiationCaps = CommerceViewerCapabilities(
  role: 'buyer',
  canManage: false,
  canEdit: false,
  canPromote: false,
  canChat: true,
  canNegotiate: false,
  canBuy: true,
  canBid: false,
  canBuyNow: false,
);

/// Seller-trust inactive: the evaluator yields an all-false capability set
/// for the buyer, so the bar renders the explanatory inactive banner.
const _sellerInactiveCaps = CommerceViewerCapabilities(
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

const _ownerCaps = CommerceViewerCapabilities(
  role: 'owner',
  canManage: true,
  canEdit: true,
  canPromote: true,
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
      id: 'listing-media-1',
      originalUrl:
          'https://cdn.example.com/gallery/listing-1.jpg?X-Amz-Signature=one',
      type: MediaType.image,
      createdAt: now,
    ),
    MediaEntity(
      id: 'listing-media-2',
      originalUrl:
          'https://cdn.example.com/gallery/listing-2.jpg?X-Amz-Signature=two',
      type: MediaType.image,
      createdAt: now,
    ),
  ];
}

Widget _wrap({
  required ForSale listing,
  required AuthState authState,
  ForSale Function()? listingLoader,
  ThemeData? theme,
  _FakeNavigationHandler? navigationHandler,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      savedItemRepositoryProvider.overrideWithValue(
        _FakeSavedItemRepository(),
      ),
      forSaleDetailProvider(
        listing.forSaleId,
      ).overrideWith((ref) async => listingLoader?.call() ?? listing),
      userDataProvider.overrideWith((ref, userId) async => _authUser(id: userId)),
      navigationHandlerProvider.overrideWithValue(
        navigationHandler ?? _FakeNavigationHandler(),
      ),
    ],
    child: MaterialApp(
      theme: theme,
      home: ForSaleDetailScreen(forSaleId: listing.forSaleId),
    ),
  );
}

void main() {
  testWidgets(
    'buyer detail renders capability-driven action bar (Chat/Nego/Buy Now)',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(320, 640));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final listing = _listing(
        id: 'listing-1',
        sellerId: 'seller-1',
        capabilities: _buyerCaps,
        media: _detailMedia(),
      );

      await tester.pumpWidget(
        _wrap(
          listing: listing,
          authState: AuthState.authenticated(
            _authUser(id: 'buyer-1'),
            emailVerified: true,
          ),
        ),
      );
      // Media renders through AppImage → CachedNetworkImage whose shimmer
      // animates until the cache-manager file IO completes (real async, not
      // drivable by fake-async pumpAndSettle). Bounded pumps are sufficient
      // for layout/assertions; the carousel itself is exercised in the
      // media-refresh test below.
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));

      expect(find.text('Detail Listing'), findsOneWidget);
      expect(find.text('Chat'), findsOneWidget);
      expect(find.text('Ajukan Penawaran'), findsOneWidget);
      expect(find.text('Beli Sekarang'), findsOneWidget);
      expect(find.text('Penjual tidak aktif'), findsNothing);
      expect(find.text('@seller_user'), findsOneWidget);
      expect(find.text('@Acme Farm'), findsOneWidget);
      expect(find.byType(PageView), findsOneWidget);
      expect(find.text('Siap kirim langsung'), findsOneWidget);
      expect(find.textContaining('Packing aman sebelum kirim'), findsOneWidget);
      expect(find.text(listing.description), findsOneWidget);
      expect(find.byIcon(Icons.more_vert), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('seller identity tap navigates by durable seller id', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final nav = _FakeNavigationHandler();
    final listing = _listing(
      id: 'listing-nav',
      sellerId: 'seller-nav-1',
      capabilities: _buyerCaps,
      media: _detailMedia(),
    );

    await tester.pumpWidget(
      _wrap(
        listing: listing,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-nav'),
          emailVerified: true,
        ),
        navigationHandler: nav,
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.text('@seller_user'), findsOneWidget);

    await tester.tap(find.text('@seller_user'));
    await tester.pumpAndSettle();

    expect(nav.lastUserId, 'seller-nav-1');
    expect(nav.lastUserId, isNot('seller_user'));
    expect(nav.lastUserId, isNot(contains('@')));
  });

  testWidgets('buyer without negotiation capability sees Chat + Buy Now only', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final listing = _listing(
      id: 'listing-no-nego',
      sellerId: 'seller-no-nego',
      capabilities: _buyerNoNegotiationCaps,
    );

    await tester.pumpWidget(
      _wrap(
        listing: listing,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-no-nego'),
          emailVerified: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Chat'), findsOneWidget);
    expect(find.text('Beli Sekarang'), findsOneWidget);
    expect(find.text('Ajukan Penawaran'), findsNothing);
    expect(find.text('Penjual tidak aktif'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'seller-trust inactive renders the explanatory banner instead of CTAs',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(320, 640));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final listing = _listing(
        id: 'listing-inactive',
        sellerId: 'seller-inactive',
        capabilities: _sellerInactiveCaps,
      );

      await tester.pumpWidget(
        _wrap(
          listing: listing,
          authState: AuthState.authenticated(
            _authUser(id: 'buyer-inactive'),
            emailVerified: true,
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Penjual tidak aktif'), findsOneWidget);
      expect(
        find.text('Transaksi baru tidak tersedia untuk seller ini.'),
        findsOneWidget,
      );
      expect(find.text('Chat'), findsNothing);
      expect(find.text('Beli Sekarang'), findsNothing);
      expect(find.text('Ajukan Penawaran'), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('owner detail hides buyer action bar and report action', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(360, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final listing = _listing(
      id: 'listing-owner',
      sellerId: 'seller-owner',
      capabilities: _ownerCaps,
    );

    await tester.pumpWidget(
      _wrap(
        listing: listing,
        authState: AuthState.authenticated(
          _authUser(id: 'seller-owner'),
          emailVerified: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Chat'), findsNothing);
    expect(find.text('Ajukan Penawaran'), findsNothing);
    expect(find.text('Beli Sekarang'), findsNothing);
    expect(find.text('Penjual tidak aktif'), findsNothing);
    // Owner still has share; report (more) is hidden for owners.
    expect(find.byTooltip('Bagikan'), findsOneWidget);
    expect(find.byIcon(Icons.more_vert), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('guest sees buyer affordances with no auth gate hiding the bar', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final listing = _listing(
      id: 'listing-guest',
      sellerId: 'seller-guest',
      capabilities: null,
      media: _detailMedia(),
    );

    await tester.pumpWidget(
      _wrap(listing: listing, authState: const AuthState.unauthenticated()),
    );
    // Bounded pumps: media shimmer never settles under fake async.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    // Model B: affordances visible; tapping routes to the canonical sign-in.
    expect(find.text('Chat'), findsOneWidget);
    expect(find.text('Ajukan Penawaran'), findsOneWidget);
    expect(find.text('Beli Sekarang'), findsOneWidget);
    expect(find.text('@seller_user'), findsOneWidget);
    expect(find.text('@Acme Farm'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('report flow opens the submission sheet for non-owners', (
    tester,
  ) async {
    // The report sheet is a fixed (non-scrollable) Column; give it a tall
    // surface so the reason selector + description fit without overflow.
    await tester.binding.setSurfaceSize(const Size(600, 2200));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final listing = _listing(
      id: 'listing-report',
      sellerId: 'seller-report',
      capabilities: _buyerNoNegotiationCaps,
    );

    await tester.pumpWidget(
      _wrap(
        listing: listing,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-report'),
          emailVerified: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Icons.more_vert));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Report'));
    await tester.pumpAndSettle();

    expect(find.text('Report Content'), findsOneWidget);
    expect(find.text('Reporting For Sale'), findsOneWidget);
    expect(find.text('Submit Report'), findsOneWidget);
  });
}