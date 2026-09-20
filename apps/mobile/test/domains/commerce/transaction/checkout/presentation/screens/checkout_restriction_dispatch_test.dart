// Checkout — canonical commerce-restriction dispatch by error CODE.
//
// PROOF: the checkout error listener (`ref.listen<CheckoutState>`) dispatches
// the restriction FAMILY through the canonical
// `CommerceRestrictionPresenter.handle(...)`:
//   - MARKET_AUTHORITY_REQUIRED → canonical seller renewal navigation
//   - COMMERCE_RESTRICTED       → unchanged restriction snackbar
//
// Non-regression: the email-verification gate, the code-specific
// pricing/shipping branches and the generic fallback keep their behavior.
//
// Authority proof: a generic `FORBIDDEN` whose MESSAGE reads like a
// subscription wall must fall through to the generic snackbar — dispatch is
// by code, never by message string.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/observability/screen_view_route_observer.dart';
import 'package:labuda/domains/chat/chat/chat.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/transaction/checkout/checkout.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/entities/shipping.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/repositories/shipping_repository.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/finance/wallet/coins/coins.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/notifiers/address_notifier.dart';
import 'package:labuda/domains/user/profile/presentation/providers/state/address_state.dart';
import 'package:labuda/shared/models/wilayah_models.dart';

class _RecordingNavigationHandler extends Fake implements NavigationHandler {
  int renewalCalls = 0;

  @override
  void navigateToSellerRenewal() => renewalCalls++;
}

class _FakeAnalyticsRepository implements IAnalyticsRepository {
  @override
  Future<Result<void>> flush() async => Result.error('unused');

  @override
  Future<Result<AnalyticsCircumventionStats>> getCircumventionStats({
    required DateTime startDate,
    required DateTime endDate,
    String? userId,
    String? violationType,
  }) async => Result.error('unused');

  @override
  Future<Result<void>> logCircumventionAttempt(
    String content,
    String userId, {
    Map<String, dynamic>? extra,
  }) async => Result.error('unused');

  @override
  Future<Result<void>> logEvent(
    String eventName, {
    Map<String, dynamic>? parameters,
    String? userId,
  }) async => Result.error('unused');

  @override
  Future<Result<void>> logUserAction(
    String action,
    String userId, {
    Map<String, dynamic>? extra,
  }) async => Result.error('unused');

  @override
  Future<Result<void>> setUserProperties(
    Map<String, dynamic> properties,
  ) async => Result.error('unused');

  @override
  Future<Result<void>> trackEngagement({
    required String userId,
    required String contentId,
    required String contentType,
    required String engagementType,
    int? duration,
  }) async => Result.error('unused');
}

class _FakeScreenViewRouteObserver extends ScreenViewRouteObserver {
  _FakeScreenViewRouteObserver() : super(_FakeAnalyticsRepository());
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
  Future<void> loadAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  ) async {}
}

class _FakeChatList extends ChatList {
  @override
  ChatListState build() => const ChatListState();

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
      displayName: 'Kurir',
      type: 'courier',
      rate: 15000,
    ),
  ]);

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

/// Checkout notifier whose error state is scripted directly so the screen's
/// `ref.listen` error dispatch can be exercised for each error code.
class _ScriptedCheckoutNotifier extends CheckoutNotifier {
  @override
  CheckoutState build() => const CheckoutState();

  void emitError({required String message, String? code}) {
    state = state.copyWith(error: message, errorCode: code);
  }
}

AuthUser _buyer() {
  return AuthUser(
    id: 'buyer-1',
    createdAt: DateTime.utc(2026, 8, 1),
    updatedAt: DateTime.utc(2026, 8, 1),
    email: 'buyer@example.com',
    username: 'buyer',
    isEmailVerified: true,
    roles: const [],
    provider: AuthProvider.email,
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

ForSale _listing() {
  return ForSale(
    forSaleId: 'sale-1',
    productId: 'product-1',
    title: 'Ikan Koi Test',
    description: 'Deskripsi test',
    price: 1250000,
    stock: 3,
    media: const [],
    sellerId: 'seller-1',
    status: ForSaleStatus.active,
    visibility: ForSaleVisibility.public,
    createdAt: DateTime.utc(2026, 8, 1),
    updatedAt: DateTime.utc(2026, 8, 1),
  );
}

class _Harness {
  _Harness(this.notifier, this.navigation);

  final _ScriptedCheckoutNotifier notifier;
  final _RecordingNavigationHandler navigation;
}

Future<_Harness> _pumpCheckout(WidgetTester tester) async {
  final notifier = _ScriptedCheckoutNotifier();
  final navigation = _RecordingNavigationHandler();

  // Wide surface: the checkout order-summary row overflows at phone width and
  // a RenderFlex overflow would fail the test for an unrelated reason.
  tester.view.physicalSize = const Size(520, 1400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(
          () => _FakeAuthController(
            AuthState.authenticated(_buyer(), emailVerified: true),
          ),
        ),
        coinProvider.overrideWith(() => _FakeCoinNotifier()),
        addressProvider.overrideWith(
          () => _FakeAddressNotifier([_shippingAddress()]),
        ),
        forSaleDetailProvider.overrideWith(
          (ref, forSaleId) async => _listing(),
        ),
        shippingRepositoryProvider.overrideWithValue(_FakeShippingRepository()),
        chatListProvider.overrideWith(() => _FakeChatList()),
        screenViewRouteObserverProvider.overrideWithValue(
          _FakeScreenViewRouteObserver(),
        ),
        checkoutNotifierProvider.overrideWith(() => notifier),
        navigationHandlerProvider.overrideWithValue(navigation),
      ],
      child: MaterialApp.router(
        routerConfig: GoRouter(
          initialLocation: '/',
          routes: [
            GoRoute(
              path: '/',
              builder: (context, state) => const CheckoutScreen(
                productId: 'product-1',
                forSaleId: 'sale-1',
              ),
            ),
          ],
        ),
        theme: ThemeData(
          brightness: Brightness.light,
          colorScheme: ColorScheme.fromSeed(
            seedColor: AppColors.primaryRed,
            brightness: Brightness.light,
          ),
          useMaterial3: true,
        ),
      ),
    ),
  );

  // Bounded pumps only: the screen renders a pricing spinner on first frame
  // (no address/shipping option is selected), so `pumpAndSettle` never
  // settles. Presentation under test is driven by explicit pumps instead.
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 300));
  return _Harness(notifier, navigation);
}

/// Emits an error into the checkout notifier and settles the presentation.
Future<void> _emit(
  WidgetTester tester,
  _ScriptedCheckoutNotifier notifier, {
  required String message,
  String? code,
}) async {
  notifier.emitError(message: message, code: code);
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));
}

void main() {
  group('Checkout restriction dispatch — canonical presenter', () {
    testWidgets(
      'MARKET_AUTHORITY_REQUIRED goes to canonical seller renewal only',
      (tester) async {
        final harness = await _pumpCheckout(tester);

        await _emit(
          tester,
          harness.notifier,
          message: 'Active seller subscription required',
          code: 'MARKET_AUTHORITY_REQUIRED',
        );

        expect(harness.navigation.renewalCalls, 1);
        // Canonical renewal replaced every other presentation for this code.
        expect(find.textContaining('dibatasi'), findsNothing);
        expect(find.text('Active seller subscription required'), findsNothing);
        // The consumed error is cleared from the notifier (local side effect).
        expect(harness.notifier.state.error, isNull);
        expect(tester.takeException(), isNull);
      },
    );

    testWidgets('COMMERCE_RESTRICTED keeps the existing restriction snackbar', (
      tester,
    ) async {
      final harness = await _pumpCheckout(tester);

      await _emit(
        tester,
        harness.notifier,
        message: 'Aktivitas commerce Anda saat ini dibatasi.',
        code: 'COMMERCE_RESTRICTED',
      );

      expect(
        find.textContaining('Aktivitas commerce Anda saat ini dibatasi'),
        findsOneWidget,
      );
      expect(find.textContaining('melakukan checkout'), findsOneWidget);
      expect(harness.navigation.renewalCalls, 0);
      expect(harness.notifier.state.error, isNull);
      expect(tester.takeException(), isNull);
    });

    testWidgets(
      'EMAIL_VERIFICATION_REQUIRED keeps the blocked-action gate (no regress)',
      (tester) async {
        final harness = await _pumpCheckout(tester);

        await _emit(
          tester,
          harness.notifier,
          message: 'Email not verified',
          code: 'EMAIL_VERIFICATION_REQUIRED',
        );

        expect(find.text('Verifikasi Email Diperlukan'), findsOneWidget);
        expect(
          find.textContaining('Untuk melakukan checkout'),
          findsOneWidget,
        );
        expect(harness.navigation.renewalCalls, 0);
        expect(harness.notifier.state.error, isNull);
        expect(tester.takeException(), isNull);
      },
    );

    testWidgets('PRICING_TOKEN_INVALID keeps its code-specific presentation', (
      tester,
    ) async {
      final harness = await _pumpCheckout(tester);

      await _emit(
        tester,
        harness.notifier,
        message: 'PRICING_TOKEN_INVALID',
        code: 'PRICING_TOKEN_INVALID',
      );

      expect(find.text('Token Tidak Valid'), findsOneWidget);
      expect(harness.navigation.renewalCalls, 0);
      expect(tester.takeException(), isNull);
    });

    testWidgets('NO_SHIPPING_OPTIONS keeps its shipping-specific presentation', (
      tester,
    ) async {
      final harness = await _pumpCheckout(tester);

      await _emit(
        tester,
        harness.notifier,
        message: 'NO_SHIPPING_OPTIONS',
        code: 'NO_SHIPPING_OPTIONS',
      );

      expect(find.text('Pengiriman Belum Diatur Penjual'), findsOneWidget);
      expect(find.text('Tutup'), findsOneWidget);
      expect(harness.navigation.renewalCalls, 0);
      expect(tester.takeException(), isNull);
    });

    testWidgets(
      'generic error whose MESSAGE reads like a subscription wall stays generic',
      (tester) async {
        final harness = await _pumpCheckout(tester);

        await _emit(
          tester,
          harness.notifier,
          message: 'Active seller subscription required — perpanjang sekarang',
          code: 'FORBIDDEN',
        );

        // Dispatch is by CODE: a misleading message never triggers renewal.
        expect(harness.navigation.renewalCalls, 0);
        expect(find.textContaining('dibatasi'), findsNothing);
        expect(
          find.textContaining('Active seller subscription required'),
          findsOneWidget,
        );
        expect(tester.takeException(), isNull);
      },
    );
  });
}
