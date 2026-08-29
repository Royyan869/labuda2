import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart';
import 'package:labuda/domains/commerce/catalog/shared/data/dto/commerce_media_request_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_media_handler.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeAuctionNotifier extends AuctionNotifier {
  int createCalls = 0;
  List<CommerceMediaRequestDto>? lastSubmittedMedia;
  List<AuctionMediaType>? lastSubmittedMediaTypes;

  @override
  AuctionNotifierState build() => const AuctionNotifierState();

  @override
  Future<bool> createAuction({
    required String title,
    required String description,
    List<CommerceMediaRequestDto> media = const [],
    required List<String> mediaUrls,
    required List<AuctionMediaType> mediaTypes,
    required KoiDetails koiDetails,
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
    AuctionLocation? location,
    required List<String> shippingOptionIds,
    String? preparationNote,
  }) async {
    createCalls += 1;
    lastSubmittedMedia = List.from(media);
    lastSubmittedMediaTypes = List.from(mediaTypes);
    state = state.copyWith(successMessage: 'Lelang berhasil dibuat');
    return true;
  }
}

class _FakeShippingRepository implements ShippingRepository {
  final List<ShippingOption> _options;

  _FakeShippingRepository()
    : _options = [
        ShippingOption(
          id: 'ship-1',
          name: 'JNE',
          type: ShippingType.custom,
          coverageAreas: const [],
          createdAt: DateTime.utc(2026, 1, 1),
          updatedAt: DateTime.utc(2026, 1, 1),
        ),
      ];

  @override
  Future<Result<List<ShippingOption>>> listMyShippingOptions() async =>
      Result.success(_options);

  @override
  Future<Result<List<ShippingOption>>> listMyActiveShippingOptions() async =>
      Result.success(_options);

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _NoOpLogger implements ILoggerService {
  @override
  dynamic noSuchMethod(Invocation invocation) async {
    if (invocation.memberName == #debug ||
        invocation.memberName == #info ||
        invocation.memberName == #warning ||
        invocation.memberName == #error ||
        invocation.memberName == #fatal) {
      return Result.success(null);
    }
    return super.noSuchMethod(invocation);
  }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

File _tempFile(String name, {int byteCount = 1}) {
  final dir = Directory.systemTemp.createTempSync('labuda_auction_media_');
  final file = File('${dir.path}${Platform.pathSeparator}$name');
  file.writeAsBytesSync(List<int>.filled(byteCount, 1));
  return file;
}

AuthUser _seller({
  required bool hasSellerProfile,
  required bool hasMarketAuthority,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: 'seller-1',
    createdAt: now,
    updatedAt: now,
    email: 'seller@example.com',
    username: 'seller',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: hasMarketAuthority ? 'active' : 'expired',
    hasMarketAuthority: hasMarketAuthority,
    sellerTier: SellerTier.sellerElite,
    isIdVerified: false,
    isFarmVerified: false,
    lifecycle: ContentLifecycle.active,
  );
}

File _tempImageFile() {
  final dir = Directory.systemTemp.createTempSync('labuda_auction_media_');
  final file = File('${dir.path}${Platform.pathSeparator}picked.jpg')
    ..writeAsBytesSync([1, 2, 3, 4]);
  return file;
}

/// Fake picker that returns one local file.
Future<void> _fakeMediaPicker({
  required BuildContext context,
  required Future<void> Function(List<File> files) onMediaSelected,
  required int currentMediaCount,
}) async {
  await onMediaSelected([_tempImageFile()]);
}

/// Fake picker that returns three local files.
Future<void> _fakeMultiMediaPicker({
  required BuildContext context,
  required Future<void> Function(List<File> files) onMediaSelected,
  required int currentMediaCount,
}) async {
  await onMediaSelected([
    _tempImageFile(),
    _tempImageFile(),
    _tempImageFile(),
  ]);
}

Widget _wrapInteractive({
  required AuthState state,
  required AuctionNotifier auctionNotifier,
  required ShippingRepository shippingRepository,
  AuctionMediaPickerLauncher? mediaPicker,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuthController(state)),
      auctionNotifierProvider.overrideWith(() => auctionNotifier),
      shippingRepositoryProvider.overrideWithValue(shippingRepository),
      apiClientProvider.overrideWithValue(ApiClient.testing()),
      loggerServiceProvider.overrideWithValue(_NoOpLogger()),
    ],
    child: MaterialApp(
      home: CreateAuctionScreen(mediaPicker: mediaPicker ?? _fakeMediaPicker),
    ),
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

void main() {
  group('CreateAuctionScreen shared validation', () {
    test('ForSaleMediaHandler exposes canonical limits', () {
      expect(ForSaleMediaHandler.maxImages, 10);
      expect(ForSaleMediaHandler.maxImageSizeMb, 10);
      final handler = ForSaleMediaHandler();
      expect(handler, isA<ForSaleMediaHandler>());
    });

    test('ForSaleMediaHandler has gallery/camera entry points', () {
      final handler = ForSaleMediaHandler();
      expect(handler.pickPhotosFromGallery, isA<Function>());
      expect(handler.openCamera, isA<Function>());
      expect(ForSaleMediaHandler.showMediaPicker, isA<Function>());
    });
  });

  group('CreateAuctionScreen media behavior', () {
    testWidgets('empty state renders before selection', (tester) async {
      final notifier = _FakeAuctionNotifier();
      final shippingRepo = _FakeShippingRepository();

      await tester.pumpWidget(
        _wrapInteractive(
          state: AuthState.authenticated(
            _seller(hasSellerProfile: true, hasMarketAuthority: true),
            emailVerified: true,
          ),
          auctionNotifier: notifier,
          shippingRepository: shippingRepo,
        ),
      );
      await tester.pumpAndSettle();

      // Placeholder should be visible
      expect(find.text('Tap untuk upload media'), findsOneWidget);
      // Count header shows 0/10
      expect(find.text('Media Ikan (0/10)'), findsOneWidget);
    });

    testWidgets('picker returns three media — three previews render', (
      tester,
    ) async {
      final notifier = _FakeAuctionNotifier();
      final shippingRepo = _FakeShippingRepository();

      await tester.pumpWidget(
        _wrapInteractive(
          state: AuthState.authenticated(
            _seller(hasSellerProfile: true, hasMarketAuthority: true),
            emailVerified: true,
          ),
          auctionNotifier: notifier,
          shippingRepository: shippingRepo,
          mediaPicker: _fakeMultiMediaPicker,
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Tap untuk upload media'));
      await tester.pumpAndSettle();

      // Three images render
      expect(find.byType(Image), findsNWidgets(3));
      // Placeholder gone
      expect(find.text('Tap untuk upload media'), findsNothing);
      // Count updated
      expect(find.text('Media Ikan (3/10)'), findsOneWidget);
    });

    testWidgets('first item is marked Cover', (tester) async {
      final notifier = _FakeAuctionNotifier();
      final shippingRepo = _FakeShippingRepository();

      await tester.pumpWidget(
        _wrapInteractive(
          state: AuthState.authenticated(
            _seller(hasSellerProfile: true, hasMarketAuthority: true),
            emailVerified: true,
          ),
          auctionNotifier: notifier,
          shippingRepository: shippingRepo,
          mediaPicker: _fakeMultiMediaPicker,
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Tap untuk upload media'));
      await tester.pumpAndSettle();

      // Exactly one Cover badge
      expect(find.text('Cover'), findsOneWidget);
    });

    testWidgets('delete current cover — next item becomes Cover', (
      tester,
    ) async {
      final notifier = _FakeAuctionNotifier();
      final shippingRepo = _FakeShippingRepository();

      await tester.pumpWidget(
        _wrapInteractive(
          state: AuthState.authenticated(
            _seller(hasSellerProfile: true, hasMarketAuthority: true),
            emailVerified: true,
          ),
          auctionNotifier: notifier,
          shippingRepository: shippingRepo,
          mediaPicker: _fakeMultiMediaPicker,
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Tap untuk upload media'));
      await tester.pumpAndSettle();

      expect(find.text('Media Ikan (3/10)'), findsOneWidget);
      expect(find.byIcon(Icons.close), findsNWidgets(3));

      // Delete the first item (the cover)
      await tester.tap(find.byIcon(Icons.close).first);
      await tester.pumpAndSettle();

      // Count decreased, Cover badge still present on new first item
      expect(find.text('Media Ikan (2/10)'), findsOneWidget);
      expect(find.text('Cover'), findsOneWidget);
      expect(find.byIcon(Icons.close), findsNWidgets(2));
    });

    testWidgets('delete all media — placeholder returns', (tester) async {
      final notifier = _FakeAuctionNotifier();
      final shippingRepo = _FakeShippingRepository();

      await tester.pumpWidget(
        _wrapInteractive(
          state: AuthState.authenticated(
            _seller(hasSellerProfile: true, hasMarketAuthority: true),
            emailVerified: true,
          ),
          auctionNotifier: notifier,
          shippingRepository: shippingRepo,
          mediaPicker: _fakeMediaPicker,
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Tap untuk upload media'));
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.close), findsOneWidget);
      await tester.tap(find.byIcon(Icons.close));
      await tester.pumpAndSettle();

      // Placeholder returns, count resets
      expect(find.text('Tap untuk upload media'), findsOneWidget);
      expect(find.text('Media Ikan (0/10)'), findsOneWidget);
    });

    testWidgets('media survives form rebuild (scroll)', (tester) async {
      final notifier = _FakeAuctionNotifier();
      final shippingRepo = _FakeShippingRepository();

      await tester.pumpWidget(
        _wrapInteractive(
          state: AuthState.authenticated(
            _seller(hasSellerProfile: true, hasMarketAuthority: true),
            emailVerified: true,
          ),
          auctionNotifier: notifier,
          shippingRepository: shippingRepo,
          mediaPicker: _fakeMediaPicker,
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Tap untuk upload media'));
      await tester.pumpAndSettle();

      expect(find.byType(Image), findsOneWidget);
      expect(find.text('Cover'), findsOneWidget);
      expect(find.text('Media Ikan (1/10)'), findsOneWidget);

      // Scroll down and up — media must survive
      final outerListView = find.byWidgetPredicate(
        (w) => w is ListView && w.scrollDirection == Axis.vertical,
      );
      await tester.drag(outerListView, const Offset(0, -500));
      await tester.pumpAndSettle();
      await tester.drag(outerListView, const Offset(0, 500));
      await tester.pumpAndSettle();

      expect(find.byType(Image), findsOneWidget);
      expect(find.text('Cover'), findsOneWidget);
      expect(find.text('Media Ikan (1/10)'), findsOneWidget);
    });

    testWidgets('no remote-URL-only preview requirement', (tester) async {
      final notifier = _FakeAuctionNotifier();
      final shippingRepo = _FakeShippingRepository();

      await tester.pumpWidget(
        _wrapInteractive(
          state: AuthState.authenticated(
            _seller(hasSellerProfile: true, hasMarketAuthority: true),
            emailVerified: true,
          ),
          auctionNotifier: notifier,
          shippingRepository: shippingRepo,
          mediaPicker: _fakeMediaPicker,
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Tap untuk upload media'));
      await tester.pumpAndSettle();

      // Local preview uses Image.file, not Image.network
      final imageWidget = tester.widget<Image>(find.byType(Image));
      expect(imageWidget.image, isA<FileImage>(),
          reason: 'Selected media should render via Image.file for local files');
    });

    testWidgets('picker cancellation is a no-op and does not show an error', (
      tester,
    ) async {
      final notifier = _FakeAuctionNotifier();
      final shippingRepo = _FakeShippingRepository();

      Future<void> cancelledPicker({
        required BuildContext context,
        required Future<void> Function(List<File> files) onMediaSelected,
        required int currentMediaCount,
      }) async {}

      await tester.pumpWidget(
        _wrapInteractive(
          state: AuthState.authenticated(
            _seller(hasSellerProfile: true, hasMarketAuthority: true),
            emailVerified: true,
          ),
          auctionNotifier: notifier,
          shippingRepository: shippingRepo,
          mediaPicker: cancelledPicker,
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Tap untuk upload media'));
      await tester.pumpAndSettle();

      expect(find.text('Tap untuk upload media'), findsOneWidget);
      expect(find.textContaining('Gagal'), findsNothing);
      expect(notifier.createCalls, 0);
    });
  });
}
