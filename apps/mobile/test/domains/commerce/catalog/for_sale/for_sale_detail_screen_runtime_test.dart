import 'dart:collection';
import 'dart:io';

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
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_instance.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/promotion_providers.dart'
    show fixedPriceSaleActivePromotionsProvider;
import 'package:labuda/domains/user/preference/saved_item/models/saved_item_model.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/commerce/catalog/shared/shared.dart';
import 'package:labuda/shared/models/seller_identity_data.dart';
import '../../../../support/queued_image_http_client.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeSavedItemRepository extends SavedItemRepository {
  _FakeSavedItemRepository({this.initialSaved = false})
    : super(dio: Dio(BaseOptions(baseUrl: 'http://localhost')));

  bool initialSaved;
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
    provider: ShonaAuthProvider.email,
    lifecycle: ContentLifecycle.active,
  );
}

ForSale _listing({
  required String id,
  required String sellerId,
  CommerceViewerCapabilities? capabilities,
  List<MediaEntity> media = const [],
  String? publicOriginLine,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return ForSale(
    forSaleId: id,
    productId: 'product-1',
    title: 'Showa Koi 30cm',
    description: 'Premium showa',
    price: 1500000,
    stock: 1,
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
    isNegotiable: true,
    createdAt: now,
    updatedAt: now,
    variety: 'Kohaku',
    sizeCm: 30,
    ageMonths: 12,
    gender: 'male',
    breeder: 'Hiro',
    bloodline: 'Miyabi',
    certificates: const ['ownership', 'health'],
    origin: 'Bogor',
    preparationTime: PreparationTime.immediate,
    preparationNote: 'Packing aman sebelum kirim',
    shippingOptions: const [
      CommerceShippingOptionSummary(
        id: 'ship-1',
        name: 'traveleo',
        transportType: 'travel',
      ),
      CommerceShippingOptionSummary(
        id: 'ship-2',
        name: 'bus',
        transportType: 'bus',
      ),
    ],
    shippingOptionIds: const ['ship-1', 'ship-2'],
    sellerIdentity: publicOriginLine == null
        ? null
        : SellerIdentityData(
            userId: sellerId,
            username: 'seller_user',
            storeName: 'Acme Farm',
            avatarUrl: null,
            publicOriginLine: publicOriginLine,
            isSeller: true,
          ),
  );
}

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
  required SavedItemRepository savedItemRepository,
  Result<List<PromotionInstance>>? promotionResult,
  ForSale Function()? listingLoader,
  ThemeData? theme,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      savedItemRepositoryProvider.overrideWithValue(savedItemRepository),
      forSaleDetailProvider(
        listing.forSaleId,
      ).overrideWith((ref) async => listingLoader?.call() ?? listing),
      navigationHandlerProvider.overrideWithValue(_FakeNavigationHandler()),
      if (promotionResult != null)
        fixedPriceSaleActivePromotionsProvider(
          listing.forSaleId,
        ).overrideWith((ref) async => promotionResult),
    ],
    child: MaterialApp(
      theme: theme,
      home: ForSaleDetailScreen(forSaleId: listing.forSaleId),
    ),
  );
}

void main() {
  testWidgets('buyer detail screen shows buyer CTAs at 320 width', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final listing =
        _listing(
          id: 'listing-1',
          sellerId: 'seller-1',
          capabilities: const CommerceViewerCapabilities(
            role: 'buyer',
            canManage: false,
            canEdit: false,
            canPromote: false,
            canChat: true,
            canNegotiate: true,
            canBuy: true,
            canBid: false,
            canBuyNow: false,
          ),
          media: _detailMedia(),
          publicOriginLine: 'Magelang, Jawa Tengah',
        ).copyWith(
          location: const ForSaleLocation(
            city: 'Magelang',
            province: 'Jawa Tengah',
          ),
        );
    final savedRepo = _FakeSavedItemRepository(initialSaved: false);

    await tester.pumpWidget(
      _wrap(
        listing: listing,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-1'),
          emailVerified: true,
        ),
        savedItemRepository: savedRepo,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Detail Listing'), findsOneWidget);
    expect(find.byTooltip('Simpan'), findsOneWidget);
    expect(find.bySemanticsLabel('Simpan'), findsOneWidget);
    expect(find.text('Simpan'), findsNothing);
    expect(find.text('Tersimpan'), findsNothing);
    expect(find.text('Chat'), findsOneWidget);
    expect(find.text('Nego'), findsOneWidget);
    expect(find.text('Beli Sekarang'), findsOneWidget);
    expect(find.text('Kelola Listing'), findsNothing);
    expect(find.text('Pantau'), findsNothing);
    expect(find.text('Acme Farm'), findsOneWidget);
    expect(find.text('@seller_user'), findsOneWidget);
    expect(find.text('Magelang, Jawa Tengah'), findsOneWidget);
    expect(find.byType(RefreshIndicator), findsOneWidget);
    expect(find.byType(CommerceDetailMediaGallery), findsOneWidget);
    expect(find.byType(PageView), findsOneWidget);
    expect(find.text('Detail Produk'), findsOneWidget);
    expect(find.text('Origin'), findsNothing);
    expect(find.text('Opsi Pengiriman'), findsNothing);
    expect(find.textContaining('Travel'), findsNothing);
    expect(find.textContaining('Bus'), findsNothing);
    expect(find.text('Detail Produk'), findsOneWidget);
    expect(find.text('Varietas'), findsOneWidget);
    expect(find.text('Kohaku'), findsOneWidget);
    expect(find.text('Ukuran'), findsOneWidget);
    expect(find.text('30 cm'), findsOneWidget);
    expect(find.text('Usia'), findsOneWidget);
    expect(find.text('12 bulan'), findsOneWidget);
    expect(find.text('Kelamin'), findsOneWidget);
    expect(find.text('Jantan'), findsOneWidget);
    expect(find.text('Breeder'), findsOneWidget);
    expect(find.text('Hiro'), findsOneWidget);
    expect(find.text('Bloodline'), findsOneWidget);
    expect(find.text('Miyabi'), findsOneWidget);
    expect(find.text('Sertifikat'), findsOneWidget);
    expect(find.text('Kepemilikan, Kesehatan'), findsOneWidget);
    expect(find.text('Berdasarkan pernyataan seller'), findsOneWidget);
    expect(find.text('Siap kirim langsung'), findsOneWidget);
    expect(find.textContaining('Penjual siap mengirim'), findsOneWidget);
    expect(find.textContaining('Packing aman sebelum kirim'), findsOneWidget);
    expect(find.text(listing.description), findsOneWidget);
    final saveAction = find.byTooltip('Simpan');
    final shareAction = find.byTooltip('Bagikan');
    final moreAction = find.byTooltip('Lainnya');
    expect(tester.getSize(saveAction), const Size(48, 48));
    expect(tester.getSize(shareAction), const Size(48, 48));
    expect(tester.getSize(moreAction), const Size(48, 48));
    final saveRect = tester.getRect(saveAction);
    final shareRect = tester.getRect(shareAction);
    final moreRect = tester.getRect(moreAction);
    expect(saveRect.width, shareRect.width);
    expect(shareRect.width, moreRect.width);
    expect(saveRect.height, shareRect.height);
    expect(shareRect.height, moreRect.height);
    expect(saveRect.center.dy, shareRect.center.dy);
    expect(shareRect.center.dy, moreRect.center.dy);
    expect(shareRect.left, saveRect.right);
    expect(moreRect.left, shareRect.right);
    expect(
      tester
          .widget<Icon>(
            find.descendant(
              of: saveAction,
              matching: find.byIcon(Icons.bookmark_border_outlined),
            ),
          )
          .size,
      20,
    );
    expect(
      tester
          .widget<Icon>(
            find.descendant(
              of: shareAction,
              matching: find.byIcon(Icons.share_outlined),
            ),
          )
          .size,
      20,
    );
    expect(
      tester
          .widget<Icon>(
            find.descendant(
              of: moreAction,
              matching: find.byIcon(Icons.more_vert),
            ),
          )
          .size,
      20,
    );
    expect(
      tester
          .widget<Icon>(
            find.descendant(
              of: saveAction,
              matching: find.byIcon(Icons.bookmark_border_outlined),
            ),
          )
          .color,
      tester
          .widget<Icon>(
            find.descendant(
              of: shareAction,
              matching: find.byIcon(Icons.share_outlined),
            ),
          )
          .color,
    );
    expect(
      tester
          .widget<Icon>(
            find.descendant(
              of: shareAction,
              matching: find.byIcon(Icons.share_outlined),
            ),
          )
          .color,
      tester
          .widget<Icon>(
            find.descendant(
              of: moreAction,
              matching: find.byIcon(Icons.more_vert),
            ),
          )
          .color,
    );
    final storeTop = tester.getTopLeft(find.text('Acme Farm'));
    final handleTop = tester.getTopLeft(find.text('@seller_user'));
    final originTop = tester.getTopLeft(find.text('Magelang, Jawa Tengah'));
    expect(handleTop.dy, greaterThan(storeTop.dy));
    expect(originTop.dy, greaterThan(handleTop.dy));

    await tester.drag(find.byType(PageView), const Offset(-400, 0));
    await tester.pumpAndSettle();

    expect(find.byType(CommerceDetailMediaGallery), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('seller origin hides when location is absent', (tester) async {
    await tester.binding.setSurfaceSize(const Size(360, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final listing = _listing(
      id: 'listing-origin-missing',
      sellerId: 'seller-origin-missing',
      capabilities: const CommerceViewerCapabilities(
        role: 'buyer',
        canManage: false,
        canEdit: false,
        canPromote: false,
        canChat: true,
        canNegotiate: false,
        canBuy: true,
        canBid: false,
        canBuyNow: false,
      ),
    );

    await tester.pumpWidget(
      _wrap(
        listing: listing,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-origin-missing'),
          emailVerified: true,
        ),
        savedItemRepository: _FakeSavedItemRepository(initialSaved: false),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Acme Farm'), findsOneWidget);
    expect(find.text('@seller_user'), findsOneWidget);
    expect(find.text('Magelang, Jawa Tengah'), findsNothing);
    expect(find.text('Detail Produk'), findsOneWidget);
    expect(find.text('Origin'), findsNothing);
    expect(find.text('Opsi Pengiriman'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'buyer detail screen keeps app bar actions aligned in dark theme',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(360, 640));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final listing =
          _listing(
            id: 'listing-dark',
            sellerId: 'seller-dark',
            capabilities: const CommerceViewerCapabilities(
              role: 'buyer',
              canManage: false,
              canEdit: false,
              canPromote: false,
              canChat: true,
              canNegotiate: false,
              canBuy: true,
              canBid: false,
              canBuyNow: false,
            ),
            media: _detailMedia(),
            publicOriginLine: 'Magelang, Jawa Tengah',
          ).copyWith(
            location: const ForSaleLocation(
              city: 'Magelang',
              province: 'Jawa Tengah',
            ),
          );
      final savedRepo = _FakeSavedItemRepository(initialSaved: false);
      final darkTheme = ThemeData.dark();

      await tester.pumpWidget(
        _wrap(
          listing: listing,
          authState: AuthState.authenticated(
            _authUser(id: 'buyer-dark'),
            emailVerified: true,
          ),
          savedItemRepository: savedRepo,
          theme: darkTheme,
        ),
      );
      await tester.pumpAndSettle();

      final saveAction = find.byTooltip('Simpan');
      final shareAction = find.byTooltip('Bagikan');
      final moreAction = find.byTooltip('Lainnya');
      expect(tester.getSize(saveAction), const Size(48, 48));
      expect(tester.getSize(shareAction), const Size(48, 48));
      expect(tester.getSize(moreAction), const Size(48, 48));
      expect(
        tester
            .widget<Icon>(
              find.descendant(
                of: saveAction,
                matching: find.byIcon(Icons.bookmark_border_outlined),
              ),
            )
            .color,
        darkTheme.colorScheme.onSurfaceVariant,
      );
      expect(
        tester
            .widget<Icon>(
              find.descendant(
                of: shareAction,
                matching: find.byIcon(Icons.share_outlined),
              ),
            )
            .color,
        darkTheme.colorScheme.onSurfaceVariant,
      );
      expect(
        tester
            .widget<Icon>(
              find.descendant(
                of: moreAction,
                matching: find.byIcon(Icons.more_vert),
              ),
            )
            .color,
        darkTheme.colorScheme.onSurfaceVariant,
      );
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('saved state switches the app bar action icon to Tersimpan', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(360, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final listing = _listing(
      id: 'listing-2',
      sellerId: 'seller-2',
      capabilities: const CommerceViewerCapabilities(
        role: 'buyer',
        canManage: false,
        canEdit: false,
        canPromote: false,
        canChat: true,
        canNegotiate: false,
        canBuy: true,
        canBid: false,
        canBuyNow: false,
      ),
    );
    final savedRepo = _FakeSavedItemRepository(initialSaved: true);

    await tester.pumpWidget(
      _wrap(
        listing: listing,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-2'),
          emailVerified: true,
        ),
        savedItemRepository: savedRepo,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byTooltip('Tersimpan'), findsOneWidget);
    expect(find.bySemanticsLabel('Tersimpan'), findsOneWidget);
    expect(find.text('Simpan'), findsNothing);
    expect(find.text('Tersimpan'), findsNothing);
    expect(find.text('Chat'), findsOneWidget);
    expect(find.text('Beli Sekarang'), findsOneWidget);
    expect(savedRepo.isSavedCalls, greaterThan(0));
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'refreshed listing media replaces a failed first image without resetting the controller',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(360, 640));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      var listing =
          _listing(
            id: 'listing-media-refresh',
            sellerId: 'seller-media-refresh',
            capabilities: const CommerceViewerCapabilities(
              role: 'buyer',
              canManage: false,
              canEdit: false,
              canPromote: false,
              canChat: true,
              canNegotiate: true,
              canBuy: true,
              canBid: false,
              canBuyNow: false,
            ),
            media: [
              MediaEntity(
                id: 'listing-media-1',
                originalUrl:
                    'https://cdn.example.com/gallery/listing-1.jpg?X-Amz-Signature=one',
                type: MediaType.image,
                createdAt: DateTime.utc(2026, 1, 1),
              ),
              MediaEntity(
                id: 'listing-media-2',
                originalUrl:
                    'https://cdn.example.com/gallery/listing-2.jpg?X-Amz-Signature=two',
                type: MediaType.image,
                createdAt: DateTime.utc(2026, 1, 1),
              ),
            ],
            publicOriginLine: 'Magelang, Jawa Tengah',
          ).copyWith(
            location: const ForSaleLocation(
              city: 'Magelang',
              province: 'Jawa Tengah',
            ),
          );

      const firstRefreshUrl =
          'https://cdn.example.com/gallery/listing-1.jpg?X-Amz-Signature=one';
      const refreshedFirstUrl =
          'https://cdn.example.com/gallery/listing-1.jpg?X-Amz-Signature=two';
      const secondRefreshUrl =
          'https://cdn.example.com/gallery/listing-2.jpg?X-Amz-Signature=two';

      final responders = <String, Queue<QueuedImageResponseSpec>>{
        firstRefreshUrl: Queue<QueuedImageResponseSpec>.of([
          QueuedImageResponseSpec.failure(),
        ]),
        refreshedFirstUrl: Queue<QueuedImageResponseSpec>.of([
          QueuedImageResponseSpec.success(onePxPngBytes),
        ]),
        secondRefreshUrl: Queue<QueuedImageResponseSpec>.of([
          QueuedImageResponseSpec.success(onePxPngBytes),
        ]),
      };

      await HttpOverrides.runZoned(() async {
        final imageCache = PaintingBinding.instance.imageCache;
        imageCache.clear();
        imageCache.clearLiveImages();

        await tester.pumpWidget(
          _wrap(
            listing: listing,
            authState: AuthState.authenticated(
              _authUser(id: 'buyer-media-refresh'),
              emailVerified: true,
            ),
            savedItemRepository: _FakeSavedItemRepository(initialSaved: false),
            listingLoader: () => listing,
          ),
        );
        await tester.pumpAndSettle();
        await tester.pump(const Duration(seconds: 1));
        await tester.pumpAndSettle();

        final pageViewBefore = tester.widget<PageView>(find.byType(PageView));
        final galleryImageFinder = find.descendant(
          of: find.byType(CommerceDetailMediaGallery),
          matching: find.byType(Image),
        );
        expect(find.text('Magelang, Jawa Tengah'), findsOneWidget);
        final initialUrls = tester
            .widgetList<Image>(galleryImageFinder)
            .map((image) => (image.image as NetworkImage).url)
            .toList();
        expect(initialUrls, isNotEmpty);
        expect(initialUrls, contains(firstRefreshUrl));
        expect(find.byIcon(Icons.image_not_supported), findsOneWidget);

        listing = listing.copyWith(
          media: [
            MediaEntity(
              id: 'listing-media-1',
              originalUrl: refreshedFirstUrl,
              type: MediaType.image,
              createdAt: DateTime.utc(2026, 1, 1),
            ),
            MediaEntity(
              id: 'listing-media-2',
              originalUrl: secondRefreshUrl,
              type: MediaType.image,
              createdAt: DateTime.utc(2026, 1, 1),
            ),
          ],
        );

        await tester
            .widget<RefreshIndicator>(find.byType(RefreshIndicator))
            .onRefresh();
        await tester.pumpAndSettle();
        await tester.pump(const Duration(seconds: 1));
        await tester.pumpAndSettle();

        final pageViewAfter = tester.widget<PageView>(find.byType(PageView));
        expect(pageViewBefore.controller, pageViewAfter.controller);
        expect(find.byType(CommerceDetailMediaGallery), findsOneWidget);
        final refreshedUrls = tester
            .widgetList<Image>(galleryImageFinder)
            .map((image) => (image.image as NetworkImage).url)
            .toList();
        expect(refreshedUrls, isNotEmpty);
        expect(refreshedUrls, contains(refreshedFirstUrl));

        await tester.drag(find.byType(PageView), const Offset(-400, 0));
        await tester.pumpAndSettle();
        await tester.pump(const Duration(seconds: 1));
        await tester.pumpAndSettle();

        final secondPageUrls = tester
            .widgetList<Image>(galleryImageFinder)
            .map((image) => (image.image as NetworkImage).url)
            .toList();
        expect(secondPageUrls, isNotEmpty);
        expect(secondPageUrls, contains(secondRefreshUrl));
        expect(tester.takeException(), isNull);
      }, createHttpClient: (_) => QueuedImageHttpClient(responders));
    },
  );

  testWidgets('save state survives screen reconstruction', (tester) async {
    await tester.binding.setSurfaceSize(const Size(360, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final listing = _listing(
      id: 'listing-2b',
      sellerId: 'seller-2b',
      capabilities: const CommerceViewerCapabilities(
        role: 'buyer',
        canManage: false,
        canEdit: false,
        canPromote: false,
        canChat: true,
        canNegotiate: false,
        canBuy: true,
        canBid: false,
        canBuyNow: false,
      ),
    );
    final savedRepo = _FakeSavedItemRepository(initialSaved: false);

    await tester.pumpWidget(
      _wrap(
        listing: listing,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-2b'),
          emailVerified: true,
        ),
        savedItemRepository: savedRepo,
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byTooltip('Simpan'), findsOneWidget);
    expect(find.bySemanticsLabel('Simpan'), findsOneWidget);
    expect(find.text('Simpan'), findsNothing);

    await tester.tap(find.byIcon(Icons.bookmark_border_outlined));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Tersimpan'), findsOneWidget);
    expect(find.bySemanticsLabel('Tersimpan'), findsOneWidget);
    expect(find.text('Tersimpan'), findsNothing);

    await tester.pumpWidget(
      _wrap(
        listing: listing,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-2b'),
          emailVerified: true,
        ),
        savedItemRepository: savedRepo,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Simpan'), findsNothing);
    expect(find.byTooltip('Tersimpan'), findsOneWidget);
    expect(find.bySemanticsLabel('Tersimpan'), findsOneWidget);
    expect(find.text('Tersimpan'), findsNothing);
    expect(savedRepo.addCalls, greaterThan(0));
    expect(tester.takeException(), isNull);
  });

  testWidgets('empty product fields stay hidden on the detail screen', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final listing =
        _listing(
          id: 'listing-empty',
          sellerId: 'seller-empty',
          capabilities: const CommerceViewerCapabilities(
            role: 'buyer',
            canManage: false,
            canEdit: false,
            canPromote: false,
            canChat: true,
            canNegotiate: false,
            canBuy: true,
            canBid: false,
            canBuyNow: false,
          ),
        ).copyWith(
          variety: '',
          sizeCm: 0,
          ageMonths: 0,
          gender: 'unknown',
          breeder: '',
          bloodline: '',
          certificates: const [],
          origin: '',
          shippingOptions: const [],
          preparationTime: PreparationTime.immediate,
          preparationNote: null,
          description: '',
        );

    await tester.pumpWidget(
      _wrap(
        listing: listing,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-empty'),
          emailVerified: true,
        ),
        savedItemRepository: _FakeSavedItemRepository(initialSaved: false),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Detail Produk'), findsOneWidget);
    expect(find.text('Siap kirim langsung'), findsOneWidget);
    expect(find.text('Varietas'), findsNothing);
    expect(find.text('Ukuran'), findsNothing);
    expect(find.text('Usia'), findsNothing);
    expect(find.text('Kelamin'), findsNothing);
    expect(find.text('Breeder'), findsNothing);
    expect(find.text('Bloodline'), findsNothing);
    expect(find.text('Sertifikat'), findsNothing);
    expect(find.text('Opsi Pengiriman'), findsNothing);
    expect(find.text('Deskripsi'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('owner detail screen hides save/watch/report and shows promote', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(360, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final listing = _listing(
      id: 'listing-3',
      sellerId: 'seller-3',
      capabilities: const CommerceViewerCapabilities(
        role: 'owner',
        canManage: true,
        canEdit: true,
        canPromote: true,
        canChat: false,
        canNegotiate: false,
        canBuy: false,
        canBid: false,
        canBuyNow: false,
      ),
    );
    final savedRepo = _FakeSavedItemRepository(initialSaved: false);

    await tester.pumpWidget(
      _wrap(
        listing: listing,
        authState: AuthState.authenticated(
          _authUser(id: 'seller-3'),
          emailVerified: true,
        ),
        savedItemRepository: savedRepo,
        promotionResult: Result.success(const <PromotionInstance>[]),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Kelola Listing'), findsOneWidget);
    expect(find.text('Promosi'), findsOneWidget);
    expect(find.text('Dipromosikan'), findsNothing);
    expect(find.text('Simpan'), findsNothing);
    expect(find.text('Tersimpan'), findsNothing);
    expect(find.text('Pantau'), findsNothing);
    final shareAction = find.byTooltip('Bagikan');
    final moreAction = find.byTooltip('Lainnya');
    expect(tester.getSize(shareAction), const Size(48, 48));
    expect(tester.getSize(moreAction), const Size(48, 48));
    expect(
      tester
          .widget<Icon>(
            find.descendant(
              of: shareAction,
              matching: find.byIcon(Icons.share_outlined),
            ),
          )
          .color,
      ThemeData.light().colorScheme.onSurfaceVariant,
    );
    expect(
      tester
          .widget<Icon>(
            find.descendant(
              of: moreAction,
              matching: find.byIcon(Icons.more_vert),
            ),
          )
          .color,
      ThemeData.light().colorScheme.onSurfaceVariant,
    );
    expect(find.text('Chat'), findsNothing);
    expect(find.text('Nego'), findsNothing);
    expect(find.text('Beli Sekarang'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('report flow opens the submission sheet for non-owners', (
    tester,
  ) async {
    final listing = _listing(
      id: 'listing-4',
      sellerId: 'seller-4',
      capabilities: const CommerceViewerCapabilities(
        role: 'buyer',
        canManage: false,
        canEdit: false,
        canPromote: false,
        canChat: true,
        canNegotiate: false,
        canBuy: true,
        canBid: false,
        canBuyNow: false,
      ),
    );

    await tester.pumpWidget(
      _wrap(
        listing: listing,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-4'),
          emailVerified: true,
        ),
        savedItemRepository: _FakeSavedItemRepository(initialSaved: false),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Icons.more_vert).first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Laporkan produk'));
    await tester.pumpAndSettle();

    expect(find.text('Report Content'), findsOneWidget);
    expect(find.text('Reporting Fixed-Price Sale'), findsOneWidget);
    expect(find.text('Kirim Laporan'), findsOneWidget);
  });
}
