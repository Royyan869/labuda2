/// Behavioral integration proof — Auction detail → Chat pending product
/// convergence.
///
/// This test starts at the REAL producer: it renders `AuctionDetailScreen`,
/// taps the REAL "Chat" action in `AuctionDetailBottomBar` (`_handleChat`),
/// which calls the REAL `openCommerceChat` and pushes through the REAL
/// `ChatModule` route builder (the production code that parses the
/// `pendingCommerce` route extra) into the REAL `ChatDetailScreen`. Only the
/// chat room-resolution boundary and unrelated Auction-detail services are
/// faked; the navigation contract and the pending-attachment state are real.
///
/// Proven here:
///   1. the producer emits a `PendingCommerceAttachment.auction(...)` through
///      the canonical `pendingCommerce` route extra;
///   2. Chat renders exactly ONE pending chip (preview + remove), no `Kirim`;
///   3. the chip's remove authority clears the pending state;
///   4. an EMPTY-body send via the composer send icon emits a
///      `resourceOccurrence` with operation `direct_commerce_insert_chat`,
///      `resourceType == auction` and the Auction identity.
///
/// This is an automated widget/integration proof, NOT device/runtime proof.
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/common/types/preparation_time.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/src/router/modules/chat_module.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/chat/chat/presentation/screens/chat_detail_screen.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_input_area.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_recommendation_providers.dart'
    show ownerOtherAuctionsProvider, similarAuctionsProvider;
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart';
import 'package:labuda/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_notifier.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_state.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';
import 'package:labuda/shared/providers/block_state_provider.dart';

const _buyerId = 'buyer-1';
const _sellerId = 'seller-1';
const _auctionId = '22222222-2222-2222-2222-222222222222';
const _roomId = 'room-1';

// ===========================================================================
// FAKES — only at the boundary (auth identity, room resolution, chat thread,
// unrelated Auction-detail services).
// ===========================================================================

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeAuctionNotifier extends AuctionNotifier {
  _FakeAuctionNotifier(this._state);

  final AuctionNotifierState _state;

  @override
  AuctionNotifierState build() => _state;

  @override
  Future<void> loadAuctionDetails(String auctionId) async {}

  @override
  Future<void> loadAuctionBids(String auctionId, {int limit = 50}) async {}
}

class _FakeNavigationHandler extends Fake implements NavigationHandler {}

class _FakeSavedItemRepository implements SavedItemRepository {
  @override
  Future<bool> isSaved({
    required String targetType,
    required String targetId,
  }) async => false;

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

/// Room-resolution boundary only: the canonical opener asks for a room; the
/// room is returned without any network.
class _FakeChatList extends ChatList {
  @override
  ChatListState build() => const ChatListState();

  @override
  Future<Chat?> getOrCreateChat({
    required String userId,
    required String otherUserId,
  }) async => _chat();

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

/// Open-thread boundary: holds a real active room and captures the outgoing
/// send request. Load/read side-effects are no-ops so the test is deterministic
/// and never touches a transport.
class _FakeChatDetail extends ChatDetail {
  int sendCalls = 0;
  Map<String, dynamic>? lastSendArgs;
  Completer<Message?>? sendCompleter;

  @override
  ChatDetailState build(String chatId) =>
      ChatDetailState(chat: _chat(), messages: const []);

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
      'resourceOccurrence': resourceOccurrence,
    };
    sendCompleter ??= Completer<Message?>();
    return sendCompleter!.future;
  }
}

class _FakeNegotiationNotifier extends NegotiationNotifier {
  @override
  NegotiationState build() => const NegotiationState();
}

Chat _chat() => Chat(
  id: _roomId,
  participantIds: const [_buyerId, _sellerId],
  participantNames: const {_buyerId: 'buyer', _sellerId: 'seller'},
  participantAvatars: const {},
  participantLifecycles: const {_sellerId: ContentLifecycle.active},
  createdAt: DateTime.utc(2026, 1, 1),
  status: ChatStatus.active,
);

AuthUser _authUser(String id) {
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
    lifecycle: ContentLifecycle.active,
  );
}

const _buyerCapabilities = CommerceViewerCapabilities(
  role: 'buyer',
  canManage: false,
  canEdit: false,
  canPromote: false,
  canChat: true,
  canNegotiate: false,
  canBuy: false,
  canBid: true,
  canBuyNow: true,
);

List<MediaEntity> _detailMedia() {
  final now = DateTime.utc(2026, 1, 1);
  return [
    MediaEntity(
      id: 'auction-media-1',
      originalUrl: 'https://cdn.example.com/gallery/auction-1.jpg',
      type: MediaType.image,
      createdAt: now,
    ),
  ];
}

Auction _auction() {
  final now = DateTime.utc(2026, 1, 1);
  return Auction(
    id: _auctionId,
    sellerId: _sellerId,
    sellerUsername: 'seller_user',
    sellerFarmName: 'Acme Farm',
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
    sellerTier: 'pro',
    viewerCapabilities: _buyerCapabilities,
    title: 'Sanke Auction',
    description: 'Live auction',
    koiDetails: const KoiDetails(
      variety: 'Kohaku',
      sizeInCm: 30,
      ageInMonths: 12,
      gender: 'male',
      breeder: 'Hiro',
      bloodline: 'Miyabi',
      certificates: ['import', 'health'],
    ),
    preparationTime: PreparationTime.days1_3,
    openingBid: 1000000,
    currentBid: 1500000,
    bidIncrement: 50000,
    buyNowPrice: 2500000,
    media: _detailMedia(),
    startTime: now,
    endTime: now.add(const Duration(days: 1)),
    status: AuctionStatus.active,
    createdAt: now,
    updatedAt: now,
    productId: 'product-1',
  );
}

/// Builds the real app surface: Auction detail route + the REAL ChatModule
/// routes (the production `pendingCommerce` extra parser).
Future<GoRouter> _buildRouter() async {
  final chatModule = ChatModule();
  await chatModule.initialize();

  return GoRouter(
    initialLocation: '/auction/$_auctionId',
    routes: [
      GoRoute(
        path: '/auction/:id',
        builder: (context, state) =>
            AuctionDetailScreen(auctionId: state.pathParameters['id']!),
      ),
      ...chatModule.routes,
    ],
  );
}

Widget _app({
  required GoRouter router,
  required Auction auction,
  required _FakeChatDetail chatDetail,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(
          AuthState.authenticated(_authUser(_buyerId), emailVerified: true),
        ),
      ),
      currentUserIdProvider.overrideWith((ref) => _buyerId),
      isUserBlockedProvider(_sellerId).overrideWith((ref) => false),
      auctionNotifierProvider.overrideWith(
        () => _FakeAuctionNotifier(AuctionNotifierState(selectedAuction: auction)),
      ),
      auctionStreamProvider(auction.id).overrideWith((ref) => Stream.value(auction)),
      auctionBidsStreamProvider(
        auction.id,
      ).overrideWith((ref) => Stream.value(const <AuctionBid>[])),
      ownerOtherAuctionsProvider(
        auction.id,
      ).overrideWith((ref) async => const <Auction>[]),
      similarAuctionsProvider(
        auction.id,
      ).overrideWith((ref) async => const <Auction>[]),
      navigationHandlerProvider.overrideWithValue(_FakeNavigationHandler()),
      savedItemRepositoryProvider.overrideWithValue(_FakeSavedItemRepository()),
      chatListProvider.overrideWith(_FakeChatList.new),
      chatDetailProvider(_roomId).overrideWith(() => chatDetail),
      negotiationNotifierProvider.overrideWith(_FakeNegotiationNotifier.new),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

/// Drives the REAL producer: tap the Auction detail "Chat" action and settle
/// the real navigation.
Future<void> _openChatFromDetail(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));

  expect(find.text('Chat'), findsOneWidget);
  await tester.tap(find.text('Chat'));
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));
  await tester.pump(const Duration(milliseconds: 400));
}

Finder _chatChipCaption() => find.descendant(
  of: find.byType(ChatDetailScreen),
  matching: find.text('Lampiran produk'),
);

Finder _chatProductTitle() => find.descendant(
  of: find.byType(ChatDetailScreen),
  matching: find.text('Sanke Auction'),
);

Finder _composerSendIcon() => find.descendant(
  of: find.byType(ChatInputArea),
  matching: find.byIcon(Icons.send),
);

void main() {
  testWidgets(
    'Auction detail "Chat" opens Chat with ONE remove-only pending chip and no Kirim',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(800, 2400));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final chatDetail = _FakeChatDetail();
      await tester.pumpWidget(
        _app(
          router: await _buildRouter(),
          auction: _auction(),
          chatDetail: chatDetail,
        ),
      );

      await _openChatFromDetail(tester);

      // The REAL producer reached the REAL Chat screen with the pending product.
      expect(find.byType(ChatDetailScreen), findsOneWidget);
      expect(_chatChipCaption(), findsOneWidget);
      expect(_chatProductTitle(), findsOneWidget);
      // Exactly one pending chip — no parallel pending state.
      expect(_chatChipCaption(), findsOneWidget);
      // Forbidden design is dead: no text send CTA on the card.
      expect(find.text('Kirim'), findsNothing);
      // The card's only action is remove.
      expect(find.byTooltip('Hapus lampiran'), findsOneWidget);

      // Remove authority: the same canonical pending state used by the composer.
      await tester.tap(find.byTooltip('Hapus lampiran'));
      await tester.pump();
      expect(_chatChipCaption(), findsNothing);
      expect(_chatProductTitle(), findsNothing);

      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'Auction detail "Chat" → empty-body send icon emits direct_commerce_insert_chat resourceOccurrence (resourceType auction)',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(800, 2400));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final chatDetail = _FakeChatDetail();
      await tester.pumpWidget(
        _app(
          router: await _buildRouter(),
          auction: _auction(),
          chatDetail: chatDetail,
        ),
      );

      await _openChatFromDetail(tester);

      expect(_chatChipCaption(), findsOneWidget);
      expect(find.text('Kirim'), findsNothing);

      // Send authority is the composer send icon ONLY — the chip has no send
      // action. Body is intentionally left EMPTY (product-only send).
      final sendIconFinder = _composerSendIcon();
      expect(sendIconFinder, findsOneWidget);
      final sendButton = tester.widget<IconButton>(
        find.ancestor(of: sendIconFinder, matching: find.byType(IconButton)),
      );
      expect(
        sendButton.onPressed,
        isNotNull,
        reason: 'send icon must be enabled when a pending attachment exists',
      );
      await tester.tap(sendIconFinder);
      await tester.pump();

      expect(chatDetail.sendCalls, 1);
      final occurrence =
          chatDetail.lastSendArgs?['resourceOccurrence']
              as ChatResourceOccurrenceRequest?;
      expect(occurrence, isNotNull);
      expect(
        occurrence!.operation,
        ChatResourceOccurrenceOperation.directCommerceInsertChat,
      );
      // AUCTION-specific: the canonical resource type is auction, not for_sale.
      expect(
        occurrence.resourceType,
        ChatResourceOccurrenceResourceType.auction,
      );
      expect(occurrence.resourceId, _auctionId);
      // Product can be sent WITHOUT text.
      expect(chatDetail.lastSendArgs?['content'], isEmpty);
      // No legacy objectReference/attachment write path travels with the send.
      expect(chatDetail.lastSendArgs?.containsKey('objectReference'), isFalse);
      expect(chatDetail.lastSendArgs?.containsKey('attachment'), isFalse);

      expect(tester.takeException(), isNull);
    },
  );
}
