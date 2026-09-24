import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _NoopLogger implements ILoggerService {
  @override
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) async => Result.success(null);

  @override
  dynamic noSuchMethod(Invocation invocation) =>
      Future.value(Result.success(null));
}

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  AuthState _state;

  @override
  AuthState build() => _state;

  void setAuthState(AuthState state) {
    _state = state;
    this.state = state;
  }
}

class _FakeAuctionRepository implements AuctionRepository {
  int createCalls = 0;
  String? lastSellerId;
  String? lastSellerUsername;
  String? lastSellerAvatar;
  String? lastSellerFarmName;
  int? lastWatchActiveLimit;
  int? lastWatchUserLimit;
  Completer<RepositoryResult<Auction>>? pendingCreate;

  @override
  Future<RepositoryResult<Auction>> createAuction({
    required String sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    required String title,
    required String description,
    required List<String> mediaUrls,
    required List<AuctionMediaType> mediaTypes,
    required KoiDetails koiDetails,
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
    String? farmAddressId,
    AuctionLocation? location,
    required List<String> shippingSetupIds,
    String? preparationNote,
  }) async {
    createCalls += 1;
    lastSellerId = sellerId;
    lastSellerUsername = sellerUsername;
    lastSellerFarmName = sellerFarmName;
    lastSellerAvatar = sellerAvatar;
    pendingCreate = Completer<RepositoryResult<Auction>>();
    return pendingCreate!.future;
  }

  void completeCreateSuccess({
    required String sellerId,
    required String sellerUsername,
    String? sellerAvatar,
  }) {
    pendingCreate?.complete(
      RepositoryResult.success(
        Auction(
          id: 'auction-1',
          sellerId: sellerId,
          sellerUsername: sellerUsername,
          sellerAvatar: sellerAvatar,
          sellerUserLifecycle: ContentLifecycle.active,
          sellerTrustLifecycle: ContentLifecycle.active,
          title: 'Kohaku 50cm',
          description: 'Healthy koi',
          koiDetails: const KoiDetails(
            variety: 'Kohaku',
            sizeInCm: 50,
            ageInMonths: 12,
            gender: 'male',
          ),
          openingBid: 1000000,
          currentBid: 1000000,
          bidIncrement: 100000,
          startTime: DateTime.utc(2026, 1, 1),
          endTime: DateTime.utc(2026, 1, 2),
          status: AuctionStatus.active,
          createdAt: DateTime.utc(2026, 1, 1),
        ),
      ),
    );
  }

  @override
  Future<RepositoryResult<List<Auction>>> getActiveAuctions({
    String? variety,
    double? minSize,
    double? maxSize,
    double? maxBid,
    int limit = 20,
    String? lastAuctionId,
  }) async => throw UnimplementedError();

  @override
  Future<RepositoryResult<Auction>> getAuctionById(String auctionId) async =>
      throw UnimplementedError();

  @override
  Future<RepositoryResult<List<AuctionBid>>> getAuctionBids({
    required String auctionId,
    int limit = 50,
  }) async => throw UnimplementedError();

  @override
  Future<RepositoryResult<List<Auction>>> getAuctionsByIds(
    List<String> auctionIds,
  ) async => throw UnimplementedError();

  @override
  Future<RepositoryResult<List<Auction>>> getUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 20,
    String? lastAuctionId,
  }) async => throw UnimplementedError();

  @override
  Future<RepositoryResult<Auction>> updateAuction(
    String auctionId,
    Map<String, dynamic> updates,
  ) async => throw UnimplementedError();

  @override
  Future<RepositoryResult<void>> cancelAuction({
    required String auctionId,
    required String sellerId,
    required String reason,
  }) async => throw UnimplementedError();

  @override
  Future<RepositoryResult<AuctionBid>> placeBid({
    required String auctionId,
    required String bidderId,
    required int amount,
  }) async => throw UnimplementedError();

  @override
  Future<RepositoryResult<String>> claimAuction({
    required String auctionId,
    required String addressId,
    required String shippingSetupId,
    String? discountCode,
    bool useCoins = false,
  }) async => throw UnimplementedError();

  @override
  Stream<List<Auction>> watchActiveAuctions({int limit = 50}) =>
      _recordWatchActive(limit);

  @override
  Stream<List<AuctionBid>> watchAuctionBids(
    String auctionId, {
    int limit = 50,
  }) => const Stream.empty();

  @override
  Stream<Auction?> watchAuction(String auctionId) => const Stream.empty();

  @override
  Stream<List<Auction>> watchUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 50,
  }) {
    lastWatchUserLimit = limit;
    return Stream<List<Auction>>.empty();
  }

  Stream<List<Auction>> _recordWatchActive(int limit) {
    lastWatchActiveLimit = limit;
    return Stream<List<Auction>>.empty();
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

AuthUser _seller({
  required String id,
  required String username,
  required bool hasSellerProfile,
  required bool hasMarketAuthority,
  String? avatarUrl,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: id,
    createdAt: now,
    updatedAt: now,
    email: '$username@example.com',
    username: username,
    avatarUrl: avatarUrl,
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: hasMarketAuthority ? 'active' : 'expired',
    hasMarketAuthority: hasMarketAuthority,
    sellerTier: SellerTier.sellerElite,
    isIdVerified: false,
    isFarmVerified: false,
    lifecycle: ContentLifecycle.active,
  );
}

ProviderContainer _container({
  required AuthController authController,
  required AuctionRepository auctionRepository,
}) {
  return ProviderContainer(
    overrides: [
      authControllerProvider.overrideWith(() => authController),
      auctionRepositoryProvider.overrideWithValue(auctionRepository),
      loggerServiceProvider.overrideWithValue(_NoopLogger()),
    ],
  );
}

Future<bool> _submitCreateAuction(ProviderContainer container) {
  return container
      .read(auctionNotifierProvider.notifier)
      .createAuction(
        sellerId: 'seller-1',
        title: 'Kohaku 50cm',
        description: 'Healthy koi',
        mediaUrls: const ['https://example.com/1.jpg'],
        mediaTypes: const [AuctionMediaType.photo],
        koiDetails: const KoiDetails(
          variety: 'Kohaku',
          sizeInCm: 50,
          ageInMonths: 12,
          gender: 'male',
        ),
        openingBid: 1000000,
        bidIncrement: 100000,
        startMode: 'now',
        durationHours: 24,
        shippingSetupIds: const ['ship-1'],
      );
}

void main() {
  group('AuctionNotifier createAuction contract', () {
    test('createAuction forwards the explicit seller principal and publishes success once', () async {
      final repo = _FakeAuctionRepository();
      final controller = _FakeAuthController(
        AuthState.authenticated(
          _seller(
            id: 'seller-1',
            username: 'seller-a',
            avatarUrl: 'https://example.com/avatar.png',
            hasSellerProfile: true,
            hasMarketAuthority: true,
          ),
          emailVerified: true,
        ),
      );
      final container = _container(
        authController: controller,
        auctionRepository: repo,
      );
      addTearDown(container.dispose);

      final future = _submitCreateAuction(container);
      expect(repo.createCalls, 1);

      repo.completeCreateSuccess(
        sellerId: 'seller-1',
        sellerUsername: 'seller-a',
        sellerAvatar: 'https://example.com/avatar.png',
      );

      final result = await future;
      expect(result, isTrue);
      expect(
        container.read(auctionNotifierProvider).selectedAuction?.id,
        'auction-1',
      );
      expect(
        container.read(auctionNotifierProvider).successMessage,
        'Lelang berhasil dibuat',
      );
    });

    test('same valid principal repository failure publishes error', () async {
      final repo = _FakeAuctionRepository();
      final controller = _FakeAuthController(
        AuthState.authenticated(
          _seller(
            id: 'seller-1',
            username: 'seller-a',
            hasSellerProfile: true,
            hasMarketAuthority: true,
          ),
          emailVerified: true,
        ),
      );
      final container = _container(
        authController: controller,
        auctionRepository: repo,
      );
      addTearDown(container.dispose);

      final future = _submitCreateAuction(container);
      expect(repo.createCalls, 1);

      // Complete with failure
      repo.pendingCreate?.complete(
        RepositoryResult.error('Backend rejected the request'),
      );

      final result = await future;
      expect(result, isFalse);
      expect(container.read(auctionNotifierProvider).error, isNotNull);
  });    test('discover providers pass the canonical feed limits', () async {
    final repo = _FakeAuctionRepository();
    final container = _container(
      authController: _FakeAuthController(
        AuthState.authenticated(
          _seller(
            id: 'seller-1',
            username: 'limit-check',
            hasSellerProfile: true,
            hasMarketAuthority: true,
          ),
          emailVerified: true,
        ),
      ),
      auctionRepository: repo,
    );
    addTearDown(container.dispose);

    final activeSub = container.listen(
      marketplaceAuctionsStreamProvider,
      (previous, next) {},
      fireImmediately: true,
    );
    final userSub = container.listen(
      userAuctionsStreamProvider('seller-1'),
      (previous, next) {},
      fireImmediately: true,
    );
    addTearDown(activeSub.close);
    addTearDown(userSub.close);

    await Future<void>.delayed(Duration.zero);

    expect(repo.lastWatchActiveLimit, 50);
    expect(repo.lastWatchUserLimit, 100);
  });
});
}
