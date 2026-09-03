import 'dart:async';

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart'
    show auctionRepositoryProvider;
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart';
import 'package:labuda/domains/commerce/catalog/shared/data/dto/commerce_media_request_dto.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_address_repository.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/models/wilayah_models.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  AuthState _state;

  @override
  AuthState build() => _state;

  void setAuthState(AuthState state) {
    _state = state;
    this.state = state;
  }
}

class _FakeAuctionNotifier extends AuctionNotifier {
  static int globalCreateCalls = 0;
  int createCalls = 0;
  List<CommerceMediaRequestDto>? lastSubmittedMedia;
  List<AuctionMediaType>? lastSubmittedMediaTypes;
  bool publicationSucceeds = true;
  String? failureMessage;
  String? failureCode;

  /// When non-null, the notifier returns this completer's future instead of
  /// completing immediately. Used by the duplicate-submit behavioral test.
  Completer<bool>? pendingCreate;

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
    required List<String> shippingSetupIds,
    String? preparationNote,
  }) async {
    createCalls += 1;
    globalCreateCalls += 1;
    lastSubmittedMedia = List.from(media);
    lastSubmittedMediaTypes = List.from(mediaTypes);
    if (pendingCreate != null) {
      state = state.copyWith(successMessage: null);
      return pendingCreate!.future;
    }
    if (!publicationSucceeds) {
      state = state.copyWith(
        isCreating: false,
        error: failureMessage ?? 'Gagal membuat lelang.',
        errorCode: failureCode,
        clearSuccess: true,
      );
      return false;
    }
    state = state.copyWith(successMessage: 'Lelang berhasil dibuat');
    return true;
  }
}

class _FakeAuctionRepository implements AuctionRepository {
  int watchActiveCalls = 0;
  int watchUserCalls = 0;

  @override
  Stream<List<Auction>> watchActiveAuctions({int limit = 50}) {
    watchActiveCalls += 1;
    return Stream.value(const []);
  }

  @override
  Stream<List<Auction>> watchUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 50,
  }) {
    watchUserCalls += 1;
    return Stream.value(const []);
  }

  @override
  Stream<Auction?> watchAuction(String auctionId) => const Stream.empty();

  @override
  Stream<List<AuctionBid>> watchAuctionBids(
    String auctionId, {
    int limit = 50,
  }) => const Stream.empty();

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _RecordingNavigatorObserver extends NavigatorObserver {
  int popCount = 0;

  @override
  void didPop(Route<dynamic> route, Route<dynamic>? previousRoute) {
    if (route is PageRoute<dynamic>) {
      popCount += 1;
    }
    super.didPop(route, previousRoute);
  }
}

class _FakeShippingRepository implements ShippingRepository {
  final List<ShippingSetup> _options;

  _FakeShippingRepository()
    : _options = [
        ShippingSetup(
          id: 'ship-1',
          name: 'JNE',
          type: ShippingType.custom,
          coverageAreas: const [],
          createdAt: DateTime.utc(2026, 1, 1),
          updatedAt: DateTime.utc(2026, 1, 1),
        ),
      ];

  @override
  Future<Result<List<ShippingSetup>>> listMyShippingSetups() async =>
      Result.success(_options);

  @override
  Future<Result<List<ShippingSetup>>> listMyActiveShippingSetups() async =>
      Result.success(_options);

  @override
  Future<Result<ShippingSetup>> getShippingSetupById(String optionId) async =>
      Result.error('not used');

  @override
  Future<Result<ShippingSetup>> createShippingSetup(
    CreateShippingSetupRequest request,
  ) async => Result.error('not used');

  @override
  Future<Result<ShippingSetup>> updateShippingSetupFull(
    String optionId,
    UpdateShippingSetupFullRequest request,
  ) async => Result.error('not used');

  @override
  Future<Result<void>> deleteShippingSetup(String optionId) async =>
      Result.error('not used');

  @override
  Future<Result<void>> toggleActiveStatus(
    String optionId,
    bool isActive,
  ) async => Result.error('not used');

  @override
  Future<Result<ShippingCoverage>> addCoverage(
    String optionId,
    AddCoverageRequest request,
  ) async => Result.error('not used');

  @override
  Future<Result<ShippingCoverage>> updateCoverage(
    String coverageId,
    UpdateCoverageRequest request,
  ) async => Result.error('not used');

  @override
  Future<Result<void>> deleteCoverage(String coverageId) async =>
      Result.error('not used');

  @override
  Future<Result<void>> setProductShippingSetups(
    String productId,
    List<String> shippingSetupIds,
  ) async => Result.error('not used');

  @override
  Future<Result<DeliveryAvailabilityResult>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async => Result.error('not used');

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _FakeS3Service extends S3Service {
  _FakeS3Service();

  final List<String> deletedKeys = [];

  @override
  Future<Result<void>> deleteFile(String storageKey) async {
    deletedKeys.add(storageKey);
    return Result.success(null);
  }
}

class _FakeAddressRepository implements IAddressRepository {
  _FakeAddressRepository(this._primarySenderAddress);

  final AddressEntity? _primarySenderAddress;

  @override
  Future<Result<AddressEntity?>> getPrimaryAddress(
    String userId, {
    AddressPurpose? purpose,
  }) async {
    if (purpose == AddressPurpose.sender) {
      return Result.success(_primarySenderAddress);
    }
    return Result.success(null);
  }

  @override
  Future<Result<List<AddressEntity>>> getAddressesByUserId(
    String userId,
  ) async {
    return Result.success(const []);
  }

  @override
  Future<Result<List<AddressEntity>>> getAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  ) async {
    return Result.success(const []);
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
  Stream<Result<List<AddressEntity>>> watchAddresses(String userId) {
    return Stream.value(Result.success(const []));
  }

  @override
  Stream<Result<List<AddressEntity>>> watchAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  ) {
    return Stream.value(Result.success(const []));
  }

  @override
  Future<Result<int>> countAddresses(
    String userId, {
    AddressPurpose? purpose,
  }) async {
    return Result.success(0);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

AuthUser _seller({
  required String id,
  required String username,
  required bool hasSellerProfile,
  required bool hasMarketAuthority,
  bool isIdVerified = false,
  bool isFarmVerified = false,
  SellerTier sellerTier = SellerTier.sellerBasic,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: id,
    createdAt: now,
    updatedAt: now,
    email: '$username@example.com',
    username: username,
    avatarUrl: 'https://example.com/$id.png',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: hasMarketAuthority ? 'active' : 'expired',
    hasMarketAuthority: hasMarketAuthority,
    sellerTier: sellerTier,
    isIdVerified: isIdVerified,
    isFarmVerified: isFarmVerified,
    lifecycle: ContentLifecycle.active,
  );
}

Future<void> _fakeMediaPicker({
  required BuildContext context,
  required Future<void> Function(List<File> files) onMediaSelected,
  required int currentMediaCount,
}) async {
  await onMediaSelected([_tempImageFile()]);
}

Future<void> _fakeVideoMediaPicker({
  required BuildContext context,
  required Future<void> Function(List<File> files) onMediaSelected,
  required int currentMediaCount,
}) async {
  await onMediaSelected([_tempVideoFile()]);
}

File _tempImageFile() {
  final dir = Directory.systemTemp.createTempSync('labuda_auction_preview_');
  final file = File('${dir.path}${Platform.pathSeparator}picked.jpg')
    ..writeAsBytesSync([5, 6, 7, 8]);
  return file;
}

File _tempVideoFile() {
  final dir = Directory.systemTemp.createTempSync('labuda_auction_preview_');
  final file = File('${dir.path}${Platform.pathSeparator}picked.mp4')
    ..writeAsBytesSync([9, 10, 11, 12]);
  return file;
}

AddressEntity _completeSenderAddress() {
  return AddressEntity(
    id: 'addr-1',
    userId: 'seller-1',
    purpose: AddressPurpose.sender,
    recipientName: 'Farm Sentosa',
    phone: '08123456789',
    province: Province(id: '33', name: 'Jawa Tengah'),
    city: City(id: '3301', name: 'Kabupaten Demak', provinceId: '33'),
    district: District(id: '330101', name: 'Mranggen', cityId: '3301'),
    village: Village(id: '3301012001', name: 'Rowosari', districtId: '330101'),
    streetAddress: 'Jl. Melati No. 12',
    postalCode: '59511',
    isPrimary: true,
    createdAt: DateTime.utc(2026, 7, 25),
    updatedAt: DateTime.utc(2026, 7, 25),
  );
}

Future<void> _fakeLocalMediaPicker({
  required BuildContext context,
  required Future<void> Function(List<File> files) onMediaSelected,
  required int currentMediaCount,
}) async {
  await onMediaSelected([_tempImageFile()]);
}

Future<List<CommerceMediaRequestDto>> _fakeMediaUploader({
  required BuildContext context,
  required List<File> photos,
}) async {
  return List.generate(
    photos.length,
    (index) => CommerceMediaRequestDto.image(
      url: 'https://example.com/auction/$index.jpg',
    ),
  );
}

Future<List<CommerceMediaRequestDto>> _fakeVideoMediaUploader({
  required BuildContext context,
  required List<File> photos,
}) async {
  return List.generate(
    photos.length,
    (index) => CommerceMediaRequestDto.video(
      url: 'https://cdn.example.com/videos/auction-failure-$index.mp4',
      thumbnailUrl:
          'https://cdn.example.com/videos/auction-failure-$index.mp4_poster.jpg',
    ),
  );
}

ProviderContainer _container({
  required AuthController authController,
  required AuctionNotifier auctionNotifier,
  AuctionRepository? auctionRepository,
  S3Service? s3Service,
}) {
  return ProviderContainer(
    overrides: [
      authControllerProvider.overrideWith(() => authController),
      auctionNotifierProvider.overrideWith(() => auctionNotifier),
      if (auctionRepository != null)
        auctionRepositoryProvider.overrideWithValue(auctionRepository),
      if (s3Service != null) s3ServiceProvider.overrideWithValue(s3Service),
      addressRepositoryProvider.overrideWithValue(
        _FakeAddressRepository(_completeSenderAddress()),
      ),
      shippingRepositoryProvider.overrideWithValue(_FakeShippingRepository()),
      apiClientProvider.overrideWithValue(ApiClient.testing()),
    ],
  );
}

Widget _wrap({
  required ProviderContainer container,
  Widget? child,
  List<NavigatorObserver> navigatorObservers = const [],
}) {
  final screen = child ??
      const CreateAuctionScreen(
        mediaPicker: _fakeMediaPicker,
      );

  return UncontrolledProviderScope(
    container: container,
    child: MaterialApp(
      home: screen,
      navigatorObservers: navigatorObservers,
    ),
  );
}

Future<void> _scrollFormIntoView(WidgetTester tester, Finder target) async {
  await tester.dragUntilVisible(
    target,
    find.byWidgetPredicate(
      (w) => w is ListView && w.scrollDirection == Axis.vertical,
    ),
    const Offset(0, -300),
  );
  await tester.pumpAndSettle();
}

Future<void> _scrollFormBy(WidgetTester tester, Offset offset) async {
  await tester.drag(
    find.byWidgetPredicate(
      (w) => w is ListView && w.scrollDirection == Axis.vertical,
    ),
    offset,
  );
  await tester.pumpAndSettle();
}

Future<void> _openVarietyPicker(WidgetTester tester) async {
  final field = find
      .byType(DropdownButtonFormField<String>, skipOffstage: false)
      .first;
  await _scrollFormIntoView(tester, field);
  await tester.tap(field);
  await tester.pumpAndSettle();
  await tester.tap(find.text('Kohaku').last);
  await tester.pumpAndSettle();
}

/// Helper: enter text into a field by finding the EditableText that is a
/// TextFormField whose decoration labelText matches [label].
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

Future<void> _fillValidForm(WidgetTester tester) async {
  // Title (first TextFormField, always visible at top)
  await _enterFieldByLabel(tester, 'Judul *', 'Kohaku 50cm');

  // Description (always visible at top)
  await _enterFieldByLabel(tester, 'Deskripsi *', 'Healthy koi');

  // Trigger media upload via fake picker
  await tester.tap(find.text('Tap untuk upload media'));
  await tester.pumpAndSettle();

  // Select variety via dropdown
  await _openVarietyPicker(tester);

  // Size field: scroll to label and enter text
  await _scrollFormIntoView(
    tester,
    find.text('Ukuran (cm) *', skipOffstage: false),
  );
  // After scrolling, we need to ensure the correct TextFormField is targeted.
  // The size field is the one with 'Ukuran (cm) *' as decoration label.
  // Find the TextFormField that is most likely the input field.
  // After scrolling for 'Ukuran (cm) *', the last EditableText in view should be
  // the one for size input (or age, if it came into view). But we can find the
  // specific TextFormField by finding the one with the label as descendant.
  await _enterFieldByLabel(tester, 'Ukuran (cm) *', '50');

  // Opening bid
  await _enterFieldByLabel(tester, 'Harga Awal *', '1000000');

  // Bid increment
  await _enterFieldByLabel(tester, 'Kenaikan Bid *', '100000');

  // Scroll back to access duration chips and shipping
  await _scrollFormBy(tester, const Offset(0, -600));

  // Select duration explicitly (start mode chips are a separate control)
  final durationChip = find.byType(ChoiceChip, skipOffstage: false).first;
  await _scrollFormIntoView(tester, durationChip);
  await tester.tap(durationChip);
  await tester.pumpAndSettle();
  expect(
    tester.widget<ChoiceChip>(durationChip).selected,
    isTrue,
    reason: 'Duration chip should stay selected after tap',
  );

  // Select shipping (first FilterChip from the fake repo)
  final shippingChip = find.byType(FilterChip, skipOffstage: false).first;
  await _scrollFormIntoView(
    tester,
    shippingChip,
  );
  await tester.tap(shippingChip);
  await tester.pumpAndSettle();
  expect(
    tester.widget<FilterChip>(shippingChip).selected,
    isTrue,
    reason: 'Shipping chip should stay selected after tap',
  );
}

void main() {
  group('CreateAuctionScreen authority behavior', () {
    testWidgets('loading hydration fails closed with no operational form', (
      tester,
    ) async {
      final container = _container(
        authController: _FakeAuthController(const AuthState.loading()),
        auctionNotifier: _FakeAuctionNotifier(),
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));

      expect(find.text('Silakan login untuk melanjutkan.'), findsOneWidget);
      expect(find.byType(TextFormField), findsNothing);
      expect(find.text('Informasi Dasar'), findsNothing);
    });

    testWidgets('unauthenticated account fails closed', (tester) async {
      final container = _container(
        authController: _FakeAuthController(const AuthState.unauthenticated()),
        auctionNotifier: _FakeAuctionNotifier(),
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));

      expect(find.text('Silakan login untuk melanjutkan.'), findsOneWidget);
      expect(find.byType(TextFormField), findsNothing);
      expect(find.text('Informasi Dasar'), findsNothing);
    });

    testWidgets('non-seller gets registration gate', (tester) async {
      final container = _container(
        authController: _FakeAuthController(
          AuthState.authenticated(
            _seller(
              id: 'buyer-1',
              username: 'buyer',
              hasSellerProfile: false,
              hasMarketAuthority: false,
            ),
            emailVerified: true,
          ),
        ),
        auctionNotifier: _FakeAuctionNotifier(),
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));

      expect(find.text('Jadi Seller Dulu'), findsOneWidget);
      expect(find.text('Informasi Dasar'), findsNothing);
    });

    testWidgets('expired seller gets renewal gate', (tester) async {
      final container = _container(
        authController: _FakeAuthController(
          AuthState.authenticated(
            _seller(
              id: 'seller-1',
              username: 'seller',
              hasSellerProfile: true,
              hasMarketAuthority: false,
            ),
            emailVerified: true,
          ),
        ),
        auctionNotifier: _FakeAuctionNotifier(),
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));

      expect(find.text('Langganan Seller Habis'), findsOneWidget);
      expect(find.text('Informasi Dasar'), findsNothing);
    });

    testWidgets('restricted account does not expose create mutation', (
      tester,
    ) async {
      final container = _container(
        authController: _FakeAuthController(
          AuthState.accountRestricted(
            _seller(
              id: 'seller-1',
              username: 'seller',
              hasSellerProfile: true,
              hasMarketAuthority: true,
            ),
            restrictionType: AccountStatus.suspended,
          ),
        ),
        auctionNotifier: _FakeAuctionNotifier(),
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));

      expect(find.text('Silakan login untuk melanjutkan.'), findsOneWidget);
      expect(find.byType(TextFormField), findsNothing);
      expect(
        (container.read(auctionNotifierProvider.notifier)
                as _FakeAuctionNotifier)
            .createCalls,
        0,
      );
    });

    testWidgets('active seller sees the operational form', (tester) async {
      final container = _container(
        authController: _FakeAuthController(
          AuthState.authenticated(
            _seller(
              id: 'seller-1',
              username: 'seller',
              hasSellerProfile: true,
              hasMarketAuthority: true,
            ),
            emailVerified: true,
          ),
        ),
        auctionNotifier: _FakeAuctionNotifier(),
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));
      await tester.pumpAndSettle();

      expect(find.text('Informasi Dasar'), findsOneWidget);
      expect(find.byType(TextFormField), findsWidgets);
    });

    testWidgets(
      'local photo preview appears immediately and survives rebuilds',
      (tester) async {
        final container = _container(
          authController: _FakeAuthController(
            AuthState.authenticated(
              _seller(
                id: 'seller-1',
                username: 'seller',
                hasSellerProfile: true,
                hasMarketAuthority: true,
              ),
              emailVerified: true,
            ),
          ),
          auctionNotifier: _FakeAuctionNotifier(),
        );
        addTearDown(container.dispose);

        await tester.pumpWidget(
          _wrap(
            container: container,
            child: CreateAuctionScreen(mediaPicker: _fakeLocalMediaPicker),
          ),
        );
        await tester.pumpAndSettle();

        await tester.tap(find.text('Tap untuk upload media'));
        await tester.pumpAndSettle();

        final imageFinder = find.byType(Image);
        expect(imageFinder, findsOneWidget);
        expect(
          tester.widget<Image>(imageFinder).image,
          isA<FileImage>(),
          reason: 'Selected media should render via Image.file for local paths',
        );

        expect(find.text('Informasi Dasar'), findsOneWidget);
        await tester.dragUntilVisible(
          find.text('Opsi Pengiriman *'),
          find.byWidgetPredicate(
            (w) => w is ListView && w.scrollDirection == Axis.vertical,
          ),
          const Offset(0, -300),
        );
        await tester.pumpAndSettle();
        expect(find.text('Opsi Pengiriman *'), findsOneWidget);
      },
    );

    testWidgets('active seller valid submission invokes notifier once', (
      tester,
    ) async {
      _FakeAuctionNotifier.globalCreateCalls = 0;
      final repository = _FakeAuctionRepository();
      final navigatorObserver = _RecordingNavigatorObserver();
      final container = _container(
        authController: _FakeAuthController(
          AuthState.authenticated(
            _seller(
              id: 'seller-1',
              username: 'seller',
              hasSellerProfile: true,
              hasMarketAuthority: true,
              sellerTier: SellerTier.sellerElite,
            ),
            emailVerified: true,
          ),
        ),
        auctionNotifier: _FakeAuctionNotifier(),
        auctionRepository: repository,
      );
      addTearDown(container.dispose);
      final notifier =
          container.read(auctionNotifierProvider.notifier)
              as _FakeAuctionNotifier;

      await tester.pumpWidget(
        _wrap(
          container: container,
          child: const CreateAuctionScreen(
            mediaPicker: _fakeLocalMediaPicker,
            mediaUploader: _fakeMediaUploader,
          ),
          navigatorObservers: [navigatorObserver],
        ),
      );
      await tester.pumpAndSettle();
      final activeSubscription = container.listen(
        exploreAuctionsStreamProvider,
        (previous, next) {},
        fireImmediately: false,
      );
      final userSubscription = container.listen(
        userAuctionsStreamProvider('seller-1'),
        (previous, next) {},
        fireImmediately: false,
      );
      addTearDown(activeSubscription.close);
      addTearDown(userSubscription.close);
      await container.read(exploreAuctionsStreamProvider.future);
      await container.read(userAuctionsStreamProvider('seller-1').future);
      await tester.pumpAndSettle();
      await _fillValidForm(tester);

      // Verify the shipping selector settled and shows FilterChips
      expect(
        find.byType(FilterChip, skipOffstage: false),
        findsAtLeast(1),
        reason: 'Shipping selector should show FilterChips from fake repo',
      );

      // Tap the real production submit button through WidgetTester
      final submitButton = find
          .byType(ElevatedButton, skipOffstage: false)
          .last;
      await tester.ensureVisible(submitButton);
      await tester.pumpAndSettle();
      await tester.tap(submitButton);
      await tester.pumpAndSettle();

      // Verify notifier was invoked exactly once
      expect(notifier.createCalls, 1);
      expect(_FakeAuctionNotifier.globalCreateCalls, 1);
      expect(notifier.lastSubmittedMedia, isNotNull);
      expect(notifier.lastSubmittedMedia, hasLength(1));
      expect(notifier.lastSubmittedMediaTypes, hasLength(1));
      expect(notifier.lastSubmittedMedia!.first.type, 'image');
    });

    testWidgets(
      'publication failure cleans uploaded video and poster keys',
      (tester) async {
        final notifier = _FakeAuctionNotifier()
          ..publicationSucceeds = false
          ..failureMessage = 'Lelang gagal dipublikasikan.';
        final s3Service = _FakeS3Service();
        final repository = _FakeAuctionRepository();
        final navigatorObserver = _RecordingNavigatorObserver();
        final container = _container(
          authController: _FakeAuthController(
            AuthState.authenticated(
              _seller(
                id: 'seller-1',
                username: 'seller',
                hasSellerProfile: true,
                hasMarketAuthority: true,
                sellerTier: SellerTier.sellerElite,
              ),
              emailVerified: true,
            ),
          ),
          auctionNotifier: notifier,
          auctionRepository: repository,
          s3Service: s3Service,
        );
        addTearDown(container.dispose);

        await tester.pumpWidget(
          _wrap(
            container: container,
            child: CreateAuctionScreen(
              mediaPicker: _fakeVideoMediaPicker,
              mediaUploader: _fakeVideoMediaUploader,
            ),
            navigatorObservers: [navigatorObserver],
          ),
        );
        await tester.pumpAndSettle();
        final activeSubscription = container.listen(
          exploreAuctionsStreamProvider,
          (previous, next) {},
          fireImmediately: false,
        );
        final userSubscription = container.listen(
          userAuctionsStreamProvider('seller-1'),
          (previous, next) {},
          fireImmediately: false,
        );
        addTearDown(activeSubscription.close);
        addTearDown(userSubscription.close);
        await container.read(exploreAuctionsStreamProvider.future);
        await container.read(userAuctionsStreamProvider('seller-1').future);
        await tester.pumpAndSettle();
        final baselineActiveCalls = repository.watchActiveCalls;
        final baselineUserCalls = repository.watchUserCalls;
        await _fillValidForm(tester);

        final submitButton = find
            .byType(ElevatedButton, skipOffstage: false)
            .last;
        await tester.ensureVisible(submitButton);
        await tester.pumpAndSettle();
        await tester.tap(submitButton);
        await tester.pumpAndSettle();

        expect(notifier.createCalls, 1);
        expect(find.text('Lelang gagal dipublikasikan.'), findsOneWidget);
        expect(find.text('Lelang berhasil dibuat.'), findsNothing);
        expect(
          s3Service.deletedKeys,
          containsAll([
            'videos/auction-failure-0.mp4',
            'videos/auction-failure-0.mp4_poster.jpg',
          ]),
        );
        expect(repository.watchActiveCalls, baselineActiveCalls);
        expect(repository.watchUserCalls, baselineUserCalls);
        expect(navigatorObserver.popCount, 0);
      },
    );

    testWidgets(
      'active seller switching to buyer B before submit blocks form',
      (tester) async {
        final controller = _FakeAuthController(
          AuthState.authenticated(
            _seller(
              id: 'seller-a',
              username: 'seller-a',
              hasSellerProfile: true,
              hasMarketAuthority: true,
            ),
            emailVerified: true,
          ),
        );
        final notifier = _FakeAuctionNotifier();
        final container = _container(
          authController: controller,
          auctionNotifier: notifier,
        );
        addTearDown(container.dispose);

        await tester.pumpWidget(_wrap(container: container));
        await tester.pumpAndSettle();
        expect(find.text('Informasi Dasar'), findsOneWidget);

        controller.setAuthState(
          AuthState.authenticated(
            _seller(
              id: 'buyer-b',
              username: 'buyer-b',
              hasSellerProfile: false,
              hasMarketAuthority: false,
            ),
            emailVerified: true,
          ),
        );
        await tester.pumpAndSettle();

        expect(find.text('Jadi Seller Dulu'), findsOneWidget);
        expect(find.text('Informasi Dasar'), findsNothing);
        expect(notifier.createCalls, 0);
      },
    );

    testWidgets(
      'active seller switching to expired seller before submit blocks form',
      (tester) async {
        final controller = _FakeAuthController(
          AuthState.authenticated(
            _seller(
              id: 'seller-a',
              username: 'seller-a',
              hasSellerProfile: true,
              hasMarketAuthority: true,
            ),
            emailVerified: true,
          ),
        );
        final notifier = _FakeAuctionNotifier();
        final container = _container(
          authController: controller,
          auctionNotifier: notifier,
        );
        addTearDown(container.dispose);

        await tester.pumpWidget(_wrap(container: container));
        await tester.pumpAndSettle();
        expect(find.text('Informasi Dasar'), findsOneWidget);

        controller.setAuthState(
          AuthState.authenticated(
            _seller(
              id: 'seller-b',
              username: 'seller-b',
              hasSellerProfile: true,
              hasMarketAuthority: false,
            ),
            emailVerified: true,
          ),
        );
        await tester.pumpAndSettle();

        expect(find.text('Langganan Seller Habis'), findsOneWidget);
        expect(find.text('Informasi Dasar'), findsNothing);
        expect(notifier.createCalls, 0);
      },
    );

    testWidgets('authority loss while the form is open fails closed', (
      tester,
    ) async {
      final controller = _FakeAuthController(
        AuthState.authenticated(
          _seller(
            id: 'seller-a',
            username: 'seller-a',
            hasSellerProfile: true,
            hasMarketAuthority: true,
          ),
          emailVerified: true,
        ),
      );
      final notifier = _FakeAuctionNotifier();
      final container = _container(
        authController: controller,
        auctionNotifier: notifier,
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));
      await tester.pumpAndSettle();
      await _enterFieldByLabel(tester, 'Judul *', 'Kohaku 50cm');

      controller.setAuthState(
        AuthState.authenticated(
          _seller(
            id: 'seller-a',
            username: 'seller-a',
            hasSellerProfile: true,
            hasMarketAuthority: false,
          ),
          emailVerified: true,
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Langganan Seller Habis'), findsOneWidget);
      expect(notifier.createCalls, 0);
    });

    testWidgets('hydration becoming null fails closed', (tester) async {
      final controller = _FakeAuthController(
        AuthState.authenticated(
          _seller(
            id: 'seller-a',
            username: 'seller-a',
            hasSellerProfile: true,
            hasMarketAuthority: true,
          ),
          emailVerified: true,
        ),
      );
      final notifier = _FakeAuctionNotifier();
      final container = _container(
        authController: controller,
        auctionNotifier: notifier,
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));
      await tester.pumpAndSettle();
      expect(find.text('Informasi Dasar'), findsOneWidget);

      controller.setAuthState(const AuthState.unauthenticated());
      await tester.pumpAndSettle();

      expect(find.text('Silakan login untuk melanjutkan.'), findsOneWidget);
      expect(find.byType(TextFormField), findsNothing);
      expect(notifier.createCalls, 0);
    });

    testWidgets('KYC and tier do not change create permission', (tester) async {
      for (final tier in [
        SellerTier.sellerBasic,
        SellerTier.sellerPro,
        SellerTier.sellerElite,
      ]) {
        final container = _container(
          authController: _FakeAuthController(
            AuthState.authenticated(
              _seller(
                id: 'seller-${tier.name}',
                username: 'seller-${tier.name}',
                hasSellerProfile: true,
                hasMarketAuthority: true,
                isIdVerified: false,
                isFarmVerified: false,
                sellerTier: tier,
              ),
              emailVerified: true,
            ),
          ),
          auctionNotifier: _FakeAuctionNotifier(),
        );
        addTearDown(container.dispose);

        await tester.pumpWidget(_wrap(container: container));
        await tester.pumpAndSettle();

        expect(find.text('Informasi Dasar'), findsOneWidget);
        expect(find.byType(TextFormField), findsWidgets);
      }
    });

    testWidgets('duplicate rapid submit blocks second invocation', (
      tester,
    ) async {
      _FakeAuctionNotifier.globalCreateCalls = 0;
      final notifier = _FakeAuctionNotifier();
      notifier.pendingCreate = Completer<bool>();
      final repository = _FakeAuctionRepository();
      final navigatorObserver = _RecordingNavigatorObserver();
      final container = _container(
        authController: _FakeAuthController(
          AuthState.authenticated(
            _seller(
              id: 'seller-1',
              username: 'seller',
              hasSellerProfile: true,
              hasMarketAuthority: true,
              sellerTier: SellerTier.sellerElite,
            ),
            emailVerified: true,
          ),
        ),
        auctionNotifier: notifier,
        auctionRepository: repository,
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(
        _wrap(
          container: container,
          child: const CreateAuctionScreen(
            mediaPicker: _fakeLocalMediaPicker,
            mediaUploader: _fakeMediaUploader,
          ),
          navigatorObservers: [navigatorObserver],
        ),
      );
      await tester.pumpAndSettle();
      final activeSubscription = container.listen(
        exploreAuctionsStreamProvider,
        (previous, next) {},
        fireImmediately: false,
      );
      final userSubscription = container.listen(
        userAuctionsStreamProvider('seller-1'),
        (previous, next) {},
        fireImmediately: false,
      );
      addTearDown(activeSubscription.close);
      addTearDown(userSubscription.close);
      await container.read(exploreAuctionsStreamProvider.future);
      await container.read(userAuctionsStreamProvider('seller-1').future);
      await tester.pumpAndSettle();
      final baselineActiveCalls = repository.watchActiveCalls;
      final baselineUserCalls = repository.watchUserCalls;
      await _fillValidForm(tester);

      // Stage 1: before first tap — button enabled, 0 calls
      final submitButton = find
          .byType(ElevatedButton, skipOffstage: false)
          .last;
      await tester.ensureVisible(submitButton);
      await tester.pumpAndSettle();
      var btn = tester.widget<ElevatedButton>(submitButton);
      expect(
        btn.onPressed,
        isNotNull,
        reason: 'Button should be enabled before first tap',
      );

      // Stage 2: first tap — button becomes disabled, 1 call
      await tester.tap(submitButton);
      await tester.pump();
      expect(
        notifier.createCalls,
        1,
        reason: 'First tap invokes notifier once',
      );
      // Button should be disabled while future is pending (isSubmitting = true)
      btn = tester.widget<ElevatedButton>(submitButton);
      expect(
        btn.onPressed,
        isNull,
        reason: 'Button should be disabled while submitting',
      );

      // Stage 3: second tap attempt — still 1 call (button disabled, no-op)
      // Tap at the button's location; the null onPressed should make this a no-op
      await tester.tap(submitButton);
      await tester.pump();
      expect(
        notifier.createCalls,
        1,
        reason: 'Second tap must NOT invoke notifier again',
      );

      // Stage 4: complete the pending future
      notifier.pendingCreate!.complete(true);
      await tester.pump();
      expect(find.byType(SnackBar), findsOneWidget);
      await tester.pumpAndSettle();

      // After completion, the screen navigates away (pop), so the button may be gone
      // Verifikasi final: notifier tetap 1
      expect(notifier.createCalls, 1);
      expect(_FakeAuctionNotifier.globalCreateCalls, 1);
      expect(repository.watchActiveCalls, baselineActiveCalls + 1);
      expect(repository.watchUserCalls, baselineUserCalls + 1);
      expect(find.text('Lelang gagal dipublikasikan.'), findsNothing);
      expect(navigatorObserver.popCount, 1);
    });
  });

  group('CreateAuctionScreen discovery contract', () {
    test('opens the canonical address editor route on sender setup', () {
      final source = File(
        'lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart',
      ).readAsStringSync();

      expect(source, contains('RoutePaths.addresses'));
      expect(source, contains('extra: true'));
      expect(source, contains('primarySenderAddressProvider('));
      expect(source, contains('exploreAuctionsStreamProvider'));
      expect(source, contains('userAuctionsStreamProvider('));
    });
  });
}
