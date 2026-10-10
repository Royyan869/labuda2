import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'package:hishumi/domains/user/profile/domain/entities/address_entity.dart';
import 'package:hishumi/domains/user/profile/domain/repositories/i_address_repository.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/notifiers/address_notifier.dart';
import 'package:hishumi/domains/user/profile/presentation/screens/address_list_screen.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/models/wilayah_models.dart';
import 'package:hishumi/shared/providers/authenticated_account_provider.dart';
import 'package:hishumi/shared/widgets/empty_state.dart';
import 'package:hishumi/shared/widgets/loading_indicator.dart';
import 'package:hishumi/shared/widgets/page_error_state.dart';

const _uid = 'buyer-1';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._user);

  final AuthUser _user;

  @override
  AuthState build() => AuthState.authenticated(_user, emailVerified: true);
}

/// Scripted repository: the screen runs the REAL AddressNotifier (canonical
/// authority), so every read/mutation is observable at the producer boundary.
class _ScriptedAddressRepository implements IAddressRepository {
  _ScriptedAddressRepository({required this.onFetch});

  Future<Result<List<AddressEntity>>> Function(int call) onFetch;
  Future<Result<void>> Function(String addressId)? onDelete;
  Future<Result<void>> Function(String addressId, String userId)? onSetPrimary;
  Future<Result<void>> Function(AddressEntity address)? onUpdate;

  int listCalls = 0;
  final List<String> deletedIds = [];
  final List<String> primarySetIds = [];
  final List<String> updatedIds = [];

  @override
  Future<Result<List<AddressEntity>>> getAddressesByUserId(String userId) {
    listCalls++;
    return onFetch(listCalls);
  }

  @override
  Future<Result<AddressEntity?>> getPrimaryAddress(String userId) async =>
      Result.success(null);

  @override
  Future<Result<AddressEntity>> getAddressById(String addressId) async =>
      Result.error('not used');

  @override
  Future<Result<void>> addAddress(AddressEntity address) async =>
      Result.success(null);

  @override
  Future<Result<void>> updateAddress(AddressEntity address) {
    updatedIds.add(address.id);
    final handler = onUpdate;
    if (handler != null) return handler(address);
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<void>> deleteAddress(String addressId) {
    deletedIds.add(addressId);
    final handler = onDelete;
    if (handler != null) return handler(addressId);
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<void>> setPrimaryAddress(String addressId, String userId) {
    primarySetIds.add(addressId);
    final handler = onSetPrimary;
    if (handler != null) return handler(addressId, userId);
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<int>> countAddresses(String userId) async => Result.success(0);
}

AuthUser _user() => AuthUser(
  id: _uid,
  createdAt: DateTime.utc(2026, 8, 1),
  updatedAt: DateTime.utc(2026, 8, 1),
  email: 'buyer@example.com',
  username: 'buyer',
  isEmailVerified: true,
  accountStatus: AccountStatus.active,
  roles: const [],
  provider: AuthProvider.email,
);

AddressEntity _address({
  String id = 'address-1',
  String nickname = 'Rumah',
  bool isPrimary = true,
}) => AddressEntity(
  id: id,
  userId: _uid,
  nickname: nickname,
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
  streetAddress: 'Jl. Test No. $id',
  postalCode: '12160',
  isPrimary: isPrimary,
  createdAt: DateTime.utc(2026, 8, 1),
  updatedAt: DateTime.utc(2026, 8, 1),
);

Widget _wrap(IAddressRepository repository) {
  final user = _user();
  return ProviderScope(
    // Retry is disabled so a failed load does not schedule timers.
    retry: (retryCount, error) => null,
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuthController(user)),
      authenticatedUserProvider.overrideWith((ref) => user),
      addressRepositoryProvider.overrideWithValue(repository),
    ],
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('id'),
      home: const AddressListScreen(),
    ),
  );
}

Future<void> _pump(
  WidgetTester tester,
  IAddressRepository repository, {
  bool settle = true,
}) async {
  await tester.pumpWidget(_wrap(repository));
  if (settle) await tester.pumpAndSettle();
}

ProviderContainer _container(_ScriptedAddressRepository repository) {
  final container = ProviderContainer(
    overrides: [addressRepositoryProvider.overrideWithValue(repository)],
  );
  addTearDown(container.dispose);
  // Keep the autoDispose notifier alive for the test.
  container.listen(addressProvider, (_, _) {});
  return container;
}

void main() {
  group('AddressListScreen — canonical authority and initial state', () {
    testWidgets(
      'first request is triggered exactly once through the notifier',
      (tester) async {
        final repository = _ScriptedAddressRepository(
          onFetch: (_) => Future.value(Result.success([_address()])),
        );
        await _pump(tester, repository);

        expect(repository.listCalls, 1);
        expect(find.text('Rumah'), findsOneWidget);
      },
    );

    testWidgets('initial request shows LoadingIndicator, never empty/error', (
      tester,
    ) async {
      final gate = Completer<Result<List<AddressEntity>>>();
      final repository = _ScriptedAddressRepository(
        onFetch: (_) => gate.future,
      );
      await _pump(tester, repository, settle: false);
      await tester.pump();
      await tester.pump();

      expect(find.byType(LoadingIndicator), findsOneWidget);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete(Result.success([_address()]));
      await tester.pumpAndSettle();
      expect(find.text('Rumah'), findsOneWidget);
    });

    testWidgets(
      'successful zero-result shows EmptyState with first-use action',
      (tester) async {
        final repository = _ScriptedAddressRepository(
          onFetch: (_) => Future.value(Result.success(const <AddressEntity>[])),
        );
        await _pump(tester, repository);

        expect(find.byType(EmptyState), findsOneWidget);
        expect(find.text('Belum ada alamat'), findsOneWidget);
        expect(find.byType(PageErrorState), findsNothing);
        expect(find.byType(LoadingIndicator), findsNothing);
        // The first-use action remains (one primary action).
        expect(find.text('Tambah Alamat'), findsWidgets);
      },
    );
  });

  group('AddressListScreen — initial failure and retry', () {
    testWidgets('failure with no data shows PageErrorState, retry reloads', (
      tester,
    ) async {
      final repository = _ScriptedAddressRepository(
        onFetch: (call) => call == 1
            ? Future.value(Result.error('INTERNAL_SERVER_ERROR: sql: no rows'))
            : Future.value(Result.success([_address()])),
      );
      await _pump(tester, repository);

      expect(find.byType(PageErrorState), findsOneWidget);
      expect(find.text('Terjadi Kesalahan'), findsOneWidget);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsOneWidget,
      );
      expect(find.text('Coba Lagi'), findsOneWidget);
      expect(find.textContaining('INTERNAL_SERVER_ERROR'), findsNothing);
      expect(find.textContaining('sql:'), findsNothing);
      expect(find.byType(EmptyState), findsNothing);

      expect(repository.listCalls, 1);
      await tester.tap(find.widgetWithText(ElevatedButton, 'Coba Lagi'));
      await tester.pumpAndSettle();

      expect(repository.listCalls, 2);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.text('Rumah'), findsOneWidget);
    });
  });

  group('AddressListScreen — refresh preserves data', () {
    testWidgets('existing addresses stay visible with refresh indicator', (
      tester,
    ) async {
      final gate = Completer<Result<List<AddressEntity>>>();
      final repository = _ScriptedAddressRepository(
        onFetch: (call) => call == 1
            ? Future.value(Result.success([_address()]))
            : gate.future,
      );
      await _pump(tester, repository);
      expect(find.text('Rumah'), findsOneWidget);

      await tester.fling(find.text('Rumah'), const Offset(0, 400), 1000);
      for (var i = 0; i < 50 && repository.listCalls < 2; i++) {
        await tester.pump(const Duration(milliseconds: 50));
      }
      expect(repository.listCalls, 2);

      // Refresh must not clear the list into full loading.
      expect(find.text('Rumah'), findsOneWidget);
      expect(find.byType(LinearProgressIndicator), findsOneWidget);
      expect(find.byType(LoadingIndicator), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete(
        Result.success([
          _address(id: 'address-2', nickname: 'Kantor', isPrimary: true),
        ]),
      );
      await tester.pumpAndSettle();

      expect(find.text('Kantor'), findsOneWidget);
      expect(find.text('Rumah'), findsNothing);
      expect(find.byType(LinearProgressIndicator), findsNothing);
    });

    testWidgets(
      'refresh failure keeps rows with inline banner, retry recovers',
      (tester) async {
        final repository = _ScriptedAddressRepository(
          onFetch: (call) {
            if (call == 1) return Future.value(Result.success([_address()]));
            if (call == 2) {
              return Future.value(Result.error('HTTP 500: boom-refresh'));
            }
            return Future.value(
              Result.success([
                _address(id: 'address-2', nickname: 'Kantor', isPrimary: true),
              ]),
            );
          },
        );
        await _pump(tester, repository);
        expect(find.text('Rumah'), findsOneWidget);

        await tester.fling(find.text('Rumah'), const Offset(0, 400), 1000);
        await tester.pumpAndSettle();

        // Valid data is preserved; failure renders inline, never full-page.
        expect(find.text('Rumah'), findsOneWidget);
        expect(find.byType(PageErrorState), findsNothing);
        expect(
          find.text('Data belum bisa dimuat. Silakan coba lagi.'),
          findsOneWidget,
        );
        expect(find.widgetWithText(TextButton, 'Coba Lagi'), findsOneWidget);
        expect(find.textContaining('boom-refresh'), findsNothing);

        await tester.tap(find.widgetWithText(TextButton, 'Coba Lagi'));
        await tester.pumpAndSettle();

        expect(repository.listCalls, 3);
        expect(find.text('Kantor'), findsOneWidget);
        expect(find.text('Rumah'), findsNothing);
        expect(
          find.text('Data belum bisa dimuat. Silakan coba lagi.'),
          findsNothing,
        );
      },
    );
  });

  group('AddressNotifier — canonical mutation authority (unit)', () {
    test('deleteAddress mutates and reloads the shared collection', () async {
      final repository = _ScriptedAddressRepository(
        onFetch: (_) => Future.value(Result.success([_address()])),
      );
      final container = _container(repository);
      final notifier = container.read(addressProvider.notifier);
      await notifier.loadAddresses(_uid);
      expect(repository.listCalls, 1);

      final ok = await notifier.deleteAddress('address-2', _uid);

      expect(ok, isTrue);
      expect(repository.deletedIds, ['address-2']);
      // The canonical authority reloaded itself — no second reload path.
      expect(repository.listCalls, 2);
    });

    test(
      'setPrimaryAddress mutates and reloads the shared collection',
      () async {
        final repository = _ScriptedAddressRepository(
          onFetch: (_) => Future.value(Result.success([_address()])),
        );
        final container = _container(repository);
        final notifier = container.read(addressProvider.notifier);
        await notifier.loadAddresses(_uid);

        final ok = await notifier.setPrimaryAddress('address-2', _uid);

        expect(ok, isTrue);
        expect(repository.primarySetIds, ['address-2']);
        expect(repository.listCalls, 2);
      },
    );

    test('addAddress mutates and reloads the shared collection', () async {
      final repository = _ScriptedAddressRepository(
        onFetch: (_) => Future.value(Result.success([_address()])),
      );
      final container = _container(repository);
      final notifier = container.read(addressProvider.notifier);
      await notifier.loadAddresses(_uid);

      final ok = await notifier.addAddress(_address(id: 'address-3'));

      expect(ok, isTrue);
      expect(repository.listCalls, 2);
    });

    test('updateAddress mutates and reloads the shared collection', () async {
      final repository = _ScriptedAddressRepository(
        onFetch: (_) => Future.value(Result.success([_address()])),
      );
      final container = _container(repository);
      final notifier = container.read(addressProvider.notifier);
      await notifier.loadAddresses(_uid);

      final ok = await notifier.updateAddress(_address(id: 'address-3'));

      expect(ok, isTrue);
      expect(repository.updatedIds, ['address-3']);
      expect(repository.listCalls, 2);
    });

    test('failed mutation keeps the collection and does not reload', () async {
      final repository = _ScriptedAddressRepository(
        onFetch: (_) => Future.value(Result.success([_address()])),
      )..onDelete = (_) => Future.value(Result.error('INTERNAL_SERVER_ERROR'));
      final container = _container(repository);
      final notifier = container.read(addressProvider.notifier);
      await notifier.loadAddresses(_uid);

      final ok = await notifier.deleteAddress('address-2', _uid);

      expect(ok, isFalse);
      expect(repository.listCalls, 1);
      // The failure is captured in the canonical state, never raw in the UI.
      expect(
        container.read(addressProvider).errorMessage,
        'INTERNAL_SERVER_ERROR',
      );
    });

    test(
      'refresh preserves the collection and sets refreshError on failure',
      () async {
        final repository = _ScriptedAddressRepository(
          onFetch: (call) => call == 1
              ? Future.value(Result.success([_address()]))
              : Future.value(Result.error('HTTP 500: boom')),
        );
        final container = _container(repository);
        final notifier = container.read(addressProvider.notifier);
        await notifier.loadAddresses(_uid);

        await notifier.loadAddresses(_uid);

        final state = container.read(addressProvider);
        // Data preserved; failure is an inline refresh error, not an empty
        // collection and not a full-page AsyncError.
        expect(state.addresses.value, hasLength(1));
        expect(state.addresses.hasError, isFalse);
        expect(state.refreshError, 'HTTP 500: boom');
        expect(state.isRefreshing, isFalse);
      },
    );
  });

  group('AddressListScreen — negative proof (static contract)', () {
    String screenSource() => File(
      'lib/domains/user/profile/presentation/screens/address_list_screen.dart',
    ).readAsStringSync().replaceAll('\r\n', '\n');

    String providersSource() => File(
      'lib/domains/user/profile/presentation/providers/address_providers.dart',
    ).readAsStringSync().replaceAll('\r\n', '\n');

    test('canonical renderers own every page state', () {
      final src = screenSource();
      expect(src.contains('LoadingIndicator('), isTrue);
      expect(src.contains('PageErrorState('), isTrue);
      expect(src.contains('EmptyState('), isTrue);
      expect(src.contains('LinearProgressIndicator'), isTrue);
    });

    test('no raw spinner or custom error renderer remains', () {
      final src = screenSource();
      expect(src.contains('CircularProgressIndicator('), isFalse);
      expect(src.contains('Gagal memuat alamat. Coba lagi.'), isFalse);
    });

    test('no raw technical error reaches the widget tree', () {
      final src = screenSource();
      expect(src.contains('result.error'), isFalse);
      expect(src.contains('error.toString()'), isFalse);
      expect(src.contains('Text(error'), isFalse);
    });

    test('the duplicate list authority is purged end-to-end', () {
      expect(providersSource().contains('addressesFutureProvider'), isFalse);
      expect(screenSource().contains('addressesFutureProvider'), isFalse);
      // No page-local direct-repository mutation path remains.
      expect(screenSource().contains('addressRepositoryProvider'), isFalse);
    });

    test('the canonical authority is singular and consumed by the screen', () {
      final src = screenSource();
      expect(src.contains('ref.watch(addressProvider)'), isTrue);
      expect(src.contains('addressProvider.notifier'), isTrue);
    });
  });
}
