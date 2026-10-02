// Checkout — uncovered-area EXIT from the shipping picker (Phase 0).
//
// PROOF: when the delivery check returns ZERO options, the picker renders the
// canonical "Hubungi Penjual" CTA wired to the SAME channel the failed-order
// dialog uses (`_openChatWithSeller` → `openCommerceChat`). The buyer is never
// left at a dead end: the primary action stays honestly disabled, but the
// seller is reachable from the surface where the dead end actually occurs.
//
// Non-regression: with delivery options present the CTA does NOT render (no
// duplicate affordance), and tapping the CTA engages the canonical chat
// channel (its truthful failure copy appears), not a checkout-local navigator.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/chat.dart';
import 'package:labuda/domains/chat/chat/data/chat_providers.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';
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
// FAKES (proven combination: restriction-dispatch harness + preview boundary)
// ===========================================================================

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
  Future<void> loadAddressesByTag(
    String userId,
    AddressTag tag,
  ) async {}
}

class _FakeChatList extends ChatList {
  @override
  ChatListState build() => const ChatListState();

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

/// Canonical chat boundary: room resolution fails (the truthful failure the
/// CTA must surface). `getOrCreateChat` is a REAL method on [ChatList], so the
/// repository must be faked at the boundary — `noSuchMethod` never intercepts
/// it.
class _UnresolvableChatRepository implements ChatRepository {
  @override
  Future<Result<Chat>> getOrCreateChat({
    required List<String> participantIds,
  }) async => Result.error('room resolution unavailable');

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

/// The uncovered-area boundary: the seller configured nothing reachable for
/// this buyer address, so the check returns ZERO delivery options.
class _EmptyShippingRepository implements ShippingRepository {
  @override
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async => Result.success(const []);

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
    tags: const [AddressTag.shipping],
    recipientName: 'Buyer',
    phone: '08123456789',
    province: const Province(id: '31', name: 'DKI Jakarta'),
    city: const City(id: '3171', name: 'Jakarta Selatan', provinceId: '31'),
    district: const District(
      id: '3171010',
      name: 'Kebayoran',
      cityId: '3171',
    ),
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

/// Canonical preview money for the single delivery option (positive control).
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
);

// ===========================================================================
// HARNESS
// ===========================================================================

Future<void> _pumpCheckout(
  WidgetTester tester, {
  required List<DeliveryOption> options,
}) async {
  tester.view.physicalSize = const Size(600, 1800);
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
        forSaleDetailProvider.overrideWith((ref, forSaleId) async => _listing()),
        shippingRepositoryProvider.overrideWithValue(
          options.isEmpty
              ? _EmptyShippingRepository()
              : _SingleOptionShippingRepository(options),
        ),
        orderPreviewProvider.overrideWith(
          (ref, params) async => _previewResult(),
        ),
        checkoutRepositoryProvider.overrideWithValue(_NoopCheckoutRepository()),
        chatListProvider.overrideWith(() => _FakeChatList()),
        chatRepositoryProvider.overrideWithValue(_UnresolvableChatRepository()),
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
      ),
    ),
  );

  // Bounded pumps only: the screen renders a pricing spinner while
  // prerequisites are in flight, so `pumpAndSettle` never settles.
  await _settle(tester);
}

/// Positive-control shipping boundary: one configured option.
class _SingleOptionShippingRepository implements ShippingRepository {
  _SingleOptionShippingRepository(this.options);

  final List<DeliveryOption> options;

  @override
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async => Result.success(options);

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

Future<void> _settle(WidgetTester tester) async {
  for (var i = 0; i < 8; i++) {
    await tester.pump(const Duration(milliseconds: 150));
  }
}

ElevatedButton? _submitButton(WidgetTester tester) {
  for (final button
      in tester.widgetList<ElevatedButton>(find.byType(ElevatedButton))) {
    final child = button.child;
    if (child is Text && (child.data ?? '').startsWith('Buat Pesanan')) {
      return button;
    }
  }
  return null;
}

void main() {
  group('Shipping picker empty state — uncovered-area exit', () {
    testWidgets(
      'zero delivery options render the Hubungi Penjual CTA and keep the order action honestly disabled',
      (tester) async {
        await _pumpCheckout(tester, options: const []);

        expect(
          find.text('Tidak ada opsi pengiriman tersedia'),
          findsOneWidget,
        );
        expect(find.text('Hubungi Penjual'), findsOneWidget);

        // Honesty: no selectable shipping → no preview → no order creation.
        final submit = _submitButton(tester);
        expect(submit, isNotNull);
        expect(submit!.onPressed, isNull);
        expect(tester.takeException(), isNull);
      },
    );

    testWidgets(
      'the picker CTA engages the canonical chat channel, not a checkout-local navigator',
      (tester) async {
        await _pumpCheckout(tester, options: const []);

        final cta = find.text('Hubungi Penjual');
        expect(cta, findsOneWidget);
        await tester.ensureVisible(cta);
        await tester.pump();
        await tester.tap(cta);
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 400));

        // The canonical channel's truthful failure copy (room resolution
        // fails in this harness) proves the tap travelled through
        // `_openChatWithSeller` → `openCommerceChat`, exactly like the
        // failed-order dialog CTA. A checkout-local navigator cannot produce
        // this copy.
        expect(
          find.text('Gagal membuka chat. Coba lagi.'),
          findsOneWidget,
        );
        expect(tester.takeException(), isNull);
      },
    );

    testWidgets(
      'delivery options present → no contact CTA (no duplicate affordance)',
      (tester) async {
        await _pumpCheckout(
          tester,
          options: [
            const DeliveryOption(
              shippingSetupId: 'ship-A',
              displayName: 'Kurir A',
              type: 'courier',
              rate: 2222,
            ),
          ],
        );

        expect(find.text('Kurir A'), findsOneWidget);
        expect(find.text('Hubungi Penjual'), findsNothing);
        expect(tester.takeException(), isNull);
      },
    );
  });
}
