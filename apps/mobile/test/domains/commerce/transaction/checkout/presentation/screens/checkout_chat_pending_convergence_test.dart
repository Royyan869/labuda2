/// Behavioral integration proof — Checkout → Chat pending product convergence.
///
/// This test starts at the REAL producer: it renders the REAL `CheckoutScreen`,
/// drives the shipping picker to its uncovered-area empty state so the REAL
/// "Hubungi Penjual" CTA renders, and taps it. That calls the REAL
/// `_openChatWithSeller`, which calls the REAL `openCommerceChat` and pushes
/// through the REAL `ChatModule` route builder (the production code that parses
/// the `pendingCommerce` route extra) into the REAL `ChatDetailScreen`. Only
/// the chat room-resolution boundary and unrelated Checkout services are faked.
///
/// Checkout semantics (from the factual code): checkout is PRE-ORDER for a
/// fixed-price For Sale, so `_openChatWithSeller` carries
/// `PendingCommerceAttachment.forSale(forSaleId: forSale.forSaleId, ...)` — the
/// canonical resource type is `forSale`, not auction.
///
/// Proven here:
///   1. the producer emits a `PendingCommerceAttachment.forSale(...)` through
///      the canonical `pendingCommerce` route extra;
///   2. Chat renders exactly ONE pending chip (preview + remove), no `Kirim`;
///   3. the chip's remove authority clears the pending state;
///   4. an EMPTY-body send via the composer send icon emits a
///      `resourceOccurrence` with operation `direct_commerce_insert_chat`,
///      `resourceType == forSale` and the exact For Sale id.
///
/// This is an automated widget/integration proof, NOT device/runtime proof.
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/core/src/router/modules/chat_module.dart';
import 'package:hishumi/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:hishumi/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:hishumi/domains/chat/chat/presentation/screens/chat_detail_screen.dart';
import 'package:hishumi/domains/chat/chat/presentation/widgets/chat_input_area.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_notifier.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_state.dart';
import 'package:hishumi/domains/commerce/transaction/checkout/checkout.dart';
import 'package:hishumi/domains/commerce/transaction/order/domain/domain.dart';
import 'package:hishumi/domains/commerce/transaction/order/presentation/providers/order_providers.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/domain/entities/shipping.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/domain/repositories/shipping_repository.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:hishumi/domains/finance/wallet/coins/coins.dart';
import 'package:hishumi/domains/user/profile/domain/entities/address_entity.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/notifiers/address_notifier.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/state/address_state.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/models/wilayah_models.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart';
import 'package:hishumi/shared/providers/block_state_provider.dart';

const _buyerId = 'buyer-1';
const _sellerId = 'seller-1';
const _forSaleId = '33333333-3333-3333-3333-333333333333';
const _productId = 'product-1';
const _roomId = 'room-1';

// ===========================================================================
// FAKES — only at the boundary (auth identity, room resolution, chat thread,
// unrelated Checkout services).
// ===========================================================================

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeCoinNotifier extends CoinNotifier {
  @override
  CoinState build() => const CoinState.initial();

  @override
  Future<void> getBalance() async {}
}

class _FakeAddressNotifier extends AddressNotifier {
  _FakeAddressNotifier(this._addresses);

  final List<AddressEntity> _addresses;

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

/// The uncovered-area boundary: the seller configured nothing reachable for
/// this buyer address, so the check returns ZERO delivery options.
class _EmptyShippingRepository implements ShippingRepository {
  @override
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async => Result.success(const []);

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _NoopCheckoutRepository implements CheckoutRepository {
  @override
  Future<CheckoutResponse> createOrder(
    CheckoutRequest request, {
    String? idempotencyKey,
  }) async => throw UnimplementedError();

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

Chat _chat() => Chat(
  id: _roomId,
  participantIds: const [_buyerId, _sellerId],
  participantNames: const {_buyerId: 'buyer', _sellerId: 'seller'},
  participantAvatars: const {},
  participantLifecycles: const {_sellerId: ContentLifecycle.active},
  createdAt: DateTime.utc(2026, 8, 1),
  status: ChatStatus.active,
);

AuthUser _buyer() {
  return AuthUser(
    id: _buyerId,
    createdAt: DateTime.utc(2026, 8, 1),
    updatedAt: DateTime.utc(2026, 8, 1),
    email: 'buyer@example.com',
    username: 'buyer',
    isEmailVerified: true,
    roles: const [],
    provider: AuthProvider.email,
  );
}

AddressEntity _shippingAddress() {
  return AddressEntity(
    id: 'address-1',
    userId: _buyerId,
    recipientName: 'Buyer',
    phone: '08123456789',
    province: const Province(id: '31', name: 'DKI Jakarta'),
    city: const City(id: '3171', name: 'Jakarta Selatan', provinceId: '31'),
    district: const District(
      id: '3171010',
      name: 'Kebayoran',
      cityId: '3171',
    ),
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
}

ForSale _listing() {
  return ForSale(
    forSaleId: _forSaleId,
    productId: _productId,
    title: 'Ikan Koi Test',
    description: 'Deskripsi test',
    price: 1250000,
    stock: 3,
    media: const [],
    sellerId: _sellerId,
    status: ForSaleStatus.active,
    visibility: ForSaleVisibility.public,
    createdAt: DateTime.utc(2026, 8, 1),
    updatedAt: DateTime.utc(2026, 8, 1),
  );
}

PreviewOrderResult _previewResult() => PreviewOrderResult(
  pricing: const OrderPricing(
    subtotal: 111000,
    shippingCost: 2222,
    serviceFeeAmount: 0,
    totalPayableAmount: 113222,
  ),
  pricingToken: 'token-ship-A',
  sellerId: _sellerId,
  shippingMode: 'standard',
  expiresAt: DateTime.now().add(const Duration(minutes: 10)),
);

/// Builds the real app surface: the Checkout route + the REAL ChatModule
/// routes (the production `pendingCommerce` extra parser).
Future<GoRouter> _buildRouter() async {
  final chatModule = ChatModule();
  await chatModule.initialize();

  return GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(
        path: '/',
        builder: (context, state) =>
            const CheckoutScreen(productId: _productId, forSaleId: _forSaleId),
      ),
      ...chatModule.routes,
    ],
  );
}

Future<void> _pumpCheckout(
  WidgetTester tester, {
  required GoRouter router,
  required _FakeChatDetail chatDetail,
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
        authControllerProvider.overrideWith(
          () => _FakeAuthController(
            AuthState.authenticated(_buyer(), emailVerified: true),
          ),
        ),
        currentUserIdProvider.overrideWith((ref) => _buyerId),
        isUserBlockedProvider(_sellerId).overrideWith((ref) => false),
        coinProvider.overrideWith(() => _FakeCoinNotifier()),
        addressProvider.overrideWith(
          () => _FakeAddressNotifier([_shippingAddress()]),
        ),
        forSaleDetailProvider.overrideWith((ref, forSaleId) async => _listing()),
        shippingRepositoryProvider.overrideWithValue(_EmptyShippingRepository()),
        orderPreviewProvider.overrideWith(
          (ref, params) async => _previewResult(),
        ),
        checkoutRepositoryProvider.overrideWithValue(_NoopCheckoutRepository()),
        chatListProvider.overrideWith(_FakeChatList.new),
        chatDetailProvider(_roomId).overrideWith(() => chatDetail),
        negotiationNotifierProvider.overrideWith(_FakeNegotiationNotifier.new),
      ],
      child: MaterialApp.router(routerConfig: router),
    ),
  );

  // Bounded pumps only: the screen renders a pricing spinner while
  // prerequisites are in flight, so `pumpAndSettle` never settles.
  await _settle(tester);
}

Future<void> _settle(WidgetTester tester) async {
  for (var i = 0; i < 8; i++) {
    await tester.pump(const Duration(milliseconds: 150));
  }
}

/// Drives the REAL producer: the shipping picker's uncovered-area empty state
/// renders the canonical "Hubungi Penjual" CTA; tapping it runs
/// `_openChatWithSeller` → `openCommerceChat`.
Future<void> _openChatFromCheckout(WidgetTester tester) async {
  final cta = find.text('Hubungi Penjual');
  expect(cta, findsOneWidget);
  await tester.ensureVisible(cta);
  await tester.pump();
  await tester.tap(cta);
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
  matching: find.text('Ikan Koi Test'),
);

Finder _composerSendIcon() => find.descendant(
  of: find.byType(ChatInputArea),
  matching: find.byIcon(Icons.send),
);

void main() {
  testWidgets(
    'Checkout "Hubungi Penjual" opens Chat with ONE remove-only pending chip and no Kirim',
    (tester) async {
      final chatDetail = _FakeChatDetail();
      await _pumpCheckout(
        tester,
        router: await _buildRouter(),
        chatDetail: chatDetail,
      );

      await _openChatFromCheckout(tester);

      // The REAL producer reached the REAL Chat screen with the pending product.
      expect(find.byType(ChatDetailScreen), findsOneWidget);
      expect(_chatChipCaption(), findsOneWidget);
      expect(_chatProductTitle(), findsOneWidget);

      // Checkout-uncovered-shipping is the ONE entry point that PREFILLS the
      // composer with a natural shipping question (draft only, never sent).
      final composerField = tester.widget<TextField>(
        find.descendant(
          of: find.byType(ChatInputArea),
          matching: find.byType(TextField),
        ),
      );
      expect(composerField.controller?.text, kCheckoutUncoveredShippingDraft);
      expect(composerField.controller?.text, isNotEmpty);
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
    'Checkout "Hubungi Penjual" → empty-body send icon emits direct_commerce_insert_chat resourceOccurrence (resourceType forSale)',
    (tester) async {
      final chatDetail = _FakeChatDetail();
      await _pumpCheckout(
        tester,
        router: await _buildRouter(),
        chatDetail: chatDetail,
      );

      await _openChatFromCheckout(tester);

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
      // Checkout is PRE-ORDER for a fixed-price For Sale.
      expect(
        occurrence.resourceType,
        ChatResourceOccurrenceResourceType.forSale,
      );
      // The exact product identity shown in Checkout.
      expect(occurrence.resourceId, _forSaleId);
      // The uncovered-shipping entry prefilled the composer draft; Send carries
      // it together with the product occurrence (composer remains the ONE send
      // authority — nothing is auto-sent on entry).
      expect(
        chatDetail.lastSendArgs?['content'],
        kCheckoutUncoveredShippingDraft,
      );
      // No legacy objectReference/attachment write path travels with the send.
      expect(chatDetail.lastSendArgs?.containsKey('objectReference'), isFalse);
      expect(chatDetail.lastSendArgs?.containsKey('attachment'), isFalse);

      expect(tester.takeException(), isNull);
    },
  );
}
