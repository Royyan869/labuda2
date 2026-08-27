import 'dart:collection';
import 'dart:io';

import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart'
    show auctionRepositoryProvider, auctionWatchRepositoryProvider;
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/remote/auction_remote_datasource.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/repositories/auction_repository_impl.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_recommendation_providers.dart'
    show ownerOtherAuctionsProvider, similarAuctionsProvider;
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart';
import 'package:labuda/domains/user/preference/saved_item/models/saved_item_model.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/firebase_principal.dart';
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
      targetType: targetType == 'listing'
          ? TargetType.listing
          : TargetType.auction,
      targetId: targetId,
      intentType: targetType == 'listing'
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

class _FakeAuctionNotifier extends AuctionNotifier {
  _FakeAuctionNotifier(this._state);

  final AuctionNotifierState _state;
  int loadAuctionDetailsCalls = 0;

  @override
  AuctionNotifierState build() => _state;

  @override
  Future<void> loadAuctionDetails(String auctionId) async {
    loadAuctionDetailsCalls += 1;
  }
}

class _NoopLogger implements ILoggerService {
  Future<Result<void>> _ok() async => Result.success(null);

  @override
  Future<Result<void>> debug(String message, {Map<String, dynamic>? extra}) =>
      _ok();

  @override
  Future<Result<void>> info(String message, {Map<String, dynamic>? extra}) =>
      _ok();

  @override
  Future<Result<void>> warning(String message, {Map<String, dynamic>? extra}) =>
      _ok();

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => _ok();

  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => _ok();

  @override
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) => _ok();

  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) => _ok();

  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) => _ok();

  @override
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
  }) => _ok();

  @override
  Future<Result<void>> setLogLevel(LogLevel level) => _ok();

  @override
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  ) async {}

  @override
  Future<Result<void>> log(String message, {LogLevel level = LogLevel.debug}) =>
      _ok();

  @override
  Future<Result<void>> clearLogs() => _ok();

  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) async => Result.success(const []);

  @override
  Future<void> debugSync(String userId) async {}

  @override
  Future<void> debugSyncSuccess(String userId) async {}

  @override
  Future<void> debugSyncFailed(String userId, String? errorMessage) async {}

  @override
  Future<void> debugCallingGetCurrentUser() async {}

  @override
  Future<void> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async {}

  @override
  Future<void> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async {}

  @override
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  ) async {}
}

class _NoopApiClient implements ApiClient {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeAuctionWatchRepository implements AuctionWatchRepository {
  const _FakeAuctionWatchRepository();

  @override
  Future<RepositoryResult<AuctionWatcher>> watchAuction({
    required String auctionId,
    required String userId,
    bool notifyOnBid = true,
    bool notifyOnEndingSoon = true,
    bool notifyOnEnded = true,
  }) async {
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
    return RepositoryResult.success(null);
  }

  @override
  Future<RepositoryResult<bool>> isWatching({
    required String auctionId,
    required String userId,
  }) async {
    return RepositoryResult.success(false);
  }

  @override
  Future<RepositoryResult<AuctionWatchStats>> getWatchStats({
    required String auctionId,
    required String currentUserId,
  }) async {
    return RepositoryResult.success(
      AuctionWatchStats(
        auctionId: auctionId,
        totalWatchers: 0,
        isWatchedByCurrentUser: false,
      ),
    );
  }

  @override
  Stream<AuctionWatchStats> watchWatchStats({
    required String auctionId,
    required String currentUserId,
  }) {
    return Stream.value(
      AuctionWatchStats(
        auctionId: auctionId,
        totalWatchers: 0,
        isWatchedByCurrentUser: false,
      ),
    );
  }

  @override
  Future<RepositoryResult<bool>> toggleWatch({
    required String auctionId,
    required String userId,
  }) async {
    return RepositoryResult.success(false);
  }
}

class _MutableAuctionRemoteDatasource extends AuctionRemoteDatasource {
  _MutableAuctionRemoteDatasource({
    required String auctionId,
    required String sellerId,
    required String sellerUsername,
    required String sellerFarmName,
    required String bidderId,
    required String bidderUsername,
  }) : super(_NoopApiClient()) {
    _auction = _auctionPayload(
      auctionId: auctionId,
      sellerId: sellerId,
      sellerUsername: sellerUsername,
      sellerFarmName: sellerFarmName,
      currentBid: 1600000,
      highestBidderId: 'seed-bidder-2',
      totalBids: 2,
    );
    _bids = [
      _bidPayload(
        id: 'seed-bid-2',
        auctionId: auctionId,
        bidderId: 'seed-bidder-2',
        bidderUsername: 'bob',
        amount: 1600000,
        createdAt: DateTime.utc(2026, 7, 31, 12, 3),
      ),
      _bidPayload(
        id: 'seed-bid-1',
        auctionId: auctionId,
        bidderId: 'seed-bidder-1',
        bidderUsername: 'alice',
        amount: 1550000,
        createdAt: DateTime.utc(2026, 7, 31, 12, 0),
      ),
    ];
    _currentBidderId = bidderId;
    _currentBidderUsername = bidderUsername;
  }

  late final String _currentBidderId;
  late final String _currentBidderUsername;
  late Map<String, dynamic> _auction;
  late List<Map<String, dynamic>> _bids;
  int _bidSequence = 2;

  AuctionDto _auctionDto() => AuctionDto.fromJson(_auction);

  List<BidDto> _bidDtos({int limit = 50}) {
    return _bids
        .take(limit)
        .map((bid) => BidDto.fromJson(bid))
        .toList(growable: false);
  }

  @override
  Future<AuctionDto> getAuctionById(String auctionId) async => _auctionDto();

  @override
  Future<List<BidDto>> getBidHistory(
    String auctionId, {
    int limit = 50,
  }) async => _bidDtos(limit: limit);

  @override
  Future<RepositoryResult<BidDto>> placeBid(
    String auctionId,
    PlaceBidDto request,
  ) async {
    _bidSequence += 1;
    final now = DateTime.utc(
      2026,
      7,
      31,
      12,
      10,
    ).add(Duration(minutes: _bidSequence));
    final bidId = 'seed-bid-${_bidSequence + 1}';
    final bidPayload = _bidPayload(
      id: bidId,
      auctionId: auctionId,
      bidderId: _currentBidderId,
      bidderUsername: _currentBidderUsername,
      amount: request.amount,
      createdAt: now,
    );
    _bids.insert(0, bidPayload);
    _auction = {
      ..._auction,
      'current_bid': request.amount,
      'current_winner_id': _currentBidderId,
      'total_bids': (_auction['total_bids'] as num).toInt() + 1,
      'minimum_bid': request.amount + 50000,
      'updated_at': now.toIso8601String(),
    };
    return RepositoryResult.success(BidDto.fromJson(bidPayload));
  }
}

class _MutableAuctionRepository extends AuctionRepositoryImpl {
  _MutableAuctionRepository({required this.datasource, required super.logger})
    : super(datasource: datasource);

  final _MutableAuctionRemoteDatasource datasource;
  int watchAuctionCalls = 0;
  int watchAuctionBidsCalls = 0;

  @override
  Stream<Auction?> watchAuction(String auctionId) {
    watchAuctionCalls += 1;
    return Stream.value(AuctionMapper.toEntity(datasource._auctionDto()));
  }

  @override
  Stream<List<AuctionBid>> watchAuctionBids(
    String auctionId, {
    int limit = 50,
  }) {
    watchAuctionBidsCalls += 1;
    return Stream.value(
      datasource
          ._bidDtos(limit: limit)
          .map(AuctionMapper.toBidEntity)
          .toList(growable: false),
    );
  }
}

Map<String, dynamic> _auctionPayload({
  required String auctionId,
  required String sellerId,
  required String sellerUsername,
  required String sellerFarmName,
  required int currentBid,
  required String highestBidderId,
  required int totalBids,
}) {
  return {
    'id': auctionId,
    'seller_id': sellerId,
    'product_id': 'product-1',
    'title': 'Sanke Auction',
    'description': 'Live auction',
    'start_price': 1500000,
    'bid_increment': 50000,
    'buy_now_price': 2500000,
    'current_bid': currentBid,
    'current_winner_id': highestBidderId,
    'total_bids': totalBids,
    'minimum_bid': currentBid + 50000,
    'start_at': '2026-07-31T00:00:00Z',
    'end_at': '2026-08-01T00:00:00Z',
    'status': 'active',
    'created_at': '2026-07-31T00:00:00Z',
    'updated_at': '2026-07-31T00:10:00Z',
    'views_count': 11,
    'watchers_count': 2,
    'can_bid': true,
    'can_buy_now': true,
    'shipping_option_ids': ['ship-1', 'ship-2'],
    'viewer_capabilities': {
      'role': 'buyer',
      'can_manage': false,
      'can_edit': false,
      'can_promote': false,
      'can_chat': true,
      'can_negotiate': false,
      'can_buy': false,
      'can_bid': true,
      'can_buy_now': true,
    },
    'auction': {
      'id': auctionId,
      'title': 'Sanke Auction',
      'thumbnail_url': null,
      'current_bid': currentBid,
      'buy_now_price': 2500000,
      'end_at': '2026-08-01T00:00:00Z',
      'lifecycle': 'active',
      'seller': {
        'user': {
          'id': sellerId,
          'username': sellerUsername,
          'avatar_url': null,
          'lifecycle': 'active',
        },
        'farm_name': sellerFarmName,
        'avatar_url': null,
        'lifecycle': 'active',
      },
    },
  };
}

Map<String, dynamic> _bidPayload({
  required String id,
  required String auctionId,
  required String bidderId,
  required String bidderUsername,
  required int amount,
  required DateTime createdAt,
}) {
  return {
    'id': id,
    'auction_id': auctionId,
    'bidder_id': bidderId,
    'amount': amount,
    'created_at': createdAt.toIso8601String(),
    'bidder': {
      'id': bidderId,
      'username': bidderUsername,
      'avatar_url': null,
      'lifecycle': 'active',
    },
  };
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

Auction _auction({
  required String id,
  required String sellerId,
  CommerceViewerCapabilities? capabilities,
  List<MediaEntity> media = const [],
  String? publicOriginLine,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return Auction(
    id: id,
    sellerId: sellerId,
    sellerUsername: 'seller_user',
    sellerFarmName: 'Acme Farm',
    sellerAvatar: null,
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
    sellerTier: 'pro',
    viewerCapabilities: capabilities,
    title: 'Sanke Auction',
    description: 'Live auction',
    koiDetails: const KoiDetails(
      variety: 'Kohaku',
      sizeInCm: 30,
      ageInMonths: 12,
      gender: 'male',
      breeder: 'Hiro',
      bloodline: 'Miyabi',
      certificates: ['ownership', 'health'],
    ),
    preparationTime: 'immediate',
    preparationNote: 'Packing aman sebelum kirim',
    openingBid: 1000000,
    currentBid: 1500000,
    bidIncrement: 50000,
    buyNowPrice: 2500000,
    media: media,
    startTime: now,
    endTime: now.add(const Duration(days: 1)),
    status: AuctionStatus.active,
    totalBidders: 2,
    totalWatchers: 0,
    totalViews: 10,
    createdAt: now,
    updatedAt: now,
    origin: 'Blitar',
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
    productId: 'product-1',
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
      id: 'auction-media-1',
      originalUrl:
          'https://cdn.example.com/gallery/auction-1.jpg?X-Amz-Signature=one',
      type: MediaType.image,
      createdAt: now,
    ),
    MediaEntity(
      id: 'auction-media-2',
      originalUrl:
          'https://cdn.example.com/gallery/auction-2.jpg?X-Amz-Signature=two',
      type: MediaType.image,
      createdAt: now,
    ),
  ];
}

Widget _wrap({
  required Auction auction,
  required AuthState authState,
  required SavedItemRepository savedItemRepository,
  AuctionNotifierState? auctionNotifierState,
  Stream<Auction?>? auctionStream,
  Stream<List<AuctionBid>>? auctionBidsStream,
  _FakeAuctionNotifier? auctionNotifier,
  Auction Function()? auctionLoader,
  ThemeData? theme,
}) {
  final notifier =
      auctionNotifier ??
      _FakeAuctionNotifier(
        auctionNotifierState ?? AuctionNotifierState(selectedAuction: auction),
      );
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      savedItemRepositoryProvider.overrideWithValue(savedItemRepository),
      auctionNotifierProvider.overrideWith(() => notifier),
      auctionStreamProvider(auction.id).overrideWith(
        (ref) =>
            auctionStream ?? Stream.value(auctionLoader?.call() ?? auction),
      ),
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
    ],
    child: MaterialApp(
      theme: theme,
      home: AuctionDetailScreen(auctionId: auction.id),
    ),
  );
}

Widget _wrapWithLiveAuctionAuthority({
  required Auction auction,
  required AuthState authState,
  required SavedItemRepository savedItemRepository,
  required AuctionRepository auctionRepository,
  required ILoggerService logger,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      loggerServiceProvider.overrideWithValue(logger),
      savedItemRepositoryProvider.overrideWithValue(savedItemRepository),
      auctionRepositoryProvider.overrideWithValue(auctionRepository),
      auctionWatchRepositoryProvider.overrideWithValue(
        const _FakeAuctionWatchRepository(),
      ),
      ownerOtherAuctionsProvider(
        auction.id,
      ).overrideWith((ref) async => const <Auction>[]),
      similarAuctionsProvider(
        auction.id,
      ).overrideWith((ref) async => const <Auction>[]),
      navigationHandlerProvider.overrideWithValue(_FakeNavigationHandler()),
    ],
    child: MaterialApp(home: AuctionDetailScreen(auctionId: auction.id)),
  );
}

void main() {
  testWidgets('buyer detail screen shows buyer CTAs at 360 width', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(360, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final auction =
        _auction(
          id: 'auction-1',
          sellerId: 'seller-1',
          capabilities: const CommerceViewerCapabilities(
            role: 'buyer',
            canManage: false,
            canEdit: false,
            canPromote: false,
            canChat: true,
            canNegotiate: false,
            canBuy: false,
            canBid: true,
            canBuyNow: true,
          ),
          media: _detailMedia(),
          publicOriginLine: 'Magelang, Jawa Tengah',
        ).copyWith(
          location: const AuctionLocation(
            cityId: 'city-1',
            cityName: 'Magelang',
            provinceId: 'province-1',
            provinceName: 'Jawa Tengah',
          ),
        );
    final savedRepo = _FakeSavedItemRepository(initialSaved: false);

    await tester.pumpWidget(
      _wrap(
        auction: auction,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-1'),
          emailVerified: true,
        ),
        savedItemRepository: savedRepo,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Auction Detail'), findsOneWidget);
    expect(find.byTooltip('Pantau'), findsOneWidget);
    expect(find.bySemanticsLabel('Pantau'), findsOneWidget);
    expect(find.text('Pantau'), findsNothing);
    expect(find.text('Dipantau'), findsNothing);
    expect(find.text('Chat'), findsOneWidget);
    expect(find.text('Ajukan Bid'), findsOneWidget);
    expect(find.text('Kelola Lelang'), findsNothing);
    expect(find.text('Detail Lelang'), findsOneWidget);
    expect(find.text('Acme Farm'), findsOneWidget);
    expect(find.text('@seller_user'), findsOneWidget);
    expect(find.text('Magelang, Jawa Tengah'), findsOneWidget);
    expect(find.byType(RefreshIndicator), findsOneWidget);
    expect(find.byType(CommerceDetailMediaGallery), findsOneWidget);
    expect(find.byType(PageView), findsOneWidget);
    expect(find.text('Origin'), findsNothing);
    expect(find.text('Opsi Pengiriman'), findsNothing);
    expect(find.textContaining('Travel'), findsNothing);
    expect(find.textContaining('Bus'), findsNothing);
    expect(find.text('Ukuran'), findsOneWidget);
    expect(find.text('30 cm'), findsOneWidget);
    expect(find.text('Usia'), findsOneWidget);
    expect(find.text('12 bulan'), findsOneWidget);
    expect(find.text('Kelamin'), findsOneWidget);
    expect(find.text('Jantan'), findsOneWidget);
    expect(find.text('Varietas'), findsOneWidget);
    expect(find.text('Kohaku'), findsOneWidget);
    expect(find.text('Breeder'), findsOneWidget);
    expect(find.text('Hiro'), findsOneWidget);
    expect(find.text('Bloodline'), findsOneWidget);
    expect(find.text('Miyabi'), findsOneWidget);
    expect(find.text('Sertifikat'), findsOneWidget);
    expect(find.text('Kepemilikan, Kesehatan'), findsOneWidget);
    expect(find.text('Berdasarkan pernyataan seller'), findsOneWidget);
    expect(find.text('Bid Increment'), findsOneWidget);
    expect(find.text('Rp 50000'), findsOneWidget);
    expect(find.text('Siap kirim langsung'), findsOneWidget);
    expect(find.textContaining('Penjual siap mengirim'), findsOneWidget);
    expect(find.textContaining('Packing aman sebelum kirim'), findsOneWidget);
    expect(find.text('Live auction'), findsOneWidget);
    final watchAction = find.byTooltip('Pantau');
    final shareAction = find.byTooltip('Bagikan');
    final moreAction = find.byTooltip('Lainnya');
    expect(tester.getSize(watchAction), const Size(48, 48));
    expect(tester.getSize(shareAction), const Size(48, 48));
    expect(tester.getSize(moreAction), const Size(48, 48));
    final watchRect = tester.getRect(watchAction);
    final shareRect = tester.getRect(shareAction);
    final moreRect = tester.getRect(moreAction);
    expect(watchRect.width, shareRect.width);
    expect(shareRect.width, moreRect.width);
    expect(watchRect.height, shareRect.height);
    expect(shareRect.height, moreRect.height);
    expect(watchRect.center.dy, shareRect.center.dy);
    expect(shareRect.center.dy, moreRect.center.dy);
    expect(shareRect.left, watchRect.right);
    expect(moreRect.left, shareRect.right);
    expect(
      tester
          .widget<Icon>(
            find.descendant(
              of: watchAction,
              matching: find.byIcon(Icons.visibility_outlined),
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
              of: watchAction,
              matching: find.byIcon(Icons.visibility_outlined),
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

    final auction = _auction(
      id: 'auction-origin-missing',
      sellerId: 'seller-origin-missing',
      capabilities: const CommerceViewerCapabilities(
        role: 'buyer',
        canManage: false,
        canEdit: false,
        canPromote: false,
        canChat: true,
        canNegotiate: false,
        canBuy: false,
        canBid: true,
        canBuyNow: true,
      ),
    );

    await tester.pumpWidget(
      _wrap(
        auction: auction,
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
    expect(find.text('Origin'), findsNothing);
    expect(find.text('Opsi Pengiriman'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'buyer detail screen keeps app bar actions aligned in dark theme',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(320, 640));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final auction =
          _auction(
            id: 'auction-dark',
            sellerId: 'seller-dark',
            capabilities: const CommerceViewerCapabilities(
              role: 'buyer',
              canManage: false,
              canEdit: false,
              canPromote: false,
              canChat: true,
              canNegotiate: false,
              canBuy: false,
              canBid: true,
              canBuyNow: true,
            ),
            media: _detailMedia(),
            publicOriginLine: 'Magelang, Jawa Tengah',
          ).copyWith(
            location: const AuctionLocation(
              cityId: 'city-dark',
              cityName: 'Magelang',
              provinceId: 'province-dark',
              provinceName: 'Jawa Tengah',
            ),
          );
      final savedRepo = _FakeSavedItemRepository(initialSaved: false);
      final darkTheme = ThemeData.dark();

      await tester.pumpWidget(
        _wrap(
          auction: auction,
          authState: AuthState.authenticated(
            _authUser(id: 'buyer-dark'),
            emailVerified: true,
          ),
          savedItemRepository: savedRepo,
          theme: darkTheme,
        ),
      );
      await tester.pumpAndSettle();

      final watchAction = find.byTooltip('Pantau');
      final shareAction = find.byTooltip('Bagikan');
      final moreAction = find.byTooltip('Lainnya');
      expect(tester.getSize(watchAction), const Size(48, 48));
      expect(tester.getSize(shareAction), const Size(48, 48));
      expect(tester.getSize(moreAction), const Size(48, 48));
      expect(
        tester
            .widget<Icon>(
              find.descendant(
                of: watchAction,
                matching: find.byIcon(Icons.visibility_outlined),
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

  testWidgets('watch active state switches the app bar icon to Dipantau', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(360, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final auction = _auction(
      id: 'auction-2',
      sellerId: 'seller-2',
      capabilities: const CommerceViewerCapabilities(
        role: 'buyer',
        canManage: false,
        canEdit: false,
        canPromote: false,
        canChat: true,
        canNegotiate: false,
        canBuy: false,
        canBid: true,
        canBuyNow: true,
      ),
    );
    final savedRepo = _FakeSavedItemRepository(initialSaved: true);

    await tester.pumpWidget(
      _wrap(
        auction: auction,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-2'),
          emailVerified: true,
        ),
        savedItemRepository: savedRepo,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byTooltip('Dipantau'), findsOneWidget);
    expect(find.bySemanticsLabel('Dipantau'), findsOneWidget);
    expect(find.text('Pantau'), findsNothing);
    expect(find.text('Dipantau'), findsNothing);
    expect(find.text('Chat'), findsOneWidget);
    expect(find.text('Ajukan Bid'), findsOneWidget);
    expect(savedRepo.isSavedCalls, greaterThan(0));
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'refreshed auction media replaces a failed first image without resetting the controller',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(360, 640));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      var auction =
          _auction(
            id: 'auction-media-refresh',
            sellerId: 'seller-media-refresh',
            capabilities: const CommerceViewerCapabilities(
              role: 'buyer',
              canManage: false,
              canEdit: false,
              canPromote: false,
              canChat: true,
              canNegotiate: false,
              canBuy: false,
              canBid: true,
              canBuyNow: true,
            ),
            media: [
              MediaEntity(
                id: 'auction-media-1',
                originalUrl:
                    'https://cdn.example.com/gallery/auction-1.jpg?X-Amz-Signature=one',
                type: MediaType.image,
                createdAt: DateTime.utc(2026, 1, 1),
              ),
              MediaEntity(
                id: 'auction-media-2',
                originalUrl:
                    'https://cdn.example.com/gallery/auction-2.jpg?X-Amz-Signature=two',
                type: MediaType.image,
                createdAt: DateTime.utc(2026, 1, 1),
              ),
            ],
            publicOriginLine: 'Magelang, Jawa Tengah',
          ).copyWith(
            location: const AuctionLocation(
              cityId: 'city-1',
              cityName: 'Magelang',
              provinceId: 'province-1',
              provinceName: 'Jawa Tengah',
            ),
          );

      const firstRefreshUrl =
          'https://cdn.example.com/gallery/auction-1.jpg?X-Amz-Signature=one';
      const refreshedFirstUrl =
          'https://cdn.example.com/gallery/auction-1.jpg?X-Amz-Signature=two';
      const secondRefreshUrl =
          'https://cdn.example.com/gallery/auction-2.jpg?X-Amz-Signature=two';

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
            auction: auction,
            authState: AuthState.authenticated(
              _authUser(id: 'buyer-media-refresh'),
              emailVerified: true,
            ),
            savedItemRepository: _FakeSavedItemRepository(initialSaved: false),
            auctionLoader: () => auction,
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

        auction = auction.copyWith(
          media: [
            MediaEntity(
              id: 'auction-media-1',
              originalUrl: refreshedFirstUrl,
              type: MediaType.image,
              createdAt: DateTime.utc(2026, 1, 1),
            ),
            MediaEntity(
              id: 'auction-media-2',
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

  testWidgets('watch state survives screen reconstruction', (tester) async {
    await tester.binding.setSurfaceSize(const Size(360, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final auction = _auction(
      id: 'auction-2b',
      sellerId: 'seller-2b',
      capabilities: const CommerceViewerCapabilities(
        role: 'buyer',
        canManage: false,
        canEdit: false,
        canPromote: false,
        canChat: true,
        canNegotiate: false,
        canBuy: false,
        canBid: true,
        canBuyNow: true,
      ),
    );
    final savedRepo = _FakeSavedItemRepository(initialSaved: false);

    await tester.pumpWidget(
      _wrap(
        auction: auction,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-2b'),
          emailVerified: true,
        ),
        savedItemRepository: savedRepo,
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byTooltip('Pantau'), findsOneWidget);
    expect(find.bySemanticsLabel('Pantau'), findsOneWidget);
    expect(find.text('Pantau'), findsNothing);

    await tester.tap(find.byIcon(Icons.visibility_outlined));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Dipantau'), findsOneWidget);
    expect(find.bySemanticsLabel('Dipantau'), findsOneWidget);
    expect(find.text('Dipantau'), findsNothing);

    await tester.pumpWidget(
      _wrap(
        auction: auction,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-2b'),
          emailVerified: true,
        ),
        savedItemRepository: savedRepo,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Pantau'), findsNothing);
    expect(find.byTooltip('Dipantau'), findsOneWidget);
    expect(find.bySemanticsLabel('Dipantau'), findsOneWidget);
    expect(find.text('Dipantau'), findsNothing);
    expect(savedRepo.addCalls, greaterThan(0));
    expect(tester.takeException(), isNull);
  });

  testWidgets('empty product fields stay hidden on the detail screen', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final auction =
        _auction(
          id: 'auction-empty',
          sellerId: 'seller-empty',
          capabilities: const CommerceViewerCapabilities(
            role: 'buyer',
            canManage: false,
            canEdit: false,
            canPromote: false,
            canChat: true,
            canNegotiate: false,
            canBuy: false,
            canBid: true,
            canBuyNow: true,
          ),
        ).copyWith(
          description: '',
          preparationTime: null,
          preparationNote: null,
          origin: '',
          shippingOptions: const [],
          koiDetails:
              _auction(
                id: 'auction-empty-shadow',
                sellerId: 'seller-empty',
              ).koiDetails.copyWith(
                variety: '',
                sizeInCm: 0,
                ageInMonths: 0,
                gender: 'unknown',
                breeder: '',
                bloodline: '',
                certificates: const [],
              ),
        );

    await tester.pumpWidget(
      _wrap(
        auction: auction,
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-empty'),
          emailVerified: true,
        ),
        savedItemRepository: _FakeSavedItemRepository(initialSaved: false),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Detail Lelang'), findsOneWidget);
    expect(find.text('Varietas'), findsNothing);
    expect(find.text('Ukuran'), findsNothing);
    expect(find.text('Usia'), findsNothing);
    expect(find.text('Kelamin'), findsNothing);
    expect(find.text('Breeder'), findsNothing);
    expect(find.text('Bloodline'), findsNothing);
    expect(find.text('Sertifikat'), findsNothing);
    expect(find.text('Opsi Pengiriman'), findsNothing);
    expect(find.text('Deskripsi'), findsNothing);
    expect(find.text('Bid Increment'), findsOneWidget);
    expect(find.text('Rp 50000'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('owner detail screen hides save/watch/report and shows manage', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final auction = _auction(
      id: 'auction-3',
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
        auction: auction,
        authState: AuthState.authenticated(
          _authUser(id: 'seller-3'),
          emailVerified: true,
        ),
        savedItemRepository: savedRepo,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Kelola Lelang'), findsOneWidget);
    expect(find.text('Promosi'), findsOneWidget);
    expect(find.text('Pantau'), findsNothing);
    expect(find.text('Dipantau'), findsNothing);
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
    expect(find.text('Ajukan Bid'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('report flow opens the submission sheet for non-owners', (
    tester,
  ) async {
    final auction = _auction(
      id: 'auction-4',
      sellerId: 'seller-4',
      capabilities: const CommerceViewerCapabilities(
        role: 'buyer',
        canManage: false,
        canEdit: false,
        canPromote: false,
        canChat: true,
        canNegotiate: false,
        canBuy: false,
        canBid: true,
        canBuyNow: true,
      ),
    );

    await tester.pumpWidget(
      _wrap(
        auction: auction,
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
    expect(find.text('Reporting Auction'), findsOneWidget);
    expect(find.text('Kirim Laporan'), findsOneWidget);
  });

  testWidgets(
    'loading state renders a body skeleton and no active bottom actions',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(320, 640));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final auction = _auction(
        id: 'auction-loading',
        sellerId: 'seller-loading',
        capabilities: const CommerceViewerCapabilities(
          role: 'buyer',
          canManage: false,
          canEdit: false,
          canPromote: false,
          canChat: true,
          canNegotiate: false,
          canBuy: false,
          canBid: true,
          canBuyNow: true,
        ),
      );
      final savedRepo = _FakeSavedItemRepository(initialSaved: false);
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
          savedItemRepository: savedRepo,
          auctionNotifierState: const AuctionNotifierState(isLoading: true),
          auctionStream: auctionController.stream,
          auctionBidsStream: bidsController.stream,
        ),
      );
      await tester.pump();

      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      expect(find.text('Chat'), findsNothing);
      expect(find.text('Ajukan Bid'), findsNothing);
      expect(find.text('Memuat detail lelang'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('loading transitions to success with body and actions together', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(360, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final auction = _auction(
      id: 'auction-transition',
      sellerId: 'seller-transition',
      capabilities: const CommerceViewerCapabilities(
        role: 'buyer',
        canManage: false,
        canEdit: false,
        canPromote: false,
        canChat: true,
        canNegotiate: false,
        canBuy: false,
        canBid: true,
        canBuyNow: true,
      ),
    );
    final savedRepo = _FakeSavedItemRepository(initialSaved: false);
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
          _authUser(id: 'buyer-transition'),
          emailVerified: true,
        ),
        savedItemRepository: savedRepo,
        auctionNotifierState: const AuctionNotifierState(isLoading: true),
        auctionStream: auctionController.stream,
        auctionBidsStream: bidsController.stream,
      ),
    );
    await tester.pump();
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.text('Ajukan Bid'), findsNothing);

    auctionController.add(auction);
    bidsController.add(const <AuctionBid>[]);
    await tester.pumpAndSettle();

    expect(find.byType(CircularProgressIndicator), findsNothing);
    expect(find.text('Auction Detail'), findsOneWidget);
    expect(find.text('Detail Lelang'), findsOneWidget);
    expect(find.text('Chat'), findsOneWidget);
    expect(find.text('Ajukan Bid'), findsOneWidget);
    expect(find.byTooltip('Pantau'), findsOneWidget);
    expect(find.bySemanticsLabel('Pantau'), findsOneWidget);
    expect(find.text('Pantau'), findsNothing);
    expect(find.text('Memuat detail lelang'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'real auction detail refreshes current bid and history after a successful bid',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(390, 760));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final auction = _auction(
        id: 'auction-runtime-bid',
        sellerId: 'seller-runtime-bid',
        capabilities: const CommerceViewerCapabilities(
          role: 'buyer',
          canManage: false,
          canEdit: false,
          canPromote: false,
          canChat: true,
          canNegotiate: false,
          canBuy: false,
          canBid: true,
          canBuyNow: true,
        ),
      );
      final savedRepo = _FakeSavedItemRepository(initialSaved: false);
      final fakeLogger = _NoopLogger();
      final bidderUser = _authUser(id: 'buyer-runtime-bid');
      final datasource = _MutableAuctionRemoteDatasource(
        auctionId: auction.id,
        sellerId: auction.sellerId,
        sellerUsername: auction.sellerUsername ?? 'seller_user',
        sellerFarmName: auction.sellerFarmName ?? 'Acme Farm',
        bidderId: bidderUser.id,
        bidderUsername: 'buyer_runtime',
      );
      final repository = _MutableAuctionRepository(
        datasource: datasource,
        logger: fakeLogger,
      );

      await tester.pumpWidget(
        _wrapWithLiveAuctionAuthority(
          auction: auction,
          authState: AuthState.authenticated(bidderUser, emailVerified: true),
          savedItemRepository: savedRepo,
          auctionRepository: repository,
          logger: fakeLogger,
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Riwayat Bid (2)'), findsOneWidget);
      expect(find.text('@bob'), findsOneWidget);
      expect(find.text('@alice'), findsOneWidget);
      expect(find.text('Rp 1600000'), findsWidgets);
      expect(repository.watchAuctionCalls, greaterThanOrEqualTo(1));
      expect(repository.watchAuctionBidsCalls, greaterThanOrEqualTo(1));
      expect(tester.takeException(), isNull);

      final bobTop = tester.getTopLeft(find.text('@bob'));
      final aliceTop = tester.getTopLeft(find.text('@alice'));
      expect(bobTop.dy, lessThan(aliceTop.dy));

      await tester.tap(find.text('Ajukan Bid'));
      await tester.pumpAndSettle();

      expect(find.text('Tawar Lelang'), findsOneWidget);
      expect(find.text('Masukkan Jumlah Bid'), findsOneWidget);
      expect(find.text('Ajukan Bid'), findsWidgets);

      await tester.tap(find.text('Ajukan Bid').last);
      await tester.pumpAndSettle();

      expect(find.text('Konfirmasi Penawaran'), findsOneWidget);
      expect(find.textContaining('Bid successful!'), findsNothing);

      await tester.tap(find.text('Konfirmasi'));
      await tester.pumpAndSettle();

      expect(find.textContaining('Bid successful!'), findsOneWidget);
      expect(find.text('Riwayat Bid (3)'), findsOneWidget);
      expect(find.text('@buyer_runtime'), findsOneWidget);
      expect(find.text('@bob'), findsOneWidget);
      expect(find.text('@alice'), findsOneWidget);
      expect(find.text('Rp 1650000'), findsWidgets);
      expect(repository.watchAuctionCalls, greaterThanOrEqualTo(2));
      expect(repository.watchAuctionBidsCalls, greaterThanOrEqualTo(2));
      expect(tester.takeException(), isNull);

      final buyerTop = tester.getTopLeft(find.text('@buyer_runtime'));
      final bobAfterTop = tester.getTopLeft(find.text('@bob'));
      final aliceAfterTop = tester.getTopLeft(find.text('@alice'));
      expect(buyerTop.dy, lessThan(bobAfterTop.dy));
      expect(bobAfterTop.dy, lessThan(aliceAfterTop.dy));

      await tester
          .widget<RefreshIndicator>(find.byType(RefreshIndicator))
          .onRefresh();
      await tester.pumpAndSettle();

      expect(find.text('Riwayat Bid (3)'), findsOneWidget);
      expect(find.text('@buyer_runtime'), findsOneWidget);
      expect(find.text('@bob'), findsOneWidget);
      expect(find.text('@alice'), findsOneWidget);
      expect(find.text('Rp 1650000'), findsWidgets);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'narrow success layout wraps long origin shipping breeder bloodline and certificate values',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(320, 640));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final auction =
          _auction(
            id: 'auction-wrap',
            sellerId: 'seller-wrap',
            capabilities: const CommerceViewerCapabilities(
              role: 'buyer',
              canManage: false,
              canEdit: false,
              canPromote: false,
              canChat: true,
              canNegotiate: false,
              canBuy: false,
              canBid: true,
              canBuyNow: true,
            ),
            publicOriginLine: 'Cipicung, Kabupaten Kuningan, Jawa Barat',
          ).copyWith(
            location: const AuctionLocation(
              cityId: 'city-wrap',
              cityName: 'Cipicung, Kabupaten Kuningan',
              provinceId: 'province-wrap',
              provinceName: 'Jawa Barat',
            ),
            koiDetails:
                _auction(
                  id: 'auction-wrap-shadow',
                  sellerId: 'seller-wrap',
                ).koiDetails.copyWith(
                  breeder:
                      'PT Panen Sejahtera Nusantara Mandiri yang Sangat Panjang',
                  bloodline:
                      'Miyabi Grand Champion Lineage From A Very Long Heritage',
                  certificates: const [
                    'breeder',
                    'contest',
                    'ownership',
                    'health',
                  ],
                ),
            shippingOptions: const [
              CommerceShippingOptionSummary(
                id: 'ship-long-1',
                name: 'Traveleo',
                transportType: 'travel',
              ),
              CommerceShippingOptionSummary(
                id: 'ship-long-2',
                name: 'Bus Cepat Nusantara',
                transportType: 'bus',
              ),
            ],
          );
      final savedRepo = _FakeSavedItemRepository(initialSaved: false);

      await tester.pumpWidget(
        _wrap(
          auction: auction,
          authState: AuthState.authenticated(
            _authUser(id: 'buyer-wrap'),
            emailVerified: true,
          ),
          savedItemRepository: savedRepo,
        ),
      );
      await tester.pumpAndSettle();

      expect(
        find.text('Cipicung, Kabupaten Kuningan, Jawa Barat'),
        findsOneWidget,
      );
      expect(find.text('Breeder'), findsOneWidget);
      expect(find.text('Bloodline'), findsOneWidget);
      expect(find.text('Sertifikat'), findsOneWidget);
      expect(
        find.text('Breeder, Kontes, Kepemilikan, Kesehatan'),
        findsOneWidget,
      );
      expect(find.text('Origin'), findsNothing);
      expect(find.text('Opsi Pengiriman'), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );
}
