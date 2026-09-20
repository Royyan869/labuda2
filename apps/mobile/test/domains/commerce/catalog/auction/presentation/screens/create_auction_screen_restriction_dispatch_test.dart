// Create Auction Screen — canonical commerce-restriction dispatch (`_submitForm`)
// and the pre-submit market-authority gate.
//
// EXECUTABLE PROOF IN THIS FILE
//   PART A (behavioral, real UI):
//     - the CANONICAL / REQUIRED pre-submit market-authority gate still gates
//       access with the correct copy per state (no profile / expired /
//       not-yet-active), and the operational form only renders for an active
//       seller;
//     - `_submitForm()` still short-circuits through its own guards before any
//       notifier/backend call (guard order preserved).
//   PART B (call-site source contract — repo convention, cf.
//   create_auction_screen_timing_contract_test.dart):
//     - `_submitForm()`'s reactive error branch dispatches through the
//       canonical `CommerceRestrictionPresenter.handle(...)` with
//       `errorCode: notifierState.errorCode` and
//       `actionDescription: 'membuat lelang'`;
//     - the obsolete direct `isCommerceRestricted` + `show(` pair is gone;
//     - the preserved side effects / fallbacks / pre-submit gate are intact.
//
// FACTUAL BLOCKER for executing PART A's reactive branch end-to-end:
//   `_mediaUrls` is private state and is ONLY ever populated through
//   `ForSaleMediaHandler.showMediaPicker` (platform `image_picker` via
//   `MediaPickerHelper`) followed by a direct `S3Service()` upload. Neither is
//   provider-injectable and `CreateAuctionScreen` exposes no media seam
//   (`const CreateAuctionScreen({super.key})`), so no widget test can pass the
//   `'Minimal 1 foto wajib diupload'` guard and reach the notifier call. The
//   dispatch itself is therefore proven by PART B, and the identical canonical
//   call is executed end-to-end at the neighbouring commerce call-sites
//   (auction detail bid/claim, checkout, my-for-sales).
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/entities/shipping.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/repositories/shipping_repository.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

const String _screenPath =
    'lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

/// Counts `createAuction` invocations — used to prove that `_submitForm()`'s
/// own guards still return before any backend mutation attempt.
class _CountingAuctionNotifier extends AuctionNotifier {
  int createCalls = 0;

  @override
  AuctionNotifierState build() => const AuctionNotifierState();

  @override
  Future<bool> createAuction({
    required String sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    required String title,
    required String description,
    required List<String> mediaUrls,
    required List<AuctionMediaType> mediaTypes,
    required KoiDetails koiDetails,
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
    String? farmAddressId,
    AuctionLocation? location,
    required List<String> shippingSetupIds,
    String? preparationNote,
  }) async {
    createCalls++;
    return false;
  }
}

class _FakeShippingRepository implements ShippingRepository {
  @override
  Future<Result<List<ShippingSetup>>> listMyActiveShippingSetups() async =>
      Result.success([
        ShippingSetup(
          id: 'ship-1',
          name: 'JNE',
          type: ShippingType.custom,
          coverageAreas: const [],
          createdAt: DateTime.utc(2026, 1, 1),
          updatedAt: DateTime.utc(2026, 1, 1),
        ),
      ]);

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

AuthUser _seller({
  String id = 'seller-1',
  bool hasSellerProfile = true,
  bool hasMarketAuthority = true,
  String sellerSubscriptionStatus = 'active',
}) {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: id,
    createdAt: now,
    updatedAt: now,
    email: '$id@example.com',
    username: id,
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: sellerSubscriptionStatus,
    hasMarketAuthority: hasMarketAuthority,
    lifecycle: ContentLifecycle.active,
  );
}

Future<_CountingAuctionNotifier> _pumpCreateAuction(
  WidgetTester tester, {
  required AuthState authState,
}) async {
  final notifier = _CountingAuctionNotifier();

  await tester.binding.setSurfaceSize(const Size(800, 1800));
  addTearDown(() => tester.binding.setSurfaceSize(null));

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(
          () => _FakeAuthController(authState),
        ),
        auctionNotifierProvider.overrideWith(() => notifier),
        shippingRepositoryProvider.overrideWithValue(
          _FakeShippingRepository(),
        ),
      ],
      child: const MaterialApp(home: CreateAuctionScreen()),
    ),
  );
  await tester.pumpAndSettle();
  return notifier;
}

Finder _formList() => find.byWidgetPredicate(
  (w) => w is ListView && w.scrollDirection == Axis.vertical,
);

Future<void> _scrollFormIntoView(WidgetTester tester, Finder target) async {
  await tester.dragUntilVisible(target, _formList(), const Offset(0, -300));
  await tester.pumpAndSettle();
}

Future<void> _enterFieldByLabel(
  WidgetTester tester,
  String labelText,
  String value,
) async {
  final labelFinder = find.text(labelText, skipOffstage: false);
  if (labelFinder.evaluate().isEmpty) {
    fail('Could not find TextFormField with label "$labelText"');
  }
  await _scrollFormIntoView(tester, labelFinder);

  final editable = find.descendant(
    of: find.ancestor(
      of: labelFinder,
      matching: find.byType(InputDecorator, skipOffstage: false),
    ),
    matching: find.byType(EditableText, skipOffstage: false),
  );
  await tester.enterText(editable.first, value);
  await tester.pumpAndSettle();
}

Future<void> _selectVariety(WidgetTester tester) async {
  final field = find
      .byType(DropdownButtonFormField<String>, skipOffstage: false)
      .first;
  await _scrollFormIntoView(tester, field);
  await tester.tap(field);
  await tester.pumpAndSettle();
  await tester.tap(find.text('Kohaku').last);
  await tester.pumpAndSettle();
}

Future<void> _tapSubmit(WidgetTester tester) async {
  final button = find.widgetWithText(ElevatedButton, 'Buat Lelang');
  await tester.dragUntilVisible(button, _formList(), const Offset(0, -300));
  await tester.pumpAndSettle();
  await tester.tap(button);
  await tester.pumpAndSettle();
}

void main() {
  group('CreateAuctionScreen pre-submit market-authority gate (no regress)', () {
    testWidgets('active seller (profile + authority) sees the operational form', (
      tester,
    ) async {
      await _pumpCreateAuction(
        tester,
        authState: AuthState.authenticated(
          _seller(),
          emailVerified: true,
        ),
      );

      expect(find.text('Informasi Dasar'), findsOneWidget);
      expect(find.widgetWithText(ElevatedButton, 'Buat Lelang'), findsOneWidget);
      expect(find.text('Langganan Seller Habis'), findsNothing);
      expect(find.text('Jadi Seller Dulu'), findsNothing);
      expect(tester.takeException(), isNull);
    });

    testWidgets('expired authority loss gates to the renewal copy', (
      tester,
    ) async {
      await _pumpCreateAuction(
        tester,
        authState: AuthState.authenticated(
          _seller(
            hasMarketAuthority: false,
            sellerSubscriptionStatus: 'expired',
          ),
          emailVerified: true,
        ),
      );

      expect(find.text('Langganan Seller Habis'), findsOneWidget);
      expect(find.text('Perpanjang Langganan'), findsOneWidget);
      expect(find.text('Informasi Dasar'), findsNothing);
      expect(tester.takeException(), isNull);
    });

    testWidgets('not-yet-active authority gates to the activation copy', (
      tester,
    ) async {
      await _pumpCreateAuction(
        tester,
        authState: AuthState.authenticated(
          _seller(
            hasMarketAuthority: false,
            sellerSubscriptionStatus: 'none',
          ),
          emailVerified: true,
        ),
      );

      expect(find.text('Langganan Belum Aktif'), findsOneWidget);
      expect(find.text('Aktifkan Langganan'), findsOneWidget);
      expect(find.text('Informasi Dasar'), findsNothing);
      expect(tester.takeException(), isNull);
    });

    testWidgets('seller without a profile still gates to the upgrade copy', (
      tester,
    ) async {
      await _pumpCreateAuction(
        tester,
        authState: AuthState.authenticated(
          _seller(hasSellerProfile: false, hasMarketAuthority: false),
          emailVerified: true,
        ),
      );

      expect(find.text('Jadi Seller Dulu'), findsOneWidget);
      expect(find.text('Mulai Jualan'), findsOneWidget);
      expect(find.text('Informasi Dasar'), findsNothing);
      expect(tester.takeException(), isNull);
    });
  });

  group('CreateAuctionScreen._submitForm guard order', () {
    testWidgets('local guards short-circuit before any backend create call', (
      tester,
    ) async {
      final notifier = await _pumpCreateAuction(
        tester,
        authState: AuthState.authenticated(
          _seller(),
          emailVerified: true,
        ),
      );

      // Empty submit → local validation guard, never the notifier.
      await _tapSubmit(tester);
      expect(find.text('Judul wajib diisi'), findsOneWidget);
      expect(notifier.createCalls, 0);

      // Fully valid form except media → media guard, still never the notifier.
      await _enterFieldByLabel(tester, 'Judul *', 'Kohaku 50cm');
      await _enterFieldByLabel(tester, 'Deskripsi *', 'Ikan sehat');
      await _enterFieldByLabel(tester, 'Harga Awal *', '1000000');
      await _enterFieldByLabel(tester, 'Kenaikan Bid *', '50000');
      await _selectVariety(tester);
      await _enterFieldByLabel(tester, 'Ukuran (cm) *', '30');
      await _tapSubmit(tester);

      expect(find.text('Minimal 1 foto wajib diupload'), findsOneWidget);
      expect(notifier.createCalls, 0);
      expect(tester.takeException(), isNull);
    });
  });

  group('CreateAuctionScreen._submitForm canonical restriction dispatch', () {
    test('restriction family is dispatched through the canonical presenter', () {
      final source = File(_screenPath).readAsStringSync();

      // Canonical dispatch with the exact error code + action description.
      expect(source, contains('CommerceRestrictionPresenter.handle('));
      expect(source, contains('errorCode: notifierState.errorCode,'));
      expect(source, contains("actionDescription: 'membuat lelang',"));

      // Obsolete direct restriction presentation purged.
      expect(source, isNot(contains('isCommerceRestricted')));
      expect(source, isNot(contains('CommerceRestrictionPresenter.show(')));

      // Canonical dispatch runs inside the reactive failure branch and before
      // the generic inline fallback.
      final failureBranch = source.indexOf('if (!success) {');
      final handleCall = source.indexOf('CommerceRestrictionPresenter.handle(');
      final genericFallback = source.indexOf(
        "'Gagal membuat lelang. Cek pesan dari backend.'",
      );
      expect(failureBranch, greaterThan(0));
      expect(handleCall, greaterThan(failureBranch));
      expect(genericFallback, greaterThan(handleCall));
    });

    test('preserved side effects, fallbacks and pre-submit gate are intact', () {
      final source = File(_screenPath).readAsStringSync();

      // Local submit-flag side effect.
      expect(source, contains('setState(() => _isSubmitting = false);'));
      // Generic inline fallback.
      expect(source, contains("'Gagal membuat lelang. Cek pesan dari backend.'"));
      // Mounted guard + success flow.
      expect(source, contains('if (!mounted) return;'));
      expect(source, contains("AppSnackBar.showSuccess(context, 'Lelang berhasil dibuat');"));
      expect(source, contains('Navigator.of(context).pop(true);'));
      // CANONICAL / REQUIRED pre-submit market-authority gate untouched.
      expect(source, contains('if (!hasSellerProfile || !hasMarketAuthority) {'));
      expect(
        source,
        contains(
          "'Langganan seller Anda sudah berakhir. Perpanjang dulu untuk membuat lelang.'",
        ),
      );
      expect(
        source,
        contains("'Langganan seller belum aktif. Aktifkan dulu untuk membuat lelang.'"),
      );
    });
  });
}
