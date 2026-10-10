import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/shared/widgets/bottom_action_bar.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/core/common/types/preparation_time.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart'
    show forSaleDetailProvider;
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/user_data_provider.dart';
import 'package:hishumi/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:hishumi/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:hishumi/domains/user/preference/saved_item/models/saved_item_model.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/domains/social/content/domain/entities/content.dart';
import 'package:hishumi/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeSavedItemRepository extends SavedItemRepository {
  _FakeSavedItemRepository()
    : super(dio: Dio(BaseOptions(baseUrl: 'http://localhost')));

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
  ForSaleStatus status = ForSaleStatus.active,
  ContentLifecycle sellerTrustLifecycle = ContentLifecycle.active,
  String? publicOriginLine,
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
    publicOriginLine: publicOriginLine,
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: sellerTrustLifecycle,
    sellerTier: 'pro',
    viewerCapabilities: capabilities,
    media: media,
     status: status,
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
    preparationTime: PreparationTime.days1_3,
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
      id: 'forSale-media-1',
      originalUrl:
          'https://cdn.example.com/gallery/forSale-1.jpg?X-Amz-Signature=one',
      type: MediaType.image,
      createdAt: now,
    ),
    MediaEntity(
      id: 'forSale-media-2',
      originalUrl:
          'https://cdn.example.com/gallery/forSale-2.jpg?X-Amz-Signature=two',
      type: MediaType.image,
      createdAt: now,
    ),
  ];
}

Widget _wrap({
  required ForSale forSale,
  required AuthState authState,
  ForSale Function()? listingLoader,
  ThemeData? theme,
  _FakeNavigationHandler? navigationHandler,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      savedItemRepositoryProvider.overrideWithValue(_FakeSavedItemRepository()),
      forSaleDetailProvider(
        forSale.forSaleId,
      ).overrideWith((ref) async => listingLoader?.call() ?? forSale),
      userDataProvider.overrideWith(
        (ref, userId) async => _authUser(id: userId),
      ),
      navigationHandlerProvider.overrideWithValue(
        navigationHandler ?? _FakeNavigationHandler(),
      ),
    ],
    child: MaterialApp.router(
      theme: theme,
      routerConfig: GoRouter(
        initialLocation: '/',
        routes: [
          GoRoute(
            path: '/',
            builder: (context, state) => ForSaleDetailScreen(
              forSaleId: forSale.forSaleId,
            ),
          ),
          GoRoute(
            path: '/report',
            builder: (context, state) => const Scaffold(
              body: Column(
                children: [Text('Reporting For Sale'), Text('Submit Report')],
              ),
            ),
          ),
        ],
      ),
    ),
  );
}

void main() {
  testWidgets(
    'buyer detail renders capability-driven action bar (Chat/Nego/Buy Now)',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(320, 640));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final forSale = _listing(
        id: 'forSale-1',
        sellerId: 'seller-1',
        capabilities: _buyerCaps,
        media: _detailMedia(),
        publicOriginLine: 'Magelang, Jawa Tengah',
      );

      await tester.pumpWidget(
        _wrap(
          forSale: forSale,
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

      expect(find.text('Detail ForSale'), findsOneWidget);
      expect(find.text('Chat'), findsOneWidget);
      expect(find.text('Nego'), findsOneWidget);
      expect(find.text('Tawar'), findsOneWidget);
      expect(find.text('Beli Sekarang'), findsOneWidget);
      expect(
        find.ancestor(of: find.text('Nego'), matching: find.byType(Row)),
        findsWidgets,
      );
      final priceRows = tester.widgetList<Row>(
        find.ancestor(of: find.text('Nego'), matching: find.byType(Row)),
      );
      expect(priceRows, isNotEmpty);
      expect(find.text('Penjual tidak aktif'), findsNothing);
      expect(find.text('@seller_user', skipOffstage: false), findsOneWidget);
      expect(find.text('Acme Farm', skipOffstage: false), findsOneWidget);
      // Buyer-facing shipping origin of the listing, straight from the detail
      // wire — the buyer must be able to see where the goods ship from.
      expect(
        find.text('Magelang, Jawa Tengah', skipOffstage: false),
        findsOneWidget,
      );
      expect(
        find.byIcon(Icons.location_on_outlined, skipOffstage: false),
        findsOneWidget,
      );
      expect(find.byType(PageView), findsOneWidget);
      // Section cards below the first viewport are mounted but laid out
      // lazily — assert against the full element tree.
      expect(
        find.text('Estimasi siap kirim: 1–3 hari', skipOffstage: false),
        findsOneWidget,
      );
      // NO REGRESS: the preparation-note concept is purged end-to-end; no note
      // copy may be rendered from any payload (even a smuggle attempt).
      expect(
        find.textContaining('Packing aman sebelum kirim', skipOffstage: false),
        findsNothing,
      );
      expect(
        find.text(forSale.description, skipOffstage: false),
        findsOneWidget,
      );
      expect(find.byIcon(Icons.more_vert), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('listing without a resolved origin hides the line', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final forSale = _listing(
      id: 'forSale-no-origin',
      sellerId: 'seller-1',
      capabilities: _buyerCaps,
      media: _detailMedia(),
    );

    await tester.pumpWidget(
      _wrap(
        forSale: forSale,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-1'),
          emailVerified: true,
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    // Missing truth is HIDDEN, never fabricated — no origin line, no icon.
    expect(
      find.byIcon(Icons.location_on_outlined, skipOffstage: false),
      findsNothing,
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('seller identity tap navigates by durable seller id', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final nav = _FakeNavigationHandler();
    final forSale = _listing(
      id: 'forSale-nav',
      sellerId: 'seller-nav-1',
      capabilities: _buyerCaps,
      media: _detailMedia(),
    );

    await tester.pumpWidget(
      _wrap(
        forSale: forSale,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-nav'),
          emailVerified: true,
        ),
        navigationHandler: nav,
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.text('@seller_user', skipOffstage: false), findsOneWidget);

    // The seller card sits below the fold — scroll it into the viewport
    // before tapping (the identity assert above resolves from the full tree).
    await tester.ensureVisible(find.text('@seller_user', skipOffstage: false));
    await tester.pump();
    await tester.tap(find.text('@seller_user'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(nav.lastUserId, 'seller-nav-1');
    expect(nav.lastUserId, isNot('seller_user'));
    expect(nav.lastUserId, isNot(contains('@')));
  });

  testWidgets('buyer without negotiation capability sees Chat + Buy Now only', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final forSale = _listing(
      id: 'forSale-no-nego',
      sellerId: 'seller-no-nego',
      capabilities: _buyerNoNegotiationCaps,
      isNegotiable: false,
    );

    await tester.pumpWidget(
      _wrap(
        forSale: forSale,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-no-nego'),
          emailVerified: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Chat'), findsOneWidget);
    expect(find.text('Beli Sekarang'), findsOneWidget);
    expect(find.text('Nego'), findsNothing);
    expect(find.text('Penjual tidak aktif'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'seller-trust inactive exposes no commerce CTA',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(320, 640));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final forSale = _listing(
        id: 'forSale-inactive',
        sellerId: 'seller-inactive',
        capabilities: _sellerInactiveCaps,
        sellerTrustLifecycle: ContentLifecycle.unavailable,
      );

      await tester.pumpWidget(
        _wrap(
          forSale: forSale,
          authState: AuthState.authenticated(
            _authUser(id: 'buyer-inactive'),
            emailVerified: true,
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Penjual tidak aktif'), findsNothing);
      expect(
        find.text('Transaksi baru tidak tersedia untuk seller ini.'),
        findsNothing,
      );
      expect(find.text('Chat'), findsNothing);
      expect(find.text('Beli Sekarang'), findsNothing);
      expect(
        find.descendant(
          of: find.byType(BottomActionBar),
          matching: find.text('Nego'),
        ),
        findsNothing,
      );
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'all-false capabilities with ACTIVE seller trust never show the banner',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(320, 640));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      // NEGATIVE PROOF: an all-false capability set also occurs when the
      // viewer identity never reached the backend. It must NOT be presented
      // as "Penjual tidak aktif" — the seller-trust axis alone owns that label.
      final forSale = _listing(
        id: 'forSale-caps-only',
        sellerId: 'seller-caps-only',
        capabilities: _sellerInactiveCaps,
        sellerTrustLifecycle: ContentLifecycle.active,
      );

      await tester.pumpWidget(
        _wrap(
          forSale: forSale,
          authState: AuthState.authenticated(
            _authUser(id: 'buyer-caps-only'),
            emailVerified: true,
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Penjual tidak aktif'), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('owner detail hides buyer action bar and report action', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(360, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final forSale = _listing(
      id: 'forSale-owner',
      sellerId: 'seller-owner',
      capabilities: _ownerCaps,
      isNegotiable: false,
    );

    await tester.pumpWidget(
      _wrap(
        forSale: forSale,
        authState: AuthState.authenticated(
          _authUser(id: 'seller-owner'),
          emailVerified: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Chat'), findsNothing);
    expect(find.text('Nego'), findsNothing);
    expect(find.text('Beli Sekarang'), findsNothing);
    expect(find.text('Penjual tidak aktif'), findsNothing);
    // Owner still has share; report (more) is hidden for owners.
    expect(find.byTooltip('Bagikan'), findsOneWidget);
    expect(find.byIcon(Icons.more_vert), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('guest sees no commerce affordance without capability', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final forSale = _listing(
      id: 'forSale-guest',
      sellerId: 'seller-guest',
      capabilities: null,
      media: _detailMedia(),
    );

    await tester.pumpWidget(
      _wrap(forSale: forSale, authState: const AuthState.unauthenticated()),
    );
    // Bounded pumps: media shimmer never settles under fake async.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(
      find.descendant(
        of: find.byType(BottomActionBar),
        matching: find.text('Chat'),
      ),
      findsNothing,
    );
    expect(
      find.descendant(
        of: find.byType(BottomActionBar),
        matching: find.text('Nego'),
      ),
      findsNothing,
    );
    expect(
      find.descendant(
        of: find.byType(BottomActionBar),
        matching: find.text('Beli Sekarang'),
      ),
      findsNothing,
    );
    expect(find.text('@seller_user', skipOffstage: false), findsOneWidget);
    expect(find.text('Acme Farm', skipOffstage: false), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('terminal For Sale hides Save and commerce Share', (tester) async {
    for (final status in [ForSaleStatus.sold, ForSaleStatus.withdrawn]) {
      final listing = _listing(
        id: 'terminal-${status.name}',
        sellerId: 'seller-terminal',
        status: status,
        capabilities: _buyerNoNegotiationCaps,
      );
      await tester.pumpWidget(
        KeyedSubtree(
          key: ValueKey('terminal-${status.name}'),
          child: _wrap(
            forSale: listing,
            authState: AuthState.authenticated(
              _authUser(id: 'buyer-terminal'),
              emailVerified: true,
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 300));
      expect(find.byTooltip('Bagikan'), findsNothing);
      expect(find.byIcon(Icons.bookmark_border), findsNothing);
      expect(find.byIcon(Icons.bookmark), findsNothing);
    }
  });

  testWidgets('Nego opens the offer sheet ON detail — no chat navigation', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(360, 740));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final forSale = _listing(
      id: 'forSale-nego-sheet',
      sellerId: 'seller-nego-sheet',
      capabilities: _buyerCaps,
    );

    await tester.pumpWidget(
      _wrap(
        forSale: forSale,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-nego-sheet'),
          emailVerified: true,
        ),
      ),
    );
    // Bounded pumps: media shimmer never settles under fake async.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    await tester.tap(find.text('Tawar'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    // The offer input opens on the detail surface — no navigation to chat,
    // no silently auto-sent product card (autoOpenNegotiation is purged).
    expect(find.text('Negosiasi Harga'), findsOneWidget);
    expect(find.text('Masukkan harga tawaran Anda'), findsOneWidget);
    expect(find.text('Kirim Penawaran'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('report flow opens the full-screen submission for non-owners', (
    tester,
  ) async {
    // The report form is a full screen; give it a tall surface so the reason
    // selector + description fit without overflow.
    await tester.binding.setSurfaceSize(const Size(600, 2200));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final forSale = _listing(
      id: 'forSale-report',
      sellerId: 'seller-report',
      capabilities: _buyerNoNegotiationCaps,
    );

    await tester.pumpWidget(
      _wrap(
        forSale: forSale,
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

    // Full-screen report form (owner decision: substantial forms belong on a
    // full screen, not in a bottom sheet).
    expect(find.text('Reporting For Sale'), findsOneWidget);
    expect(find.text('Submit Report'), findsOneWidget);
  });
}
