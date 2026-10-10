/// Behavioral integration proof — For Sale detail → Chat pending product
/// convergence.
///
/// This test starts at the REAL producer: it renders `ForSaleDetailScreen`,
/// taps the REAL "Chat" action (`_ForSaleActionBar._openChat`), which calls the
/// REAL `openCommerceChat` and pushes through the REAL `ChatModule` route
/// builder (the production code that parses the `pendingCommerce` route extra)
/// into the REAL `ChatDetailScreen`. Only the chat room-resolution boundary is
/// faked; the navigation contract and the pending-attachment state are real.
///
/// Proven here:
///   1. the producer emits a `PendingCommerceAttachment` (For Sale identity)
///      through the canonical `pendingCommerce` route extra;
///   2. Chat renders exactly ONE pending chip (preview + remove), no `Kirim`;
///   3. the chip's remove authority clears the pending state;
///   4. an EMPTY-body send via the composer send icon emits a
///      `resourceOccurrence` with operation `direct_commerce_insert_chat` and
///      the For Sale identity.
///
/// This is an automated widget/integration proof, NOT device/runtime proof.
library;

import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/common/types/preparation_time.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/core/src/router/modules/chat_module.dart';
import 'package:hishumi/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:hishumi/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:hishumi/domains/chat/chat/presentation/screens/chat_detail_screen.dart';
import 'package:hishumi/domains/chat/chat/presentation/widgets/chat_input_area.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart'
    show forSaleDetailProvider;
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart';
import 'package:hishumi/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_notifier.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_state.dart';
import 'package:hishumi/domains/social/content/domain/entities/content.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/user_data_provider.dart';
import 'package:hishumi/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:hishumi/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:hishumi/domains/user/preference/saved_item/models/saved_item_model.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart';
import 'package:hishumi/shared/providers/block_state_provider.dart';

const _buyerId = 'buyer-1';
const _sellerId = 'seller-1';
const _forSaleId = '11111111-1111-1111-1111-111111111111';
const _roomId = 'room-1';

// ===========================================================================
// FAKES — only at the boundary (auth identity, room resolution, chat thread).
// ===========================================================================

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
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

class _FakeSavedItemRepository extends SavedItemRepository {
  _FakeSavedItemRepository()
    : super(dio: Dio(BaseOptions(baseUrl: 'http://localhost')));

  @override
  Future<bool> isSaved({
    required String targetType,
    required String targetId,
  }) async => false;

  @override
  Future<SavedItemModel> addSavedItem({
    required String targetType,
    required String targetId,
  }) async => SavedItemModel(
    id: 'saved-$targetType-$targetId',
    userId: _buyerId,
    targetType: TargetType.forSale,
    targetId: targetId,
    intentType: IntentType.bookmark,
    createdAt: DateTime.utc(2026, 1, 1),
  );

  @override
  Future<void> removeSavedItem({
    required String targetType,
    required String targetId,
  }) async {}
}

class _FakeNavigationHandler extends Fake implements NavigationHandler {}

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

const _buyerCaps = CommerceViewerCapabilities(
  role: 'buyer',
  canManage: false,
  canEdit: false,
  canPromote: false,
  canChat: true,
  canNegotiate: true,
  canBuy: true,
  canBid: false,
  canBuyNow: false,
);

List<MediaEntity> _detailMedia() {
  final now = DateTime.utc(2026, 1, 1);
  return [
    MediaEntity(
      id: 'forSale-media-1',
      originalUrl: 'https://cdn.example.com/gallery/forSale-1.jpg',
      type: MediaType.image,
      createdAt: now,
    ),
  ];
}

ForSale _listing() {
  final now = DateTime.utc(2026, 1, 1);
  return ForSale(
    forSaleId: _forSaleId,
    productId: 'product-1',
    title: 'Showa Koi 30cm',
    description: 'Premium showa',
    price: 1500000,
    stock: 1,
    sellerId: _sellerId,
    sellerUsername: 'seller_user',
    sellerFarmName: 'Acme Farm',
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
    sellerTier: 'pro',
    viewerCapabilities: _buyerCaps,
    media: _detailMedia(),
    status: ForSaleStatus.active,
    visibility: ForSaleVisibility.public,
    isNegotiable: true,
    createdAt: now,
    updatedAt: now,
    variety: 'Kohaku',
    sizeCm: 30,
    ageMonths: 12,
    gender: 'male',
    preparationTime: PreparationTime.days1_3,
  );
}

/// Builds the real app surface: For Sale detail route + the REAL ChatModule
/// routes (the production `pendingCommerce` extra parser).
Future<GoRouter> _buildRouter() async {
  final chatModule = ChatModule();
  await chatModule.initialize();

  return GoRouter(
    initialLocation: '/for-sale/$_forSaleId',
    routes: [
      GoRoute(
        path: '/for-sale/:id',
        builder: (context, state) =>
            ForSaleDetailScreen(forSaleId: state.pathParameters['id']!),
      ),
      ...chatModule.routes,
    ],
  );
}

Widget _app({
  required GoRouter router,
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
      forSaleDetailProvider(_forSaleId).overrideWith((ref) async => _listing()),
      savedItemRepositoryProvider.overrideWithValue(_FakeSavedItemRepository()),
      userDataProvider.overrideWith((ref, userId) async => _authUser(userId)),
      navigationHandlerProvider.overrideWithValue(_FakeNavigationHandler()),
      chatListProvider.overrideWith(_FakeChatList.new),
      chatDetailProvider(_roomId).overrideWith(() => chatDetail),
      negotiationNotifierProvider.overrideWith(_FakeNegotiationNotifier.new),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

/// Drives the REAL producer: tap the For Sale detail "Chat" action and settle
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
  matching: find.text('Showa Koi 30cm'),
);

Finder _composerSendIcon() => find.descendant(
  of: find.byType(ChatInputArea),
  matching: find.byIcon(Icons.send),
);

void main() {
  testWidgets(
    'For Sale detail "Chat" opens Chat with ONE remove-only pending chip and no Kirim',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(360, 760));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final chatDetail = _FakeChatDetail();
      await tester.pumpWidget(
        _app(router: await _buildRouter(), chatDetail: chatDetail),
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
    'For Sale detail "Chat" → empty-body send icon emits direct_commerce_insert_chat resourceOccurrence',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(360, 760));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final chatDetail = _FakeChatDetail();
      await tester.pumpWidget(
        _app(router: await _buildRouter(), chatDetail: chatDetail),
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
      expect(
        occurrence.resourceType,
        ChatResourceOccurrenceResourceType.forSale,
      );
      expect(occurrence.resourceId, _forSaleId);
      // Product can be sent WITHOUT text.
      expect(chatDetail.lastSendArgs?['content'], isEmpty);
      // No legacy objectReference/attachment write path travels with the send.
      expect(chatDetail.lastSendArgs?.containsKey('objectReference'), isFalse);
      expect(chatDetail.lastSendArgs?.containsKey('attachment'), isFalse);

      expect(tester.takeException(), isNull);
    },
  );
}
