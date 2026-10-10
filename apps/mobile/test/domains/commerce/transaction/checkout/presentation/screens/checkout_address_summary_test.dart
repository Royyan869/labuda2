// Checkout shipping-address SUMMARY — the canonical "one address in Checkout" proof.
//
// OWNER TRUTH ENCODED HERE:
// - 0 addresses  → empty state + "Tambah Alamat" CTA (opens AddressFormDialog).
// - 1 address    → exactly one summary tile; "Ubah alamat" still available.
// - >1 addresses → Checkout renders EXACTLY ONE address (the canonical primary
//   by default). The remaining addresses exist ONLY inside the canonical
//   AddressPickerSheet (selection surface), never as Checkout inventory.
// - Choosing a non-primary address updates the caller-owned selection for this
//   order only: no `setPrimaryAddress`, no address-book mutation, and the
//   chosen address id is what reaches the preview/order pipeline.
//
// Behavior tests: real screen, real readiness, real notifier graph, faked
// network boundaries (same proven harness family as the preview-convergence
// and empty-state-CTA suites).
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:hishumi/domains/commerce/transaction/checkout/checkout.dart';
import 'package:hishumi/domains/commerce/transaction/order/domain/domain.dart';
import 'package:hishumi/domains/commerce/transaction/order/presentation/providers/order_providers.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/domain/entities/shipping.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/domain/repositories/shipping_repository.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:hishumi/domains/finance/transaction/payment/domain/entities/payment.dart';
import 'package:hishumi/domains/finance/transaction/payment/domain/repositories/payment_repository.dart';
import 'package:hishumi/domains/finance/transaction/payment/presentation/providers/payment_providers.dart';
import 'package:hishumi/domains/finance/wallet/coins/coins.dart';
import 'package:hishumi/domains/user/profile/domain/entities/address_entity.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/notifiers/address_notifier.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/state/address_state.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/address_form_dialog.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/shared.dart';

// ===========================================================================
// FAKES
// ===========================================================================

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => AuthState.authenticated(_buyer(), emailVerified: true);
}

class _FakeCoinNotifier extends CoinNotifier {
  @override
  CoinState build() => const CoinState.initial();

  @override
  Future<void> getBalance() async {}
}

/// Canonical address authority fake: seeds the collection, derives the primary
/// exactly like the real notifier, and RECORDS every `setPrimaryAddress` call
/// so the tests can prove Checkout never mutates primary.
class _FakeAddressNotifier extends AddressNotifier {
  _FakeAddressNotifier(this._addresses);

  final List<AddressEntity> _addresses;

  int setPrimaryCalls = 0;
  final List<String> loadedUserIds = [];

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
  Future<void> loadAddresses(String userId) async {
    loadedUserIds.add(userId);
  }

  @override
  Future<bool> setPrimaryAddress(String addressId, String userId) async {
    setPrimaryCalls++;
    return true;
  }
}

class _SingleOptionShippingRepository implements ShippingRepository {
  @override
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async => Result.success(const [
    DeliveryOption(
      shippingSetupId: 'ship-A',
      displayName: 'Kurir A',
      type: 'courier',
      rate: 2222,
    ),
  ]);

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

/// Minimal pre-order payment fake: one always-available method so the preview
/// pipeline is never blocked by the payment boundary.
class _FakePreOrderPaymentRepository implements PaymentRepository {
  @override
  Future<Result<PreOrderPaymentPricing>> getPreOrderPaymentPricing(
    String pricingToken, {
    bool useCoins = false,
  }) async => Result.success(
    PreOrderPaymentPricing(
      pricingToken: pricingToken,
      expiresAt: DateTime.now().add(const Duration(minutes: 10)),
      escrowAmount: 113222,
      coinsToUse: 0,
      cashAmount: 113222,
      currency: 'IDR',
      methods: const [
        PreOrderPaymentMethodOption(
          methodCode: 'bank_transfer',
          displayName: 'Transfer Bank',
          buyerPaymentFeeAmount: 0,
          finalPayableAmount: 113222,
        ),
      ],
    ),
  );

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _NoopCheckoutRepository implements CheckoutRepository {
  @override
  Future<CheckoutResponse> createOrder(
    CheckoutRequest request, {
    String? idempotencyKey,
  }) async => throw UnimplementedError();

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

AuthUser _buyer() => AuthUser(
  id: 'buyer-1',
  createdAt: DateTime.utc(2026, 8, 1),
  updatedAt: DateTime.utc(2026, 8, 1),
  email: 'buyer@example.com',
  username: 'buyer',
  isEmailVerified: true,
  roles: const [],
  provider: AuthProvider.email,
);

AddressEntity _address({
  required String id,
  required String recipient,
  bool isPrimary = false,
}) => AddressEntity(
  id: id,
  userId: 'buyer-1',
  recipientName: recipient,
  phone: '08123456789',
  province: const Province(id: '31', name: 'DKI Jakarta'),
  city: const City(id: '3171', name: 'Jakarta Selatan', provinceId: '31'),
  district: const District(id: '3171010', name: 'Kebayoran', cityId: '3171'),
  village: const Village(
    id: '3171010001',
    name: 'Melawai',
    districtId: '3171010',
  ),
  streetAddress: 'Jl. $recipient No. 1',
  postalCode: '12160',
  isPrimary: isPrimary,
  createdAt: DateTime.utc(2026, 8, 1),
  updatedAt: DateTime.utc(2026, 8, 1),
);

ForSale _listing() => ForSale(
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

PreviewOrderResult _previewResult() => PreviewOrderResult(
  pricing: const OrderPricing(
    subtotal: 111000,
    shippingCost: 2222,
    serviceFeeAmount: 0,
    totalPayableAmount: 113222,
  ),
  pricingToken: 'token-ship-A',
  sellerId: 'seller-1',
  shippingMode: 'standard',
  expiresAt: DateTime.now().add(const Duration(minutes: 10)),
);

// ===========================================================================
// HARNESS
// ===========================================================================

class _Harness {
  _Harness(this.addresses);

  final _FakeAddressNotifier addresses;
  PreviewOrderParams? lastPreviewParams;
  int previewCalls = 0;

  Future<void> pump(WidgetTester tester) async {
    tester.view.physicalSize = const Size(600, 1800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          loggerServiceProvider.overrideWithValue(LoggerService.instance),
          authControllerProvider.overrideWith(_FakeAuthController.new),
          coinProvider.overrideWith(_FakeCoinNotifier.new),
          addressProvider.overrideWith(() => addresses),
          forSaleDetailProvider.overrideWith(
            (ref, forSaleId) async => _listing(),
          ),
          shippingRepositoryProvider.overrideWithValue(
            _SingleOptionShippingRepository(),
          ),
          paymentRepositoryProvider.overrideWithValue(
            _FakePreOrderPaymentRepository(),
          ),
          orderPreviewProvider.overrideWith((ref, params) async {
            previewCalls++;
            lastPreviewParams = params;
            return _previewResult();
          }),
          checkoutRepositoryProvider.overrideWithValue(
            _NoopCheckoutRepository(),
          ),
        ],
        child: MaterialApp.router(
          // AddressFormDialog renders localized postal-code labels; every
          // pump that can open it must carry the canonical delegates.
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
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
        ),
      ),
    );

    await _settle(tester);
  }
}

_Harness _harnessWith(List<AddressEntity> addresses) =>
    _Harness(_FakeAddressNotifier(addresses));

/// Bounded pumps only: the screen renders pricing spinners that never settle.
Future<void> _settle(WidgetTester tester) async {
  for (var i = 0; i < 10; i++) {
    await tester.pump(const Duration(milliseconds: 150));
  }
}

Future<void> _openPicker(WidgetTester tester) async {
  final cta = find.text('Ubah alamat');
  expect(cta, findsOneWidget, reason: 'change-address CTA must be available');
  await tester.ensureVisible(cta);
  await tester.tap(cta);
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 350));
}

// ===========================================================================
// TESTS
// ===========================================================================

void main() {
  group('Checkout address summary — 0 addresses', () {
    testWidgets('renders the canonical empty state and no address card', (
      tester,
    ) async {
      final harness = _harnessWith([]);
      await harness.pump(tester);

      expect(find.text('Alamat Pengiriman'), findsOneWidget);
      expect(find.text('Belum ada alamat pengiriman'), findsOneWidget);
      expect(find.byType(ShippingAddressCard), findsNothing);
      // No address to change → no change affordance.
      expect(find.text('Ubah alamat'), findsNothing);
      expect(tester.takeException(), isNull);
    });

    testWidgets('the empty-state CTA opens the canonical AddressFormDialog', (
      tester,
    ) async {
      final harness = _harnessWith([]);
      await harness.pump(tester);

      final cta = find.text('Tambah Alamat');
      expect(cta, findsOneWidget);
      await tester.ensureVisible(cta);
      await tester.tap(cta);
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 350));

      // The ONE address form (not a checkout-local form) opened.
      expect(find.byType(AddressFormDialog), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  });

  group('Checkout address summary — exactly 1 address', () {
    testWidgets('renders exactly one summary tile with Ubah alamat available', (
      tester,
    ) async {
      final harness = _harnessWith([
        _address(id: 'address-a', recipient: 'Buyer A', isPrimary: true),
      ]);
      await harness.pump(tester);

      expect(find.byType(ShippingAddressCard), findsOneWidget);
      expect(find.text('Buyer A'), findsOneWidget);
      expect(find.text('Ubah alamat'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  });

  group('Checkout address summary — >1 addresses', () {
    testWidgets(
      'renders EXACTLY ONE address (canonical primary); the rest stay out of Checkout',
      (tester) async {
        final harness = _harnessWith([
          _address(id: 'address-a', recipient: 'Buyer A', isPrimary: true),
          _address(id: 'address-b', recipient: 'Buyer B'),
          _address(id: 'address-c', recipient: 'Buyer C'),
        ]);
        await harness.pump(tester);

        // ONE tile only — no address inventory in Checkout.
        expect(find.byType(ShippingAddressCard), findsOneWidget);
        expect(find.text('Buyer A'), findsOneWidget);
        expect(find.text('Buyer B'), findsNothing);
        expect(find.text('Buyer C'), findsNothing);
        expect(find.text('Utama'), findsOneWidget);
        expect(tester.takeException(), isNull);
      },
    );

    testWidgets(
      'non-primary selection via the picker becomes the summary, without mutating primary',
      (tester) async {
        final harness = _harnessWith([
          _address(id: 'address-a', recipient: 'Buyer A', isPrimary: true),
          _address(id: 'address-b', recipient: 'Buyer B'),
          _address(id: 'address-c', recipient: 'Buyer C'),
        ]);
        await harness.pump(tester);

        // Default summary = canonical primary A.
        expect(find.text('Buyer A'), findsOneWidget);

        await _openPicker(tester);

        // The picker is the ONLY surface that lists the inventory. While it
        // is open, the summary tile behind it still renders → 1 + 3 cards.
        expect(find.text('Pilih Alamat Pengiriman'), findsOneWidget);
        expect(find.byType(ShippingAddressCard), findsNWidgets(4));
        expect(find.text('Buyer B'), findsOneWidget);
        expect(find.text('Buyer C'), findsOneWidget);
        // A appears in the summary (behind) AND as the picker's primary row.
        expect(find.text('Buyer A'), findsNWidgets(2));
        // Current selection (A) marked: summary card + picker card both
        // checked; B and C are the only unselected radios.
        expect(find.byIcon(Icons.radio_button_checked), findsNWidgets(2));
        expect(find.byIcon(Icons.radio_button_off), findsNWidgets(2));

        // Choose B — a non-primary address.
        await tester.ensureVisible(find.text('Buyer B'));
        await tester.tap(find.text('Buyer B'));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));
        await _settle(tester);

        // Checkout now summarizes B ONLY; the inventory is gone again.
        expect(find.text('Pilih Alamat Pengiriman'), findsNothing);
        expect(find.byType(ShippingAddressCard), findsOneWidget);
        expect(find.text('Buyer B'), findsOneWidget);
        expect(find.text('Buyer A'), findsNothing);
        expect(find.text('Buyer C'), findsNothing);
        // B is not primary → no primary badge on the summary.
        expect(find.text('Utama'), findsNothing);

        // ORDER-SCOPED SELECTION: no primary mutation, ever.
        expect(harness.addresses.setPrimaryCalls, 0);

        // The chosen address is what reaches the pricing pipeline.
        expect(harness.previewCalls, greaterThan(0));
        expect(harness.lastPreviewParams?.addressId, 'address-b');
        expect(tester.takeException(), isNull);
      },
    );
  });
}
