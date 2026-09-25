// Auction detail — canonical commerce-restriction dispatch by error CODE.
//
// PROOF for the two migrated call-sites in auction_detail_screen.dart:
//   1. `_handlePlaceBid()`  — bid rejection chain
//   2. `_showClaimDialog().onClaim` — winner claim callback
//
// Both now dispatch the restriction FAMILY through the canonical
// `CommerceRestrictionPresenter.handle(...)`:
//   - MARKET_AUTHORITY_REQUIRED → canonical seller renewal navigation
//   - COMMERCE_RESTRICTED       → unchanged restriction snackbar
//
// Specialized behavior is preserved and proven here as well:
//   - bid: EMAIL_VERIFICATION_REQUIRED gate, BNR_AUCTION_RESTRICTED dialog
//     with details, generic fallback
//   - claim: modal callback lifecycle (modal stays open on failure, pops with
//     the order id on success → payment-result navigation), generic fallback
//
// Authority proof: a generic `FORBIDDEN` whose MESSAGE reads like a
// subscription wall never triggers renewal — dispatch is by code.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_recommendation_providers.dart'
    show ownerOtherAuctionsProvider, similarAuctionsProvider;
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_action_modal.dart';
import 'package:labuda/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/entities/shipping.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/repositories/shipping_repository.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/finance/wallet/coins/coins.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_address_repository.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/models/wilayah_models.dart';

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

class _FakeAddressRepository implements IAddressRepository {
  _FakeAddressRepository(this._addresses);

  final List<AddressEntity> _addresses;

  @override
  Future<Result<List<AddressEntity>>> getAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  ) async => Result.success(_addresses);

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
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

/// Auction notifier with scripted bid/claim outcomes so the screen's rejection
/// chains can be exercised for each error code.
class _ScriptedAuctionNotifier extends AuctionNotifier {
  _ScriptedAuctionNotifier({
    required this.auction,
    this.bidErrorCode,
    this.bidErrorMessage,
    this.bidErrorDetails,
    this.claimErrorCode,
    this.claimErrorMessage,
    this.claimOrderId,
  });

  final Auction auction;
  final String? bidErrorCode;
  final String? bidErrorMessage;
  final Map<String, dynamic>? bidErrorDetails;
  final String? claimErrorCode;
  final String? claimErrorMessage;
  final String? claimOrderId;

  int placeBidCalls = 0;
  int claimCalls = 0;

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

  @override
  Future<String?> claimAuction({
    required String auctionId,
    required String addressId,
    required String shippingSetupId,
    String? discountCode,
    bool useCoins = false,
  }) async {
    claimCalls++;
    if (claimOrderId != null) {
      state = state.copyWith(
        isLoading: false,
        clearError: true,
        successMessage: 'Klaim berhasil! Pesanan telah dibuat',
      );
      return claimOrderId;
    }
    state = state.copyWith(
      isLoading: false,
      error: claimErrorMessage ?? 'Gagal mengklaim lelang',
      errorCode: claimErrorCode,
    );
    return null;
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
      certificates: ['ownership'],
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
    purpose: AddressPurpose.shipping,
    recipientName: 'Buyer',
    phone: '08123456789',
    province: const Province(id: 'province-1', name: 'Jawa Barat'),
    city: const City(id: 'city-1', name: 'Bandung', provinceId: 'province-1'),
    district: const District(id: 'district-1', name: 'Coblong', cityId: 'city-1'),
    village: const Village(id: 'village-1', name: 'Dago', districtId: 'district-1'),
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
  String? claimErrorCode,
  String? claimErrorMessage,
  String? claimOrderId,
  String currentUserId = 'buyer-1',
}) async {
  final notifier = _ScriptedAuctionNotifier(
    auction: auction,
    bidErrorCode: bidErrorCode,
    bidErrorMessage: bidErrorMessage,
    bidErrorDetails: bidErrorDetails,
    claimErrorCode: claimErrorCode,
    claimErrorMessage: claimErrorMessage,
    claimOrderId: claimOrderId,
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
        addressRepositoryProvider.overrideWithValue(
          _FakeAddressRepository([_shippingAddress()]),
        ),
        shippingRepositoryProvider.overrideWithValue(_FakeShippingRepository()),
        navigationHandlerProvider.overrideWithValue(navigation),
        savedItemRepositoryProvider.overrideWithValue(
          _FakeSavedItemRepository(),
        ),
      ],
      child: MaterialApp(
        home: AuctionDetailScreen(auctionId: auction.id),
        onGenerateRoute: (settings) {
          final name = settings.name;
          if (name != null && name.startsWith('/payment-result/')) {
            return MaterialPageRoute<void>(
              settings: settings,
              builder: (_) => Scaffold(
                body: Center(child: Text('payment-result:$name')),
              ),
            );
          }
          return null;
        },
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

/// Drives the real claim UI: winner CTA → claim modal → claim action.
Future<void> _claimThroughUi(WidgetTester tester) async {
  await tester.tap(find.text('Klaim Sekarang'));
  await tester.pumpAndSettle();
  await tester.tap(find.text('Klaim & Lanjutkan'));
  await tester.pumpAndSettle();
}

void main() {
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
          find.text('Verifikasi email kamu diperlukan sebelum menempatkan bid.'),
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

  group('Auction detail claim — canonical restriction dispatch', () {
    Auction winnerAuction(String id) => _auction(
      id: id,
      sellerId: 'seller-1',
      status: AuctionStatus.waitingSettlement,
      winnerId: 'buyer-1',
    );

    testWidgets('MARKET_AUTHORITY_REQUIRED → canonical seller renewal', (
      tester,
    ) async {
      final harness = await _pumpDetail(
        tester,
        auction: winnerAuction('auction-claim-ma'),
        claimErrorCode: 'MARKET_AUTHORITY_REQUIRED',
        claimErrorMessage: 'Active seller subscription required',
      );

      await _claimThroughUi(tester);

      expect(harness.notifier.claimCalls, 1);
      expect(harness.navigation.renewalCalls, 1);
      // Modal lifecycle preserved: callback returned null → modal stays open.
      expect(find.text('Klaim & Lanjutkan'), findsOneWidget);
      expect(
        find.text('Gagal mengklaim lelang. Silakan coba lagi.'),
        findsOneWidget,
      );
      expect(find.textContaining('payment-result:'), findsNothing);
      expect(tester.takeException(), isNull);
    });

    testWidgets('COMMERCE_RESTRICTED keeps restriction presentation', (
      tester,
    ) async {
      final harness = await _pumpDetail(
        tester,
        auction: winnerAuction('auction-claim-cr'),
        claimErrorCode: 'COMMERCE_RESTRICTED',
        claimErrorMessage: 'Aktivitas commerce Anda saat ini dibatasi.',
      );

      await _claimThroughUi(tester);

      expect(
        find.textContaining('Aktivitas commerce Anda saat ini dibatasi'),
        findsOneWidget,
      );
      // Canonical snackbar copy for this action (the modal's own generic
      // banner is separate and also mentions "mengklaim lelang").
      expect(find.textContaining('Tidak dapat mengklaim lelang'), findsOneWidget);
      expect(harness.navigation.renewalCalls, 0);
      expect(find.text('Klaim & Lanjutkan'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets('generic error keeps the generic fallback', (tester) async {
      final harness = await _pumpDetail(
        tester,
        auction: winnerAuction('auction-claim-gen'),
        claimErrorCode: 'FORBIDDEN',
        claimErrorMessage: 'Lelang sudah diklaim sebelumnya',
      );

      await _claimThroughUi(tester);

      expect(find.text('Lelang sudah diklaim sebelumnya'), findsOneWidget);
      expect(harness.navigation.renewalCalls, 0);
      expect(find.text('Klaim & Lanjutkan'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets(
      'successful claim still returns the order id to the modal lifecycle',
      (tester) async {
        final harness = await _pumpDetail(
          tester,
          auction: winnerAuction('auction-claim-ok'),
          claimOrderId: 'order-1',
        );

        await _claimThroughUi(tester);

        expect(harness.notifier.claimCalls, 1);
        expect(find.text('payment-result:/payment-result/order-1'),
            findsOneWidget);
        expect(harness.navigation.renewalCalls, 0);
        expect(tester.takeException(), isNull);
      },
    );
  });
}
