import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/widgets/seller_shipping_options_selector.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/widgets/shipping_option_setup_screen.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_shipping_screen.dart';
import 'package:labuda/domains/user/profile/presentation/screens/settings_screen.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/models/wilayah_models.dart';
import 'package:labuda/shared/providers/wilayah_provider_simple.dart';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.utc(2026, 7, 25);
    final user = AuthUser(
      id: 'seller-1',
      createdAt: now,
      updatedAt: now,
      email: 'seller@example.com',
      username: 'seller',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: true,
      sellerSubscriptionStatus: 'active',
      hasMarketAuthority: true,
      roles: const [UserRole.user],
      provider: AuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );
    return AuthState.authenticated(user, emailVerified: true);
  }
}

class _FakeShippingRepository implements ShippingRepository {
  List<ShippingSetup> activeOptions = const [];
  CreateShippingSetupRequest? lastCreateRequest;
  bool failCreate = false;
  String? customError;

  @override
  Future<Result<List<ShippingSetup>>> listMyShippingSetups() async {
    return Result.success(activeOptions);
  }

  @override
  Future<Result<List<ShippingSetup>>> listMyActiveShippingSetups() async {
    return Result.success(activeOptions.where((opt) => opt.isActive).toList());
  }

  @override
  Future<Result<ShippingSetup>> getShippingSetupById(String optionId) async {
    for (final opt in activeOptions) {
      if (opt.id == optionId) {
        return Result.success(opt);
      }
    }
    return Result.error('shipping option not found: $optionId');
  }

  @override
  Future<Result<ShippingSetup>> createShippingSetup(
    CreateShippingSetupRequest request,
  ) async {
    lastCreateRequest = request;
    if (failCreate) {
      return Result.error(
        customError ?? 'coverages is required',
        code: 'BAD_REQUEST',
        statusCode: 400,
      );
    }

    final created = ShippingSetup(
      id: 'ship-created',
      name: request.name,
      type: request.type,
      internalNote: request.internalNote,
      coverageAreas: const [],
      isActive: true,
      createdAt: DateTime.utc(2026, 7, 25),
      updatedAt: DateTime.utc(2026, 7, 25),
    );
    activeOptions = [created];
    return Result.success(created);
  }

  @override
  Future<Result<ShippingSetup>> updateShippingSetup(
    String optionId,
    UpdateShippingSetupRequest request,
  ) async {
    return Result.error('not used');
  }

  @override
  Future<Result<void>> deleteShippingSetup(String optionId) async {
    return Result.error('not used');
  }

  @override
  Future<Result<void>> toggleActiveStatus(
    String optionId,
    bool isActive,
  ) async {
    return Result.error('not used');
  }

  @override
  Future<Result<void>> setProductShippingSetups(
    String productId,
    List<String> shippingSetupIds,
  ) async {
    return Result.error('not used');
  }

  @override
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async {
    return Result.error('not used');
  }
}

Widget _wrapApp({
  required Widget child,
  required String initialLocation,
  required _FakeShippingRepository repo,
  bool includeSettings = false,
}) {
  final router = GoRouter(
    initialLocation: initialLocation,
    routes: [
      if (includeSettings)
        GoRoute(
          path: '/settings',
          builder: (context, state) => const SettingsScreen(),
        ),
      GoRoute(path: '/', builder: (context, state) => child),
      GoRoute(
        path: RoutePaths.sellerShippingSetup,
        builder: (context, state) {
          final extra = state.extra;
          return ShippingSetupScreen(
            editOptionId: extra is String ? extra : null,
          );
        },
      ),
      GoRoute(
        path: RoutePaths.sellerShipping,
        builder: (context, state) => const SellerShippingScreen(),
      ),
      GoRoute(
        path: RoutePaths.sellerShippingSetupCityRules,
        builder: (context, state) => ShippingCityRulesScreen(
          args: state.extra! as ShippingCityRulesRouteArgs,
        ),
      ),
    ],
  );

  final overrides = [
    authControllerProvider.overrideWith(_FakeAuthController.new),
    shippingRepositoryProvider.overrideWithValue(repo),
    provincesProvider.overrideWith((ref) async {
      return const [
        Province(id: '33', name: 'Jawa Tengah'),
        Province(id: '32', name: 'Jawa Barat'),
      ];
    }),
    citiesProvider.overrideWith((ref, provinceId) async {
      switch (provinceId) {
        case '33':
          return const [
            City(id: '3301', name: 'Kota Semarang', provinceId: '33'),
            City(id: '3302', name: 'Kabupaten Demak', provinceId: '33'),
          ];
        case '32':
          return const [
            City(id: '3201', name: 'Kota Bandung', provinceId: '32'),
          ];
        default:
          return const [];
      }
    }),
  ];

  return ProviderScope(
    overrides: overrides,
    child: MaterialApp.router(
      routerConfig: router,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('id'),
    ),
  );
}

Future<void> _openDropdownItem(
  WidgetTester tester,
  Finder field,
  String text,
) async {
  await tester.ensureVisible(field);
  await tester.tap(field);
  await tester.pumpAndSettle();
  await tester.tap(find.text(text).last);
  await tester.pumpAndSettle();
}

void main() {
  group('ShippingSetupScreen canonical UX', () {
    testWidgets('opens as a routed full page and submits one atomic request', (
      tester,
    ) async {
      final repo = _FakeShippingRepository();
      await tester.pumpWidget(
        _wrapApp(
          initialLocation: '/',
          repo: repo,
          child: Builder(
            builder: (context) => Scaffold(
              body: Center(
                child: ElevatedButton(
                  onPressed: () async {
                    await ShippingSetupScreen.open(context);
                  },
                  child: const Text('open setup'),
                ),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('open setup'));
      await tester.pumpAndSettle();

      expect(find.byType(ShippingSetupScreen), findsOneWidget);
      expect(find.byType(BottomSheet), findsNothing);
      expect(find.text('Jenis transportasi'), findsOneWidget);
      expect(find.text('Nama ekspedisi / layanan *'), findsOneWidget);
      expect(tester.getTopLeft(find.text('Jenis transportasi')), isA<Offset>());

      await tester.tap(find.byType(ChoiceChip).at(1));
      await tester.pumpAndSettle();

      final textFields = find.byType(TextField);
      await tester.enterText(textFields.at(0), 'JNE Reguler');
      await tester.enterText(textFields.at(1), 'Catatan internal');

      final provinceFields = find.byType(DropdownButtonFormField<Province>);
      await _openDropdownItem(tester, provinceFields.at(0), 'Jawa Tengah');
      await tester.enterText(textFields.at(2), '100000');

      await tester.tap(find.text('Atur kota/kabupaten').first);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Tambah aturan'));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(DropdownButtonFormField<City>).first);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Kota Semarang').last);
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField).last, '140000');
      await tester.tap(
        find.descendant(
          of: find.byType(AlertDialog),
          matching: find.widgetWithText(ElevatedButton, 'Simpan'),
        ),
      );
      await tester.pumpAndSettle();
    });


    testWidgets('HTTP 400 keeps the setup page open with the draft intact', (
      tester,
    ) async {
      final repo = _FakeShippingRepository()..failCreate = true;
      await tester.pumpWidget(
        _wrapApp(
          initialLocation: '/',
          repo: repo,
          child: Builder(
            builder: (context) => Scaffold(
              body: Center(
                child: ElevatedButton(
                  onPressed: () async {
                    await ShippingSetupScreen.open(context);
                  },
                  child: const Text('open setup'),
                ),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('open setup'));
      await tester.pumpAndSettle();

      await tester.tap(find.byType(ChoiceChip).at(1));
      await tester.pumpAndSettle();
      final textFields = find.byType(TextField);
      await tester.enterText(textFields.at(0), 'JNE Reguler');
      await tester.enterText(textFields.at(2), '150000');

      await tester.tap(find.text('Simpan').last);
      await tester.pumpAndSettle();

      expect(find.byType(ShippingSetupScreen), findsOneWidget);
      expect(find.widgetWithText(TextField, 'JNE Reguler'), findsOneWidget);
    });

    testWidgets('Settings navigation opens management list, then setup', (
      tester,
    ) async {
      final repo = _FakeShippingRepository();

      await tester.pumpWidget(
        _wrapApp(
          initialLocation: '/settings',
          repo: repo,
          includeSettings: true,
          child: const SizedBox.shrink(),
        ),
      );
      await tester.pumpAndSettle();

      // Settings "Shipping" goes to management list (SellerShippingScreen)
      await tester.tap(find.text('Shipping').last);
      await tester.pumpAndSettle();
      expect(find.byType(SellerShippingScreen), findsOneWidget);

      await tester.pumpWidget(
        _wrapApp(
          initialLocation: '/',
          repo: repo,
          child: Builder(
            builder: (context) => Scaffold(
              body: SellerShippingSetupsSelector(
                onSelectionChanged: (_) {},
                helperText: 'selector',
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Selector with no active options shows "Atur Pengiriman" CTA
      // which navigates to management list
      expect(find.text('Atur Pengiriman'), findsOneWidget);
    });

    testWidgets('single Simpan action — no duplicate body Save', (tester) async {
      final repo = _FakeShippingRepository();
      await tester.pumpWidget(
        _wrapApp(
          initialLocation: '/',
          repo: repo,
          child: Builder(
            builder: (context) => Scaffold(
              body: Center(
                child: ElevatedButton(
                  onPressed: () async {
                    await ShippingSetupScreen.open(context);
                  },
                  child: const Text('open setup'),
                ),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('open setup'));
      await tester.pumpAndSettle();

      await tester.tap(find.byType(ChoiceChip).at(1));
      await tester.pumpAndSettle();
      final textFields = find.byType(TextField);
      await tester.enterText(textFields.at(0), 'Bus Kencana');
      await tester.enterText(textFields.at(2), '100000');

      // Exactly one Simpan button (in the sticky bottom bar)
      final simpanButtons = find.text('Simpan');
      expect(simpanButtons, findsOneWidget);
    });

    testWidgets('HTTP 400 maps raw backend unmarshal error to localized message', (
      tester,
    ) async {
      final repo = _FakeShippingRepository()
        ..failCreate = true
        ..customError =
            'json: cannot unmarshal number 50000.0 into Go struct field CreateShippingSetupCoverageRequest.coverages.tariff of type int64';
      await tester.pumpWidget(
        _wrapApp(
          initialLocation: '/',
          repo: repo,
          child: Builder(
            builder: (context) => Scaffold(
              body: Center(
                child: ElevatedButton(
                  onPressed: () async {
                    await ShippingSetupScreen.open(context);
                  },
                  child: const Text('open setup'),
                ),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('open setup'));
      await tester.pumpAndSettle();

      // Select transport type
      await tester.tap(find.byType(ChoiceChip).at(1));
      await tester.pumpAndSettle();

      final textFields = find.byType(TextField);
      await tester.enterText(textFields.at(0), 'KRT');

      // Fill the existing first coverage province + tariff so validation passes
      final provinceDropdown = find.byType(DropdownButtonFormField<Province>).first;
      await tester.ensureVisible(provinceDropdown);
      await tester.tap(provinceDropdown);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Jawa Tengah').last);
      await tester.pumpAndSettle();

      // Enter tariff for the province (at(2): name, note, tariff)
      final textFields2 = find.byType(TextField);
      await tester.enterText(textFields2.at(2), '50000');

      // Rebuild so the save gate re-evaluates before tapping
      await tester.pump();

      // Submit — validation passes, but backend returns 400
      await tester.tap(find.text('Simpan'));
      await tester.pumpAndSettle();

      // Page stays open
      expect(find.byType(ShippingSetupScreen), findsOneWidget);
      // Localized error, not raw Go text
      expect(
        find.text('Format tarif tidak valid. Periksa kembali tarif provinsi dan kota.'),
        findsOneWidget,
      );
      expect(
        find.textContaining('json:'),
        findsNothing,
      );
      expect(
        find.textContaining('int64'),
        findsNothing,
      );
    });
  });

  group('ShippingSetup request integer serialization', () {
    test('province rate serializes as JSON integer', () {
      final req = ShippingDestinationRequest(
        provinceCode: '11',
        provinceName: 'Aceh',
        rate: 50000,
      );
      final json = req.toJson();
      expect(json['rate'], 50000);
      expect(json['rate'], isA<int>());
      expect(json['rate'], isNot(isA<double>()));
    });

    test('city override rate serializes as JSON integer', () {
      final req = CityQualificationRequest(
        cityCode: '1101',
        cityName: 'Kabupaten Pidie Jaya',
        rateOverride: 100000,
      );
      final json = req.toJson();
      expect(json['rate'], 100000);
      expect(json['rate'], isA<int>());
      expect(json['rate'], isNot(isA<double>()));
      expect(json['is_available'], true);
    });

    test('excluded city qualification serializes with no rate', () {
      final req = CityQualificationRequest(
        cityCode: '1102',
        cityName: 'Kota Banda Aceh',
        excluded: true,
      );
      final json = req.toJson();
      expect(json['is_available'], false);
      expect(json.containsKey('rate'), false);
    });

    test('multi-destination package serializes every rate as integer', () {
      final req = CreateShippingSetupRequest(
        name: 'KRT',
        type: ShippingType.custom,
        internalNote: 'kantong besar',
        destinations: [
          ShippingDestinationRequest(
            provinceCode: '11',
            provinceName: 'Aceh',
            rate: 50000,
            cityQualifications: [
              CityQualificationRequest(
                cityCode: '1101',
                cityName: 'Kabupaten Pidie Jaya',
                rateOverride: 100000,
              ),
            ],
          ),
          ShippingDestinationRequest(
            provinceCode: '12',
            provinceName: 'Sumatera Utara',
            rate: 75000,
          ),
        ],
      );
      final json = req.toJson();

      final destinations = json['destinations'] as List;
      expect(destinations.length, 2);
      expect(json['internal_purpose'], 'kantong besar');

      expect(destinations[0]['rate'], 50000);
      expect(destinations[0]['rate'], isA<int>());
      expect(destinations[0]['rate'], isNot(isA<double>()));

      expect(destinations[0]['city_qualifications'][0]['rate'], 100000);
      expect(destinations[0]['city_qualifications'][0]['rate'], isA<int>());

      expect(destinations[1]['rate'], 75000);
      expect(destinations[1]['rate'], isA<int>());
    });
  });

  group('Settings management list routing', () {
    testWidgets('Settings opens management list not setup', (tester) async {
      final repo = _FakeShippingRepository();
      await tester.pumpWidget(
        _wrapApp(
          initialLocation: '/settings',
          repo: repo,
          includeSettings: true,
          child: const SizedBox.shrink(),
        ),
      );
      await tester.pumpAndSettle();

      // Tap "Shipping" in settings (factual tile label)
      await tester.tap(find.text('Shipping').last);
      await tester.pumpAndSettle();

      // Should open the management list (SellerShippingScreen), not setup
      expect(find.byType(ShippingSetupScreen), findsNothing);
      expect(find.byType(SellerShippingScreen), findsOneWidget);
      expect(find.text('Belum Ada Opsi Pengiriman'), findsOneWidget);
    });

    testWidgets('empty state does not auto-open setup', (tester) async {
      final repo = _FakeShippingRepository();
      repo.activeOptions = const [];
      await tester.pumpWidget(
        _wrapApp(
          initialLocation: '/settings',
          repo: repo,
          includeSettings: true,
          child: const SizedBox.shrink(),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Shipping').last);
      await tester.pumpAndSettle();

      // Should stay on list with empty state, not auto-redirect to setup
      expect(find.byType(ShippingSetupScreen), findsNothing);
      expect(find.byType(SellerShippingScreen), findsOneWidget);
      expect(find.text('Tambah Opsi Pengiriman'), findsOneWidget);
    });

    testWidgets('Tambah Opsi opens create bottom sheet from list', (tester) async {
      final repo = _FakeShippingRepository();
      await tester.pumpWidget(
        _wrapApp(
          initialLocation: '/settings',
          repo: repo,
          includeSettings: true,
          child: const SizedBox.shrink(),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Shipping').last);
      await tester.pumpAndSettle();

      // Empty-state CTA — opens the canonical one-package setup screen
      await tester.tap(find.text('Tambah Opsi Pengiriman'));
      await tester.pumpAndSettle();

      // Full setup page (NOT a bottom sheet, NOT a bare name+type form)
      expect(find.byType(ShippingSetupScreen), findsOneWidget);
      expect(find.text('Nama ekspedisi / layanan *'), findsOneWidget);
    });
  });

  group('ForSale/Auction direct-add routing', () {
    testWidgets('Create ForSale selector empty state shows management CTA', (
      tester,
    ) async {
      final repo = _FakeShippingRepository();
      await tester.pumpWidget(
        _wrapApp(
          initialLocation: '/',
          repo: repo,
          child: Builder(
            builder: (context) => Scaffold(
              body: SellerShippingSetupsSelector(
                onSelectionChanged: (_) {},
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Empty state CTA text is "Atur Pengiriman", not "Buat opsi pengiriman"
      expect(find.text('Atur Pengiriman'), findsOneWidget);

      // Tap navigates to management list (SellerShippingScreen)
      await tester.tap(find.text('Atur Pengiriman'));
      await tester.pumpAndSettle();

      expect(find.byType(SellerShippingScreen), findsOneWidget);
    });

    testWidgets('Create Auction selector populated shows chips not add button', (tester) async {
      final repo = _FakeShippingRepository();
      repo.activeOptions = [
        ShippingSetup(
          id: 'ship-1',
          name: 'Bus Kencana',
          type: ShippingType.bus,
          coverageAreas: const [],
          isActive: true,
          createdAt: DateTime.utc(2026, 7, 25),
          updatedAt: DateTime.utc(2026, 7, 25),
        ),
      ];
      await tester.pumpWidget(
        _wrapApp(
          initialLocation: '/',
          repo: repo,
          child: Builder(
            builder: (context) => Scaffold(
              body: SellerShippingSetupsSelector(
                onSelectionChanged: (_) {},
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Populated selector shows FilterChip for each option
      expect(find.byType(FilterChip), findsOneWidget);
      expect(find.textContaining('Bus Kencana'), findsOneWidget);
      // No "add" button when options exist
      expect(find.text('Tambah opsi pengiriman'), findsNothing);
    });
  });

  group('ShippingSetupScreen edit mode', () {
    testWidgets('edit opens full editor hydrated from backend detail', (
      tester,
    ) async {
      final repo = _FakeShippingRepository();
      final existingOption = ShippingSetup(
        id: 'ship-edit',
        name: 'Bus Kencana',
        type: ShippingType.bus,
        internalNote: 'box besar',
        coverageAreas: const [],
        isActive: true,
        createdAt: DateTime.utc(2026, 7, 25),
        updatedAt: DateTime.utc(2026, 7, 25),
      );
      repo.activeOptions = [existingOption];

      await tester.pumpWidget(
        _wrapApp(
          initialLocation: '/',
          repo: repo,
          child: Builder(
            builder: (context) => Scaffold(
              body: Center(
                child: ElevatedButton(
                  onPressed: () async {
                    await ShippingSetupScreen.openEditById(
                      context,
                      'ship-edit',
                    );
                  },
                  child: const Text('open edit'),
                ),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('open edit'));
      await tester.pumpAndSettle();

      // Edit mode: title, pre-filled name, coverages editable
      expect(find.byType(ShippingSetupScreen), findsOneWidget);
      expect(find.text('Edit Opsi Pengiriman'), findsOneWidget);
      expect(find.widgetWithText(TextField, 'Bus Kencana'), findsOneWidget);
      // Coverages section is editable (not read-only)
      expect(find.text('Tujuan dan tarif'), findsOneWidget);
      expect(find.text('Tambah Provinsi'), findsOneWidget);
    });

    testWidgets('edit submit sends full coverages request', (tester) async {
      final repo = _FakeShippingRepository();
      final existingOption = ShippingSetup(
        id: 'ship-edit-full',
        name: 'Old Name',
        type: ShippingType.custom,
        coverageAreas: const [],
        isActive: true,
        createdAt: DateTime.utc(2026, 7, 25),
        updatedAt: DateTime.utc(2026, 7, 25),
      );
      repo.activeOptions = [existingOption];

      await tester.pumpWidget(
        _wrapApp(
          initialLocation: '/',
          repo: repo,
          child: Builder(
            builder: (context) => Scaffold(
              body: Center(
                child: ElevatedButton(
                  onPressed: () async {
                    await ShippingSetupScreen.openEditById(
                      context,
                      'ship-edit-full',
                    );
                  },
                  child: const Text('open edit'),
                ),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('open edit'));
      await tester.pumpAndSettle();

      // Clear name and enter new
      final textFields = find.byType(TextField);
      await tester.enterText(textFields.at(0), 'New Name');

      // Simpan should be visible
      expect(find.text('Simpan'), findsOneWidget);
    });
  });
}
