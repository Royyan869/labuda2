import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/core/src/auth/app_role.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/models/pending_commerce_attachment.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/chat/chat/presentation/screens/chat_detail_screen.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_input_area.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/seller_auctions_pager.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/shipping_quote_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/repositories/for_sale_repository.dart';
import 'package:labuda/domains/commerce/transaction/shipping/data/repositories/shipping_quote_repository.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/shared/models/wilayah_models.dart';
import 'package:labuda/shared/providers/wilayah_provider_simple.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/create_for_sale_route_contract.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_controller.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/seller_fps_pager.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_notifier.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_state.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';
import 'package:labuda/shared/providers/block_state_provider.dart';

const _chatId = '00000000-0000-0000-0000-000000001111';
const _currentUserId = '00000000-0000-0000-0000-000000002222';
const _otherUserId = '00000000-0000-0000-0000-000000003333';
const _fixedPriceSaleId = '00000000-0000-0000-0000-000000004444';
const _createdFixedPriceSaleId = '00000000-0000-0000-0000-00000000aaaa';
const _auctionId = '00000000-0000-0000-0000-000000005555';
const _productId = '00000000-0000-0000-0000-000000006666';

class _FakeAuthController extends AuthController {
  /// Seller is the DEFAULT here: this file exercises the direct-commerce attach
  /// capability, and "Lampirkan Produk" is a SELLER-only entry (owner decision).
  /// `seller: false` proves the entry is absent for a non-seller.
  _FakeAuthController({this.seller = true});

  final bool seller;

  @override
  AuthState build() {
    final now = DateTime.utc(2026, 7, 30, 8);
    final user = AuthUser(
      id: _currentUserId,
      createdAt: now,
      updatedAt: now,
      email: 'me@example.com',
      username: 'me',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: seller,
      hasMarketAuthority: seller,
      sellerSubscriptionStatus: seller ? 'active' : 'none',
      // Seller-ness is backend-derived `hasMarketAuthority`, NOT a role: there
      // is no UserRole.seller in the canonical vocabulary.
      roles: const [UserRole.user],
      provider: AuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );
    return AuthState.authenticated(user, emailVerified: true);
  }
}

class _FakeNegotiationNotifier extends NegotiationNotifier {
  @override
  NegotiationState build() => const NegotiationState();
}

class _FakeSellerFPSPagerController extends SellerFPSPagerController {
  @override
  SellerFPSPagerState build() {
    return SellerFPSPagerState(
      items: [_fixedPriceForSale()],
      hasMore: false,
      isInitialLoading: false,
      isLoadingMore: false,
      initialError: null,
      loadMoreError: null,
      ownerId: _currentUserId,
      pageSize: 20,
    );
  }
}

class _FakeSellerAuctionsPagerController extends SellerAuctionsPagerController {
  @override
  SellerAuctionsPagerState build() {
    return SellerAuctionsPagerState(
      activeFilter: null,
      auctions: [_auction()],
      pageSize: 20,
      hasMore: false,
      isInitialLoading: false,
      isLoadMoreLoading: false,
      isRefreshing: false,
      initialError: null,
      loadMoreError: null,
      refreshError: null,
      ownerId: _currentUserId,
    );
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> retryInitial() async {}

  @override
  Future<void> loadMore() async {}

  @override
  Future<void> refresh() async {}
}

class _NoOpForSaleRepository implements ForSaleRepository {
  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _NoOpLogger implements ILoggerService {
  @override
  Future<Result<void>> debug(String message, {Map<String, dynamic>? extra}) {
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<void>> info(String message, {Map<String, dynamic>? extra}) {
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<void>> warning(String message, {Map<String, dynamic>? extra}) {
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) {
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) {
    return Future.value(Result.success(null));
  }





  @override
  Future<Result<void>> setLogLevel(LogLevel level) {
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<void>> clearLogs() {
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) {
    return Future.value(Result.success(const []));
  }

  @override
  Future<void> debugSync(String userId) async {}

  @override
  Future<void> debugSyncSuccess(String userId) async {}

  @override
  Future<void> debugSyncFailed(String userId, String? errorMessage) async {}

  @override
  Future<void> debugCallingGetCurrentUser() async {}

  @override
  Future<void> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async {}

  @override
  Future<void> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async {}

  @override
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  ) async {}

  @override
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  ) async {}

  @override
  Future<void> log(String message, {LogLevel level = LogLevel.debug}) async {}
}

class _FakeLookupForSaleController extends ForSaleController {
  _FakeLookupForSaleController()
    : super(repository: _NoOpForSaleRepository(), logger: _NoOpLogger());

  @override
  Future<Result<ForSale?>> getForSaleById(
    String forSaleId,
  ) async {
    return Result.success(
      _fixedPriceForSale(
        fixedPriceSaleId: forSaleId,
        title: 'Created ForSale $forSaleId',
      ),
    );
  }
}

/// Records the canonical create-quote request. Proves the product-bubble path
/// reaches the ONE Shipping-domain creation authority.
class _RecordingShippingQuoteRepository implements ShippingQuoteRepository {
  CreateShippingQuoteRequestDto? lastRequest;
  String? lastChatId;

  @override
  Future<ShippingQuoteResponseDto> createShippingQuote({
    required String chatId,
    required CreateShippingQuoteRequestDto request,
  }) async {
    lastChatId = chatId;
    lastRequest = request;
    return ShippingQuoteResponseDto(
      id: 'quote-1',
      chatId: chatId,
      productId: request.productId,
      sourceType: request.sourceType,
      sourceId: request.sourceId,
      sellerId: _currentUserId,
      buyerId: _otherUserId,
      cost: request.cost,
      status: 'ACTIVE',
      createdAt: DateTime.utc(2026, 7, 30).toIso8601String(),
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _FakeChatDetailNotifier extends ChatDetail {
  _FakeChatDetailNotifier({required this.initialState});

  final ChatDetailState initialState;
  int sendCalls = 0;
  Map<String, dynamic>? lastSendArgs;
  Completer<Message?>? sendCompleter;

  @override
  ChatDetailState build(String chatId) => initialState;

  // Load/read side-effects are no-ops so the seeded initial state (messages in
  // particular) is deterministic and never replaced by a transport read.
  @override
  Future<void> loadChat(String userId) async {}

  @override
  Future<void> loadMessages(String userId) async {}

  @override
  Future<void> markAsRead(String userId) async {}

  @override
  Future<Message?> sendMessage({
    required String senderId,
    required String senderName,
    required String content,
    MessageType type = MessageType.text,
    List<String> mediaUrls = const [],
    List<String> mediaAssetIds = const [],
    String? replyToId,
    String? replyToMessageId,
    List<String> mentionedUserIds = const [],
    String? idempotencyKey,
    ChatResourceOccurrenceRequest? resourceOccurrence,
  }) {
    sendCalls += 1;
    lastSendArgs = <String, dynamic>{
      'senderId': senderId,
      'senderName': senderName,
      'content': content,
      'type': type,
      'mediaAssetIds': mediaAssetIds,
      'replyToId': replyToId,
      'replyToMessageId': replyToMessageId,
      'idempotencyKey': idempotencyKey,
      'resourceOccurrence': resourceOccurrence,
    };
    sendCompleter ??= Completer<Message?>();
    return sendCompleter!.future;
  }
}

Chat _makeChat() => Chat(
  id: _chatId,
  participantIds: const [_currentUserId, _otherUserId],
  participantNames: const {_currentUserId: 'me', _otherUserId: 'other'},
  participantAvatars: const {},
  participantLifecycles: const {_otherUserId: ContentLifecycle.active},
  createdAt: DateTime.utc(2026, 7, 30),
  status: ChatStatus.active,
);

ForSale _fixedPriceForSale({
  String fixedPriceSaleId = _fixedPriceSaleId,
  String title = 'Koi Test FPS',
}) => ForSale(
  forSaleId: fixedPriceSaleId,
  productId: _productId,
  title: title,
  description: 'Test fixed-price sale',
  price: 500000,
  stock: 1,
  sellerId: _currentUserId,
  status: ForSaleStatus.active,
  createdAt: DateTime.utc(2026, 7, 29),
  updatedAt: DateTime.utc(2026, 7, 29),
);

Auction _auction() => Auction(
  id: _auctionId,
  sellerId: _currentUserId,
  title: 'Koi Test Auction',
  description: 'Test auction',
  koiDetails: const KoiDetails(
    variety: 'Kohaku',
    sizeInCm: 20,
    ageInMonths: 12,
    gender: 'Male',
  ),
  openingBid: 400000,
  currentBid: 450000,
  bidIncrement: 25000,
  startTime: DateTime.utc(2026, 7, 28),
  endTime: DateTime.utc(2026, 8, 12),
  status: AuctionStatus.active,
  createdAt: DateTime.utc(2026, 7, 28),
  productId: _productId,
);

/// A product bubble in the conversation: a message carrying the canonical LIVE
/// For Sale resource projection. [senderId] controls WHO sent it — the product
/// CONTEXT is ownership-independent. [canManage] is the SERVER-PROJECTED viewer
/// ownership capability (the canonical authorization the gate must read), NOT
/// derived from the sender.
Message _productBubble({
  required String senderId,
  bool canManage = true,
  String forSaleId = _fixedPriceSaleId,
  String title = 'Koi Test FPS',
}) {
  return Message(
    id: 'msg-product-$senderId',
    chatId: _chatId,
    senderId: senderId,
    senderName: senderId == _currentUserId ? 'me' : 'other',
    content: '',
    type: MessageType.text,
    createdAt: DateTime.utc(2026, 7, 30, 9),
    resourceProjection: LiveResourceProjection(
      state: ResourceProjectionState.live,
      resourceType: ResourceProjectionType.fixedPriceSale,
      viewerCapabilities: ResourceViewerCapabilities.live(
        canInteract: true,
        canManage: canManage,
      ),
      resourceId: forSaleId,
      canonicalUrl: '/for-sale/$forSaleId',
      payload: ForSaleLivePayload(
        title: title,
        media: const [],
        price: const LivePrice(amount: 500000, currency: 'IDR'),
        status: 'active',
        quantityAvailable: 1,
        seller: const ResourceSellerCard(
          user: ResourceUserCard(id: _currentUserId, username: 'me'),
        ),
      ),
    ),
  );
}

String _composerText(WidgetTester tester) {
  final field = tester.widget<TextField>(find.byType(TextField));
  return field.controller?.text ?? '';
}

Finder _composerSendButton() {
  return find.descendant(
    of: find.byType(ChatInputArea),
    matching: find.byIcon(Icons.send),
  );
}

Future<void> _selectExistingFixedPriceSale(WidgetTester tester) async {
  await tester.tap(find.byIcon(Icons.add_circle));
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));
  await tester.tap(find.text('Lampirkan Produk'));
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));
  await tester.tap(find.text('Koi Test FPS'));
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));
}

ProviderScope _buildScope({
  required Widget child,
  ChatDetailState? initialState,
  _FakeChatDetailNotifier? notifier,
  bool seller = true,
}) {
  final chatNotifier =
      notifier ??
      _FakeChatDetailNotifier(
        initialState:
            initialState ??
            ChatDetailState(chat: _makeChat(), messages: const []),
      );

  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(seller: seller),
      ),
      currentUserIdProvider.overrideWith((ref) => _currentUserId),
      isUserBlockedProvider(_otherUserId).overrideWith((ref) => false),
      negotiationNotifierProvider.overrideWith(_FakeNegotiationNotifier.new),
      chatDetailProvider(_chatId).overrideWith(() => chatNotifier),
      sellerFPSPagerProvider.overrideWith(
        () => _FakeSellerFPSPagerController(),
      ),
      sellerAuctionsPagerProvider.overrideWith(
        () => _FakeSellerAuctionsPagerController(),
      ),
    ],
    child: MaterialApp(home: child),
  );
}

/// A product bubble renders a tall marketplace card; give the test surface
/// enough height so the card's `⋮` affordance is on-screen and hittable.
void _useTallSurface(WidgetTester tester) {
  tester.view.physicalSize = const Size(800, 1600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}

ProviderScope _buildShippingScope({
  bool seller = true,
  List<Message> messages = const [],
  ShippingQuoteRepository? shippingQuoteRepository,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(seller: seller),
      ),
      // Destination lock data for the canonical shipping quote form.
      provincesProvider.overrideWith(
        (ref) async => const [Province(id: '31', name: 'DKI Jakarta')],
      ),
      citiesProvider.overrideWith(
        (ref, provinceId) async =>
            const [City(id: '3171', name: 'Jakarta Selatan', provinceId: '31')],
      ),
      if (shippingQuoteRepository != null)
        shippingQuoteRepositoryProvider.overrideWithValue(
          shippingQuoteRepository,
        ),
      currentUserIdProvider.overrideWith((ref) => _currentUserId),
      isUserBlockedProvider(_otherUserId).overrideWith((ref) => false),
      negotiationNotifierProvider.overrideWith(_FakeNegotiationNotifier.new),
      chatDetailProvider(_chatId).overrideWith(
        () => _FakeChatDetailNotifier(
          initialState: ChatDetailState(chat: _makeChat(), messages: messages),
        ),
      ),
      sellerFPSPagerProvider.overrideWith(() => _FakeSellerFPSPagerController()),
      sellerAuctionsPagerProvider.overrideWith(
        () => _FakeSellerAuctionsPagerController(),
      ),
      // Canonical Commerce detail resolution (physical product id) needed by
      // the seller shipping-quote intent.
      forSaleControllerProvider.overrideWithValue(
        _FakeLookupForSaleController(),
      ),
    ],
    child: const MaterialApp(home: ChatDetailScreen(chatId: _chatId)),
  );
}

Widget _buildChatCommerceScope({  required Widget createForSaleRoute,
  required _FakeChatDetailNotifier chatDetailNotifier,
  ValueChanged<Object?>? onCreateForSaleRouteExtra,
}) {
  final router = GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(
        path: '/',
        pageBuilder: (context, state) => MaterialPage(
          key: state.pageKey,
          child: const ChatDetailScreen(chatId: _chatId),
        ),
      ),
      GoRoute(
        path: RoutePaths.createForSale,
        name: RouteNames.createForSale,
        pageBuilder: (context, state) {
          onCreateForSaleRouteExtra?.call(state.extra);
          return MaterialPage(key: state.pageKey, child: createForSaleRoute);
        },
      ),
    ],
  );

  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(_FakeAuthController.new),
      currentUserIdProvider.overrideWith((ref) => _currentUserId),
      isUserBlockedProvider(_otherUserId).overrideWith((ref) => false),
      negotiationNotifierProvider.overrideWith(_FakeNegotiationNotifier.new),
      chatDetailProvider(_chatId).overrideWith(() => chatDetailNotifier),
      sellerFPSPagerProvider.overrideWith(
        () => _FakeSellerFPSPagerController(),
      ),
      sellerAuctionsPagerProvider.overrideWith(
        () => _FakeSellerAuctionsPagerController(),
      ),
      forSaleControllerProvider.overrideWithValue(
        _FakeLookupForSaleController(),
      ),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

Future<void> _openChatCreateForSaleRoute(WidgetTester tester) async {
  await tester.tap(find.byIcon(Icons.add_circle));
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));
  await tester.tap(find.text('Lampirkan Produk'));
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));

  expect(find.text('Pilih Produk'), findsOneWidget);
  await tester.tap(find.text('Buat Produk Baru'));
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 1200));
}

class _ChatCreateForSaleRoute extends StatelessWidget {
  final Object? successResult;

  const _ChatCreateForSaleRoute({this.successResult});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ElevatedButton(
              onPressed: () => Navigator.of(context).pop(),
              child: const Text('Cancel'),
            ),
            const SizedBox(height: 12),
            ElevatedButton(
              onPressed: () => Navigator.of(context).pop(successResult),
              child: const Text('Create'),
            ),
          ],
        ),
      ),
    );
  }
}

void main() {
  setUpAll(() async {
    await initializeDateFormatting();
  });

  testWidgets('non-seller is never offered the commerce entry', (
    tester,
  ) async {
    await tester.pumpWidget(
      _buildScope(
        seller: false,
        child: const ChatDetailScreen(chatId: _chatId),
      ),
    );
    await tester.pump(const Duration(milliseconds: 300));

    await tester.tap(find.byIcon(Icons.add_circle));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.text('Galeri'), findsOneWidget);
    expect(find.text('Kamera'), findsOneWidget);
    expect(find.text('Lampirkan Produk'), findsNothing);
  });

  testWidgets('attachment sheet exposes galeri, kamera and commerce entry', (
    tester,
  ) async {
    await tester.pumpWidget(
      _buildScope(child: const ChatDetailScreen(chatId: _chatId)),
    );
    await tester.pump(const Duration(milliseconds: 300));

    await tester.tap(find.byIcon(Icons.add_circle));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    // ONE attachment sheet: Galeri carries foto AND video (one system picker),
    // so the old fake 'Foto' / 'Video' split is gone, and the commerce entry is
    // a seller-only capability.
    expect(find.text('Galeri'), findsOneWidget);
    expect(find.text('Kamera'), findsOneWidget);
    expect(find.text('Foto'), findsNothing);
    expect(find.text('Video'), findsNothing);
    expect(find.text('Lampirkan Produk'), findsOneWidget);
    expect(find.text('Bagikan ForSale'), findsNothing);
    // The generic attachment sheet NO LONGER carries the shipping entry: the
    // product bubble is the entry context (owner decision).
    expect(find.text('Penawaran Ongkir'), findsNothing);

    await tester.tap(find.text('Lampirkan Produk'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.text('Pilih Produk'), findsOneWidget);
    expect(find.text('Buat Produk Baru'), findsOneWidget);
  });

  /// MATRIX A — the product OWNER (seller) views their own product bubble that
  /// the seller sent. The server-projected `can_manage=true` authorizes it.
  testWidgets(
    'matrix A: owner sees Penawaran Ongkir on own bubble (seller-sent)',
    (tester) async {
      _useTallSurface(tester);
      await tester.pumpWidget(
        _buildShippingScope(
          messages: [
            _productBubble(senderId: _currentUserId, canManage: true),
          ],
        ),
      );
      await tester.pump(const Duration(milliseconds: 300));

      final more = find.byTooltip('Opsi produk');
      expect(more, findsOneWidget);
      await tester.tap(more);
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      expect(find.text('Penawaran Ongkir'), findsOneWidget);
    },
  );

  /// MATRIX B — the product OWNER (seller) views their own product bubble that
  /// the BUYER sent. The projected viewer capability is unchanged by the
  /// sender, so the owner still gets the action.
  testWidgets(
    'matrix B: owner sees Penawaran Ongkir on own bubble (buyer-sent)',
    (tester) async {
      _useTallSurface(tester);
      await tester.pumpWidget(
        _buildShippingScope(
          messages: [
            _productBubble(senderId: _otherUserId, canManage: true),
          ],
        ),
      );
      await tester.pump(const Duration(milliseconds: 300));

      final more = find.byTooltip('Opsi produk');
      expect(more, findsOneWidget);
      await tester.tap(more);
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      expect(find.text('Penawaran Ongkir'), findsOneWidget);
    },
  );

  /// End-to-end through the ONE canonical creation authority: the product
  /// bubble action opens the form, the form submits, and the Shipping repository
  /// receives the canonical request assembled for THIS bubble's product.
  testWidgets(
    'product bubble action submits the canonical shipping-quote request',
    (tester) async {
      _useTallSurface(tester);
      final repo = _RecordingShippingQuoteRepository();
      await tester.pumpWidget(
        _buildShippingScope(
          messages: [_productBubble(senderId: _otherUserId)],
          shippingQuoteRepository: repo,
        ),
      );
      await tester.pump(const Duration(milliseconds: 300));

      await tester.tap(find.byTooltip('Opsi produk'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      await tester.tap(find.text('Penawaran Ongkir'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 600));

      expect(find.text('Kirim Penawaran Ongkir'), findsOneWidget);

      // The cost field is the sheet's first AppTextField (a TextFormField);
      // the chat composer uses a plain TextField, so it is not matched here.
      final costField = find.byType(TextFormField).first;
      await tester.enterText(costField, '25000');
      await tester.pump();

      await tester.tap(find.byType(DropdownButtonFormField<Province>));
      await tester.pumpAndSettle();
      await tester.tap(find.text('DKI Jakarta').last);
      await tester.pumpAndSettle();

      await tester.tap(find.byType(DropdownButtonFormField<City>));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Jakarta Selatan').last);
      await tester.pumpAndSettle();

      await tester.tap(find.text('Kirim ke Pembeli'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));

      final request = repo.lastRequest;
      expect(request, isNotNull);
      expect(repo.lastChatId, _chatId);
      // The bubble's identity drives the request — no listing picker.
      expect(request!.productId, _productId);
      expect(request.sourceType, 'for_sale');
      expect(request.sourceId, _fixedPriceSaleId);
      expect(request.cost, 25000);
      expect(request.destinationCityId, '3171');
      expect(request.destinationProvinceId, '31');
    },
  );

  /// MATRIX C/D/E — a NON-OWNER viewer never sees the action, regardless of
  /// who sent the bubble AND regardless of the viewer's own platform role
  /// (a seller-role viewer looking at a THIRD PARTY's product is a buyer for
  /// that product). The projected `can_manage=false` is the sole gate.
  testWidgets(
    'matrix C/D/E: non-owner sees no product action (any sender, seller-role viewer)',
    (tester) async {
      for (final sender in [_currentUserId, _otherUserId]) {
        await tester.pumpWidget(
          _buildShippingScope(
            // Even a seller-role viewer gets nothing: role is not ownership.
            seller: true,
            messages: [_productBubble(senderId: sender, canManage: false)],
          ),
        );
        await tester.pump(const Duration(milliseconds: 300));
        expect(
          find.byTooltip('Opsi produk'),
          findsNothing,
          reason: 'non-owner must not see the action (sender=$sender)',
        );
      }
    },
  );

  testWidgets('generic attachment sheet carries no shipping entry', (
    tester,
  ) async {
    await tester.pumpWidget(
      _buildShippingScope(
        messages: [_productBubble(senderId: _otherUserId, canManage: false)],
      ),
    );
    await tester.pump(const Duration(milliseconds: 300));

    await tester.tap(find.byIcon(Icons.add_circle));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.text('Galeri'), findsOneWidget);
    // Even a seller-role viewer's attachment sheet has NO shipping entry: the
    // generic bottom-sheet producer is purged to root.
    expect(find.text('Lampirkan Produk'), findsOneWidget);
    expect(find.text('Penawaran Ongkir'), findsNothing);
  });

  group('viewerCanOfferShippingQuote — canonical projected authority', () {
    test('true only when the projection grants can_manage (owner)', () {
      expect(
        viewerCanOfferShippingQuote(
          _productBubble(senderId: _otherUserId, canManage: true),
        ),
        isTrue,
      );
    });

    test('false for a non-owner regardless of bubble sender', () {
      for (final sender in [_currentUserId, _otherUserId]) {
        expect(
          viewerCanOfferShippingQuote(
            _productBubble(senderId: sender, canManage: false),
          ),
          isFalse,
          reason: 'sender=$sender must not influence authorization',
        );
      }
    });

    test('false when the message carries no LIVE projection (fail-closed)', () {
      expect(
        viewerCanOfferShippingQuote(
          Message(
            id: 'm-plain',
            chatId: _chatId,
            senderId: _otherUserId,
            senderName: 'other',
            content: 'halo',
            createdAt: DateTime.utc(2026, 7, 30),
          ),
        ),
        isFalse,
      );
    });
  });

  testWidgets('canceling commerce picker preserves draft and sends nothing', (
    tester,
  ) async {
    final notifier = _FakeChatDetailNotifier(
      initialState: ChatDetailState(chat: _makeChat(), messages: const []),
    );

    await tester.pumpWidget(
      _buildScope(
        notifier: notifier,
        child: const ChatDetailScreen(chatId: _chatId),
      ),
    );
    await tester.pump(const Duration(milliseconds: 300));

    await tester.enterText(find.byType(TextField), 'draft before picker');
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));
    expect(_composerText(tester), 'draft before picker');

    await tester.tap(find.byIcon(Icons.add_circle));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));
    await tester.tap(find.text('Lampirkan Produk'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.text('Pilih Produk'), findsOneWidget);
    await tester.tapAt(const Offset(8, 8));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(_composerText(tester), 'draft before picker');
    expect(find.text('Lampiran produk'), findsNothing);
    expect(notifier.sendCalls, 0);
  });

  testWidgets(
    'fixed-price selection stays pending until Send, and remove keeps draft',
    (tester) async {
      final notifier = _FakeChatDetailNotifier(
        initialState: ChatDetailState(chat: _makeChat(), messages: const []),
      );

      await tester.pumpWidget(
        _buildScope(
          notifier: notifier,
          child: const ChatDetailScreen(chatId: _chatId),
        ),
      );
      await tester.pump(const Duration(milliseconds: 300));

      await tester.enterText(find.byType(TextField), 'fixed-price draft');
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 50));
      expect(_composerText(tester), 'fixed-price draft');

      await tester.tap(find.byIcon(Icons.add_circle));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      await tester.tap(find.text('Lampirkan Produk'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      await tester.tap(find.text('Koi Test FPS'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));

      expect(find.text('Lampiran produk'), findsOneWidget);
      expect(find.text('Koi Test FPS'), findsOneWidget);
      expect(_composerText(tester), 'fixed-price draft');
      expect(notifier.sendCalls, 0);

      await tester.tap(find.byTooltip('Hapus lampiran'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 50));

      expect(find.text('Lampiran produk'), findsNothing);
      expect(_composerText(tester), 'fixed-price draft');

      await tester.tap(_composerSendButton());
      await tester.pump();

      expect(notifier.sendCalls, 1);
      expect(notifier.lastSendArgs?['content'], 'fixed-price draft');
      expect(notifier.lastSendArgs?['resourceOccurrence'], isNull);

      notifier.sendCompleter?.complete(
        Message(
          id: 'sent-fixed-1',
          chatId: _chatId,
          senderId: _currentUserId,
          senderName: 'me',
          content: 'fixed-price draft',
          createdAt: DateTime.utc(2026, 7, 30, 10, 0),
          status: MessageStatus.sent,
          mentionedUserIds: const [],
          deletedBy: const [],
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 200));
      expect(_composerText(tester), isEmpty);
    },
  );

  testWidgets(
    'create for-sale cancel preserves existing pending selection and draft',
    (tester) async {
      final notifier = _FakeChatDetailNotifier(
        initialState: ChatDetailState(chat: _makeChat(), messages: const []),
      );

      await tester.pumpWidget(
        _buildChatCommerceScope(
          chatDetailNotifier: notifier,
          createForSaleRoute: const _ChatCreateForSaleRoute(
            successResult: CreatedForSaleResult(
              forSaleId: _createdFixedPriceSaleId,
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 300));

      await tester.enterText(find.byType(TextField), 'draft before create');
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 50));
      expect(_composerText(tester), 'draft before create');

      await _selectExistingFixedPriceSale(tester);
      expect(find.text('Lampiran produk'), findsOneWidget);
      expect(find.text('Koi Test FPS'), findsOneWidget);

      await tester.tap(find.byIcon(Icons.add_circle));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      await tester.tap(find.text('Lampirkan Produk'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));

      expect(find.text('Pilih Produk'), findsOneWidget);
      await tester.tap(find.text('Buat Produk Baru'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 1200));

      expect(find.text('Cancel'), findsOneWidget);
      await tester.tap(find.text('Cancel'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 600));

      expect(_composerText(tester), 'draft before create');
      expect(find.text('Lampiran produk'), findsOneWidget);
      expect(find.text('Koi Test FPS'), findsOneWidget);
      expect(notifier.sendCalls, 0);
    },
  );

  testWidgets(
    'create for-sale success replaces pending selection and stays unsent',
    (tester) async {
      final notifier = _FakeChatDetailNotifier(
        initialState: ChatDetailState(chat: _makeChat(), messages: const []),
      );

      await tester.pumpWidget(
        _buildChatCommerceScope(
          chatDetailNotifier: notifier,
          createForSaleRoute: const _ChatCreateForSaleRoute(
            successResult: CreatedForSaleResult(
              forSaleId: _createdFixedPriceSaleId,
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 300));

      await tester.enterText(find.byType(TextField), 'draft before create');
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 50));
      expect(_composerText(tester), 'draft before create');

      await _selectExistingFixedPriceSale(tester);
      expect(find.text('Lampiran produk'), findsOneWidget);
      expect(find.text('Koi Test FPS'), findsOneWidget);

      await tester.tap(find.byIcon(Icons.add_circle));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      await tester.tap(find.text('Lampirkan Produk'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));

      expect(find.text('Pilih Produk'), findsOneWidget);
      await tester.tap(find.text('Buat Produk Baru'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 1200));

      expect(find.text('Create'), findsOneWidget);
      await tester.tap(find.text('Create'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 1200));

      expect(_composerText(tester), 'draft before create');
      expect(find.text('Koi Test FPS'), findsNothing);
      expect(
        find.text('Created ForSale $_createdFixedPriceSaleId'),
        findsOneWidget,
      );
      expect(notifier.sendCalls, 0);

      await tester.tap(_composerSendButton());
      await tester.pump();

      expect(notifier.sendCalls, 1);
      final occurrence =
          notifier.lastSendArgs?['resourceOccurrence']
              as ChatResourceOccurrenceRequest?;
      expect(occurrence, isNotNull);
      expect(
        occurrence!.operation,
        ChatResourceOccurrenceOperation.directCommerceInsertChat,
      );
      expect(
        occurrence.resourceType,
        ChatResourceOccurrenceResourceType.forSale,
      );
      expect(occurrence.resourceId, _createdFixedPriceSaleId);
      expect(notifier.lastSendArgs?['content'], 'draft before create');
    },
  );

  testWidgets(
    'auction resource-only send uses direct_commerce_insert_chat on Send',
    (tester) async {
      final notifier = _FakeChatDetailNotifier(
        initialState: ChatDetailState(chat: _makeChat(), messages: const []),
      );

      await tester.pumpWidget(
        _buildScope(
          notifier: notifier,
          child: const ChatDetailScreen(chatId: _chatId),
        ),
      );
      await tester.pump(const Duration(milliseconds: 300));

      await tester.tap(find.byIcon(Icons.add_circle));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      await tester.tap(find.text('Lampirkan Produk'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      await tester.tap(find.text('Lelang'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      await tester.tap(find.text('Koi Test Auction'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));

      expect(find.text('Lampiran produk'), findsOneWidget);
      expect(_composerSendButton(), findsOneWidget);
      expect(notifier.sendCalls, 0);

      await tester.tap(_composerSendButton());
      await tester.pump();

      expect(notifier.sendCalls, 1);

      final occurrence =
          notifier.lastSendArgs?['resourceOccurrence']
              as ChatResourceOccurrenceRequest?;
      expect(occurrence, isNotNull);
      expect(
        occurrence!.operation,
        ChatResourceOccurrenceOperation.directCommerceInsertChat,
      );
      expect(
        occurrence.resourceType,
        ChatResourceOccurrenceResourceType.auction,
      );
      expect(occurrence.resourceId, _auctionId);
      expect(notifier.lastSendArgs?['content'], isEmpty);

      notifier.sendCompleter?.complete(
        Message(
          id: 'sent-auction-1',
          chatId: _chatId,
          senderId: _currentUserId,
          senderName: 'me',
          content: '',
          createdAt: DateTime.utc(2026, 7, 30, 10, 0),
          status: MessageStatus.sent,
          mentionedUserIds: const [],
          deletedBy: const [],
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 200));
    },
  );

  testWidgets('rapid send only invokes notifier once', (tester) async {
    final notifier = _FakeChatDetailNotifier(
      initialState: ChatDetailState(chat: _makeChat(), messages: const []),
    );

    await tester.pumpWidget(
      _buildScope(
        notifier: notifier,
        child: const ChatDetailScreen(chatId: _chatId),
      ),
    );
    await tester.pump(const Duration(milliseconds: 300));

    await tester.enterText(find.byType(TextField), 'hello chat');
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));
    expect(_composerText(tester), 'hello chat');

    await tester.tap(_composerSendButton());
    await tester.tap(_composerSendButton());
    await tester.pump();

    expect(notifier.sendCalls, 1);
    expect(notifier.lastSendArgs?['content'], 'hello chat');

    notifier.sendCompleter?.complete(
      Message(
        id: 'sent-1',
        chatId: _chatId,
        senderId: _currentUserId,
        senderName: 'me',
        content: 'hello chat',
        createdAt: DateTime.utc(2026, 7, 30, 10, 0),
        status: MessageStatus.sent,
        mentionedUserIds: const [],
        deletedBy: const [],
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));
  });

  testWidgets(
    'chat create for-sale route uses explicit fixed-price-sale return mode',
    (tester) async {
      final notifier = _FakeChatDetailNotifier(
        initialState: ChatDetailState(chat: _makeChat(), messages: const []),
      );
      Object? capturedExtra;

      await tester.pumpWidget(
        _buildChatCommerceScope(
          chatDetailNotifier: notifier,
          onCreateForSaleRouteExtra: (extra) => capturedExtra = extra,
          createForSaleRoute: const _ChatCreateForSaleRoute(),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 300));

      await _openChatCreateForSaleRoute(tester);

      expect(capturedExtra, isA<CreateForSaleRouteArgs>());
      final args = capturedExtra! as CreateForSaleRouteArgs;
      expect(args.returnMode, CreateForSaleReturnMode.forSaleId);
    },
  );

  testWidgets('chat create for-sale null result attaches nothing', (
    tester,
  ) async {
    final notifier = _FakeChatDetailNotifier(
      initialState: ChatDetailState(chat: _makeChat(), messages: const []),
    );

    await tester.pumpWidget(
      _buildChatCommerceScope(
        chatDetailNotifier: notifier,
        createForSaleRoute: const _ChatCreateForSaleRoute(successResult: null),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    await _openChatCreateForSaleRoute(tester);
    await tester.tap(find.text('Create'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 600));

    expect(find.text('Lampiran produk'), findsNothing);
    expect(find.textContaining('Created ForSale'), findsNothing);
    expect(notifier.sendCalls, 0);
  });

  testWidgets('chat create for-sale raw ForSale result attaches nothing', (
    tester,
  ) async {
    final notifier = _FakeChatDetailNotifier(
      initialState: ChatDetailState(chat: _makeChat(), messages: const []),
    );

    await tester.pumpWidget(
      _buildChatCommerceScope(
        chatDetailNotifier: notifier,
        createForSaleRoute: _ChatCreateForSaleRoute(
          successResult: ForSale(
            forSaleId: _createdFixedPriceSaleId,
            productId: _productId,
            title: 'Legacy ForSale Result',
            description: 'Should not attach in Chat',
            price: 500000,
            stock: 1,
            sellerId: _currentUserId,
            status: ForSaleStatus.active,
            createdAt: DateTime.utc(2026, 7, 30),
            updatedAt: DateTime.utc(2026, 7, 30),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    await _openChatCreateForSaleRoute(tester);
    await tester.tap(find.text('Create'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 600));

    expect(find.text('Lampiran produk'), findsNothing);
    expect(find.textContaining('Created ForSale'), findsNothing);
    expect(notifier.sendCalls, 0);
  });

  testWidgets('chat create for-sale arbitrary object result attaches nothing', (
    tester,
  ) async {
    final notifier = _FakeChatDetailNotifier(
      initialState: ChatDetailState(chat: _makeChat(), messages: const []),
    );

    await tester.pumpWidget(
      _buildChatCommerceScope(
        chatDetailNotifier: notifier,
        createForSaleRoute: const _ChatCreateForSaleRoute(
          successResult: {'fixedPriceSaleId': _createdFixedPriceSaleId},
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    await _openChatCreateForSaleRoute(tester);
    await tester.tap(find.text('Create'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 600));

    expect(find.text('Lampiran produk'), findsNothing);
    expect(find.textContaining('Created ForSale'), findsNothing);
    expect(notifier.sendCalls, 0);
  });

  testWidgets(
    'For Sale detail entry seeds pending attachment: no Kirim CTA, send icon '
    'authority, empty body sends resourceOccurrence',
    (tester) async {
      final notifier = _FakeChatDetailNotifier(
        initialState: ChatDetailState(chat: _makeChat(), messages: const []),
      );

      await tester.pumpWidget(
        _buildScope(
          notifier: notifier,
          child: ChatDetailScreen(
            chatId: _chatId,
            pendingCommerce: PendingCommerceAttachment.forSale(
              forSaleId: _fixedPriceSaleId,
              title: 'Koi Test FPS',
            ),
          ),
        ),
      );
      await tester.pump(const Duration(milliseconds: 300));

      // Pending card is preview + remove only — the forbidden text send CTA is
      // dead.
      expect(find.text('Lampiran produk'), findsOneWidget);
      expect(find.text('Koi Test FPS'), findsOneWidget);
      expect(find.byTooltip('Hapus lampiran'), findsOneWidget);
      expect(find.text('Kirim'), findsNothing);

      // The composer send icon is the ONE send authority, enabled with an
      // empty draft because a pending attachment exists.
      await tester.tap(_composerSendButton());
      await tester.pump();

      expect(notifier.sendCalls, 1);
      final occurrence =
          notifier.lastSendArgs?['resourceOccurrence']
              as ChatResourceOccurrenceRequest?;
      expect(occurrence, isNotNull);
      expect(
        occurrence!.operation,
        ChatResourceOccurrenceOperation.directCommerceInsertChat,
      );
      expect(
        occurrence.resourceType,
        ChatResourceOccurrenceResourceType.forSale,
      );
      expect(occurrence.resourceId, _fixedPriceSaleId);
      expect(notifier.lastSendArgs?['content'], isEmpty);
      // No redundant reference snapshot travels with the product send.
      expect(notifier.lastSendArgs?.containsKey('objectReference'), isFalse);
      expect(notifier.lastSendArgs?.containsKey('attachment'), isFalse);
    },
  );

  testWidgets(
    'Auction detail entry seeds pending attachment: no Kirim CTA, send icon '
    'authority, empty body sends resourceOccurrence',
    (tester) async {
      final notifier = _FakeChatDetailNotifier(
        initialState: ChatDetailState(chat: _makeChat(), messages: const []),
      );

      await tester.pumpWidget(
        _buildScope(
          notifier: notifier,
          child: ChatDetailScreen(
            chatId: _chatId,
            pendingCommerce: PendingCommerceAttachment.auction(
              auctionId: _auctionId,
              title: 'Koi Test Auction',
            ),
          ),
        ),
      );
      await tester.pump(const Duration(milliseconds: 300));

      expect(find.text('Lampiran produk'), findsOneWidget);
      expect(find.text('Koi Test Auction'), findsOneWidget);
      expect(find.byTooltip('Hapus lampiran'), findsOneWidget);
      expect(find.text('Kirim'), findsNothing);

      await tester.tap(_composerSendButton());
      await tester.pump();

      expect(notifier.sendCalls, 1);
      final occurrence =
          notifier.lastSendArgs?['resourceOccurrence']
              as ChatResourceOccurrenceRequest?;
      expect(occurrence, isNotNull);
      expect(
        occurrence!.operation,
        ChatResourceOccurrenceOperation.directCommerceInsertChat,
      );
      expect(
        occurrence.resourceType,
        ChatResourceOccurrenceResourceType.auction,
      );
      expect(occurrence.resourceId, _auctionId);
      expect(notifier.lastSendArgs?['content'], isEmpty);
    },
  );
}
