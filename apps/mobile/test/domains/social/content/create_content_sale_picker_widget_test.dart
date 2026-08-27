import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/seller_auctions_pager.dart';
import 'package:labuda/domains/commerce/catalog/listing/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/listing/presentation/providers/listing_controller.dart';
import 'package:labuda/domains/commerce/catalog/listing/presentation/providers/listing_providers.dart';
import 'package:labuda/domains/social/content/data/content_providers.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_occurrence.dart';
import 'package:labuda/domains/social/content/domain/repositories/content_repository.dart';
import 'package:labuda/domains/social/content/presentation/screens/create_content_screen.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _NoopLogger implements ILoggerService {
  const _NoopLogger();

  @override
  Future<Result<void>> clearLogs() async => Result.success(null);

  @override
  Future<Result<void>> debug(
    String message, {
    Map<String, dynamic>? extra,
  }) async => Result.success(null);

  @override
  Future<void> debugCallingGetCurrentUser() async {}

  @override
  Future<void> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async {}

  @override
  Future<void> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async {}

  @override
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  ) async {}

  @override
  Future<void> debugSync(String userId) async {}

  @override
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  ) async {}

  @override
  Future<void> debugSyncFailed(String userId, String? errorMessage) async {}

  @override
  Future<void> debugSyncSuccess(String userId) async {}

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => Result.success(null);

  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) async => Result.success(const <LogEntry>[]);

  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => Result.success(null);

  @override
  Future<Result<void>> info(
    String message, {
    Map<String, dynamic>? extra,
  }) async => Result.success(null);

  @override
  Future<void> log(String message, {LogLevel level = LogLevel.debug}) async {}

  @override
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
  }) async => Result.success(null);

  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) async => Result.success(null);

  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) async => Result.success(null);

  @override
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) async => Result.success(null);

  @override
  Future<Result<void>> setLogLevel(LogLevel level) async =>
      Result.success(null);

  @override
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) async => Result.success(null);
}

class _FakeAuthController extends AuthController {
  final AuthState _state;

  _FakeAuthController(this._state);

  @override
  AuthState build() => _state;
}

class _FakeListingRepository implements ListingRepository {
  final List<Listing> listings;

  _FakeListingRepository(this.listings);

  @override
  Future<Result<List<Listing>>> getListings(GetListingsParams params) async {
    return Result.success(listings);
  }

  @override
  Future<Result<Listing?>> getFixedPriceSaleById(
    String fixedPriceSaleId,
  ) async {
    for (final listing in listings) {
      if (listing.fixedPriceSaleId == fixedPriceSaleId) {
        return Result.success(listing);
      }
    }
    return Result.success(null);
  }

  @override
  Future<Result<List<Listing>>> getListingsByIds(
    List<String> listingIds,
  ) async {
    return Result.success(
      listings
          .where((listing) => listingIds.contains(listing.fixedPriceSaleId))
          .toList(),
    );
  }

  @override
  Future<Result<List<Listing>>> getSellerListings(
    String sellerId, {
    int page = 1,
    int pageSize = 20,
  }) async {
    return Result.success(listings);
  }

  @override
  Future<Result<Listing>> createListing(CreateListingRequest request) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<Listing>> updateFixedPriceSale(
    String fixedPriceSaleId,
    UpdateListingRequest request,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<void>> deleteFixedPriceSale(String fixedPriceSaleId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<Listing>> updateFixedPriceSaleStatus(
    String fixedPriceSaleId,
    ListingStatus status,
  ) async {
    throw UnimplementedError();
  }
}

class _FakeSellerAuctionsPagerController extends SellerAuctionsPagerController {
  @override
  SellerAuctionsPagerState build() {
    return SellerAuctionsPagerState(
      activeFilter: SellerAuctionFilter.all,
      auctions: [_activeAuction()],
      pageSize: 20,
      hasMore: false,
      isInitialLoading: false,
      isLoadMoreLoading: false,
      isRefreshing: false,
      initialError: null,
      loadMoreError: null,
      refreshError: null,
      ownerId: 'seller-1',
    );
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}

  @override
  Future<void> refresh() async {}

  @override
  Future<void> retryInitial() async {}

  @override
  Future<void> retryLoadMore() async {}
}

class _NoopNavigationHandler extends Fake implements NavigationHandler {
  @override
  void navigateToHome() {}
}

class _CountingContentRepository implements ContentRepository {
  _CountingContentRepository({this.nextResult});

  final ContentRepositoryResult<Content>? nextResult;
  int createContentCalls = 0;

  @override
  Future<ContentRepositoryResult<Content>> createContent({
    required String authorId,
    String? authorUsername,
    String? authorAvatarUrl,
    required String content,
    List<MediaEntity> media = const [],
    List<String> tags = const [],
    List<String> taggedUsers = const [],
    ContentSettings settings = const ContentSettings(),
    ContentLocation? location,
  }) async {
    createContentCalls += 1;
    return nextResult ?? ContentRepositoryResult.success(_submittedContent());
  }

  @override
  Future<ContentRepositoryResult<void>> deleteContent(String contentId) async {
    return ContentRepositoryResult<void>.error('not used');
  }

  @override
  Future<ContentRepositoryResult<Content>> getContentById(
    String contentId,
  ) async {
    return ContentRepositoryResult<Content>.error('not used');
  }

  @override
  Future<ContentRepositoryResult<List<Content>>> getContents({
    int? limit,
    int? offset,
    String? location,
    ContentStatus? status,
  }) async {
    return ContentRepositoryResult<List<Content>>.error('not used');
  }

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByAuthor(
    String authorId, {
    int? limit,
    int? offset,
  }) async {
    return ContentRepositoryResult<List<Content>>.error('not used');
  }

  @override
  Future<ContentRepositoryResult<ContentAuthorPage>> getContentsByAuthorPaged(
    String authorId, {
    int limit = 20,
    String? cursor,
  }) async {
    return ContentRepositoryResult<ContentAuthorPage>.error('not used');
  }

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByLocation({
    required String location,
    int? limit,
  }) async {
    return ContentRepositoryResult<List<Content>>.error('not used');
  }

  @override
  Future<ContentRepositoryResult<List<Content>>> getTrendingContents({
    int? limit,
  }) async {
    return ContentRepositoryResult<List<Content>>.error('not used');
  }

  @override
  Future<ContentRepositoryResult<Content>> updateContent(
    String contentId,
    Content content,
  ) async {
    return ContentRepositoryResult<Content>.error('not used');
  }

  @override
  Future<ContentRepositoryResult<ContentSearchResult>> searchContents({
    required String query,
    int? limit,
    int? offset,
    String? location,
  }) async {
    return ContentRepositoryResult<ContentSearchResult>.error('not used');
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Content _submittedContent({
  String id = 'content-1',
  String caption = 'submitted content',
}) {
  return Content(
    id: id,
    content: caption,
    author: const ContentAuthor(id: 'author-1', username: 'seller'),
    media: const [],
    tags: const [],
    settings: const ContentSettings(visibility: ContentVisibility.public),
    engagement: const ContentEngagement(),
    moderationInfo: const ContentModerationInfo(),
    createdAt: DateTime.utc(2026, 1, 1),
    updatedAt: DateTime.utc(2026, 1, 1),
  );
}

AuthUser _seller() {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: 'seller-1',
    createdAt: now,
    updatedAt: now,
    email: 'seller@example.com',
    username: 'seller',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
    hasSellerProfile: true,
    sellerSubscriptionStatus: 'active',
    hasMarketAuthority: true,
    sellerTier: SellerTier.sellerElite,
    isIdVerified: false,
    isFarmVerified: false,
    lifecycle: ContentLifecycle.active,
  );
}

Listing _activeListing() {
  final now = DateTime.utc(2026, 1, 1);
  return Listing(
    fixedPriceSaleId: 'sale-1',
    productId: 'product-1',
    title: 'Kohaku 50cm',
    description: 'Healthy koi',
    price: 500000,
    stock: 1,
    sellerId: 'seller-1',
    status: ListingStatus.active,
    createdAt: now,
    updatedAt: now,
  );
}

Auction _activeAuction() {
  final now = DateTime.utc(2026, 1, 1);
  return Auction(
    id: 'auction-1',
    sellerId: 'seller-1',
    sellerUsername: 'seller',
    sellerFarmName: 'Koi Farm',
    title: 'Koi Auction',
    description: 'Healthy auction koi',
    koiDetails: const KoiDetails(
      variety: 'Kohaku',
      sizeInCm: 20,
      ageInMonths: 12,
      gender: 'male',
    ),
    openingBid: 500000,
    currentBid: 600000,
    bidIncrement: 50000,
    startTime: now,
    endTime: now.add(const Duration(days: 1)),
    status: AuctionStatus.active,
    createdAt: now,
  );
}

Widget _wrap({
  ContentRepository? contentRepository,
  NavigationHandler? navigationHandler,
  S3Service? s3Service,
}) {
  final controller = ListingController(
    repository: _FakeListingRepository([_activeListing()]),
    logger: const _NoopLogger(),
  );

  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(
          AuthState.authenticated(_seller(), emailVerified: true),
        ),
      ),
      listingControllerProvider.overrideWithValue(controller),
      sellerAuctionsPagerProvider.overrideWith(
        () => _FakeSellerAuctionsPagerController(),
      ),
      if (contentRepository != null)
        contentRepositoryProvider.overrideWithValue(contentRepository),
      if (navigationHandler != null)
        navigationHandlerProvider.overrideWithValue(navigationHandler),
      if (s3Service != null) s3ServiceProvider.overrideWithValue(s3Service),
    ],
    child: const MaterialApp(home: CreateContentScreen()),
  );
}

void main() {
  testWidgets('sale picker selects preview and remove clears it', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap());
    await tester.pumpAndSettle();

    expect(find.text('Link'), findsNothing);
    expect(find.text('Lelang'), findsNothing);
    expect(find.text('URL'), findsNothing);
    expect(find.text('Produk Dijual'), findsWidgets);

    await tester.tap(find.text('Produk Dijual').first);
    await tester.pumpAndSettle();

    expect(find.text('Pilih Produk'), findsOneWidget);
    expect(find.text('Kohaku 50cm'), findsOneWidget);

    await tester.tap(find.text('Kohaku 50cm'));
    await tester.pumpAndSettle();

    expect(find.text('Kohaku 50cm'), findsOneWidget);
    expect(find.text('Produk Dijual'), findsWidgets);
  });

  testWidgets('auction picker selects a live auction into the composer', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap());
    await tester.pumpAndSettle();

    expect(find.text('Produk Dijual'), findsWidgets);

    await tester.tap(find.text('Produk Dijual').first);
    await tester.pumpAndSettle();

    expect(find.byType(TabBarView), findsOneWidget);

    await tester.drag(find.byType(TabBarView), const Offset(-400, 0));
    await tester.pumpAndSettle();

    expect(find.text('Koi Auction'), findsOneWidget);

    await tester.tap(find.text('Koi Auction'));
    await tester.pumpAndSettle();

    expect(find.text('Produk Tertaut'), findsOneWidget);
    expect(find.text('Koi Auction'), findsWidgets);
    expect(find.text('Produk Dijual'), findsWidgets);
  });

  testWidgets(
    'selection and replacement do not auto-publish until explicit submit',
    (tester) async {
      final repository = _CountingContentRepository();
      await tester.pumpWidget(
        _wrap(
          contentRepository: repository,
          navigationHandler: _NoopNavigationHandler(),
          s3Service: S3Service(),
        ),
      );
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextField).first, 'Draft caption');
      await tester.pump();
      tester.testTextInput.hide();
      await tester.pumpAndSettle();
      tester
          .widget<InkWell>(
            find.byKey(const ValueKey('toolbar-action-Produk Dijual')).first,
          )
          .onTap
          ?.call();
      await tester.pumpAndSettle();
      await tester.tap(find.text('Kohaku 50cm'));
      await tester.pumpAndSettle();

      expect(repository.createContentCalls, 0);
      expect(find.text('Produk Tertaut'), findsOneWidget);
      expect(find.text('Kohaku 50cm'), findsOneWidget);
      expect(find.byIcon(Icons.close), findsNWidgets(2));

      await tester.ensureVisible(
        find.byKey(const ValueKey('remove-section-Produk Tertaut')),
      );
      await tester.tap(
        find.byKey(const ValueKey('remove-section-Produk Tertaut')),
      );
      await tester.pumpAndSettle();

      expect(repository.createContentCalls, 0);
      expect(find.text('Produk Tertaut'), findsNothing);
      expect(find.byIcon(Icons.close), findsOneWidget);

      tester
          .widget<InkWell>(
            find.byKey(const ValueKey('toolbar-action-Produk Dijual')).first,
          )
          .onTap
          ?.call();
      await tester.pumpAndSettle();
      await tester.tap(find.text('Lelang'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Koi Auction'));
      await tester.pumpAndSettle();

      expect(repository.createContentCalls, 0);
      expect(find.text('Produk Tertaut'), findsOneWidget);
      expect(find.text('Koi Auction'), findsOneWidget);
      expect(find.text('Kohaku 50cm'), findsNothing);

      await tester.ensureVisible(
        find.byKey(const ValueKey('create-content-submit-button')),
      );
      await tester.tap(
        find.byKey(const ValueKey('create-content-submit-button')),
      );
      await tester.runAsync(() async {
        for (var i = 0; i < 40 && repository.createContentCalls == 0; i++) {
          await Future<void>.delayed(const Duration(milliseconds: 25));
        }
      });
      await tester.pumpAndSettle();

      expect(repository.createContentCalls, 1);
      await tester.pump(const Duration(seconds: 4));
      await tester.pumpAndSettle();
    },
  );

  testWidgets(
    'failed submit preserves the current draft and selected resource',
    (tester) async {
      final repository = _CountingContentRepository(
        nextResult: ContentRepositoryResult<Content>.error('backend rejected'),
      );
      await tester.pumpWidget(
        _wrap(
          contentRepository: repository,
          navigationHandler: _NoopNavigationHandler(),
          s3Service: S3Service(),
        ),
      );
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextField).first, 'Draft caption');
      await tester.pump();
      tester.testTextInput.hide();
      await tester.pumpAndSettle();

      tester
          .widget<InkWell>(
            find.byKey(const ValueKey('toolbar-action-Produk Dijual')).first,
          )
          .onTap
          ?.call();
      await tester.pumpAndSettle();
      await tester.tap(find.text('Lelang'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Koi Auction'));
      await tester.pumpAndSettle();

      final submitButton = tester.widget<TextButton>(
        find.byKey(const ValueKey('create-content-submit-button')),
      );
      expect(submitButton.onPressed, isNotNull);
      submitButton.onPressed?.call();
      await tester.runAsync(() async {
        for (var i = 0; i < 40 && repository.createContentCalls == 0; i++) {
          await Future<void>.delayed(const Duration(milliseconds: 25));
        }
      });
      await tester.pumpAndSettle();

      expect(repository.createContentCalls, 1);
      expect(find.text('Produk Tertaut'), findsOneWidget);
      expect(find.text('Koi Auction'), findsOneWidget);
      expect(
        tester.widget<TextField>(find.byType(TextField).first).controller?.text,
        'Draft caption',
      );
    },
  );
}
