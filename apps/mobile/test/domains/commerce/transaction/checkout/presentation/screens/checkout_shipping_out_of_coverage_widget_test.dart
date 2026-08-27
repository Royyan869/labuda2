import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/observability/screen_view_route_observer.dart';
import 'package:labuda/domains/chat/chat/chat.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/entities/shipping.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/repositories/shipping_repository.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/finance/wallet/coins/coins.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/notifiers/address_notifier.dart';
import 'package:labuda/domains/user/profile/presentation/providers/state/address_state.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/models/wilayah_models.dart';

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
  int callCount = 0;
  String? lastUserId;
  String? lastOtherUserId;
  ShareReference? lastContext;

  @override
  ChatListState build() => const ChatListState();

  @override
  Future<Chat?> getOrCreateChat({
    required String userId,
    required String otherUserId,
    ShareReference? context,
  }) async {
    callCount++;
    lastUserId = userId;
    lastOtherUserId = otherUserId;
    lastContext = context;
    return Chat(
      id: 'chat-room-1',
      participantIds: [userId, otherUserId],
      participantNames: {userId: 'Buyer', otherUserId: 'Seller'},
      participantAvatars: const {},
      createdAt: DateTime.utc(2026, 8, 1),
      updatedAt: DateTime.utc(2026, 8, 1),
    );
  }
}

class _FakeShippingRepository implements ShippingRepository {
  _FakeShippingRepository(this._deliveryResult);

  final Result<DeliveryAvailabilityResult> _deliveryResult;

  @override
  Future<Result<DeliveryAvailabilityResult>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async => _deliveryResult;

  @override
  Future<Result<void>> deleteCoverage(String coverageId) async =>
      Result.error('unused');

  @override
  Future<Result<ShippingCoverage>> addCoverage(
    String optionId,
    AddCoverageRequest request,
  ) async => Result.error('unused');

  @override
  Future<Result<ShippingOption>> createShippingOption(
    CreateShippingOptionRequest request,
  ) async => Result.error('unused');

  @override
  Future<Result<ShippingOption>> getShippingOptionById(String optionId) async =>
      Result.error('unused');

  @override
  Future<Result<List<ShippingOption>>> listMyActiveShippingOptions() async =>
      Result.error('unused');

  @override
  Future<Result<List<ShippingOption>>> listMyShippingOptions() async =>
      Result.error('unused');

  @override
  Future<Result<void>> deleteShippingOption(String optionId) async =>
      Result.error('unused');

  @override
  Future<Result<void>> setProductShippingOptions(
    String productId,
    List<String> shippingOptionIds,
  ) async => Result.error('unused');

  @override
  Future<Result<void>> toggleActiveStatus(
    String optionId,
    bool isActive,
  ) async => Result.error('unused');

  @override
  Future<Result<ShippingOption>> updateShippingOptionFull(
    String optionId,
    UpdateShippingOptionFullRequest request,
  ) async => Result.error('unused');

  @override
  Future<Result<ShippingCoverage>> updateCoverage(
    String coverageId,
    UpdateCoverageRequest request,
  ) async => Result.error('unused');
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
    provider: ShonaAuthProvider.email,
    storeName: '',
  );
}

AddressEntity _shippingAddress({bool isPrimary = true}) {
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
    isPrimary: isPrimary,
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

Future<_FakeChatList> _pumpCheckout(
  WidgetTester tester, {
  required Result<DeliveryAvailabilityResult> deliveryResult,
  Brightness brightness = Brightness.light,
  double textScaleFactor = 1.0,
}) async {
  final chatNotifier = _FakeChatList();
  final shippingRepository = _FakeShippingRepository(deliveryResult);
  final address = _shippingAddress();
  final authState = AuthState.authenticated(_buyer(), emailVerified: true);

  tester.view.physicalSize = const Size(360, 800);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(
          () => _FakeAuthController(authState),
        ),
        coinProvider.overrideWith(() => _FakeCoinNotifier()),
        addressProvider.overrideWith(() => _FakeAddressNotifier([address])),
        forSaleDetailProvider.overrideWith((
          ref,
          fixedPriceSaleId,
        ) async {
          return _listing();
        }),
        shippingRepositoryProvider.overrideWithValue(shippingRepository),
        chatListProvider.overrideWith(() => chatNotifier),
        screenViewRouteObserverProvider.overrideWithValue(
          _FakeScreenViewRouteObserver(),
        ),
      ],
      child: MaterialApp.router(
        routerConfig: GoRouter(
          initialLocation: '/',
          routes: [
            GoRoute(
              path: '/',
              builder: (context, state) => const CheckoutScreen(
                productId: 'product-1',
                fixedPriceSaleId: 'sale-1',
              ),
            ),
            GoRoute(
              path: '/chat/:chatId',
              builder: (context, state) {
                return Scaffold(
                  body: Center(
                    child: Text('chat-${state.pathParameters['chatId']}'),
                  ),
                );
              },
            ),
          ],
        ),
        builder: (context, child) {
          final mediaQuery = MediaQuery.of(
            context,
          ).copyWith(textScaler: TextScaler.linear(textScaleFactor));
          return MediaQuery(data: mediaQuery, child: child!);
        },
        theme: ThemeData(
          brightness: Brightness.light,
          colorScheme: ColorScheme.fromSeed(
            seedColor: AppColors.primaryRed,
            brightness: Brightness.light,
          ),
          useMaterial3: true,
        ),
        darkTheme: ThemeData(
          brightness: Brightness.dark,
          colorScheme: ColorScheme.fromSeed(
            seedColor: AppColors.primaryRed,
            brightness: Brightness.dark,
          ),
          useMaterial3: true,
        ),
        themeMode: brightness == Brightness.dark
            ? ThemeMode.dark
            : ThemeMode.light,
      ),
    ),
  );

  await tester.pump();
  await tester.pumpAndSettle();
  return chatNotifier;
}

void main() {
  group('Checkout shipping out-of-coverage widget', () {
    testWidgets(
      'renders the out-of-coverage message once in the shipping section and invokes canonical chat exactly once',
      (tester) async {
        final result = Result.success(
          DeliveryAvailabilityResult.fromBackend(
            productConfigured: true,
            options: const [],
          ),
        );

        for (final brightness in [Brightness.light, Brightness.dark]) {
          final chatNotifier = await _pumpCheckout(
            tester,
            deliveryResult: result,
            brightness: brightness,
            textScaleFactor: 1.3,
          );

          const expectedMessage =
              'Alamat yang dipilih berada di luar coverage pengiriman seller. Hubungi penjual untuk menanyakan quote pengiriman.';

          expect(find.text(expectedMessage), findsOneWidget);
          expect(find.text('Hubungi Penjual'), findsOneWidget);
          expect(
            find.text('Di luar coverage pengiriman yang tersedia.'),
            findsNothing,
          );
          expect(find.text('Memuat harga dari server...'), findsNothing);
          expect(find.text('Memuat harga'), findsNothing);

          final button = tester.widget<ElevatedButton>(
            find.widgetWithText(ElevatedButton, 'Buat Pesanan'),
          );
          expect(button.onPressed, isNull);

          await tester.ensureVisible(find.text('Hubungi Penjual'));
          await tester.tap(find.text('Hubungi Penjual'));
          await tester.pumpAndSettle();

          expect(chatNotifier.callCount, 1);
          expect(chatNotifier.lastUserId, 'buyer-1');
          expect(chatNotifier.lastOtherUserId, 'seller-1');
          expect(chatNotifier.lastContext, isNull);
          expect(find.text('chat-chat-room-1'), findsOneWidget);
          expect(tester.takeException(), isNull);
        }
      },
    );

    testWidgets(
      'covered shipping still shows options and seller-not-configured stays distinct',
      (tester) async {
        final coveredResult = Result.success(
          DeliveryAvailabilityResult.fromBackend(
            productConfigured: true,
            options: const [
              DeliveryOption(
                shippingOptionId: 'ship-1',
                displayName: 'JNE Reguler',
                type: 'courier',
                rate: 15000,
                source: 'backend',
              ),
              DeliveryOption(
                shippingOptionId: 'ship-2',
                displayName: 'SiCepat',
                type: 'courier',
                rate: 18000,
                source: 'backend',
              ),
            ],
          ),
        );
        await _pumpCheckout(tester, deliveryResult: coveredResult);

        expect(find.text('JNE Reguler'), findsOneWidget);
        expect(find.text('SiCepat'), findsOneWidget);
        expect(find.text('Hubungi Penjual'), findsNothing);

        final sellerNotConfigured = Result.success(
          DeliveryAvailabilityResult.fromBackend(
            productConfigured: false,
            options: const [],
          ),
        );
        await _pumpCheckout(tester, deliveryResult: sellerNotConfigured);

        expect(find.text('Penjual belum mengatur pengiriman'), findsOneWidget);
        expect(
          find.text(
            'Alamat yang dipilih berada di luar coverage pengiriman seller. Hubungi penjual untuk menanyakan quote pengiriman.',
          ),
          findsNothing,
        );
        expect(find.text('Hubungi Penjual'), findsNothing);
        expect(find.text('Buat Pesanan'), findsOneWidget);
      },
    );
  });
}
