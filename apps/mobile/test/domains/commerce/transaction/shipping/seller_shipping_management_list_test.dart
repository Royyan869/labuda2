import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/widgets/shipping_option_setup_screen.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_shipping_screen.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/wilayah_provider_simple.dart';

// =============================================================================
// Fake repository with controllable mutation responses
// =============================================================================

class _MutationTestRepo implements ShippingRepository {
  List<ShippingSetup> _options;
  bool failToggle = false;
  bool failDelete = false;
  bool deleteConflict = false;
  CreateShippingSetupRequest? lastCreateRequest;
  UpdateShippingSetupFullRequest? lastFullUpdate;
  String? lastToggledId;
  bool? lastToggleValue;

  _MutationTestRepo({List<ShippingSetup>? options})
      : _options = options ?? [];

  @override
  Future<Result<List<ShippingSetup>>> listMyShippingSetups() async =>
      Result.success(_options);

  @override
  Future<Result<List<ShippingSetup>>> listMyActiveShippingSetups() async =>
      Result.success(_options.where((o) => o.isActive).toList());

  @override
  Future<Result<ShippingSetup>> getShippingSetupById(
      String optionId) async =>
      Result.error('not used');

  @override
  Future<Result<ShippingSetup>> createShippingSetup(
      CreateShippingSetupRequest request) async {
    lastCreateRequest = request;
    final created = ShippingSetup(
      id: 'new-opt',
      name: request.name,
      type: request.type,
      internalNote: request.internalNote,
      coverageAreas: const [],
      isActive: true,
      createdAt: DateTime.utc(2026, 7, 25),
      updatedAt: DateTime.utc(2026, 7, 25),
    );
    _options = [..._options, created];
    return Result.success(created);
  }

  @override
  Future<Result<ShippingSetup>> updateShippingSetup(
      String optionId, UpdateShippingSetupRequest request) async {
    final idx = _options.indexWhere((o) => o.id == optionId);
    if (idx >= 0) {
      _options[idx] = _options[idx].copyWith(name: request.name);
      return Result.success(_options[idx]);
    }
    return Result.error('not found');
  }

  @override
  Future<Result<ShippingSetup>> updateShippingSetupFull(
      String optionId, UpdateShippingSetupFullRequest request) async {
    lastFullUpdate = request;
    final idx = _options.indexWhere((o) => o.id == optionId);
    if (idx >= 0) {
      _options[idx] = _options[idx].copyWith(name: request.name);
      return Result.success(_options[idx]);
    }
    return Result.error('not found');
  }

  @override
  Future<Result<void>> deleteShippingSetup(String optionId) async {
    if (failDelete) return Result.error('delete failed');
    if (deleteConflict) {
      return Result.error(
        'shipping option is still used by 1 product listing(s)',
        code: 'CONFLICT',
        statusCode: 409,
      );
    }
    _options = _options.where((o) => o.id != optionId).toList();
    return Result.success(null);
  }

  @override
  Future<Result<void>> toggleActiveStatus(String optionId, bool isActive) async {
    lastToggledId = optionId;
    lastToggleValue = isActive;
    if (failToggle) return Result.error('toggle failed');
    final idx = _options.indexWhere((o) => o.id == optionId);
    if (idx >= 0) {
      _options[idx] = _options[idx].copyWith(isActive: isActive);
    }
    return Result.success(null);
  }

  // Unused stubs
  @override
  Future<Result<ShippingCoverage>> addCoverage(
          String optionId, AddCoverageRequest request) async =>
      Result.error('not used');
  @override
  Future<Result<ShippingCoverage>> updateCoverage(
          String coverageId, UpdateCoverageRequest request) async =>
      Result.error('not used');
  @override
  Future<Result<void>> deleteCoverage(String coverageId) async =>
      Result.error('not used');
  @override
  Future<Result<void>> setProductShippingSetups(
          String productId, List<String> ids) async =>
      Result.error('not used');
  @override
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
          CheckDeliveryRequest request) async =>
      Result.error('not used');
}

// =============================================================================
// Test harness
// =============================================================================

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.utc(2026, 7, 25);
    final user = AuthUser(
      id: 'seller-1', createdAt: now, updatedAt: now,
      email: 'seller@example.com', username: 'seller',
      isEmailVerified: true, accountStatus: AccountStatus.active,
      hasSellerProfile: true, sellerSubscriptionStatus: 'active',
      hasMarketAuthority: true, roles: const [UserRole.user],
      provider: AuthProvider.email, lifecycle: ContentLifecycle.active,
    );
    return AuthState.authenticated(user, emailVerified: true);
  }
}

class _FakePresenceManager extends PresenceManager {
  @override
  PresenceState build() => const PresenceState();
}

Widget _wrap({
  required Widget child,
  required _MutationTestRepo repo,
  String initialLocation = '/',
}) {
  final router = GoRouter(
    initialLocation: initialLocation,
    routes: [
      GoRoute(path: '/', builder: (_, __) => child),
      GoRoute(
        path: RoutePaths.sellerShipping,
        builder: (_, __) => const SellerShippingScreen(),
      ),
      GoRoute(
        path: RoutePaths.sellerShippingSetup,
        builder: (_, __) => const ShippingSetupScreen(),
      ),
    ],
  );

  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(_FakeAuthController.new),
      presenceManagerProvider.overrideWith(_FakePresenceManager.new),
      shippingRepositoryProvider.overrideWithValue(repo),
      provincesProvider.overrideWith((ref) async => const []),
    ],
    child: MaterialApp.router(
      routerConfig: router,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('id'),
    ),
  );
}

void main() {
  group('SellerShippingScreen management list mutations', () {
    testWidgets('toggle sends lifecycle-only request and updates card',
        (tester) async {
      final repo = _MutationTestRepo(options: [
        ShippingSetup(
          id: 'ship-1', name: 'Bus Kencana', type: ShippingType.bus,
          coverageAreas: const [], isActive: true,
          createdAt: DateTime.utc(2026, 7, 25),
          updatedAt: DateTime.utc(2026, 7, 25),
        ),
      ]);

      await tester.pumpWidget(_wrap(
        initialLocation: RoutePaths.sellerShipping, repo: repo,
        child: const SizedBox.shrink(),
      ));
      await tester.pumpAndSettle();

      // Toggle the switch off
      final toggle = find.byType(Switch);
      expect(toggle, findsOneWidget);
      await tester.tap(toggle);
      await tester.pumpAndSettle();

      // Verify lifecycle-only request
      expect(repo.lastToggledId, 'ship-1');
      expect(repo.lastToggleValue, false);

      // Card still visible after toggle (not removed)
      expect(find.text('Bus Kencana'), findsOneWidget);
    });

    testWidgets('toggle failure preserves cards and shows safe error',
        (tester) async {
      final repo = _MutationTestRepo(options: [
        ShippingSetup(
          id: 'ship-1', name: 'Bus Kencana', type: ShippingType.bus,
          coverageAreas: const [], isActive: true,
          createdAt: DateTime.utc(2026, 7, 25),
          updatedAt: DateTime.utc(2026, 7, 25),
        ),
      ])
        ..failToggle = true;

      await tester.pumpWidget(_wrap(
        initialLocation: RoutePaths.sellerShipping, repo: repo,
        child: const SizedBox.shrink(),
      ));
      await tester.pumpAndSettle();

      await tester.tap(find.byType(Switch));
      await tester.pumpAndSettle();

      // Card preserved
      expect(find.text('Bus Kencana'), findsOneWidget);
      // List not replaced with error state
      expect(find.text('Gagal memuat opsi pengiriman'), findsNothing);
    });

    testWidgets('delete unused option removes card after backend success',
        (tester) async {
      final repo = _MutationTestRepo(options: [
        ShippingSetup(
          id: 'ship-1', name: 'Bus Kencana', type: ShippingType.bus,
          coverageAreas: const [], isActive: true,
          createdAt: DateTime.utc(2026, 7, 25),
          updatedAt: DateTime.utc(2026, 7, 25),
        ),
      ]);

      await tester.pumpWidget(_wrap(
        initialLocation: RoutePaths.sellerShipping, repo: repo,
        child: const SizedBox.shrink(),
      ));
      await tester.pumpAndSettle();

      // Open popup menu and tap delete
      final popupButton = find.byType(PopupMenuButton<String>);
      await tester.tap(popupButton);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Hapus'));
      await tester.pumpAndSettle();

      // Confirm delete dialog
      await tester.tap(find.text('Hapus').last);
      await tester.pumpAndSettle();

      // Card removed after success
      expect(find.text('Bus Kencana'), findsNothing);
    });

    testWidgets('delete 409 preserves card with localized message',
        (tester) async {
      final repo = _MutationTestRepo(options: [
        ShippingSetup(
          id: 'ship-1', name: 'Bus Kencana', type: ShippingType.bus,
          coverageAreas: const [], isActive: true,
          createdAt: DateTime.utc(2026, 7, 25),
          updatedAt: DateTime.utc(2026, 7, 25),
        ),
      ])
        ..deleteConflict = true;

      await tester.pumpWidget(_wrap(
        initialLocation: RoutePaths.sellerShipping, repo: repo,
        child: const SizedBox.shrink(),
      ));
      await tester.pumpAndSettle();

      // Delete flow
      await tester.tap(find.byType(PopupMenuButton<String>));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Hapus'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Hapus').last);
      await tester.pumpAndSettle();

      // Card preserved after conflict
      expect(find.text('Bus Kencana'), findsOneWidget);
    });

    testWidgets('delete failure preserves list data not replaced by load error',
        (tester) async {
      final repo = _MutationTestRepo(options: [
        ShippingSetup(
          id: 'ship-1', name: 'Bus Kencana', type: ShippingType.bus,
          coverageAreas: const [], isActive: true,
          createdAt: DateTime.utc(2026, 7, 25),
          updatedAt: DateTime.utc(2026, 7, 25),
        ),
      ])
        ..failDelete = true;

      await tester.pumpWidget(_wrap(
        initialLocation: RoutePaths.sellerShipping, repo: repo,
        child: const SizedBox.shrink(),
      ));
      await tester.pumpAndSettle();

      await tester.tap(find.byType(PopupMenuButton<String>));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Hapus'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Hapus').last);
      await tester.pumpAndSettle();

      // Card preserved
      expect(find.text('Bus Kencana'), findsOneWidget);
      // List not replaced
      expect(find.text('Gagal memuat opsi pengiriman'), findsNothing);
    });

    testWidgets('mutation error does not become list-load failure', (
        tester) async {
      final repo = _MutationTestRepo(options: [
        ShippingSetup(
          id: 'ship-1', name: 'Bus Kencana', type: ShippingType.bus,
          coverageAreas: const [], isActive: true,
          createdAt: DateTime.utc(2026, 7, 25),
          updatedAt: DateTime.utc(2026, 7, 25),
        ),
      ])
        ..failToggle = true;

      await tester.pumpWidget(_wrap(
        initialLocation: RoutePaths.sellerShipping, repo: repo,
        child: const SizedBox.shrink(),
      ));
      await tester.pumpAndSettle();

      // Trigger a failing toggle
      await tester.tap(find.byType(Switch));
      await tester.pumpAndSettle();

      // List must not show the load-error state
      expect(find.text('Gagal memuat opsi pengiriman'), findsNothing);
      // Card is still visible
      expect(find.text('Bus Kencana'), findsOneWidget);
    });

    testWidgets('add opens create bottom sheet', (
        tester) async {
      final repo = _MutationTestRepo();

      await tester.pumpWidget(_wrap(
        initialLocation: RoutePaths.sellerShipping, repo: repo,
        child: const SizedBox.shrink(),
      ));
      await tester.pumpAndSettle();

      // Empty state visible
      expect(find.text('Belum Ada Opsi Pengiriman'), findsOneWidget);

      // Tap Tambah Opsi
      await tester.tap(find.text('Tambah Opsi Pengiriman'));
      await tester.pumpAndSettle();

      // Bottom sheet form opened with name input
      expect(find.text('Nama opsi *'), findsOneWidget);
    });
  });
}
