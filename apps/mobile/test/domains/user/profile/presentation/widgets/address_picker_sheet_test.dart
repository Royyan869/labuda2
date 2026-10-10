// Canonical AddressPickerSheet — selection-surface behavior proof.
//
// PROOFS:
// - lists the account's addresses from the canonical address state;
// - marks the caller's current selection;
// - returns the chosen AddressEntity (null on dismissal);
// - NEVER calls setPrimaryAddress (selection ≠ primary mutation);
// - empty book renders the canonical empty state whose CTA opens the one
//   AddressFormDialog;
// - no Checkout knowledge: the picker is a generic selection surface.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/profile/domain/entities/address_entity.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/notifiers/address_notifier.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/state/address_state.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/address_form_dialog.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/address_picker_sheet.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/shared.dart';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => AuthState.authenticated(_user(), emailVerified: true);
}

class _FakeAddressNotifier extends AddressNotifier {
  _FakeAddressNotifier(this._addresses);

  final List<AddressEntity> _addresses;

  int setPrimaryCalls = 0;

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
  Future<void> loadAddresses(String userId) async {}

  @override
  Future<bool> setPrimaryAddress(String addressId, String userId) async {
    setPrimaryCalls++;
    return true;
  }
}

AuthUser _user() => AuthUser(
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

/// Launcher surface: a button that opens the picker and captures the pending
/// selection future (the caller-owned contract).
class _Launcher extends StatelessWidget {
  const _Launcher({required this.onOpen, this.selectedAddressId});

  final void Function(Future<AddressEntity?> future) onOpen;
  final String? selectedAddressId;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: ElevatedButton(
          onPressed: () => onOpen(
            AddressPickerSheet.show(
              context,
              selectedAddressId: selectedAddressId,
            ),
          ),
          child: const Text('Open picker'),
        ),
      ),
    );
  }
}

Future<void> _pumpLauncher(
  WidgetTester tester, {
  required _FakeAddressNotifier notifier,
  String? selectedAddressId,
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
        authControllerProvider.overrideWith(_FakeAuthController.new),
        addressProvider.overrideWith(() => notifier),
      ],
      child: MaterialApp(
        // AddressFormDialog (empty-state CTA) renders localized labels.
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: _Launcher(
          selectedAddressId: selectedAddressId,
          onOpen: (future) {
            // Keep the caller-owned future alive for assertion; the picker
            // resolves it on choose/dismiss.
            _pendingSelection = future;
          },
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

Future<AddressEntity?>? _pendingSelection;

Future<void> _openSheet(WidgetTester tester) async {
  await tester.tap(find.text('Open picker'));
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 350));
}

void main() {
  group('AddressPickerSheet', () {
    testWidgets('lists every address from the canonical state', (tester) async {
      final notifier = _FakeAddressNotifier([
        _address(id: 'address-a', recipient: 'Buyer A', isPrimary: true),
        _address(id: 'address-b', recipient: 'Buyer B'),
        _address(id: 'address-c', recipient: 'Buyer C'),
      ]);
      await _pumpLauncher(tester, notifier: notifier);
      await _openSheet(tester);

      expect(find.text('Pilih Alamat Pengiriman'), findsOneWidget);
      expect(find.byType(ShippingAddressCard), findsNWidgets(3));
      expect(find.text('Buyer A'), findsOneWidget);
      expect(find.text('Buyer B'), findsOneWidget);
      expect(find.text('Buyer C'), findsOneWidget);
      // Primary badge comes from the canonical card, not a picker copy.
      expect(find.text('Utama'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets('marks the caller-selected address', (tester) async {
      final notifier = _FakeAddressNotifier([
        _address(id: 'address-a', recipient: 'Buyer A', isPrimary: true),
        _address(id: 'address-b', recipient: 'Buyer B'),
      ]);
      await _pumpLauncher(
        tester,
        notifier: notifier,
        selectedAddressId: 'address-b',
      );
      await _openSheet(tester);

      // B is the caller's pick → checked; A is the only unselected row.
      expect(find.byIcon(Icons.radio_button_checked), findsOneWidget);
      expect(find.byIcon(Icons.radio_button_off), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets('returns the chosen AddressEntity and never mutates primary', (
      tester,
    ) async {
      final notifier = _FakeAddressNotifier([
        _address(id: 'address-a', recipient: 'Buyer A', isPrimary: true),
        _address(id: 'address-b', recipient: 'Buyer B'),
      ]);
      await _pumpLauncher(tester, notifier: notifier);
      await _openSheet(tester);

      await tester.ensureVisible(find.text('Buyer B'));
      await tester.tap(find.text('Buyer B'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 350));

      final picked = await _pendingSelection;
      expect(picked, isNotNull);
      expect(picked!.id, 'address-b');
      expect(picked.recipientName, 'Buyer B');
      // Selection is NOT a primary mutation.
      expect(notifier.setPrimaryCalls, 0);
      expect(tester.takeException(), isNull);
    });

    testWidgets('dismissal returns null', (tester) async {
      final notifier = _FakeAddressNotifier([
        _address(id: 'address-a', recipient: 'Buyer A', isPrimary: true),
      ]);
      await _pumpLauncher(tester, notifier: notifier);
      await _openSheet(tester);

      // Drag-dismiss the sheet.
      await tester.fling(
        find.text('Pilih Alamat Pengiriman'),
        const Offset(0, 300),
        1000,
      );
      await tester.pumpAndSettle();

      final picked = await _pendingSelection;
      expect(picked, isNull);
      expect(tester.takeException(), isNull);
    });

    testWidgets(
      'empty book renders the canonical empty state whose CTA opens AddressFormDialog',
      (tester) async {
        final notifier = _FakeAddressNotifier([]);
        await _pumpLauncher(tester, notifier: notifier);
        await _openSheet(tester);

        expect(find.text('Belum ada alamat pengiriman'), findsOneWidget);

        await tester.ensureVisible(find.text('Tambah Alamat'));
        await tester.tap(find.text('Tambah Alamat'));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));

        // The ONE address form (not a picker-local form).
        expect(find.byType(AddressFormDialog), findsOneWidget);
        expect(tester.takeException(), isNull);
      },
    );
  });
}
