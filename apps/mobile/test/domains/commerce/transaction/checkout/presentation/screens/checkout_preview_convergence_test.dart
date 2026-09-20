// Checkout preview convergence — R1 / R1.1 / R1.2 behavior proof.
//
// ROOT CAUSE PROVEN HERE (R1): the screen read the canonical preview provider
// WITHOUT awaiting its future. The provider family key was built from the
// current inputs, so a fresh key was AsyncLoading and `hasValue` stayed false:
// the backend pricing was never applied, readiness never became true, and
// "Buat Pesanan" was permanently unreachable.
//
// These are BEHAVIOR tests: they drive the real screen, the real readiness
// projection and the real order-creation guard through the real provider graph
// (with the pricing/shipping providers faked at the boundary). No source-text
// assertion appears in this file.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/transaction/checkout/checkout.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/order/presentation/providers/order_providers.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/entities/shipping.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/repositories/shipping_repository.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/finance/wallet/coins/coins.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/notifiers/address_notifier.dart';
import 'package:labuda/domains/user/profile/presentation/providers/state/address_state.dart';
import 'package:labuda/shared/models/wilayah_models.dart';

// ===========================================================================
// FAKES
// ===========================================================================

/// Records whether order creation was ever attempted.
class _RecordingCheckoutRepository implements CheckoutRepository {
  int createOrderCalls = 0;
  CheckoutRequest? lastRequest;

  @override
  Future<CheckoutResponse> createOrder(
    CheckoutRequest request, {
    String? idempotencyKey,
  }) async {
    createOrderCalls++;
    lastRequest = request;
    return CheckoutResponse(
      orderId: 'order-1',
      orderNumber: 'ORD-1',
      status: 'pending_payment',
      subtotal: 0,
      shippingTotal: 0,
      commissionAmount: 0,
      totalBeforeCoinsAmount: 0,
      createdAt: DateTime.utc(2026, 9, 18),
    );
  }
}

/// Drives the canonical preview boundary: every request is recorded, and the
/// result can be held open so overlapping-input scenarios are deterministic.
class _PreviewEngine {
  final Map<String, Completer<PreviewOrderResult>> _pending = {};

  /// Signatures (shipping option id) of every preview request, in order.
  final List<String> started = [];

  /// When true a request completes immediately with the canonical result.
  bool autoComplete = true;

  /// When set, every request fails with this error.
  Object? failure;

  static const subtotals = {'ship-A': 111000.0, 'ship-B': 222000.0};
  static const shipping = {'ship-A': 2222.0, 'ship-B': 3333.0};

  static String keyOf(PreviewOrderParams params) =>
      params.shippingSetupId ?? params.shippingQuoteId ?? 'none';

  Future<PreviewOrderResult> request(PreviewOrderParams params) {
    final key = keyOf(params);
    started.add(key);
    if (failure != null) {
      return Future<PreviewOrderResult>.error(failure!);
    }
    final completer = Completer<PreviewOrderResult>();
    _pending[key] = completer;
    if (autoComplete) {
      completer.complete(resultFor(key));
    }
    return completer.future;
  }

  PreviewOrderResult resultFor(String key) {
    final subtotal = subtotals[key] ?? 0.0;
    final ship = shipping[key] ?? 0.0;
    return PreviewOrderResult(
      pricing: OrderPricing(
        subtotal: subtotal,
        shippingCost: ship,
        serviceFeeAmount: 0,
        totalPayableAmount: subtotal + ship,
      ),
      pricingToken: 'token-$key',
      sellerId: 'seller-1',
      shippingMode: 'standard',
    );
  }

  /// Resolves the request that is still in flight for [key].
  void complete(String key) {
    final completer = _pending.remove(key);
    completer?.complete(resultFor(key));
  }
}

class _FakeShippingRepository implements ShippingRepository {
  _FakeShippingRepository(this.options);

  final List<DeliveryOption> options;

  @override
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async => Result.success(options);

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _FakeCoinNotifier extends CoinNotifier {
  @override
  CoinState build() => const CoinState.initial();

  @override
  Future<void> getBalance() async {}
}

class _FakeAuthController extends AuthController {
  @override
  AuthState build() =>
      AuthState.authenticated(_buyer(), emailVerified: true);
}

class _FakeAddressNotifier extends AddressNotifier {
  _FakeAddressNotifier(this.addresses);

  final List<AddressEntity> addresses;

  @override
  AddressState build() => AddressState(
    addresses: AsyncValue.data(addresses),
    primaryAddress: AsyncValue.data(addresses.first),
  );

  @override
  Future<void> loadAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  ) async {}
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

AddressEntity _shippingAddress() => AddressEntity(
  id: 'address-1',
  userId: 'buyer-1',
  purpose: AddressPurpose.shipping,
  recipientName: 'Buyer',
  phone: '08123456789',
  province: const Province(id: '31', name: 'DKI Jakarta'),
  city: const City(id: '3171', name: 'Jakarta Selatan', provinceId: '31'),
  district: const District(id: '3171010', name: 'Kebayoran', cityId: '3171'),
  village: const Village(
    id: '3171010001',
    name: 'Melawai',
    districtId: '3171010',
  ),
  streetAddress: 'Jl. Test No. 1',
  postalCode: '12160',
  isPrimary: true,
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

DeliveryOption _option(String id, String label, double rate) => DeliveryOption(
  shippingSetupId: id,
  displayName: label,
  type: 'courier',
  rate: rate,
);

// ===========================================================================
// HARNESS
// ===========================================================================

/// Compact thousand-separated amount, exactly as the checkout surfaces render
/// money (summary rows and the primary action label).
String _money(double amount) => amount
    .toStringAsFixed(0)
    .replaceAllMapped(
      RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'),
      (match) => '${match[1]}.',
    );

final _shipA = _option('ship-A', 'Kurir A', 2222);
final _shipB = _option('ship-B', 'Kurir B', 3333);

class _Harness {
  _Harness(this.engine, this.repository);

  final _PreviewEngine engine;
  final _RecordingCheckoutRepository repository;
}

Future<_Harness> _pumpCheckout(
  WidgetTester tester, {
  required _PreviewEngine engine,
  required List<DeliveryOption> options,
}) async {
  final repository = _RecordingCheckoutRepository();

  tester.view.physicalSize = const Size(600, 1800);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(_FakeAuthController.new),
        coinProvider.overrideWith(_FakeCoinNotifier.new),
        addressProvider.overrideWith(
          () => _FakeAddressNotifier([_shippingAddress()]),
        ),
        forSaleDetailProvider.overrideWith((ref, forSaleId) async => _listing()),
        shippingRepositoryProvider.overrideWithValue(
          _FakeShippingRepository(options),
        ),
        orderPreviewProvider.overrideWith(
          (ref, params) => engine.request(params),
        ),
        checkoutRepositoryProvider.overrideWithValue(repository),
      ],
      child: const MaterialApp(
        home: CheckoutScreen(productId: 'product-1', forSaleId: 'sale-1'),
      ),
    ),
  );

  await _settle(tester);
  return _Harness(engine, repository);
}

/// Bounded pumps only: the pricing indicator renders a spinner while it is
/// loading, so `pumpAndSettle` would never settle.
Future<void> _settle(WidgetTester tester) async {
  for (var i = 0; i < 8; i++) {
    await tester.pump(const Duration(milliseconds: 150));
  }
}

/// Taps like a user would: the checkout body scrolls, so bring the target into
/// view first.
Future<void> _tap(WidgetTester tester, Finder finder) async {
  await tester.ensureVisible(finder);
  await tester.pump();
  await tester.tap(finder);
  await tester.pump();
}

/// The primary checkout action ("Buat Pesanan…").
ElevatedButton? _submitButton(WidgetTester tester) {
  for (final button in tester.widgetList<ElevatedButton>(
    find.byType(ElevatedButton),
  )) {
    final child = button.child;
    if (child is Text && (child.data ?? '').startsWith('Buat Pesanan')) {
      return button;
    }
  }
  return null;
}

void main() {
  group('R1 — the canonical preview actually resolves', () {
    testWidgets(
      'backend pricing becomes the checkout money and the action becomes reachable',
      (tester) async {
        final engine = _PreviewEngine();
        await _pumpCheckout(
          tester,
          engine: engine,
          options: [_shipA], // single option → auto-selected by the screen
        );

        // The preview was requested through the canonical provider boundary…
        expect(engine.started, ['ship-A']);

        // …and its result is what the buyer now sees: subtotal 111.000 +
        // shipping 2.222 = 113.222.
        expect(find.textContaining(_money(113222)), findsWidgets);

        // The local projection is gone: this is backend money, not a local one.
        expect(
          find.text('Harga lokal sementara — menunggu harga dari server'),
          findsNothing,
        );

        final submit = _submitButton(tester);
        expect(submit, isNotNull, reason: 'the primary action must exist');
        expect(
          submit!.onPressed,
          isNotNull,
          reason: 'a fresh backend preview must make checkout reachable',
        );
        expect(find.textContaining('Buat Pesanan'), findsWidgets);
      },
    );
  });

  group('R1.1 — stale / out-of-order results can never become current', () {
    testWidgets(
      'a result for changed inputs is discarded, the latest inputs win, and no refresh is dropped',
      (tester) async {
        final engine = _PreviewEngine()..autoComplete = false;
        final harness = await _pumpCheckout(
          tester,
          engine: engine,
          options: [_shipA, _shipB],
        );

        // No option selected yet: the honest state is a prerequisite, and no
        // order can be created.
        expect(find.text('Pilih opsi pengiriman untuk memuat harga dari server.'),
            findsWidgets);
        expect(_submitButton(tester)!.onPressed, isNull);

        // Select A → request A is in flight.
        await _tap(tester, find.text('Kurir A'));
        await _settle(tester);
        expect(engine.started, ['ship-A']);
        expect(_submitButton(tester)!.onPressed, isNull);
        expect(find.textContaining(_money(113222)), findsNothing);

        // Change the input while A is still in flight: B must not be issued
        // concurrently, and the change must be QUEUED.
        await _tap(tester, find.text('Kurir B'));
        await _settle(tester);
        expect(
          engine.started,
          ['ship-A'],
          reason: 'a single preview request may be in flight at a time',
        );

        // A completes AFTER the inputs changed → it must be discarded.
        engine.complete('ship-A');
        await _settle(tester);
        expect(
          find.textContaining(_money(113222)),
          findsNothing,
          reason: 'a preview for other inputs must never be applied',
        );
        expect(
          engine.started,
          ['ship-A', 'ship-B'],
          reason: 'the queued refresh must be re-issued with the latest inputs',
        );
        expect(
          _submitButton(tester)!.onPressed,
          isNull,
          reason: 'stale pricing can never enable the primary action',
        );

        // Even a direct tap while not ready must not create an order.
        await tester.tap(find.textContaining('Buat Pesanan'));
        await _settle(tester);
        expect(harness.repository.createOrderCalls, 0);

        // B completes → B is the current preview.
        engine.complete('ship-B');
        await _settle(tester);
        expect(find.textContaining(_money(225333)), findsWidgets);
        expect(find.textContaining(_money(113222)), findsNothing);
        expect(_submitButton(tester)!.onPressed, isNotNull);
      },
    );

    testWidgets(
      'changing an input invalidates the applied preview until it is refreshed',
      (tester) async {
        final engine = _PreviewEngine();
        final harness = await _pumpCheckout(
          tester,
          engine: engine,
          options: [_shipA, _shipB],
        );

        // A → applied and ready.
        await _tap(tester, find.text('Kurir A'));
        await _settle(tester);
        expect(find.textContaining(_money(113222)), findsWidgets);
        expect(_submitButton(tester)!.onPressed, isNotNull);

        // Now hold the next request open and change the input.
        engine.autoComplete = false;
        await _tap(tester, find.text('Kurir B'));
        await _settle(tester);

        // The preview applied for A is no longer current, so it must not be
        // shown as checkout money, and the action must be unreachable.
        expect(
          find.textContaining(_money(113222)),
          findsNothing,
          reason: 'the previous preview is not current anymore',
        );
        expect(_submitButton(tester)!.onPressed, isNull);

        await tester.tap(find.textContaining('Buat Pesanan'));
        await _settle(tester);
        expect(harness.repository.createOrderCalls, 0);

        engine.complete('ship-B');
        await _settle(tester);
        expect(find.textContaining(_money(225333)), findsWidgets);
        expect(_submitButton(tester)!.onPressed, isNotNull);
      },
    );
  });

  group('R1.2 — readiness is truthful', () {
    testWidgets(
      'a failed preview becomes an actionable retry state, never a permanent spinner',
      (tester) async {
        final engine = _PreviewEngine()..failure = Exception('preview down');
        await _pumpCheckout(tester, engine: engine, options: [_shipA]);

        expect(
          engine.started,
          ['ship-A'],
          reason: 'a failed preview must not be silently retried in the dark',
        );
        expect(find.text('Gagal Memuat Harga'), findsWidgets);
        expect(
          find.text('Memuat harga dari server...'),
          findsNothing,
          reason: 'a failure must not be presented as loading',
        );
        expect(find.text('Refresh'), findsWidgets);
        expect(_submitButton(tester)!.onPressed, isNull);

        // Retry actually re-requests pricing and recovers.
        engine.failure = null;
        await _tap(tester, find.text('Refresh').first);
        await _settle(tester);

        expect(engine.started.length, 2);
        expect(find.textContaining(_money(113222)), findsWidgets);
        expect(_submitButton(tester)!.onPressed, isNotNull);
      },
    );

    testWidgets(
      'a not-ready checkout cannot create an order',
      (tester) async {
        final engine = _PreviewEngine();
        final harness = await _pumpCheckout(
          tester,
          engine: engine,
          options: [_shipA, _shipB],
        );

        expect(engine.started, isEmpty);
        expect(_submitButton(tester)!.onPressed, isNull);

        await tester.tap(find.textContaining('Buat Pesanan'));
        await _settle(tester);
        expect(harness.repository.createOrderCalls, 0);
      },
    );
  });
}
