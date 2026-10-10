// Auction detail — canonical commerce-restriction dispatch by error CODE.
//
// PROOF for the migrated call-site in auction_detail_screen.dart:
//   `_handlePlaceBid()` — bid rejection chain
//
// The bid chain dispatches the restriction FAMILY through the canonical
// `CommerceRestrictionPresenter.handle(...)`:
//   - MARKET_AUTHORITY_REQUIRED → canonical seller renewal navigation
//   - COMMERCE_RESTRICTED       → unchanged restriction snackbar
//
// The winner (bid-win) path no longer presents claim errors: the winner CTA
// forwards to the SHARED Checkout with the bid-win intent, and Checkout owns
// restriction presentation for order creation. This suite proves the CTA
// routing; Checkout's own restriction dispatch is covered by the checkout
// domain's contracts.
//
// Authority proof: a generic `FORBIDDEN` whose MESSAGE reads like a
// subscription wall never triggers renewal — dispatch is by code.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/providers/auction_recommendation_providers.dart'
    show ownerOtherAuctionsProvider, similarAuctionsProvider;
import 'package:hishumi/domains/commerce/catalog/auction/presentation/providers/auction_state.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/widgets/detail/auction_action_modal.dart';
import 'package:hishumi/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/domain/entities/shipping.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/domain/repositories/shipping_repository.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:hishumi/domains/finance/wallet/coins/coins.dart';
import 'package:hishumi/domains/user/profile/domain/entities/address_entity.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/notifiers/address_notifier.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/state/address_state.dart';
import 'package:hishumi/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:hishumi/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/models/wilayah_models.dart';

class _RecordingNavigationHandler extends Fake implements NavigationHandler {
  int renewalCalls = 0;

  @override
  void navigateToSellerRenewal() => renewalCalls++;
}

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeCoinNotifier extends CoinNotifier {
  @override
  CoinState build() => const CoinState.initial();

  @override
  Future<void> getBalance() async {}
}

/// Canonical address authority fake: the claim modal consumes
/// `addressProvider` (never a direct repository); seeds the collection and
/// derives the primary exactly like the real notifier.
class _FakeAddressNotifier extends AddressNotifier {
  _FakeAddressNotifier(this._addresses);

  final List<AddressEntity> _addresses;

  @override
  AddressState build() {
    AddressEntity? primary;
    for (final address in _addresses) {
      if (address.isPrimary) {
        primary = address;
        break;
      }
    }
    return AddressState(
      addresses: AsyncValue.data(_addresses),
      primaryAddress: AsyncValue.data(primary),
    );
  }

  @override
  Future<void> loadAddresses(String userId) async {}
}

class _FakeShippingRepository implements ShippingRepository {
  @override
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async => Result.success(const [
    DeliveryOption(
      shippingSetupId: 'ship-1',
      displayName: 'JNE',
      type: 'courier',
      rate: 15000,
    ),
  ]);

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

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

/// Auction notifier with a scripted bid outcome so the screen's rejection
/// chain can be exercised for each error code.
class _ScriptedAuctionNotifier extends AuctionNotifier {
  _ScriptedAuctionNotifier({
    required this.auction,
    this.bidErrorCode,
    this.bidErrorMessage,
    this.bidErrorDetails,
  });

  final Auction auction;
  final String? bidErrorCode;
  final String? bidErrorMessage;
  final Map<String, dynamic>? bidErrorDetails;

  int placeBidCalls = 0;

  @override
  AuctionNotifierState build() =>
      AuctionNotifierState(selectedAuction: auction);

  @override
  Future<void> loadAuctionDetails(String auctionId) async {}

  @override
  Future<void> loadAuctionBids(String auctionId, {int limit = 50}) async {}

  @override
  Future<bool> placeBid({
    required String auctionId,
    required String bidderId,
    required int amount,
  }) async {
    placeBidCalls++;
    state = state.copyWith(
      isPlacingBid: false,
      error: bidErrorMessage ?? 'Gagal memasang bid. Coba lagi.',
      errorCode: bidErrorCode,
      errorDetails: bidErrorDetails,
    );
    return false;
  }
}

AuthUser _authUser(String id) {
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

Auction _auction({
  required String id,
  required String sellerId,
  AuctionStatus status = AuctionStatus.active,
  String? winnerId,
  CommerceViewerCapabilities? capabilities = _buyerCapabilities,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return Auction(
    id: id,
    sellerId: sellerId,
    sellerUsername: 'seller_user',
    sellerFarmName: 'Acme Farm',
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
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
      certificates: ['import'],
    ),
    openingBid: 1000000,
    currentBid: 1500000,
    bidIncrement: 50000,
    buyNowPrice: 2500000,
    media: const [],
    startTime: now,
    endTime: now.add(const Duration(days: 1)),
    status: status,
    winnerId: winnerId,
    createdAt: now,
    updatedAt: now,
    productId: 'product-1',
  );
}

AddressEntity _shippingAddress() {
  return AddressEntity(
    id: 'address-1',
    userId: 'buyer-1',
    recipientName: 'Buyer',
    phone: '08123456789',
    province: const Province(id: 'province-1', name: 'Jawa Barat'),
    city: const City(id: 'city-1', name: 'Bandung', provinceId: 'province-1'),
    district: const District(
      id: 'district-1',
      name: 'Coblong',
      cityId: 'city-1',
    ),
    village: const Village(
      id: 'village-1',
      name: 'Dago',
      districtId: 'district-1',
    ),
    streetAddress: 'Jl. Test No. 1',
    postalCode: '40135',
    isPrimary: true,
    createdAt: DateTime.utc(2026, 8, 1),
    updatedAt: DateTime.utc(2026, 8, 1),
  );
}

class _Harness {
  _Harness(this.notifier, this.navigation);

  final _ScriptedAuctionNotifier notifier;
  final _RecordingNavigationHandler navigation;
}

Future<_Harness> _pumpDetail(
  WidgetTester tester, {
  required Auction auction,
  String? bidErrorCode,
  String? bidErrorMessage,
  Map<String, dynamic>? bidErrorDetails,
  String currentUserId = 'buyer-1',
}) async {
  final notifier = _ScriptedAuctionNotifier(
    auction: auction,
    bidErrorCode: bidErrorCode,
    bidErrorMessage: bidErrorMessage,
    bidErrorDetails: bidErrorDetails,
  );
  final navigation = _RecordingNavigationHandler();

  await tester.binding.setSurfaceSize(const Size(800, 1600));
  addTearDown(() => tester.binding.setSurfaceSize(null));

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(
          () => _FakeAuthController(
            AuthState.authenticated(
              _authUser(currentUserId),
              emailVerified: true,
            ),
          ),
        ),
        coinProvider.overrideWith(() => _FakeCoinNotifier()),
        auctionNotifierProvider.overrideWith(() => notifier),
        // The winner CTA's commerce intent resolves the LIVE auction through
        // this provider; the harness serves the fixture directly.
        auctionDetailProvider(
          auction.id,
        ).overrideWith((ref) async => auction),
        auctionStreamProvider(
          auction.id,
        ).overrideWith((ref) => Stream.value(auction)),
        auctionBidsStreamProvider(
          auction.id,
        ).overrideWith((ref) => Stream.value(const <AuctionBid>[])),
        ownerOtherAuctionsProvider(
          auction.id,
        ).overrideWith((ref) async => const <Auction>[]),
        similarAuctionsProvider(
          auction.id,
        ).overrideWith((ref) async => const <Auction>[]),
        addressProvider.overrideWith(
          () => _FakeAddressNotifier([_shippingAddress()]),
        ),
        shippingRepositoryProvider.overrideWithValue(_FakeShippingRepository()),
        navigationHandlerProvider.overrideWithValue(navigation),
        savedItemRepositoryProvider.overrideWithValue(
          _FakeSavedItemRepository(),
        ),
      ],
      // The winner CTA navigates through GoRouter to the shared checkout
      // route, so the harness must provide a real GoRouter with that
      // destination — a plain MaterialApp cannot.
      child: MaterialApp.router(
        routerConfig: GoRouter(
          initialLocation: '/',
          routes: [
            GoRoute(
              path: '/',
              builder: (context, state) =>
                  AuctionDetailScreen(auctionId: auction.id),
            ),
            GoRoute(
              path: '/checkout/:forSaleId',
              builder: (context, state) => Scaffold(
                body: Center(
                  child: Text('checkout:${state.uri}'),
                ),
              ),
            ),
          ],
        ),
      ),
    ),
  );

  await tester.pumpAndSettle();
  return _Harness(notifier, navigation);
}

/// Drives the real bid UI: bottom-bar CTA → action modal → confirmation.
Future<void> _placeBidThroughUi(WidgetTester tester) async {
  await tester.tap(find.widgetWithText(ElevatedButton, 'Pasang Bid').first);
  await tester.pumpAndSettle();
  await tester.tap(
    find.descendant(
      of: find.byType(AuctionActionModal),
      matching: find.widgetWithText(ElevatedButton, 'Pasang Bid'),
    ),
  );
  await tester.pumpAndSettle();
  await tester.tap(find.text('Konfirmasi'));
  await tester.pumpAndSettle();
}

/// Drives the real winner UI: bottom-bar CTA "Klaim Sekarang" → shared
/// checkout bid-win intent.
Future<void> _winnerCheckoutThroughUi(WidgetTester tester) async {
  await tester.tap(find.text('Klaim Sekarang'));
  await tester.pumpAndSettle();
}

void main() {
  // The real auction detail chrome renders dates through AppFormatters (intl);
  // initialize the locale data so the widget tree can build. Harness-only.
  setUpAll(() async {
    await initializeDateFormatting();
  });

  group('Auction detail place bid — canonical restriction dispatch', () {
    testWidgets('MARKET_AUTHORITY_REQUIRED → canonical seller renewal', (
      tester,
    ) async {
      final auction = _auction(id: 'auction-bid-ma', sellerId: 'seller-1');
      final harness = await _pumpDetail(
        tester,
        auction: auction,
        bidErrorCode: 'MARKET_AUTHORITY_REQUIRED',
        bidErrorMessage: 'Active seller subscription required',
      );

      await _placeBidThroughUi(tester);

      expect(harness.notifier.placeBidCalls, 1);
      expect(harness.navigation.renewalCalls, 1);
      // Canonical renewal replaced the generic snackbar for this code.
      expect(find.textContaining('dibatasi'), findsNothing);
      expect(find.text('Active seller subscription required'), findsNothing);
      expect(tester.takeException(), isNull);
    });

    testWidgets('COMMERCE_RESTRICTED → unchanged restriction presentation', (
      tester,
    ) async {
      final auction = _auction(id: 'auction-bid-cr', sellerId: 'seller-1');
      final harness = await _pumpDetail(
        tester,
        auction: auction,
        bidErrorCode: 'COMMERCE_RESTRICTED',
        bidErrorMessage: 'Aktivitas commerce Anda saat ini dibatasi.',
      );

      await _placeBidThroughUi(tester);

      expect(
        find.textContaining('Aktivitas commerce Anda saat ini dibatasi'),
        findsOneWidget,
      );
      expect(find.textContaining('menempatkan bid'), findsOneWidget);
      expect(harness.navigation.renewalCalls, 0);
      expect(tester.takeException(), isNull);
    });

    testWidgets(
      'EMAIL_VERIFICATION_REQUIRED keeps the blocked-action gate (no regress)',
      (tester) async {
        final auction = _auction(id: 'auction-bid-ev', sellerId: 'seller-1');
        final harness = await _pumpDetail(
          tester,
          auction: auction,
          bidErrorCode: 'EMAIL_VERIFICATION_REQUIRED',
          bidErrorMessage: 'Email not verified',
        );

        await _placeBidThroughUi(tester);

        // Canonical presentation is the ERROR SNACKBAR (aligned with the
        // chat channel's identical EMAIL_VERIFICATION_REQUIRED handling) —
        // NOT a dialog. The old dialog expectations chased UI that never
        // existed in the auction detail screen.
        expect(
          find.text(
            'Verifikasi email kamu diperlukan sebelum menempatkan bid.',
          ),
          findsOneWidget,
        );
        expect(find.text('Verifikasi Email Diperlukan'), findsNothing);
        expect(harness.navigation.renewalCalls, 0);
        expect(tester.takeException(), isNull);
      },
    );

    testWidgets('BNR_AUCTION_RESTRICTED stays specialized', (tester) async {
      final auction = _auction(id: 'auction-bid-bnr', sellerId: 'seller-1');
      final harness = await _pumpDetail(
        tester,
        auction: auction,
        bidErrorCode: 'BNR_AUCTION_RESTRICTED',
        bidErrorMessage: 'Akses lelang dibatasi',
        bidErrorDetails: const {'permanent_ban': true},
      );

      await _placeBidThroughUi(tester);

      expect(find.text('Akses Lelang Dibatasi'), findsOneWidget);
      expect(
        find.textContaining('tidak menyelesaikan pembayaran lelang'),
        findsOneWidget,
      );
      expect(harness.navigation.renewalCalls, 0);
      expect(tester.takeException(), isNull);
    });

    testWidgets(
      'BNR_AUCTION_RESTRICTED acknowledgement closes the info notice',
      (tester) async {
        final auction = _auction(
          id: 'auction-bid-bnr-close',
          sellerId: 'seller-1',
        );
        final harness = await _pumpDetail(
          tester,
          auction: auction,
          bidErrorCode: 'BNR_AUCTION_RESTRICTED',
          bidErrorMessage: 'Akses lelang dibatasi',
          bidErrorDetails: const {'permanent_ban': true},
        );

        await _placeBidThroughUi(tester);

        expect(find.text('Akses Lelang Dibatasi'), findsOneWidget);
        expect(find.byType(AlertDialog), findsOneWidget);

        await tester.tap(find.text('Mengerti'));
        await tester.pumpAndSettle();

        // The acknowledgement closes the notice; the caller proceeds and
        // fires no unrelated navigation.
        expect(find.text('Akses Lelang Dibatasi'), findsNothing);
        expect(harness.navigation.renewalCalls, 0);
        expect(tester.takeException(), isNull);
      },
    );

    testWidgets(
      'generic error with a subscription-like MESSAGE stays generic',
      (tester) async {
        final auction = _auction(id: 'auction-bid-gen', sellerId: 'seller-1');
        final harness = await _pumpDetail(
          tester,
          auction: auction,
          bidErrorCode: 'FORBIDDEN',
          bidErrorMessage:
              'Active seller subscription required — perpanjang sekarang',
        );

        await _placeBidThroughUi(tester);

        // Dispatch is by CODE: a misleading message never triggers renewal.
        expect(harness.navigation.renewalCalls, 0);
        expect(find.text('Akses Lelang Dibatasi'), findsNothing);
        expect(
          find.textContaining('Active seller subscription required'),
          findsOneWidget,
        );
        expect(tester.takeException(), isNull);
      },
    );
  });

  group('Auction detail winner CTA — shared checkout entry', () {
    Auction winnerAuction(String id) => _auction(
      id: id,
      sellerId: 'seller-1',
      status: AuctionStatus.waitingSettlement,
      winnerId: 'buyer-1',
    );

    testWidgets(
      'winner CTA opens the SAME shared checkout with the bid-win intent',
      (tester) async {
        final auction = winnerAuction('auction-bidwin-route');
        final harness = await _pumpDetail(tester, auction: auction);

        await _winnerCheckoutThroughUi(tester);

        // ONE PURCHASE FUNNEL: no claim modal, no claim RPC — the winner
        // lands on the shared CheckoutScreen with bid_win=1. Checkout owns
        // address/shipping/pricing/order creation and its own restriction
        // presentation.
        expect(
          find.textContaining(
            'checkout:/checkout/${auction.id}',
            findRichText: true,
          ),
          findsOneWidget,
        );
        expect(find.textContaining('bid_win=1'), findsOneWidget);
        expect(find.textContaining('auction_id=${auction.id}'), findsOneWidget);
        expect(harness.notifier.placeBidCalls, 0);
        expect(tester.takeException(), isNull);
      },
    );

    testWidgets('non-winner never reaches the checkout entry', (tester) async {
      final auction = _auction(
        id: 'auction-not-winner',
        sellerId: 'seller-1',
        status: AuctionStatus.waitingSettlement,
        winnerId: 'someone-else',
      );
      final harness = await _pumpDetail(tester, auction: auction);

      // The winner CTA is not offered to non-winners at all.
      expect(find.text('Klaim Sekarang'), findsNothing);
      expect(harness.notifier.placeBidCalls, 0);
      expect(tester.takeException(), isNull);
    });
  });
}
