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
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/chat/chat/presentation/screens/chat_detail_screen.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_input_area.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/message_bubble.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/seller_auctions_pager.dart';
import 'package:labuda/domains/commerce/catalog/listing/domain/entities/listing.dart';
import 'package:labuda/domains/commerce/catalog/listing/domain/repositories/listing_repository.dart';
import 'package:labuda/domains/commerce/catalog/listing/presentation/create_listing_route_contract.dart';
import 'package:labuda/domains/commerce/catalog/listing/presentation/providers/listing_controller.dart';
import 'package:labuda/domains/commerce/catalog/listing/presentation/providers/listing_providers.dart';
import 'package:labuda/domains/commerce/catalog/listing/presentation/providers/seller_fps_pager.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_notifier.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_state.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
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
      hasSellerProfile: false,
      hasMarketAuthority: false,
      sellerSubscriptionStatus: 'none',
      roles: const [UserRole.user],
      provider: ShonaAuthProvider.email,
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
      items: [_fixedPriceListing()],
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
      activeFilter: SellerAuctionFilter.all,
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

class _NoOpListingRepository implements ListingRepository {
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
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) {
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) {
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) {
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
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

class _FakeLookupListingController extends ListingController {
  _FakeLookupListingController()
    : super(repository: _NoOpListingRepository(), logger: _NoOpLogger());

  @override
  Future<Result<Listing?>> getFixedPriceSaleById(
    String fixedPriceSaleId,
  ) async {
    return Result.success(
      _fixedPriceListing(
        forSaleId: fixedPriceSaleId,
        title: 'Created Listing $fixedPriceSaleId',
      ),
    );
  }
}

class _FakeChatDetailNotifier extends ChatDetail {
  _FakeChatDetailNotifier({required this.initialState});

  final ChatDetailState initialState;
  int sendCalls = 0;
  Map<String, dynamic>? lastSendArgs;
  Completer<Message?>? sendCompleter;

  @override
  ChatDetailState build(String chatId) => initialState;

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
    ShareReference? objectReference,
    ChatResourceOccurrenceRequest? resourceOccurrence,
    Map<String, dynamic>? workflowAttachment,
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
      'objectReference': objectReference,
      'resourceOccurrence': resourceOccurrence,
      'workflowAttachment': workflowAttachment,
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

Message _makeReplyTarget() => Message(
  id: 'msg-target',
  chatId: _chatId,
  senderId: _otherUserId,
  senderName: 'other',
  content: 'original message',
  createdAt: DateTime.utc(2026, 7, 30, 9, 0),
  status: MessageStatus.sent,
  mentionedUserIds: const [],
  deletedBy: const [],
);

Listing _fixedPriceListing({
  String fixedPriceSaleId = _fixedPriceSaleId,
  String title = 'Koi Test FPS',
}) => Listing(
  forSaleId: fixedPriceSaleId,
  productId: _productId,
  title: title,
  description: 'Test fixed-price sale',
  price: 500000,
  stock: 1,
  sellerId: _currentUserId,
  status: ListingStatus.active,
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
      authControllerProvider.overrideWith(_FakeAuthController.new),
      currentUserIdProvider.overrideWith((ref) => _currentUserId),
      typingIndicatorEnabledProvider.overrideWithValue(false),
      isUserBlockedProvider(_otherUserId).overrideWith((ref) => false),
      negotiationNotifierProvider.overrideWith(_FakeNegotiationNotifier.new),
      presenceProvider.overrideWithValue(const PresenceState()),
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

Widget _buildChatCommerceScope({
  required Widget createListingRoute,
  required _FakeChatDetailNotifier chatDetailNotifier,
  ValueChanged<Object?>? onCreateListingRouteExtra,
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
        path: RoutePaths.createListing,
        name: RouteNames.createListing,
        pageBuilder: (context, state) {
          onCreateListingRouteExtra?.call(state.extra);
          return MaterialPage(key: state.pageKey, child: createListingRoute);
        },
      ),
    ],
  );

  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(_FakeAuthController.new),
      currentUserIdProvider.overrideWith((ref) => _currentUserId),
      typingIndicatorEnabledProvider.overrideWithValue(false),
      isUserBlockedProvider(_otherUserId).overrideWith((ref) => false),
      negotiationNotifierProvider.overrideWith(_FakeNegotiationNotifier.new),
      presenceProvider.overrideWithValue(const PresenceState()),
      chatDetailProvider(_chatId).overrideWith(() => chatDetailNotifier),
      sellerFPSPagerProvider.overrideWith(
        () => _FakeSellerFPSPagerController(),
      ),
      sellerAuctionsPagerProvider.overrideWith(
        () => _FakeSellerAuctionsPagerController(),
      ),
      listingControllerProvider.overrideWithValue(
        _FakeLookupListingController(),
      ),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

Future<void> _openChatCreateListingRoute(WidgetTester tester) async {
  await tester.tap(find.byIcon(Icons.add_circle));
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));
  await tester.tap(find.text('Lampirkan Produk'));
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));

  expect(find.text('Pilih Produk'), findsOneWidget);
  await tester.tap(find.text('Buat Listing Baru'));
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 1200));
}

class _ChatCreateListingRoute extends StatelessWidget {
  final Object? successResult;

  const _ChatCreateListingRoute({this.successResult});

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
  testWidgets('attachment menu exposes direct commerce entry only', (
    tester,
  ) async {
    await tester.pumpWidget(
      _buildScope(child: const ChatDetailScreen(chatId: _chatId)),
    );
    await tester.pump(const Duration(milliseconds: 300));

    await tester.tap(find.byIcon(Icons.add_circle));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.text('Foto'), findsOneWidget);
    expect(find.text('Video'), findsOneWidget);
    expect(find.text('Lampirkan Produk'), findsOneWidget);
    expect(find.text('Bagikan Listing'), findsNothing);

    await tester.tap(find.text('Lampirkan Produk'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.text('Pilih Produk'), findsOneWidget);
    expect(find.text('Buat Listing Baru'), findsOneWidget);
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
    'create listing cancel preserves existing pending selection and draft',
    (tester) async {
      final notifier = _FakeChatDetailNotifier(
        initialState: ChatDetailState(chat: _makeChat(), messages: const []),
      );

      await tester.pumpWidget(
        _buildChatCommerceScope(
          chatDetailNotifier: notifier,
          createListingRoute: const _ChatCreateListingRoute(
            successResult: CreatedFixedPriceSaleResult(
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
      await tester.tap(find.text('Buat Listing Baru'));
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
    'create listing success replaces pending selection and stays unsent',
    (tester) async {
      final notifier = _FakeChatDetailNotifier(
        initialState: ChatDetailState(chat: _makeChat(), messages: const []),
      );

      await tester.pumpWidget(
        _buildChatCommerceScope(
          chatDetailNotifier: notifier,
          createListingRoute: const _ChatCreateListingRoute(
            successResult: CreatedFixedPriceSaleResult(
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
      await tester.tap(find.text('Buat Listing Baru'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 1200));

      expect(find.text('Create'), findsOneWidget);
      await tester.tap(find.text('Create'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 1200));

      expect(_composerText(tester), 'draft before create');
      expect(find.text('Koi Test FPS'), findsNothing);
      expect(
        find.text('Created Listing $_createdFixedPriceSaleId'),
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

  testWidgets('reply preview can be selected and cancelled from bubble', (
    tester,
  ) async {
    await tester.pumpWidget(
      _buildScope(
        initialState: ChatDetailState(
          chat: _makeChat(),
          messages: [_makeReplyTarget()],
        ),
        child: const ChatDetailScreen(chatId: _chatId),
      ),
    );
    await tester.pump(const Duration(milliseconds: 300));

    final bubble = tester.widget<MessageBubble>(find.byType(MessageBubble));
    expect(bubble.onLongPress, isNotNull);
    bubble.onLongPress!.call();
    await tester.pump(const Duration(milliseconds: 500));

    final replyTile = tester.widget<ListTile>(
      find.widgetWithText(ListTile, 'Reply'),
    );
    expect(replyTile.onTap, isNotNull);
    replyTile.onTap!.call();
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.text('Membalas other'), findsOneWidget);
    expect(find.text('original message'), findsWidgets);

    await tester.tap(find.byIcon(Icons.close).last);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.text('Membalas other'), findsNothing);
  });

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
    'chat create listing route uses explicit fixed-price-sale return mode',
    (tester) async {
      final notifier = _FakeChatDetailNotifier(
        initialState: ChatDetailState(chat: _makeChat(), messages: const []),
      );
      Object? capturedExtra;

      await tester.pumpWidget(
        _buildChatCommerceScope(
          chatDetailNotifier: notifier,
          onCreateListingRouteExtra: (extra) => capturedExtra = extra,
          createListingRoute: const _ChatCreateListingRoute(),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 300));

      await _openChatCreateListingRoute(tester);

      expect(capturedExtra, isA<CreateListingRouteArgs>());
      final args = capturedExtra! as CreateListingRouteArgs;
      expect(args.returnMode, CreateListingReturnMode.forSaleId);
    },
  );

  testWidgets('chat create listing null result attaches nothing', (
    tester,
  ) async {
    final notifier = _FakeChatDetailNotifier(
      initialState: ChatDetailState(chat: _makeChat(), messages: const []),
    );

    await tester.pumpWidget(
      _buildChatCommerceScope(
        chatDetailNotifier: notifier,
        createListingRoute: const _ChatCreateListingRoute(successResult: null),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    await _openChatCreateListingRoute(tester);
    await tester.tap(find.text('Create'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 600));

    expect(find.text('Lampiran produk'), findsNothing);
    expect(find.textContaining('Created Listing'), findsNothing);
    expect(notifier.sendCalls, 0);
  });

  testWidgets('chat create listing raw Listing result attaches nothing', (
    tester,
  ) async {
    final notifier = _FakeChatDetailNotifier(
      initialState: ChatDetailState(chat: _makeChat(), messages: const []),
    );

    await tester.pumpWidget(
      _buildChatCommerceScope(
        chatDetailNotifier: notifier,
        createListingRoute: _ChatCreateListingRoute(
          successResult: Listing(
            forSaleId: _createdFixedPriceSaleId,
            productId: _productId,
            title: 'Legacy Listing Result',
            description: 'Should not attach in Chat',
            price: 500000,
            stock: 1,
            sellerId: _currentUserId,
            status: ListingStatus.active,
            createdAt: DateTime.utc(2026, 7, 30),
            updatedAt: DateTime.utc(2026, 7, 30),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    await _openChatCreateListingRoute(tester);
    await tester.tap(find.text('Create'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 600));

    expect(find.text('Lampiran produk'), findsNothing);
    expect(find.textContaining('Created Listing'), findsNothing);
    expect(notifier.sendCalls, 0);
  });

  testWidgets('chat create listing arbitrary object result attaches nothing', (
    tester,
  ) async {
    final notifier = _FakeChatDetailNotifier(
      initialState: ChatDetailState(chat: _makeChat(), messages: const []),
    );

    await tester.pumpWidget(
      _buildChatCommerceScope(
        chatDetailNotifier: notifier,
        createListingRoute: const _ChatCreateListingRoute(
          successResult: {'fixedPriceSaleId': _createdFixedPriceSaleId},
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    await _openChatCreateListingRoute(tester);
    await tester.tap(find.text('Create'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 600));

    expect(find.text('Lampiran produk'), findsNothing);
    expect(find.textContaining('Created Listing'), findsNothing);
    expect(notifier.sendCalls, 0);
  });
}
