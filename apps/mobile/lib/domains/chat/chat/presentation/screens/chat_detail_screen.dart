import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/models/pending_commerce_attachment.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_identity_display.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_input_area.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/message_bubble.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat/chat_order_status_banner.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_lifecycle_redaction.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/core/media/media_upload_config.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';
import 'package:labuda/shared/providers/block_state_provider.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/create_for_sale_route_contract.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/checkout_intent.dart';
// Display-title source for the pending commerce chip: chat reads commerce
// data for DISPLAY only — commerce decisions (pricing, trust gate, checkout)
// stay behind openForSaleCheckout / the negotiation authority.
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/social/comment/presentation/widgets/commerce_resource_picker.dart';
import 'package:labuda/domains/social/comment/presentation/widgets/resource_identity.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/checkout_intent.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/shipping_quote_intent.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/widgets/negotiation_offer_sheet.dart';
import 'package:labuda/domains/user/profile/profile.dart' show userDataProvider;
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/support/presentation/utils/support_category_label.dart';
import 'package:labuda/domains/system/support/presentation/utils/support_status_label.dart';

@visibleForTesting
class ShippingQuoteCheckoutTarget {
  /// Non-null for for-sale path; null for auction path.
  final String? forSaleId;

  /// Non-null for auction path; null for for-sale path.
  final String? auctionId;

  const ShippingQuoteCheckoutTarget({this.forSaleId, this.auctionId});
}

/// Resolves the shipping-quote host SURFACE only: which commerce entry chat
/// must forward. Everything beyond the surface id — product id resolution,
/// seller trust gate, route construction — is Commerce's job, never chat's:
/// the for-sale path forwards to [openForSaleCheckout]; the auction winner
/// path forwards to the SAME shared Checkout via the bid-win auction intent
/// (`openAuctionCheckout` + `AuctionCheckoutIntent(bidWin: true)`), carrying
/// the quote + conversation provenance.
@visibleForTesting
Future<ShippingQuoteCheckoutTarget?> resolveShippingQuoteCheckoutTarget({
  required ShippingQuoteAttachment shippingQuote,
}) async {
  final linkedItemType = shippingQuote.linkedItemType.toLowerCase();
  if (linkedItemType == 'auction') {
    // Canonical identity: source_id = auction.id
    final auctionId = shippingQuote.linkedItemId.trim();
    if (auctionId.isEmpty) return null;
    return ShippingQuoteCheckoutTarget(auctionId: auctionId);
  }

  final forSaleId = shippingQuote.linkedItemId.trim();
  if (forSaleId.isEmpty) return null;

  return ShippingQuoteCheckoutTarget(forSaleId: forSaleId);
}

/// Resolves the seller shipping-quote target a PRODUCT BUBBLE refers to — the
/// identity + display title only — from the canonical, viewer-scoped resource
/// projection (LIVE for_sale / auction). Null when the message is not a LIVE
/// product bubble.
///
/// The product bubble is the Commerce CONTEXT and is valid regardless of who
/// sent it. Whether the action is OFFERED is a SEPARATE, server-decided
/// authorization ([viewerCanOfferShippingQuote]); this function never computes
/// ownership or eligibility.
@visibleForTesting
SellerShippingQuoteTarget? sellerShippingQuoteTargetForMessage(
  Message message,
) {
  final projection = message.resourceProjection;
  if (projection == null || !projection.isLive) return null;
  switch (projection.resourceType) {
    case ResourceProjectionType.fixedPriceSale:
      return SellerShippingQuoteTarget.forSale(
        forSaleId: projection.resourceId,
        title: projection.titleText,
      );
    case ResourceProjectionType.auction:
      return SellerShippingQuoteTarget.auction(
        auctionId: projection.resourceId,
        title: projection.titleText,
      );
    case ResourceProjectionType.profile:
    case ResourceProjectionType.content:
      return null;
  }
}

/// Canonical Commerce authorization for the product-bubble `⋮` →
/// "Penawaran Ongkir" action.
///
/// The authorization is the SERVER-PROJECTED viewer capability
/// `resource_projection.viewer_capabilities.can_manage`, evaluated by the SAME
/// Commerce authority that powers the product detail wire
/// (`EvaluateForSaleViewerCapabilities` / `EvaluateAuctionViewerCapabilities`
/// → `Role == "owner"`, i.e. viewer == product seller). The bubble SENDER and
/// the viewer's platform role are NEVER consulted; Chat computes no ownership.
///
/// Fail-closed: a product bubble without a LIVE projection, or whose projection
/// does not grant ownership to this viewer, exposes no action.
@visibleForTesting
bool viewerCanOfferShippingQuote(Message message) {
  final projection = message.resourceProjection;
  if (projection == null || !projection.isLive) return false;
  return projection.viewerCapabilities.canManage;
}

/// Pending commerce attachment held by the composer — the canonical
/// [PendingCommerceAttachment]: identity + display snapshot only.
/// Chat never stores commerce payload data; the server re-resolves the
/// viewer-aware projection at send.
///
/// Chat Detail Screen
///
/// **DOMAIN BOUNDARY:**
/// - This screen is a THIN UI LAYER - displays chat messages and handles user input
/// - Business logic is delegated to appropriate domain services
/// - Negotiation → NegotiationNotifier (features/negotiation/)
/// - Chat does NOT make commerce decisions - only triggers domain actions
/// - State is managed by providers (chatDetailProvider, etc)
///
/// **COMMERCE FLOW:**
/// 1. User triggers action (nego, quote, purchase) from UI
/// 2. UI calls the appropriate domain service directly
/// 3. Domain service processes and returns result
/// 4. UI displays result (success/error) and updates state
///
/// **DO NOT:**
/// - Add business decision logic here - delegate to domain services
/// - Make state mutations outside of providers
/// - Process pricing, availability, or validation - these belong in domain
class ChatDetailScreen extends ConsumerStatefulWidget {
  final String chatId;
  final String? initialMessage;

  /// Pending product attachment delivered by the canonical commerce chat
  /// opener (openCommerceChat). It seeds the SAME composer pending state used
  /// by `Lampirkan Produk`; the user sends it through the composer send icon
  /// only — the chip carries no send CTA.
  final PendingCommerceAttachment? pendingCommerce;

  /// Draft text delivered by a commerce opener that ALREADY knows why the user
  /// is entering Chat (the Checkout uncovered-shipping shortcut). It PRE-FILLS
  /// the composer textarea and is NEVER auto-sent — the composer send icon
  /// stays the one send authority. Every other Chat entry leaves this null.
  final String? draftMessage;

  const ChatDetailScreen({
    super.key,
    required this.chatId,
    this.initialMessage,
    this.pendingCommerce,
    this.draftMessage,
  });

  @override
  ConsumerState<ChatDetailScreen> createState() => _ChatDetailScreenState();
}

class _ChatDetailScreenState extends ConsumerState<ChatDetailScreen> {
  final ScrollController _scrollController = ScrollController();
  final TextEditingController _messageController = TextEditingController();

  // Concurrency guards to prevent race conditions
  bool _isLoadingData = false;
  bool _isLoadingMore = false;
  bool _isSendingMessage = false;

  // Pending commerce attachment (composer "Lampirkan Produk" and every chat
  // entry point) — identity + display snapshot only. Chat never resolves
  // commerce data beyond display; the resource is sent on Send as a
  // resourceOccurrence (O4: chat is a display layer that delegates to
  // commerce authority).
  PendingCommerceAttachment? _pendingCommerce;

  /// Local media waiting to be uploaded at Send (canonical deferred upload).
  final List<MediaPendingItem> _pendingMedia = [];

  @override
  void initState() {
    super.initState();
    // Seed the canonical pending attachment delivered by openCommerceChat so
    // the entry point lands in the SAME composer lifecycle as a picker attach.
    _pendingCommerce = widget.pendingCommerce;
    // Pre-fill the composer for the ONE entry point that knows the question the
    // buyer is about to ask (Checkout uncovered-shipping). It is a draft only:
    // the user still owns Send. No other entry passes a draft.
    final draft = widget.draftMessage;
    if (draft != null && draft.isNotEmpty) {
      _messageController.text = draft;
      _messageController.selection = TextSelection.collapsed(
        offset: draft.length,
      );
    }
    // Conversation open is issued after the first frame: chatDetailProvider is
    // autoDispose, so the canonical read must run once the screen's build has
    // attached its listener to the room state. Same ordering as the deep-link
    // send below.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _loadChatData();
    });
    _scrollController.addListener(_onScroll);

    // Send initial message if provided (for deep links)
    if (widget.initialMessage != null && widget.initialMessage!.isNotEmpty) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _sendInitialMessage();
      });
    }
  }

  Future<void> _sendInitialMessage() async {
    // Guard against concurrent calls
    if (_isSendingMessage) return;

    try {
      _isSendingMessage = true;
      final notifier = ref.read(chatDetailProvider(widget.chatId).notifier);
      final userId = ref.read(currentUserIdProvider);
      final user = _getCurrentUserName();

      final result = await notifier.sendMessage(
        senderId: userId,
        senderName: user,
        content: widget.initialMessage!,
      );

      if (result == null && mounted) {
        // Message send failed - show error to user
        final chatState = ref.read(chatDetailProvider(widget.chatId));
        // Backend-rejection handler (defense-in-depth): the backend stays
        // the single authority for EMAIL_VERIFICATION_REQUIRED.
        if (chatState.errorCode == 'EMAIL_VERIFICATION_REQUIRED') {
          if (!context.mounted) return;
          AppSnackBar.showError(
            context,
            'Verifikasi email kamu diperlukan sebelum mengirim pesan.',
          );
          return;
        }
        final error = chatState.error;
        final errorMessage = error?.toLowerCase().contains('blocked') ?? false
            ? 'Tidak dapat mengirim pesan. Anda telah diblokir oleh pengguna ini.'
            : 'Gagal mengirim pesan. Silakan coba lagi.';
        AppSnackBar.showError(context, errorMessage);
      }
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Gagal mengirim pesan. Coba lagi.');
      }
    } finally {
      _isSendingMessage = false;
    }
  }

  @override
  void dispose() {
    _scrollController.dispose();
    _messageController.dispose();
    super.dispose();
  }

  void _onScroll() {
    // Load more messages when scrolling to top
    if (_scrollController.position.pixels ==
        _scrollController.position.minScrollExtent) {
      _loadMoreMessages();
    }
  }

  Future<void> _loadChatData() async {
    // Guard against concurrent calls
    if (_isLoadingData) return;

    try {
      _isLoadingData = true;
      final notifier = ref.read(chatDetailProvider(widget.chatId).notifier);
      final userId = ref.read(currentUserIdProvider);
      await Future.wait<void>([
        notifier.loadChat(userId),
        notifier.loadMessages(userId),
      ]);

      // Opening a conversation marks it read through the canonical read
      // authority (POST /chat/rooms/:room_id/read → Service.MarkAsRead, plus the
      // notification read sync inside the same canonical notifier method).
      if (userId.isNotEmpty) {
        await notifier.markAsRead(userId);
      }

      // PER-ROOM negotiation session load — the sticky banner is scoped to
      // THIS room. Loading (and exact-CLEARING) on every open kills the
      // cross-room stale-session leak (copyWith(null) could not clear).
      await ref
          .read(negotiationNotifierProvider.notifier)
          .getNegotiation(chatRoomId: widget.chatId);
    } catch (e) {
      // Error will be reflected in state - UI will show error view
      // State already handles the error through notifier's error handling
      // No need to swallow here
    } finally {
      _isLoadingData = false;
    }
  }

  Future<void> _loadMoreMessages() async {
    // Guard against concurrent pagination requests
    if (_isLoadingMore) return;

    try {
      _isLoadingMore = true;
      final notifier = ref.read(chatDetailProvider(widget.chatId).notifier);
      final userId = ref.read(currentUserIdProvider);
      await notifier.loadMoreMessages(userId);
    } catch (e) {
      // Error will be reflected in state through notifier's error handling
      // Pagination errors are handled at state level
    } finally {
      _isLoadingMore = false;
    }
  }

  /// Brings the thread to its newest edge.
  ///
  /// The list renders with `reverse: true`, so the newest message is index 0 and
  /// sits at scroll offset 0 — the bottom edge is `minScrollExtent`. Animating
  /// to `maxScrollExtent` did the opposite: it scrolled to the OLDEST message at
  /// the top, landing the user in history right after they sent.
  void _scrollToBottom() {
    if (!_scrollController.hasClients) return;
    _scrollController.animateTo(
      _scrollController.position.minScrollExtent,
      duration: AppMotion.settled,
      curve: Curves.easeOut,
    );
  }

  @override
  Widget build(BuildContext context) {
    final chatDetailState = ref.watch(chatDetailProvider(widget.chatId));
    final chat = chatDetailState.chat;

    // Check if the other user is blocked
    String? otherUserId;
    if (chat != null && !chat.isSupportChat) {
      try {
        final currentUserId = ref.read(currentUserIdProvider);
        otherUserId = chat.getOtherParticipantId(currentUserId);
      } catch (_) {
        otherUserId = null;
      }
    }
    final isUserBlocked = otherUserId != null
        ? ref.watch(isUserBlockedProvider(otherUserId))
        : false;

    // Fetch order status when chat has linkedOrderId
    // Listen to order status stream instead of direct provider access
    if (chat?.linkedOrderId != null) {
      // Order status fetching is now handled by commerce domain through event bus
      // Chat UI subscribes to order status changes without direct dependency
    }

    return Scaffold(
      appBar: _buildAppBar(context, chatDetailState),
      body: Column(
        children: [
          // Order Status Banner (shows when chat has linkedOrderId)
          _buildOrderStatusBanner(context, chat?.linkedOrderId),

          // Blocked User Banner (shows when user is blocked)
          if (isUserBlocked) _buildBlockedUserBanner(context, otherUserId),
          Expanded(child: _buildMessagesList(context, chatDetailState)),

          // Pending product attachment chip — the ONE pending state for every
          // entry point (picker, For Sale detail, Auction detail, checkout).
          // Stays until removed or sent; never auto-sends and carries no send
          // CTA of its own.
          if (!isUserBlocked && _pendingCommerce != null)
            _buildCommerceAttachmentChip(context),
          // Disable input when user is blocked
          if (!isUserBlocked) _buildInputArea(context),
        ],
      ),
    );
  }

  PreferredSizeWidget _buildAppBar(
    BuildContext context,
    ChatDetailState state,
  ) {
    final chat = state.chat;

    return AppBar(
      title: _buildAppBarTitle(context, chat),
      actions: [
        if (chat?.isSupportChat == true)
          IconButton(
            icon: Icon(
              chat?.supportStatus == SupportStatus.resolved
                  ? Icons.check_circle
                  : Icons.help_outline,
              semanticLabel: 'Info dukungan',
            ),
            onPressed: () => _showSupportInfo(context, chat),
          )
        else
          IconButton(
            icon: const Icon(Icons.info_outline, semanticLabel: 'Info chat'),
            onPressed: () => _showChatInfo(context, chat),
          ),
      ],
    );
  }

  Widget _buildAppBarTitle(BuildContext context, Chat? chat) {
    if (chat == null) {
      return const Text('Loading...');
    }

    if (chat.isSupportChat) {
      // Support chat: the agent/admin avatar leads the title row. Admin is
      // never a seller — single personal avatar, no dual layout.
      final adminId = chat.assignedToAdmin;
      final adminAvatar = adminId != null
          ? chat.participantAvatars[adminId]
          : null;

      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          ProfileAvatar(userId: adminId ?? '', size: 36, imageUrl: adminAvatar),
          const SizedBox(width: 10),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              const Text('Support'),
              if (chat.assignedAdminName != null)
                Text(
                  'Agent: ${chat.assignedAdminName}',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
            ],
          ),
        ],
      );
    }

    try {
      final userId = ref.read(currentUserIdProvider);
      final otherUserName = chat.getOtherParticipantName(userId);
      final otherUserHandle = formatChatHandle(otherUserName);
      final otherUserId = chat.getOtherParticipantId(userId);
      final otherUserAvatar = chat.participantAvatars[otherUserId];
      // PRESENCE IS OPTIONAL DECORATION — IDENTITY IS CANONICAL.
      // A presence provider in error state (e.g. transport not wired) must
      // never collapse the whole title into the generic fallback; the peer
      // handle has to keep rendering. Isolate the watch.
      var isOnline = false;
      try {
        isOnline = ref.watch(isUserOnlineProvider(otherUserId));
      } catch (_) {
        isOnline = false;
      }

      // E4.3 — Participant lifecycle redaction in the chat appbar. When
      // the other participant is unavailable/removed:
      //   - Title text collapses to the redaction placeholder, rendered
      //     italic + muted to match the chat-card and message-bubble
      //     treatment.
      //   - The verification badge is suppressed (it is an identity-trust
      //     signal and must not affirm a redacted account, same doctrine
      //     as the "Respons Penjual" badge suppression in E3.1 comments).
      //   - The "Online" presence indicator is suppressed (no presence
      //     surfaced for a redacted identity).
      // Slot-persistence: the chat itself remains open and readable; only
      // the participant identity in the appbar is degraded.
      final otherLifecycle = chat.getOtherParticipantLifecycle(userId);
      final participantDegraded = otherLifecycle.isDegraded;
      final displayName = participantDegraded
          ? chatLifecycleRedactionLabel(otherLifecycle)
          : otherUserHandle;

      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          ProfileAvatar(
            userId: otherUserId,
            size: 36,
            // E4.3 parity: degraded identity never surfaces a network image.
            imageUrl: participantDegraded ? null : otherUserAvatar,
          ),
          const SizedBox(width: 10),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Flexible(
                    child: Text(
                      displayName,
                      overflow: TextOverflow.ellipsis,
                      style: participantDegraded
                          ? TextStyle(
                              fontStyle: FontStyle.italic,
                              color: Theme.of(
                                context,
                              ).colorScheme.onSurfaceVariant,
                            )
                          : null,
                    ),
                  ),
                  if (!participantDegraded) ...[
                    const SizedBox(width: 6),
                    // VERIFICATION UI: compact badge in chat header
                    _ChatVerificationBadge(userId: otherUserId),
                  ],
                ],
              ),
              if (isOnline && !participantDegraded)
                Text(
                  'Online',
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: context.statusColors.success,
                  ),
                ),
            ],
          ),
        ],
      );
    } catch (_) {
      return const Text('Chat');
    }
  }

  /// ForSale Context Banner
  ///
  /// Shows the active forSale context at the top of the chat.
  /// This helps users understand what forSale the chat is about.
  /// Only displays when context is a ShareReference with targetType.forSale.
  /// Order Status Banner
  ///
  /// Shows order status for linked orders in chat.
  /// Provides navigation to order detail screen for full information.
  /// This maintains commerce continuity after checkout/order creation.
  ///
  /// Simplified version - commerce provider integration removed
  Widget _buildOrderStatusBanner(BuildContext context, String? linkedOrderId) {
    if (linkedOrderId == null) {
      return const SizedBox.shrink();
    }

    // Placeholder banner - actual order status fetched by commerce domain
    return ChatOrderStatusBanner(
      orderId: linkedOrderId,
      status: null, // Status fetched by commerce domain
      paymentStatus: null, // Payment status fetched by commerce domain
      isLoading: false,
      onTap: () => _navigateToOrderDetail(linkedOrderId),
    );
  }

  Widget _buildMessagesList(BuildContext context, ChatDetailState state) {
    if (state.isLoading && state.messages.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (state.error != null && state.messages.isEmpty) {
      return _buildErrorView(state.error!);
    }

    final messages = state.messages;

    if (messages.isEmpty) {
      return _buildEmptyView(context);
    }

    // Resource display authority lives on each message: a row renders its
    // server-resolved projection when it has one, otherwise the transport
    // snapshot it already carries. Nothing is resolved client-side.
    return _MessageListWidget(
      messages: messages,
      hasMoreMessages: state.hasMoreMessages,
      scrollController: _scrollController,
      currentUserId: ref.read(currentUserIdProvider),
      onLongPress: _showMessageOptions,
      onForSaleTap: _navigateToForSaleDetail,
      onPurchase: (message) =>
          _handleCommerceAction(context, message, 'purchase'),
      onDealBuy: _handleDealBuy,
      onRetry: _handleRetrySend,
      // The `⋮` product action is authorized by the SERVER-PROJECTED viewer
      // Commerce capability (see viewerCanOfferShippingQuote), never by the
      // viewer's platform role.
      onOfferShippingQuote: _showProductBubbleActions,
    );
  }

  Widget _buildErrorView(String error) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.error_outline,
            size: AppIconSize.display,
            color: context.statusColors.error,
          ),
          const SizedBox(height: 16),
          Text(
            'Failed to load messages',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text(
            error,
            style: Theme.of(context).textTheme.bodyMedium,
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 16),
          ElevatedButton(onPressed: _loadChatData, child: const Text('Retry')),
        ],
      ),
    );
  }

  Widget _buildEmptyView(BuildContext context) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.chat_bubble_outline,
            size: AppIconSize.display,
            color: Theme.of(context).colorScheme.outline,
          ),
          const SizedBox(height: 16),
          Text(
            'No messages yet',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text(
            'Send a message to start the conversation',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildInputArea(BuildContext context) {
    return ChatInputArea(
      chatId: widget.chatId,
      messageController: _messageController,
      onSendMessage: _handleSendMessage,
      onAttachmentTap: _handleAttachmentTap,
      isSending: _isSendingMessage,
      hasPendingAttachment:
          _pendingCommerce != null || _pendingMedia.isNotEmpty,
      pendingMedia: _pendingMedia,
      onRemovePendingMedia: (i) => setState(() => _pendingMedia.removeAt(i)),
      onRetryUpload: _retryUploads,
    );
  }

  /// Re-runs the uploads the strip marks as failed. Only those files go again —
  /// anything already holding an asset id is skipped by the loop.
  Future<void> _retryUploads() async {
    if (_isSendingMessage || _pendingMedia.isEmpty) return;
    await MediaUploadOrchestrator.uploadPending(
      context: context,
      config: MediaUploadConfig.forChat,
      items: _pendingMedia,
      roomId: widget.chatId,
      onChanged: () {
        if (mounted) setState(() {});
      },
    );
  }

  Future<void> _handleSendMessage(
    String content, {
    MessageType type = MessageType.text,
  }) async {
    final pending = _pendingCommerce;
    final media = List<MediaPendingItem>.from(_pendingMedia);
    // Empty text is only valid as an attachment-only send.
    if (content.trim().isEmpty && pending == null && media.isEmpty) return;

    // Guard against double-tap / concurrent sends
    if (_isSendingMessage) return;

    try {
      _isSendingMessage = true;
      // Register + upload at Send: each file becomes a room-scoped PENDING
      // asset whose id the message attaches atomically. Nothing is attached
      // until the message exists, so an abandoned composer leaves no attached
      // media and the backend expires the pending asset on its own — a cancel
      // costs nothing. A failure stops here and keeps the composer intact.
      // ONE upload loop for every composer (orchestrator), not a private copy:
      // each file reports its own state to the strip, a failure on one file does
      // not strand the rest, and the batch is refused only if ANY file failed —
      // which is exactly what a retry re-runs.
      List<String> mediaAssetIds = const [];
      if (media.isNotEmpty) {
        final uploaded = await MediaUploadOrchestrator.uploadPending(
          context: context,
          config: MediaUploadConfig.forChat,
          items: media,
          roomId: widget.chatId,
          onChanged: () {
            if (mounted) setState(() {});
          },
        );
        if (!uploaded) return;
        mediaAssetIds = media
            .map((item) => item.assetId!)
            .toList(growable: false);
      }
      final notifier = ref.read(chatDetailProvider(widget.chatId).notifier);
      final userId = ref.read(currentUserIdProvider);
      final user = _getCurrentUserName();

      // The composer hands the draft over BEFORE the request: the optimistic
      // bubble appears the moment Send is tapped and can be retried from the
      // bubble itself. Media is released only now, because a failed UPLOAD has
      // no asset to retry with — that is the one path where the files must not
      // be dropped, and it returns above.
      if (mounted) {
        _messageController.clear();
        setState(() {
          _pendingMedia.clear();
          _pendingCommerce = null;
        });
      }

      final result = await notifier.sendMessage(
        senderId: userId,
        senderName: user,
        content: content,
        type: type,
        mediaAssetIds: mediaAssetIds,
        resourceOccurrence: pending?.toSendRequest(),
      );

      if (result != null) {
        _scrollToBottom();
      } else if (mounted) {
        // Message send failed - show error to user
        AppSnackBar.showError(
          context,
          'Gagal mengirim pesan. Silakan coba lagi.',
        );
      }
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Gagal mengirim pesan. Coba lagi.');
      }
    } finally {
      _isSendingMessage = false;
    }
  }

  /// Retries a failed send from its own bubble. The notifier replays the stored
  /// payload, so nothing has to be re-typed or re-picked.
  Future<void> _handleRetrySend(Message message) async {
    await ref
        .read(chatDetailProvider(widget.chatId).notifier)
        .retrySend(message.id);
  }

  String _getCurrentUserName() {
    // TODO: Get from auth provider
    return 'User';
  }

  void _handleAttachmentTap() {
    // "Lampirkan Produk" is a SELLER-only capability: a non-seller must never
    // see it (owner decision) — the same gate the comment composer applies.
    final authState = ref.read(authControllerProvider);
    final isSeller =
        authState is AuthStateAuthenticated &&
        PermissionHelper.canAccessSellerFeatures(authState.user);

    MediaUploadOrchestrator.showAttachSheet(
      context: context,
      config: MediaUploadConfig.forChat,
      current: MediaUploadOrchestrator.countsOfFiles(
        _pendingMedia.map((e) => e.file),
      ),
      extraActions: isSeller
          ? [
              MediaSheetAction(
                icon: Icons.storefront,
                label: 'Lampirkan Produk',
                subtitle: 'For Sale atau Lelang',
                onTap: _showCommercePicker,
              ),
            ]
          : const [],
      onPicked: (files) async {
        if (!mounted) return;
        setState(() => _pendingMedia.addAll(files.map(MediaPendingItem.new)));
      },
    );
  }

  /// Seller-only explicit Commerce intent: offer a manual shipping quote for
  /// the product this bubble refers to. The PRODUCT BUBBLE is the Commerce
  /// context — the seller does not pick a listing: the target is the product
  /// reference the bubble already carries, whether the buyer or the seller
  /// originally sent it. Chat forwards the intent to the Shipping domain entry
  /// ([openSellerShippingQuoteSheet]); the form, physical-product resolution,
  /// the wire and every business rule stay in Commerce.
  Future<void> _offerShippingQuote(SellerShippingQuoteTarget target) async {
    await openSellerShippingQuoteSheet(
      context: context,
      ref: ref,
      chatRoomId: widget.chatId,
      target: target,
      onSuccess: () async {
        if (mounted) await _loadChatData();
      },
    );
  }

  /// Opens the product-bubble action menu (`⋮` → More). The ONE canonical
  /// product-context action is "Penawaran Ongkir" for the product this bubble
  /// refers to. It is never a primary CTA on the card.
  Future<void> _showProductBubbleActions(Message message) async {
    // Defense-in-depth: the canonical projected ownership gate must hold even
    // if this handler is ever reached by another path.
    if (!viewerCanOfferShippingQuote(message)) return;
    final target = sellerShippingQuoteTargetForMessage(message);
    if (target == null) return;

    await AppBottomSheetActions.showActions<void>(
      context: context,
      actions: [
        BottomSheetAction<void>(
          title: 'Penawaran Ongkir',
          subtitle: 'Ongkir manual untuk pembeli ini',
          icon: Icons.local_shipping_outlined,
          onPressed: () {
            Navigator.of(context).pop();
            unawaited(_offerShippingQuote(target));
          },
        ),
      ],
    );
  }

  /// Opens the canonical commerce resource picker. A selection only becomes
  /// a pending composer attachment — nothing is sent until the user taps
  /// Send (pending-until-Send authority).
  Future<void> _showCommercePicker() async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) return;
    // Seller-only capability (defense in depth: the entry is not offered to
    // non-sellers either).
    if (!PermissionHelper.canAccessSellerFeatures(authState.user)) return;
    final currentUserId = authState.user.id;

    final selection = await CommerceResourcePicker.show(
      context,
      sellerId: currentUserId,
      selectedResourceId: _pendingCommerce?.resourceId,
      onCreateNewForSale: () async {
        Navigator.of(context).pop(); // close picker
        final result = await context.pushNamed(
          RouteNames.createForSale,
          extra: const CreateForSaleRouteArgs.chatDirectCommerce(),
        );
        if (!mounted) return;
        // Only the canonical CreatedForSaleResult attaches: null / raw
        // ForSale / unknown shapes attach nothing (route contract).
        if (result is CreatedForSaleResult) {
          await _attachCreatedForSale(result.forSaleId);
        }
      },
    );

    // Cancel/dismiss keeps any existing pending attachment untouched.
    if (!mounted || selection == null) return;
    setState(() {
      _pendingCommerce = PendingCommerceAttachment(
        resourceType: selection.resource.resourceType == ResourceType.auction
            ? ChatResourceOccurrenceResourceType.auction
            : ChatResourceOccurrenceResourceType.forSale,
        resourceId: selection.resource.resourceId,
        title: selection.title,
        imageUrl: selection.imageUrl,
        price: selection.price,
      );
    });
  }

  /// Resolves ONLY the display snapshot for a freshly created for-sale, then
  /// attaches it as the pending composer selection.
  Future<void> _attachCreatedForSale(String forSaleId) async {
    final result = await ref
        .read(forSaleControllerProvider)
        .getForSaleById(forSaleId);
    if (!mounted) return;
    final data = result.isSuccess ? result.data : null;
    setState(() {
      _pendingCommerce = PendingCommerceAttachment(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: forSaleId,
        title: data?.title ?? forSaleId,
        imageUrl: data != null && data.media.isNotEmpty
            ? data.media.first.originalUrl
            : null,
        price: data?.price.toInt(),
      );
    });
  }

  /// Pending commerce attachment chip above the composer: canonical
  /// [PendingCommerceChip], removable, never auto-sends.
  Widget _buildCommerceAttachmentChip(BuildContext context) {
    final pending = _pendingCommerce!;
    return Container(
      padding: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p8,
        AppMetrics.p16,
        AppMetrics.p0,
      ),
      child: PendingCommerceChip(
        title: pending.title,
        imageUrl: pending.imageUrl,
        price: pending.price,
        caption: 'Lampiran produk',
        onRemove: () {
          setState(() {
            _pendingCommerce = null;
          });
        },
      ),
    );
  }

  /// Navigate to for-sale detail screen when user taps on attachment
  ///
  /// **CANONICAL NAVIGATION PATH:** Always uses targetId from ShareReference (the canonical reference)
  /// to navigate to ForSaleDetailScreen. Preview data in attachment is NOT used
  /// for navigation - screen will fetch fresh data from backend.
  ///
  /// **FINAL CLEANUP:** Updated to accept ShareReference directly (extends Attachment).
  void _navigateToForSaleDetail(ShareReference item) {
    // Validate for-sale ID before navigation
    if (item.targetId.isEmpty) {
      if (mounted) {
        AppSnackBar.showError(context, 'Gagal membuka produk. Coba lagi.');
      }
      return;
    }

    try {
      context.push(RoutePaths.forSaleDetailPath(item.targetId));
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Gagal membuka forSale. Coba lagi.');
      }
    }
  }

  /// Navigate to Order Detail Screen
  ///
  /// Provides navigation from chat to order detail for:
  /// - Viewing full order status
  /// - Payment recovery (retry payment, change method)
  /// - Order actions (cancel, confirm receipt, etc.)
  ///
  /// **COMMERCE CONTINUITY:** This ensures users don't lose track of orders
  /// that originated from chat conversations.
  void _navigateToOrderDetail(String orderId) {
    // Validate orderId before navigation
    if (orderId.isEmpty) {
      if (mounted) {
        AppSnackBar.showError(context, 'Gagal membuka pesanan. Coba lagi.');
      }
      return;
    }

    try {
      // Canonical route keeps the location observable; the returned future
      // still resolves when the user comes back from order detail (order
      // status refresh is handled by the commerce domain through its event
      // bus, exactly as before).
      context.push(RoutePaths.orderDetailPath(orderId)).then((_) {});
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Gagal membuka pesanan. Coba lagi.');
      }
    }
  }

  Future<void> _handleShippingQuotePurchase(
    BuildContext context,
    ShippingQuoteAttachment shippingQuote,
  ) async {
    final target = await resolveShippingQuoteCheckoutTarget(
      shippingQuote: shippingQuote,
    );

    if (target == null) {
      if (!mounted) return;
      AppSnackBar.showError(this.context, 'Gagal membuka checkout');
      return;
    }

    if (target.auctionId != null) {
      // Auction shipping quote (settlement context): the canonical winner
      // path is the SAME shared Checkout with the bid-win intent. Chat
      // forwards the intent WITH the quote + conversation provenance; the
      // commerce authority resolves the auction, product id and trust gate,
      // and POST /orders consumes the quote via the ONE ShippingQuote
      // authority inside order creation.
      await openAuctionCheckout(
        this.context,
        ref,
        AuctionCheckoutIntent(
          auctionId: target.auctionId!,
          bidWin: true,
          shippingQuoteId: shippingQuote.offerId,
          chatId: widget.chatId,
        ),
      );
      return;
    }

    // N3 convergence: accepted negotiation of THIS room binds the deal
    // price into checkout (single authority: _acceptedNegotiationIdFor).
    _navigateToCheckout(
      target.forSaleId!,
      negotiationId: _acceptedNegotiationIdFor(target.forSaleId),
      shippingQuoteId: shippingQuote.offerId,
      chatId: widget.chatId,
    );
  }

  /// DEAL PRICE BINDING — pure forwarder. The rule ("which accepted
  /// session binds this product's deal price") belongs to the commerce
  /// negotiation authority; chat only carries the returned id to checkout.
  String? _acceptedNegotiationIdFor(String? forSaleId) {
    if (forSaleId == null || forSaleId.isEmpty) return null;
    try {
      return ref
          .read(negotiationNotifierProvider.notifier)
          .acceptedNegotiationIdFor(forSaleId);
    } catch (_) {
      // best-effort; negotiation provider unavailable → no binding
      return null;
    }
  }

  /// DEAL → checkout intent from the commerce proposal card: chat resolves
  /// NOTHING — the commerce authority owns product id, trust gate and pricing.
  void _handleDealBuy(Message message) {
    final att = message.negotiationProposal;
    final forSaleId = att?.resourceId;
    if (forSaleId == null || forSaleId.isEmpty) return;
    unawaited(
      openForSaleCheckout(
        context,
        ref,
        CheckoutIntent(
          forSaleId: forSaleId,
          negotiationId: _acceptedNegotiationIdFor(forSaleId),
        ),
      ),
    );
  }

  void _handleCommerceAction(
    BuildContext context,
    Message message,
    String action,
  ) {
    // **SHIPPING QUOTE FIX:** Handle shipping quote attachments
    if (message.shippingQuote != null && action == 'purchase') {
      unawaited(_handleShippingQuotePurchase(context, message.shippingQuote!));
      return;
    }

    if (message.objectReference == null) return;

    final shareReference = message.objectReference!;

    // **FINAL CLEANUP:** ShareReference is now the standard attachment type
    _handleShareReferenceAction(context, shareReference, action);
  }

  /// **FINAL CLEANUP:** Handle ShareReference directly (extends Attachment now)
  void _handleShareReferenceAction(
    BuildContext context,
    ShareReference shareRef,
    String action,
  ) {
    if (shareRef.targetType == ShareTargetType.forSale) {
      if (action == 'negotiate') {
        _showNegotiationDialogForShareReference(context, shareRef);
      } else if (action == 'purchase') {
        _navigateToCheckout(
          shareRef.targetId,
          negotiationId: _acceptedNegotiationIdFor(shareRef.targetId),
        );
      }
    }
  }

  /// **FINAL CLEANUP:** Nego intent for a ShareReference — the nominal form
  /// is the commerce-owned NegotiationOfferSheet (single form authority);
  /// chat only forwards the offer to the negotiation authority and reports
  /// the outcome. No dialog, no navigation (owner rule: chat never handles
  /// commerce).
  void _showNegotiationDialogForShareReference(
    BuildContext context,
    ShareReference shareRef,
  ) {
    if (shareRef.targetType != ShareTargetType.forSale) return;

    NegotiationOfferSheet.show(
      context: context,
      productTitle: shareRef.preview.title,
      onSubmit: (price) => _submitNegotiationOffer(shareRef.targetId, price),
    ).then((sent) {
      if (!sent || !mounted) return;
      AppSnackBar.showSuccess(this.context, 'Tawaran negosiasi terkirim');
    });
  }

  /// Forwards the offer to the commerce negotiation authority. `null` means
  /// accepted (the sheet closes); a string is the inline failure copy.
  Future<String?> _submitNegotiationOffer(String forSaleId, int price) async {
    try {
      final result = await ref
          .read(negotiationNotifierProvider.notifier)
          .createNegotiation(
            chatRoomId: widget.chatId,
            fixedPriceSaleId: forSaleId,
            price: price,
          );
      if (result.isSuccess) {
        // The proposal MESSAGE is produced asynchronously (outbox → chat
        // consumer), so it does not exist the instant this POST returns.
        // Reload once past the dispatch window so the card lands in the
        // already-open conversation instead of waiting for the next entry.
        unawaited(
          Future<void>.delayed(const Duration(seconds: 2), () {
            if (mounted) unawaited(_loadChatData());
          }),
        );
        return null;
      }
      return 'Gagal mengirim tawaran. Coba lagi.';
    } catch (_) {
      return 'Gagal mengirim tawaran. Coba lagi.';
    }
  }

  /// For-sale checkout is opened ONLY through the commerce-owned intent
  /// (openForSaleCheckout): product resolution, seller trust gate and pricing
  /// are commerce decisions. Chat forwards ids and context, nothing else.
  Future<void> _navigateToCheckout(
    String forSaleId, {
    String? negotiationId,
    String? shippingQuoteId,
    String? chatId,
  }) {
    return openForSaleCheckout(
      context,
      ref,
      CheckoutIntent(forSaleId: forSaleId, negotiationId: negotiationId),
      shippingQuoteId: shippingQuoteId,
      chatId: chatId,
    );
  }

  void _showMessageOptions(Message message) {
    final isFromUser = message.isFromUser(ref.read(currentUserIdProvider));

    AppBottomSheetActions.showActions<void>(
      context: context,
      actions: [
        // Report option (shown for messages from other users)
        if (!isFromUser)
          BottomSheetAction<void>(
            title: 'Report Message',
            icon: Icons.report,
            onPressed: () {
              Navigator.of(context).pop();
              _handleReportMessage(context, message);
            },
          ),
        BottomSheetAction<void>(
          title: 'Copy',
          icon: Icons.copy,
          onPressed: () {
            Navigator.of(context).pop();
            _handleCopyMessage(message);
          },
        ),
      ],
    );
  }

  /// Handle report message
  ///
  /// Allows reporting a specific message with context about the sender.
  Future<void> _handleReportMessage(
    BuildContext context,
    Message message,
  ) async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      if (mounted) {
        ref.read(navigationHandlerProvider).navigateToSignIn();
      }
      return;
    }

    // Get chat info for context
    final chat = ref.read(chatDetailProvider(widget.chatId)).chat;
    if (chat == null) return;

    // Open the canonical report destination with message context.
    // Canonical moderation (SLICE 2): chat_message is NOT a canonical target.
    // The message sender's user profile is the canonical report subject;
    // message context is carried in the report description.
    await context.push<bool>(
      RoutePaths.reportLocation(
        targetType: ReportTargetType.user.name,
        targetId: message.senderId,
      ),
    );
  }

  void _handleCopyMessage(Message message) {
    // TODO: Implement copy to clipboard
    AppSnackBar.showInfo(context, 'Pesan disalin');
  }

  void _showChatInfo(BuildContext context, Chat? chat) {
    if (chat == null) return;

    try {
      final userId = ref.read(currentUserIdProvider);
      final otherUserName = chat.getOtherParticipantName(userId);
      final otherUserHandle = formatChatHandle(otherUserName);
      final otherUserId = chat.getOtherParticipantId(userId);

      AppBottomSheetActions.showActions<void>(
        context: context,
        title: otherUserHandle,
        subtitle: 'ID: $otherUserId',
        actions: [
          BottomSheetAction<void>(
            title: 'Report User',
            icon: Icons.report,
            onPressed: () {
              Navigator.of(context).pop();
              _handleReportUser(context, otherUserId, otherUserHandle);
            },
          ),
          BottomSheetAction<void>(
            title: 'Block',
            icon: Icons.block,
            onPressed: () {
              Navigator.of(context).pop();
              _handleBlockUser(context, otherUserId, otherUserHandle);
            },
          ),
        ],
      );
    } catch (e) {
      // Failed to show chat info - log and optionally show to user
      if (mounted) {
        AppSnackBar.showError(context, 'Gagal memuat info chat');
      }
    }
  }

  /// Handle report user from chat context
  ///
  /// Allows reporting a user from chat with context about the conversation.
  /// The chat ID is included in the description for moderator context.
  Future<void> _handleReportUser(
    BuildContext context,
    String targetUserId,
    String targetUserName,
  ) async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      if (mounted) {
        ref.read(navigationHandlerProvider).navigateToSignIn();
      }
      return;
    }

    // Don't allow reporting yourself
    if (authState.user.id == targetUserId) {
      if (mounted) {
        AppSnackBar.showError(context, 'Tidak dapat melaporkan diri sendiri');
      }
      return;
    }

    // Open the canonical report destination with user context.
    final result = await context.push<bool>(
      RoutePaths.reportLocation(
        targetType: ReportTargetType.user.name,
        targetId: targetUserId,
      ),
    );

    // After reporting, offer to block the user for immediate protection
    if (result == true && mounted) {
      _showReportFollowUpDialog(this.context, targetUserId, targetUserName);
    }
  }

  /// Show follow-up dialog after reporting offering additional protection
  void _showReportFollowUpDialog(
    BuildContext context,
    String targetUserId,
    String targetUserName,
  ) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        icon: Icon(
          Icons.shield_outlined,
          color: Theme.of(context).colorScheme.primary,
          size: AppIconSize.display,
        ),
        title: const Text('Report Submitted'),
        content: Text(
          'Thank you for your report. Would you also like to block $targetUserName to prevent further contact?',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('No, Thanks'),
          ),
          FilledButton(
            onPressed: () {
              Navigator.pop(context);
              _handleBlockUser(context, targetUserId, targetUserName);
            },
            child: const Text('Block User'),
          ),
        ],
      ),
    );
  }

  void _showSupportInfo(BuildContext context, Chat? chat) {
    if (chat == null || !chat.isSupportChat) return;

    AppBottomSheetBase.show<void>(
      context: context,
      title: 'Support',
      padding: EdgeInsets.zero,
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          ListTile(
            title: Text(
              'Category: ${chat.supportCategory?.label(context.l10n) ?? 'N/A'}',
            ),
          ),
          ListTile(
            title: Text(
              'Status: ${chat.supportStatus?.label(context.l10n) ?? 'N/A'}',
            ),
          ),
          if (chat.assignedAdminName != null)
            ListTile(title: Text('Agent: ${chat.assignedAdminName}')),
          if (chat.linkedOrderId != null)
            ListTile(title: Text('Order ID: ${chat.linkedOrderId}')),
        ],
      ),
    );
  }

  /// Handle block user from chat
  ///
  /// Shows confirmation dialog and blocks the user if confirmed.
  /// After blocking, exits the chat screen to return to chat list.
  Future<void> _handleBlockUser(
    BuildContext context,
    String targetUserId,
    String targetDisplayName,
  ) async {
    final confirmed = await AppDialog.confirm(
      context: context,
      title: 'Block $targetDisplayName?',
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(
                Icons.visibility_off_outlined,
                size: AppIconSize.action,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  'You will not see content from this user',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: Theme.of(context).colorScheme.onSurface,
                    height: 1.3,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(
                Icons.person_off_outlined,
                size: AppIconSize.action,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  'This user will not be able to see your content',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: Theme.of(context).colorScheme.onSurface,
                    height: 1.3,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(
                Icons.chat_bubble_outline,
                size: AppIconSize.action,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  'Chat with this user will be hidden',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: Theme.of(context).colorScheme.onSurface,
                    height: 1.3,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(
                Icons.people_outline,
                size: AppIconSize.action,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  'Follow relationship will be removed',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: Theme.of(context).colorScheme.onSurface,
                    height: 1.3,
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
      confirmLabel: 'Block',
      cancelLabel: 'Cancel',
      intent: AppDialogIntent.destructive,
    );

    if (!confirmed || !mounted) return;

    final success = await ref
        .read(blockActionsProvider.notifier)
        .blockUser(
          targetUserId: targetUserId,
          targetDisplayName: targetDisplayName,
        );

    if (!mounted) return;

    if (success) {
      AppSnackBar.showSuccess(
        this.context,
        '$targetDisplayName has been blocked',
      );
      // Exit the chat screen after blocking
      if (Navigator.of(this.context).canPop()) {
        Navigator.of(this.context).pop();
      }
    } else {
      final error = ref.read(blockActionsProvider).error;
      AppSnackBar.showError(
        this.context,
        error ?? 'Gagal memblokir. Coba lagi.',
      );
    }
  }

  /// Build blocked user banner for chat screen
  ///
  /// Shows a banner when chatting with a blocked user, with an unblock option.
  /// After unblocking, invalidates the blocked users provider to refresh state.
  Widget _buildBlockedUserBanner(BuildContext context, String blockedUserId) {
    final chat = ref.read(chatDetailProvider(widget.chatId)).chat;
    final displayName = chat != null
        ? formatChatHandle(
            chat.getOtherParticipantName(ref.read(currentUserIdProvider)),
          )
        : 'this user';

    return BlockedUserBanner(
      displayName: displayName,
      onUnblock: () async {
        final success = await ref
            .read(blockActionsProvider.notifier)
            .unblockUser(targetUserId: blockedUserId);
        if (mounted && success) {
          AppSnackBar.showSuccess(this.context, '$displayName unblocked');
          // Invalidate blocked users provider to refresh state
          ref.invalidate(blockedUserIdsProvider);
        } else if (mounted) {
          final error = ref.read(blockActionsProvider).error;
          AppSnackBar.showError(
            this.context,
            error ?? 'Gagal membuka blokir. Coba lagi.',
          );
        }
      },
    );
  }
}

/// Chat Verification Badge Widget
///
/// Shows compact verification level badge in chat header
class _ChatVerificationBadge extends ConsumerWidget {
  final String userId;

  const _ChatVerificationBadge({required this.userId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Fetch user's verification data
    final userAsync = ref.watch(userDataProvider(userId));

    return userAsync.when(
      data: (user) {
        if (user == null) return const SizedBox.shrink();
        final isVerified =
            user.isEmailVerified ||
            (user.isPhoneVerified ?? false) ||
            (user.isIdVerified ?? false) ||
            (user.isFarmVerified ?? false);
        if (!isVerified) return const SizedBox.shrink();
        return Icon(
          Icons.verified,
          size: AppIconSize.inlineGlyph,
          color: context.statusColors.info,
        );
      },
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
    );
  }
}

/// Extension for DateTime comparison
extension DateTimeComparison on DateTime {
  bool isSameDate(DateTime other) {
    return year == other.year && month == other.month && day == other.day;
  }
}

/// Message list.
///
/// Ordered newest-first with `reverse: true`, so index 0 is the newest message
/// and is rendered at the BOTTOM — index + 1 is the OLDER neighbour.
///
/// One message per row. Resource-bearing rows are rendered from their own
/// payload only: the canonical `resource_projection` when the server resolved
/// one, else the transport snapshot the message already carries. The list makes
/// no per-item resource resolution — no detail-endpoint fan-out per page.
class _MessageListWidget extends ConsumerWidget {
  final List<Message> messages;
  final bool hasMoreMessages;
  final ScrollController scrollController;
  final String currentUserId;
  final Function(Message) onLongPress;
  final Function(ShareReference) onForSaleTap;
  final Function(Message) onPurchase;
  final void Function(Message) onDealBuy;
  final Future<void> Function(Message) onRetry;

  /// Screen-owned handler that resolves the bubble's Commerce target and
  /// forwards it. The per-bubble authorization is derived from the message's
  /// own server-projected viewer capability (see [viewerCanOfferShippingQuote]).
  final void Function(Message) onOfferShippingQuote;

  const _MessageListWidget({
    required this.messages,
    required this.hasMoreMessages,
    required this.scrollController,
    required this.currentUserId,
    required this.onLongPress,
    required this.onForSaleTap,
    required this.onPurchase,
    required this.onDealBuy,
    required this.onRetry,
    required this.onOfferShippingQuote,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return ListView.builder(
      controller: scrollController,
      reverse:
          true, // Backend already returns newest-first; render newest at bottom.
      itemCount: messages.length + (hasMoreMessages ? 1 : 0),
      itemBuilder: (context, index) {
        if (index == messages.length) {
          // Loading indicator for more messages
          return const Center(
            child: Padding(
              padding: EdgeInsets.all(AppMetrics.p16),
              child: CircularProgressIndicator(),
            ),
          );
        }

        final message = messages[index];
        // The list is newest-first, so index + 1 is the OLDER neighbour: the one
        // rendered ABOVE this message, not the "next" one.
        final olderMessage = index < messages.length - 1
            ? messages[index + 1]
            : null;

        try {
          final isFromUser = message.isFromUser(currentUserId);
          final showAvatar =
              olderMessage == null || olderMessage.senderId != message.senderId;
          final showDateHeader = _shouldShowDateHeader(message, olderMessage);

          // The `⋮` product action is offered ONLY when the SERVER-PROJECTED
          // viewer capability grants product ownership to this viewer AND the
          // bubble is a LIVE product context. The bubble SENDER is irrelevant.
          final productMenu =
              viewerCanOfferShippingQuote(message) &&
                  sellerShippingQuoteTargetForMessage(message) != null
              ? () => onOfferShippingQuote(message)
              : null;

          return Column(
            children: [
              if (showDateHeader) _buildDateHeader(context, message.createdAt),
              MessageBubble(
                message: message,
                isFromUser: isFromUser,
                showAvatar: showAvatar,
                onLongPress: () => onLongPress(message),
                onRetry: message.status == MessageStatus.failed
                    ? () => onRetry(message)
                    : null,
                onTap:
                    message.objectReference?.targetType ==
                            ShareTargetType.forSale &&
                        message.objectReference != null
                    ? () => onForSaleTap(message.objectReference!)
                    : null,
                currentUserId: currentUserId,
                onPurchase: message.hasAttachment
                    ? () => onPurchase(message)
                    : null,
                onDealBuy: () => onDealBuy(message),
                onProductMenu: productMenu,
              ),
            ],
          );
        } catch (_) {
          return const SizedBox.shrink();
        }
      },
    );
  }

  Widget _buildDateHeader(BuildContext context, DateTime date) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: AppMetrics.p8),
      child: Center(
        child: Container(
          padding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p12,
            vertical: AppMetrics.p4,
          ),
          decoration: BoxDecoration(
            color: Theme.of(context).colorScheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(AppShape.r12),
          ),
          child: Text(
            _formatDate(date),
            style: context.typeRoles.labelMicro.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ),
      ),
    );
  }

  bool _shouldShowDateHeader(Message current, Message? older) {
    if (older == null) return false;
    return !current.createdAt.isSameDate(older.createdAt);
  }

  /// Day-separator labels are presentation-specific to this chat surface
  /// (Today/Yesterday + calendar fallback). They intentionally do NOT flow
  /// through TimeFormatService: this is a day pill, not a relative fact.
  String _formatDate(DateTime dateTime) {
    final now = DateTime.now();
    final diff = now.difference(dateTime);

    if (diff.inDays == 0) {
      return 'Today';
    } else if (diff.inDays == 1) {
      return 'Yesterday';
    } else {
      return '${dateTime.day}/${dateTime.month}/${dateTime.year}';
    }
  }
}
