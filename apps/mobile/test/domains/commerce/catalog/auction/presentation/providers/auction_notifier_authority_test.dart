import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/shared/data/dto/commerce_media_request_dto.dart';
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
    List<CommerceMediaRequestDto> media = const [],
    required List<String> mediaUrls,
    required List<AuctionMediaType> mediaTypes,
    required KoiDetails koiDetails,
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
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
  Future<RepositoryResult<Auction>> updateAuctionStatus({
    required String auctionId,
    required AuctionStatus status,
  }) async => throw UnimplementedError();

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

class _FakeAuctionWatchRepository implements AuctionWatchRepository {
  @override
  Future<RepositoryResult<AuctionWatcher>> watchAuction({
    required String auctionId,
    required String userId,
    bool notifyOnBid = true,
    bool notifyOnEndingSoon = true,
    bool notifyOnEnded = true,
  }) async => throw UnimplementedError();

  @override
  Future<RepositoryResult<void>> unwatchAuction({
    required String auctionId,
    required String userId,
  }) async => throw UnimplementedError();

  @override
  Future<RepositoryResult<bool>> isWatching({
    required String auctionId,
    required String userId,
  }) async => throw UnimplementedError();

  @override
  Future<RepositoryResult<AuctionWatchStats>> getWatchStats({
    required String auctionId,
    required String currentUserId,
  }) async => throw UnimplementedError();

  @override
  Stream<AuctionWatchStats> watchWatchStats({
    required String auctionId,
    required String currentUserId,
  }) => const Stream.empty();

  @override
  Future<RepositoryResult<bool>> toggleWatch({
    required String auctionId,
    required String userId,
  }) async => throw UnimplementedError();

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
  required AuctionWatchRepository auctionWatchRepository,
}) {
  return ProviderContainer(
    overrides: [
      authControllerProvider.overrideWith(() => authController),
      auctionRepositoryProvider.overrideWithValue(auctionRepository),
      auctionWatchRepositoryProvider.overrideWithValue(auctionWatchRepository),
      loggerServiceProvider.overrideWithValue(_NoopLogger()),
    ],
  );
}

Future<bool> _submitCreateAuction(ProviderContainer container) {
  return container
      .read(auctionNotifierProvider.notifier)
      .createAuction(
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
  group('AuctionNotifier createAuction authority boundary', () {
    test('blocked states never call the repository', () async {
      final blockedStates = [
        const AuthState.unauthenticated(),
        AuthState.authenticated(
          _seller(
            id: 'seller-1',
            username: 'nonseller',
            hasSellerProfile: false,
            hasMarketAuthority: false,
          ),
          emailVerified: true,
        ),
        AuthState.authenticated(
          _seller(
            id: 'seller-1',
            username: 'expired',
            hasSellerProfile: true,
            hasMarketAuthority: false,
          ),
          emailVerified: true,
        ),
      ];

      for (final state in blockedStates) {
        final repo = _FakeAuctionRepository();
        final container = _container(
          authController: _FakeAuthController(state),
          auctionRepository: repo,
          auctionWatchRepository: _FakeAuctionWatchRepository(),
        );
        addTearDown(container.dispose);

        final result = await _submitCreateAuction(container);

        expect(result, isFalse);
        expect(repo.createCalls, 0);
      }
    });

    test('active seller derives seller identity from the live principal', () {
      final repo = _FakeAuctionRepository();
      final controller = _FakeAuthController(
        AuthState.authenticated(
          _seller(
            id: 'seller-1',
            username: 'live-seller',
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
        auctionWatchRepository: _FakeAuctionWatchRepository(),
      );
      addTearDown(container.dispose);

      final future = _submitCreateAuction(container);

      expect(repo.createCalls, 1);
      expect(repo.lastSellerId, 'seller-1');
      expect(repo.lastSellerUsername, 'live-seller');
      expect(repo.lastSellerAvatar, 'https://example.com/avatar.png');
      expect(repo.lastSellerFarmName, isNull);

      repo.completeCreateSuccess(
        sellerId: 'seller-1',
        sellerUsername: 'live-seller',
        sellerAvatar: 'https://example.com/avatar.png',
      );

      expect(future, completes);
    });

    test('principal switch discards stale create results', () async {
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
        auctionWatchRepository: _FakeAuctionWatchRepository(),
      );
      addTearDown(container.dispose);

      final future = _submitCreateAuction(container);
      expect(repo.createCalls, 1);

      controller.setAuthState(
        AuthState.authenticated(
          _seller(
            id: 'seller-2',
            username: 'seller-b',
            hasSellerProfile: true,
            hasMarketAuthority: true,
          ),
          emailVerified: true,
        ),
      );

      repo.completeCreateSuccess(
        sellerId: 'seller-1',
        sellerUsername: 'seller-a',
      );

      expect(await future, isFalse);
      expect(container.read(auctionNotifierProvider).selectedAuction, isNull);
      expect(container.read(auctionNotifierProvider).successMessage, isNull);
    });

    test('authority loss discards stale create results', () async {
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
        auctionWatchRepository: _FakeAuctionWatchRepository(),
      );
      addTearDown(container.dispose);

      final future = _submitCreateAuction(container);
      expect(repo.createCalls, 1);

      controller.setAuthState(
        AuthState.authenticated(
          _seller(
            id: 'seller-1',
            username: 'seller-a',
            hasSellerProfile: true,
            hasMarketAuthority: false,
          ),
          emailVerified: true,
        ),
      );

      repo.completeCreateSuccess(
        sellerId: 'seller-1',
        sellerUsername: 'seller-a',
      );

      expect(await future, isFalse);
      expect(container.read(auctionNotifierProvider).selectedAuction, isNull);
      expect(container.read(auctionNotifierProvider).successMessage, isNull);
    });

    test('unhydrated/loading state never calls the repository', () async {
      final repo = _FakeAuctionRepository();
      final container = _container(
        authController: _FakeAuthController(const AuthState.loading()),
        auctionRepository: repo,
        auctionWatchRepository: _FakeAuctionWatchRepository(),
      );
      addTearDown(container.dispose);

      final result = await _submitCreateAuction(container);

      expect(result, isFalse);
      expect(repo.createCalls, 0);
    });

    test('restricted account never calls the repository', () async {
      final repo = _FakeAuctionRepository();
      final container = _container(
        authController: _FakeAuthController(
          AuthState.accountRestricted(
            _seller(
              id: 'seller-1',
              username: 'restricted',
              hasSellerProfile: true,
              hasMarketAuthority: true,
            ),
            restrictionType: AccountStatus.suspended,
          ),
        ),
        auctionRepository: repo,
        auctionWatchRepository: _FakeAuctionWatchRepository(),
      );
      addTearDown(container.dispose);

      final result = await _submitCreateAuction(container);

      expect(result, isFalse);
      expect(repo.createCalls, 0);
    });

    test('logout while request is pending discards stale results', () async {
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
        auctionWatchRepository: _FakeAuctionWatchRepository(),
      );
      addTearDown(container.dispose);

      final future = _submitCreateAuction(container);
      expect(repo.createCalls, 1);

      // Logout - user becomes null via unauthenticated state
      controller.setAuthState(const AuthState.unauthenticated());

      repo.completeCreateSuccess(
        sellerId: 'seller-1',
        sellerUsername: 'seller-a',
      );

      expect(await future, isFalse);
      expect(container.read(auctionNotifierProvider).selectedAuction, isNull);
      expect(container.read(auctionNotifierProvider).successMessage, isNull);
    });

    test('same valid principal success publishes result once', () async {
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
        auctionWatchRepository: _FakeAuctionWatchRepository(),
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
        auctionWatchRepository: _FakeAuctionWatchRepository(),
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
  });

  test('discover providers stay within backend auction limit', () async {
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
      auctionWatchRepository: _FakeAuctionWatchRepository(),
    );
    addTearDown(container.dispose);

    final activeSub = container.listen(
      exploreAuctionsStreamProvider,
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
    expect(repo.lastWatchUserLimit, 50);
  });
});
}
