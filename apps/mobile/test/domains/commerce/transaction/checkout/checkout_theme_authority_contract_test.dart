// R5 — Checkout theme-authority contract (BEHAVIOR, not source text).
//
// PROOF: the SAME checkout screen renders its surfaces from the canonical
// `Theme.of(context).colorScheme`. The test pumps it twice — once under the
// app's light theme, once under the app's dark theme — and asserts the rendered
// colours follow the scheme in both. A widget that had a local colour authority
// (a hardcoded `AppColors.neutral*` constant, or a checkout-local
// `brightness == dark` branch) cannot satisfy both halves of this test.
//
// The negative guard at the bottom is what keeps the residue from coming back:
// no checkout-owned presentation file may reintroduce a palette neutral, a raw
// Material colour, or a local theme branch.
import 'dart:io';

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

class _NoopCheckoutRepository implements CheckoutRepository {
  @override
  Future<CheckoutResponse> createOrder(
    CheckoutRequest request, {
    String? idempotencyKey,
  }) async =>
      throw UnimplementedError();

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _FakeShippingRepository implements ShippingRepository {
  @override
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async => Result.success([
    DeliveryOption(       shippingSetupId: 'ship-A',
      displayName: 'Kurir A',
      type: 'courier',
      rate: 2222,
    ),
  ]);

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
  AuthState build() => AuthState.authenticated(_buyer(), emailVerified: true);
}

class _FakeAddressNotifier extends AddressNotifier {
  @override
  AddressState build() => AddressState(
    addresses: AsyncValue.data([_shippingAddress()]),
    primaryAddress: AsyncValue.data(_shippingAddress()),
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

/// The canonical preview money the fake backend returns for the single
/// auto-selected shipping option.
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

Future<void> _pumpCheckout(WidgetTester tester, ThemeData theme) async {
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
        addressProvider.overrideWith(_FakeAddressNotifier.new),
        forSaleDetailProvider.overrideWith((ref, forSaleId) async => _listing()),
        shippingRepositoryProvider.overrideWithValue(_FakeShippingRepository()),
        orderPreviewProvider.overrideWith(
          (ref, params) async => _previewResult(),
        ),
        checkoutRepositoryProvider.overrideWithValue(
          _NoopCheckoutRepository(),
        ),
      ],
      child: MaterialApp(
        theme: theme,
        home: const CheckoutScreen(
          productId: 'product-1',
          forSaleId: 'sale-1',
        ),
      ),
    ),
  );

  // Bounded pumps: the pricing indicator shows a spinner while loading.
  for (var i = 0; i < 8; i++) {
    await tester.pump(const Duration(milliseconds: 150));
  }
}

/// Every colour any checkout container decoration actually renders with.
Set<Color> _decorationColors(WidgetTester tester) => tester
    .widgetList<Container>(find.byType(Container))
    .map((c) => c.decoration)
    .whereType<BoxDecoration>()
    .map((d) => d.color)
    .whereType<Color>()
    .toSet();

/// Every border colour any checkout container decoration renders with.
Set<Color> _borderColors(WidgetTester tester) => tester
    .widgetList<Container>(find.byType(Container))
    .map((c) => c.decoration)
    .whereType<BoxDecoration>()
    .map((d) => d.border)
    .whereType<Border>()
    .map((b) => b.top.color)
    .toSet();

ElevatedButton _submitButton(WidgetTester tester) => tester
    .widgetList<ElevatedButton>(find.byType(ElevatedButton))
    .firstWhere(
      (b) => b.child is Text && (b.child! as Text).data!.startsWith('Buat'),
    );

void main() {
  testWidgets(
    'checkout surfaces and primary action follow the canonical color scheme in BOTH themes',
    (tester) async {
      // ---- DARK -------------------------------------------------------------
      await _pumpCheckout(tester, AppTheme.darkTheme);
      final dark = AppTheme.darkTheme.colorScheme;

      expect(Theme.of(tester.element(find.byType(Scaffold))).brightness,
          Brightness.dark);

      final darkScaffold = tester.widget<Scaffold>(find.byType(Scaffold));
      expect(
        darkScaffold.backgroundColor,
        dark.surfaceContainerLowest,
        reason: 'the screen background is the scheme tone, not a local branch',
      );
      expect(
        _decorationColors(tester),
        contains(dark.surface),
        reason: 'checkout cards render the scheme surface tone',
      );
      expect(
        _borderColors(tester),
        contains(dark.outlineVariant),
        reason: 'checkout card borders render the scheme outline tone',
      );
      expect(
        _decorationColors(tester),
        contains(dark.surfaceContainerHighest),
        reason: 'subtle fills render the scheme container tone',
      );

      // Enabled primary action: canonical primary + onPrimary.
      final darkSubmit = _submitButton(tester);
      expect(darkSubmit.onPressed, isNotNull);
      expect(
        darkSubmit.style?.backgroundColor?.resolve({}),
        dark.primary,
      );
      expect(
        darkSubmit.style?.foregroundColor?.resolve({}),
        dark.onPrimary,
      );

      // The rendered surface must NOT be the light-mode palette constant.
      expect(
        _decorationColors(tester),
        isNot(contains(AppColors.neutralWhite)),
        reason: 'a hardcoded light-mode surface cannot survive dark mode',
      );

      // ---- LIGHT ------------------------------------------------------------
      await _pumpCheckout(tester, AppTheme.lightTheme);
      final light = AppTheme.lightTheme.colorScheme;

      expect(
        Theme.of(tester.element(find.byType(Scaffold))).brightness,
        Brightness.light,
      );
      expect(
        tester.widget<Scaffold>(find.byType(Scaffold)).backgroundColor,
        light.surfaceContainerLowest,
      );
      expect(_decorationColors(tester), contains(light.surface));
      expect(_borderColors(tester), contains(light.outlineVariant));

      final lightSubmit = _submitButton(tester);
      expect(
        lightSubmit.style?.backgroundColor?.resolve({}),
        light.primary,
      );
    },
  );

  group('checkout colour-authority negative guard', () {
    // Checkout-owned presentation only. The payment-result screens live in this
    // folder but belong to the Payment Result domain, which this scope must not
    // touch (see the scope handoff), so they are excluded here.
    final checkoutUiFiles = <String>[
      'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart',
      ...Directory('lib/domains/commerce/transaction/checkout/presentation/widgets')
          .listSync()
          .whereType<File>()
          .where((f) => f.path.endsWith('.dart'))
          .map((f) => f.path.replaceAll(r'\', '/')),
    ];

    test('no checkout-owned UI file binds a palette neutral or a raw theme colour', () {
      // Palette neutrals and brand primary/secondary are all expressed by the
      // ColorScheme; binding them directly is the competing authority this
      // scope removed.
      final forbidden = RegExp(
        r'AppColors\.(neutral\w*|darkGray\w*|light|dark|primaryRed|primaryBlue)\b'
        r'|Colors\.(green|orange|red|grey|gray|blue)\b',
      );

      final violations = <String>[];
      for (final path in checkoutUiFiles) {
        final lines = File(path).readAsLinesSync();
        for (var i = 0; i < lines.length; i++) {
          if (forbidden.hasMatch(lines[i])) {
            violations.add('$path:${i + 1}: ${lines[i].trim()}');
          }
        }
      }
      expect(
        violations,
        isEmpty,
        reason: 'checkout owns no colour authority:\n${violations.join('\n')}',
      );
    });

    test('status/brand colours that have NO scheme role are still the palette authority', () {
      // These are business semantics (warning / success / Labuda Coins), not
      // theme roles: they must come from the core palette, never from ad-hoc
      // Material colours.
      final screenSource = File(checkoutUiFiles.first).readAsStringSync();
      expect(screenSource.contains('Colors.amber'), isFalse);
      expect(screenSource.contains('Colors.yellow'), isFalse);
    });
  });
}
