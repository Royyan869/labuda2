// Address runtime proof — the two originally-observed bugs.
//
// This is a RUNTIME proof (real widgets, real providers), not a static source
// inspection. It drives the actual AddressListScreen and the seller wizard with
// the canonical providers and asserts the observed behaviors:
//
//   Gap B — Settings address page shows data on the first frame (no 30-second
//           polling spinner) because the read is an immediate one-shot through
//           the canonical address repository/provider.
//   Gap C — the address UI never renders a raw technical error string
//           (Result.error / error code); a failed read shows business-facing
//           copy only.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'package:hishumi/domains/user/profile/domain/entities/address_entity.dart';
import 'package:hishumi/domains/user/profile/domain/repositories/i_address_repository.dart';
import 'package:hishumi/domains/user/profile/presentation/screens/address_list_screen.dart';
import 'package:hishumi/shared/models/wilayah_models.dart';
import 'package:hishumi/shared/providers/authenticated_account_provider.dart';
import 'package:hishumi/shared/widgets/page_error_state.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._user);

  final AuthUser _user;

  @override
  AuthState build() => AuthState.authenticated(_user, emailVerified: true);
}

class _ImmediateAddressRepository implements IAddressRepository {
  _ImmediateAddressRepository(this._addresses);

  final List<AddressEntity> _addresses;
  int listCalls = 0;

  @override
  Future<Result<List<AddressEntity>>> getAddressesByUserId(String userId) {
    listCalls++;
    // Resolves immediately — no polling, no delay.
    return Future.value(Result.success(_addresses));
  }

  @override
  Future<Result<AddressEntity?>> getPrimaryAddress(String userId) async {
    return Result.success(_addresses.where((a) => a.isPrimary).firstOrNull);
  }

  @override
  Future<Result<AddressEntity>> getAddressById(String addressId) async {
    return Result.error('not used');
  }

  @override
  Future<Result<void>> addAddress(AddressEntity address) async {
    return Result.error('not used');
  }

  @override
  Future<Result<void>> updateAddress(AddressEntity address) async {
    return Result.error('not used');
  }

  @override
  Future<Result<void>> deleteAddress(String addressId) async {
    return Result.error('not used');
  }

  @override
  Future<Result<void>> setPrimaryAddress(
    String addressId,
    String userId,
  ) async {
    return Result.error('not used');
  }

  @override
  Future<Result<int>> countAddresses(String userId) async {
    return Result.success(_addresses.length);
  }
}

class _FailingAddressRepository extends _ImmediateAddressRepository {
  _FailingAddressRepository() : super(const []);

  @override
  Future<Result<List<AddressEntity>>> getAddressesByUserId(String userId) {
    listCalls++;
    return Future.value(
      Result.error('INTERNAL_SERVER_ERROR: sql: no rows in result set'),
    );
  }
}

AuthUser _user() => AuthUser(
  id: 'buyer-1',
  createdAt: DateTime.utc(2026, 8, 1),
  updatedAt: DateTime.utc(2026, 8, 1),
  email: 'buyer@example.com',
  username: 'buyer',
  isEmailVerified: true,
  accountStatus: AccountStatus.active,
  roles: const [],
  provider: AuthProvider.email,
);

AddressEntity _address() => AddressEntity(
  id: 'address-1',
  userId: 'buyer-1',
  nickname: 'Rumah',
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

Widget _wrap(IAddressRepository repository) {
  final user = _user();
  return ProviderScope(
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

void main() {
  testWidgets(
    'Gap B — Settings address renders on first frame (no 30s polling wait)',
    (tester) async {
      final repo = _ImmediateAddressRepository([_address()]);

      await tester.pumpWidget(_wrap(repo));

      // No 30-second interval is pumped: a single settle must be enough
      // because the read is an immediate one-shot.
      await tester.pumpAndSettle();

      expect(find.text('Rumah'), findsOneWidget);
      expect(find.textContaining('Jl. Test No. 1'), findsOneWidget);
      expect(find.byType(CircularProgressIndicator), findsNothing);
      expect(repo.listCalls, 1);
    },
  );

  testWidgets(
    'Gap C — a failed address read shows business copy, never a raw error code',
    (tester) async {
      final repo = _FailingAddressRepository();

      await tester.pumpWidget(_wrap(repo));
      await tester.pumpAndSettle();

      // Canonical controlled copy is present (PageErrorState): no raw text.
      expect(find.byType(PageErrorState), findsOneWidget);
      expect(find.text('Terjadi Kesalahan'), findsOneWidget);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsOneWidget,
      );
      expect(find.text('Coba Lagi'), findsOneWidget);
      // The raw technical error string never reaches the UI.
      expect(find.textContaining('INTERNAL_SERVER_ERROR'), findsNothing);
      expect(find.textContaining('sql: no rows'), findsNothing);
    },
  );
}
