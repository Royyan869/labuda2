import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/shared/object/object_preview.dart' as obj;
import 'package:labuda/shared/object/object_preview_batch_provider.dart';
import 'package:labuda/shared/object/object_reference.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_identity_display.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_input_area.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/message_bubble.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/typing_indicator.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/shipping_quote_creation_modal.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat/chat_order_status_banner.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_lifecycle_redaction.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/providers/block_state_provider.dart';
import 'package:labuda/shared/widgets/block_confirmation_dialog.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_picker_bottom_sheet.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/screens/create_for_sale_screen.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_providers.dart';
import 'package:labuda/domains/commerce/transaction/order/presentation/screens/order_detail_screen.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/domains/user/profile/profile.dart' show userDataProvider;
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/screens/report_screen.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/shipping_quote_dto.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/blocked_action_gate.dart';

@visibleForTesting
class ShippingQuoteCheckoutTarget {
  /// Non-null for for-sale path; null for auction path.
  final String? forSaleId;

  /// Non-null for auction path; null for for-sale path.
  final String? auctionId;

  /// Non-null for auction path — the physical product ID distinct from auctionId.
  final String? productId;

  const ShippingQuoteCheckoutTarget({
    this.forSaleId,
    this.auctionId,
    this.productId,
  });
}

@visibleForTesting
Future<ShippingQuoteCheckoutTarget?> resolveShippingQuoteCheckoutTarget({
  required ShippingQuoteAttachment shippingQuote,
  required Future<String?> Function(String auctionId) resolveAuctionProductId,
}) async {
  final linkedItemType = shippingQuote.linkedItemType.toLowerCase();
  if (linkedItemType == 'auction') {
    final auctionId = (shippingQuote.auctionId ?? shippingQuote.linkedItemId)
        .trim();
    if (auctionId.isEmpty) return null;

    // resolveAuctionProductId returns the physical product ID (auction.productId),
    // which is distinct from auctionId and must NOT be placed in forSaleId.
    final productId = (await resolveAuctionProductId(auctionId))?.trim();
    if (productId == null || productId.isEmpty) return null;

    return ShippingQuoteCheckoutTarget(
      auctionId: auctionId,
      productId: productId,
    );
  }

  final forSaleId = shippingQuote.linkedItemId.trim();
  if (forSaleId.isEmpty) return null;

  return ShippingQuoteCheckoutTarget(forSaleId: forSaleId);
}

@visibleForTesting
String resolveChatForSaleAttachmentId(ForSalePickerSelection selection) {
  return selection.forSaleId;
}

@visibleForTesting
CreateShippingQuoteRequestDto buildForSaleShippingQuoteRequest({
  required String productId,
  required String forSaleId,
  required int cost,
  String? note,
}) {
  return CreateShippingQuoteRequestDto(
    productId: productId,
    sourceType: 'for_sale',
    sourceId: forSaleId,
    cost: cost,
    note: note,
  );
}

/// Chat Detail Screen
///
/// **DOMAIN BOUNDARY:**
/// - This screen is a THIN UI LAYER - displays chat messages and handles user input
/// - Business logic is delegated to appropriate domain services
/// - Negotiation → NegotiationNotifier (features/negotiation/)
/// - Seller Quote → ChatCommerceProvider (chat-specific convenience)
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

  const ChatDetailScreen({
    super.key,
    required this.chatId,
    this.initialMessage,
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
  bool _isCreatingShippingQuote = false;

  @override
  void initState() {
    super.initState();
    _loadChatData();
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
        // Inline gate: backend rejected because the user's email is not
        // verified (HTTP 403 EMAIL_VERIFICATION_REQUIRED).
        if (chatState.errorCode == 'EMAIL_VERIFICATION_REQUIRED') {
          if (!context.mounted) return;
          await showBlockedActionGate(
            context,
            actionDescription: 'mengirim pesan',
          );
          return;
        }
        final error = chatState.error;
        final errorMessage = error?.toLowerCase().contains('blocked') ?? false
            ? 'Tidak dapat mengirim pesan. Anda telah diblokir oleh pengguna ini.'
            : 'Gagal mengirim pesan. Silakan coba lagi.';
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(errorMessage)));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Gagal mengirim pesan. Coba lagi.')),
        );
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

  void _scrollToBottom() {
    if (_scrollController.hasClients) {
      _scrollController.animateTo(
        _scrollController.position.maxScrollExtent,
        duration: const Duration(milliseconds: 300),
        curve: Curves.easeOut,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final chatDetailState = ref.watch(chatDetailProvider(widget.chatId));
    final typingIndicatorEnabled = ref.watch(typingIndicatorEnabledProvider);
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
          // For Sale Context Banner (shows when chat has for-sale context)
          _buildForSaleContextBanner(context, chat),
          // Blocked User Banner (shows when user is blocked)
          if (isUserBlocked) _buildBlockedUserBanner(context, otherUserId),
          Expanded(child: _buildMessagesList(context, chatDetailState)),
          if (typingIndicatorEnabled) _buildTypingIndicator(context),
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
            ),
            onPressed: () => _showSupportInfo(context, chat),
          )
        else
          IconButton(
            icon: const Icon(Icons.info_outline),
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
      return Column(
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
      );
    }

    try {
      final userId = ref.read(currentUserIdProvider);
      final otherUserName = chat.getOtherParticipantName(userId);
      final otherUserHandle = formatChatHandle(otherUserName);
      final otherUserId = chat.getOtherParticipantId(userId);
      final isOnline = ref.watch(presenceProvider).isUserOnline(otherUserId);

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

      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                displayName,
                style: participantDegraded
                    ? const TextStyle(
                        fontStyle: FontStyle.italic,
                        color: AppColors.neutralGray500,
                      )
                    : null,
              ),
              if (!participantDegraded) ...[
                const SizedBox(width: 6),
                // VERIFICATION UI: Show compact verification badge in chat header
                _ChatVerificationBadge(userId: otherUserId),
              ],
            ],
          ),
          if (isOnline && !participantDegraded)
            Text(
              'Online',
              style: Theme.of(
                context,
              ).textTheme.bodySmall?.copyWith(color: Colors.green),
            ),
        ],
      );
    } catch (_) {
      return const Text('Chat');
    }
  }

  /// Listing Context Banner
  ///
  /// Shows the active listing context at the top of the chat.
  /// This helps users understand what listing the chat is about.
  /// Only displays when context is a ShareReference with targetType.listing.
  ///
  /// **CV3:** Updated to emphasize purchase continuity - changed label from
  /// "Terkait Listing" to "Diskusi Pembelian" to make it clear this chat
  /// is part of the purchase flow, not just a generic conversation.
  ///
  /// **FINAL CLEANUP:** Updated to handle ShareReference (extends Attachment).
  Widget _buildForSaleContextBanner(BuildContext context, Chat? chat) {
    // Only show for ShareReference context with for-sale targetType
    final contextRef = chat?.context;
    if (contextRef == null ||
        contextRef.targetType != ShareTargetType.forSale) {
      return const SizedBox.shrink();
    }

    return Container(
      margin: const EdgeInsets.fromLTRB(12, 8, 12, 4),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: AppColors.primaryRed.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: AppColors.primaryRed.withValues(alpha: 0.25),
          width: 1,
        ),
      ),
      child: InkWell(
        onTap: () => _navigateToForSaleDetail(contextRef),
        borderRadius: BorderRadius.circular(12),
        child: Row(
          children: [
            // Thumbnail
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(
                color: Colors.grey[200],
                borderRadius: BorderRadius.circular(8),
              ),
              clipBehavior: Clip.antiAlias,
              child:
                  contextRef.preview.imageUrl != null &&
                      contextRef.preview.imageUrl!.isNotEmpty
                  ? Image.network(
                      contextRef.preview.imageUrl!,
                      fit: BoxFit.cover,
                      errorBuilder: (context, error, stackTrace) {
                        return const Icon(
                          Icons.storefront,
                          size: 24,
                          color: AppColors.neutralGray500,
                        );
                      },
                    )
                  : const Icon(
                      Icons.storefront,
                      size: 24,
                      color: AppColors.neutralGray500,
                    ),
            ),
            const SizedBox(width: 12),
            // Listing info
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        Icons.shopping_bag_outlined,
                        size: 14,
                        color: AppColors.primaryRed,
                      ),
                      const SizedBox(width: 4),
                      Text(
                        // **CV3:** Changed from "Terkait Listing" to "Diskusi Pembelian"
                        // to emphasize this is a purchase-related conversation
                        'Diskusi Pembelian',
                        style: TextStyle(
                          fontSize: 11,
                          color: AppColors.primaryRed,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 2),
                  Text(
                    contextRef.preview.title,
                    style: const TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w600,
                      color: AppColors.neutralGray900,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  // ShareReference only contains preview data (title, image), not price
                  // Price would need to be fetched from backend for display
                  Text(
                    'Lihat Detail',
                    style: const TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w500,
                      color: AppColors.primaryRed,
                    ),
                  ),
                ],
              ),
            ),
            // View listing icon
            Icon(Icons.chevron_right, color: AppColors.neutralGray400),
          ],
        ),
      ),
    );
  }

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

    // BATCH RESOLUTION: Use batch widget for all messages
    return _MessagesBatchWidget(
      messages: messages,
      hasMoreMessages: state.hasMoreMessages,
      scrollController: _scrollController,
      currentUserId: ref.read(currentUserIdProvider),
      onLongPress: _showMessageOptions,
      onForSaleTap: _navigateToForSaleDetail,
      onNegotiate: (message) =>
          _handleCommerceAction(context, message, 'negotiate'),
      onPurchase: (message) =>
          _handleCommerceAction(context, message, 'purchase'),
    );
  }

  Widget _buildErrorView(String error) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.error_outline, size: 64, color: Colors.red),
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
          Icon(Icons.chat_bubble_outline, size: 64, color: Colors.grey[400]),
          const SizedBox(height: 16),
          Text(
            'No messages yet',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text(
            'Send a message to start the conversation',
            style: Theme.of(
              context,
            ).textTheme.bodyMedium?.copyWith(color: Colors.grey[600]),
          ),
        ],
      ),
    );
  }

  Widget _buildTypingIndicator(BuildContext context) {
    return TypingIndicatorWidget(chatId: widget.chatId);
  }

  Widget _buildInputArea(BuildContext context) {
    final chat = ref.read(chatDetailProvider(widget.chatId)).chat;
    final hasForSaleContext =
        chat?.context?.targetType == ShareTargetType.forSale;

    // Check if current user is the seller (has market authority)
    final authState = ref.read(authControllerProvider);
    final isSeller =
        authState is AuthStateAuthenticated &&
        authState.user.hasMarketAuthority == true;

    return ChatInputArea(
      chatId: widget.chatId,
      messageController: _messageController,
      onSendMessage: _handleSendMessage,
      onAttachmentTap: _handleAttachmentTap,
      onStartNegotiation: hasForSaleContext
          ? () => _handleNegotiateFromInput()
          : null,
      // Direct checkout callback
      onBuyNow: hasForSaleContext ? () => _navigateToCheckoutFromInput() : null,
      // Shipping quote callback (only for sellers)
      onSendQuote: (hasForSaleContext && isSeller)
          ? () => _handleCreateShippingQuote(context)
          : null,
    );
  }

  /// Navigate to checkout from input area
  ///
  /// **SOCIAL FIX 1.1:** Now uses ShareReference instead of ListingAttachment.
  /// Provides direct checkout path from the commerce action bar.
  /// Preserves chat context for seamless return.
  void _navigateToCheckoutFromInput() {
    final chat = ref.read(chatDetailProvider(widget.chatId)).chat;
    if (chat?.context?.targetType != ShareTargetType.forSale) return;

    final forSaleId = chat?.context?.targetId;
    if (forSaleId != null) {
      _navigateToCheckout(forSaleId, returnToChat: true);
    }
  }

  /// **SOCIAL FIX 1.1:** Negotiation now uses ShareReference.
  void _handleNegotiateFromInput() {
    final chat = ref.read(chatDetailProvider(widget.chatId)).chat;
    if (chat?.context?.targetType == ShareTargetType.forSale) {
      _showNegotiationDialogForShareReference(context, chat!.context!);
    }
  }

  /// Handle create shipping quote action
  ///
  /// Shows modal for seller to input shipping cost and note,
  /// then calls API to create shipping quote.
  Future<void> _handleCreateShippingQuote(BuildContext context) async {
    // Guard against concurrent calls
    if (_isCreatingShippingQuote) return;

    final chat = ref.read(chatDetailProvider(widget.chatId)).chat;
    if (chat?.context?.targetType != ShareTargetType.forSale) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Tidak ada listing yang terkait dengan chat ini'),
          ),
        );
      }
      return;
    }

    final forSaleId = chat?.context?.targetId;
    if (forSaleId == null) return;

    // **VALIDATION:** Check for existing active shipping quotes for this item
    final chatState = ref.read(chatDetailProvider(widget.chatId));
    final hasActiveQuote = chatState.messages.any(
      (msg) =>
          msg.shippingQuote != null &&
          msg.shippingQuote!.linkedItemId == forSaleId &&
          msg.shippingQuote!.status.toLowerCase() == 'active',
    );

    if (hasActiveQuote) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text(
              'Anda sudah memiliki ongkir aktif untuk listing ini. Batalkan quote yang ada terlebih dahulu.',
            ),
            backgroundColor: AppColors.statusError,
          ),
        );
      }
      return;
    }

    // Show shipping quote creation modal
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (sheetContext) => Container(
        padding: MediaQuery.of(sheetContext).viewInsets,
        child: ShippingQuoteCreationModal(
          listingName: chat?.context?.preview.title ?? 'For Sale',
          onCreate: (cost, note) async {
            final messenger = ScaffoldMessenger.of(context);
            // Guard against concurrent calls
            if (_isCreatingShippingQuote) return;

            try {
              _isCreatingShippingQuote = true;

              // Create shipping quote via API
              final listing = await ref.read(
                forSaleDetailProvider(forSaleId).future,
              );
              final productId = listing?.productId?.trim();
              if (productId == null || productId.isEmpty) {
                throw StateError('Missing productId for forSaleId $forSaleId');
              }

              final request = buildForSaleShippingQuoteRequest(
                productId: productId,
                forSaleId: forSaleId,
                cost: cost,
                note: note,
              );

              // Call API through chat notifier
              final notifier = ref.read(
                chatDetailProvider(widget.chatId).notifier,
              );
              await notifier.createShippingQuote(request);

              // Refresh messages to show the new shipping quote message
              final userId = ref.read(currentUserIdProvider);
              await notifier.loadMessages(userId);

              if (mounted) {
                messenger.showSnackBar(
                  const SnackBar(
                    content: Text('Ongkir berhasil dikirim'),
                    backgroundColor: AppColors.successGreen,
                  ),
                );
              }
            } catch (e) {
              if (mounted) {
                messenger.showSnackBar(
                  SnackBar(
                    content: const Text('Gagal mengirim ongkir. Coba lagi.'),
                    backgroundColor: AppColors.statusError,
                  ),
                );
              }
              rethrow;
            } finally {
              _isCreatingShippingQuote = false;
            }
          },
        ),
      ),
    );
  }

  Future<void> _handleSendMessage(
    String content, {
    MessageType type = MessageType.text,
  }) async {
    if (content.trim().isEmpty) return;

    // Guard against double-tap / concurrent sends
    if (_isSendingMessage) return;

    try {
      _isSendingMessage = true;
      final notifier = ref.read(chatDetailProvider(widget.chatId).notifier);
      final userId = ref.read(currentUserIdProvider);
      final user = _getCurrentUserName();

      final result = await notifier.sendMessage(
        senderId: userId,
        senderName: user,
        content: content,
        type: type,
      );

      if (result != null) {
        _messageController.clear();
        _scrollToBottom();
      } else if (mounted) {
        // Message send failed - show error to user
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Gagal mengirim pesan. Silakan coba lagi.'),
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Gagal mengirim pesan. Coba lagi.')),
        );
      }
    } finally {
      _isSendingMessage = false;
    }
  }

  String _getCurrentUserName() {
    // TODO: Get from auth provider
    return 'User';
  }

  void _handleAttachmentTap() {
    final authState = ref.read(authControllerProvider);
    // **PHASE 1A AUTHORITY NORMALIZATION:**
    // Use hasMarketAuthority (capability check) instead of sellerBadge (deprecated tier indicator)
    // Seller-only options require active subscription capability
    final isSeller =
        authState is AuthStateAuthenticated &&
        authState.user.hasMarketAuthority == true;

    showModalBottomSheet(
      context: context,
      builder: (context) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Seller-only listing options
            if (isSeller) ...[
              ListTile(
                leading: Icon(
                  Icons.storefront,
                  color: Theme.of(context).colorScheme.primary,
                ),
                title: const Text('Kirim For Sale'),
                subtitle: const Text('Pilih for sale yang sudah ada'),
                onTap: () {
                  Navigator.pop(context);
                  _showForSalePicker();
                },
              ),
              ListTile(
                leading: Icon(
                  Icons.add_circle_outline,
                  color: Theme.of(context).colorScheme.primary,
                ),
                title: const Text('Buat For Sale Baru'),
                subtitle: const Text('Buat for sale dari chat ini'),
                onTap: () {
                  Navigator.pop(context);
                  _navigateToCreateForSale();
                },
              ),
              const Divider(),
            ],
            // Photo - disabled: no backend support
            // ListTile(
            //   leading: const Icon(Icons.photo_library),
            //   title: const Text('Photo'),
            //   onTap: () {
            //     Navigator.pop(context);
            //     // TODO: Implement photo picker
            //   },
            // ),
            // Camera - disabled: no backend support
            // ListTile(
            //   leading: const Icon(Icons.camera_alt),
            //   title: const Text('Camera'),
            //   onTap: () {
            //     Navigator.pop(context);
            //     // TODO: Implement camera
            //   },
            // ),
            // File - disabled: no backend support
            // ListTile(
            //   leading: const Icon(Icons.attach_file),
            //   title: const Text('File'),
            //   onTap: () {
            //     Navigator.pop(context);
            //     // TODO: Implement file picker
            //   },
            // ),
            // Location - disabled: no backend support
            // ListTile(
            //   leading: const Icon(Icons.location_on),
            //   title: const Text('Location'),
            //   onTap: () {
            //     Navigator.pop(context);
            //     // TODO: Implement location sharing
            //   },
            // ),
          ],
        ),
      ),
    );
  }

  /// Show for-sale picker for seller to attach existing item
  void _showForSalePicker() async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) return;

    await ForSalePickerBottomSheet.show(
      context,
      intent: ForSalePickerIntent.forSaleAttachment,
      selectedForSaleId: ref
          .read(chatDetailProvider(widget.chatId))
          .chat
          ?.context
          ?.targetId,
      onForSaleSelected: (selection) {
        _sendForSaleAttachment(selection.forSaleId);
      },
      onCreateNewForSale: () {
        _navigateToCreateForSale();
      },
    );
  }

  /// Navigate to create for-sale screen with chat context
  void _navigateToCreateForSale() async {
    final result = await Navigator.of(context).push<ForSale>(
      MaterialPageRoute(
        builder: (context) => const CreateForSaleScreen(origin: 'chat_context'),
      ),
    );

    // Auto-attach for-sale if created successfully
    if (result != null) {
      await _sendForSaleAttachment(result.forSaleId);
    }
  }

  Future<void> _sendForSaleAttachment(String forSaleId) async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) return;

    final senderId = authState.user.id;
    final senderName = authState.user.username.isNotEmpty
        ? authState.user.username
        : 'User';

    // **ARCHITECTURE FIX:** Create ShareReference with minimal preview data
    // Commerce domain provides actual data through separate flow
    final shareReference = ShareReference.forSale(
      forSaleId: forSaleId,
      title: 'Produk Dijual',
      imageUrl: null,
    );

    // **FINAL CLEANUP:** Send message with ShareReference as objectReference
    final chatNotifier = ref.read(chatDetailProvider(widget.chatId).notifier);
    await chatNotifier.sendMessage(
      senderId: senderId,
      senderName: senderName,
      content: 'Mengirimkan produk dijual untuk Anda',
      objectReference: shareReference,
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
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('For sale ID tidak valid')),
        );
      }
      return;
    }

    try {
      Navigator.of(context).push(
        MaterialPageRoute(
          builder: (context) => ForSaleDetailScreen(forSaleId: item.targetId),
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Gagal membuka listing. Coba lagi.')),
        );
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
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('Order ID tidak valid')));
      }
      return;
    }

    try {
      Navigator.of(context)
          .push(
            MaterialPageRoute(
              builder: (context) => OrderDetailScreen(orderId: orderId),
            ),
          )
          .then((_) {
            // Refresh chat data when returning from order detail
            // This ensures any status updates are reflected
            // Order status refresh is handled by commerce domain through event bus
          });
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Gagal membuka pesanan. Coba lagi.')),
        );
      }
    }
  }

  Future<void> _handleShippingQuotePurchase(
    BuildContext context,
    ShippingQuoteAttachment shippingQuote,
  ) async {
    try {
      final target = await resolveShippingQuoteCheckoutTarget(
        shippingQuote: shippingQuote,
        resolveAuctionProductId: (auctionId) async {
          final auction = await ref.read(
            auctionDetailProvider(auctionId).future,
          );
          return auction?.productId;
        },
      );

      if (target == null) {
        if (!mounted) return;
        ScaffoldMessenger.of(
          this.context,
        ).showSnackBar(const SnackBar(content: Text('Gagal membuka checkout')));
        return;
      }

      if (target.auctionId != null) {
        // Auction shipping quote: navigate with explicit auction identity.
        // source_type=auction, source_id=auctionId, product_id=productId (distinct).
        final productId = target.productId;
        if (productId == null || productId.isEmpty) {
          if (!mounted) return;
          ScaffoldMessenger.of(this.context).showSnackBar(
            const SnackBar(content: Text('Gagal membuka checkout')),
          );
          return;
        }
        if (!mounted) return;
        final queryParams = <String, String>{
          'product_id': productId,
          'auction_id': target.auctionId!,
          'shipping_quote_id': shippingQuote.offerId,
          'return_to_chat': widget.chatId,
        };
        final uri = Uri(
          path: '/checkout/${target.auctionId}',
          queryParameters: queryParams,
        );
        this.context.push(uri.toString());
      } else {
        _navigateToCheckout(
          target.forSaleId!,
          shippingQuoteId: shippingQuote.offerId,
          returnToChat: true,
        );
      }
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(
        this.context,
      ).showSnackBar(const SnackBar(content: Text('Gagal membuka checkout')));
    }
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
        _navigateToCheckout(shareRef.targetId);
      }
    }
  }

  /// **FINAL CLEANUP:** Show negotiation dialog for ShareReference
  /// NOTE: ShareReference only contains preview data (title, image), not price.
  /// Users will need to enter their desired price manually.
  void _showNegotiationDialogForShareReference(
    BuildContext context,
    ShareReference shareRef,
  ) {
    if (shareRef.targetType != ShareTargetType.forSale) return;

    final priceController = TextEditingController();
    final formKey = GlobalKey<FormState>();

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Negosiasi Harga'),
        content: Form(
          key: formKey,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                shareRef.preview.title,
                style: const TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w500,
                ),
              ),
              const SizedBox(height: 4),
              const Text(
                'Masukkan harga tawaran Anda',
                style: TextStyle(fontSize: 12, color: Colors.grey),
              ),
              const SizedBox(height: 16),
              TextFormField(
                controller: priceController,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(
                  labelText: 'Tawaran harga Anda',
                  prefixText: 'Rp ',
                  border: OutlineInputBorder(),
                ),
                validator: (value) {
                  if (value == null || value.isEmpty) {
                    return 'Masukkan harga tawaran';
                  }
                  final price = double.tryParse(value);
                  if (price == null) {
                    return 'Harga tidak valid';
                  }
                  if (price <= 0) {
                    return 'Harga harus lebih dari 0';
                  }
                  return null;
                },
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            onPressed: () {
              if (formKey.currentState!.validate()) {
                Navigator.of(context).pop();
                final price = double.tryParse(priceController.text) ?? 0;
                _startNegotiation(shareRef.targetId, price);
              }
            },
            child: const Text('Kirim Tawaran'),
          ),
        ],
      ),
    );
  }

  /// Start negotiation via chat-owned API endpoint.
  Future<void> _startNegotiation(String forSaleId, double price) async {
    try {
      final negotiationNotifier = ref.read(
        negotiationNotifierProvider.notifier,
      );

      final result = await negotiationNotifier.createNegotiation(
        chatRoomId: widget.chatId,
        fixedPriceSaleId: forSaleId,
        price: price.toInt(),
      );

      if (!mounted) return;

      if (result.isSuccess) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Tawaran negosiasi terkirim')),
        );
      } else {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Gagal mengirim tawaran. Coba lagi.')),
        );
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Gagal mengirim tawaran. Coba lagi.')),
      );
    }
  }

  void _navigateToCheckout(
    String forSaleId, {
    String? negotiationId,
    String? auctionId,
    String? shippingQuoteId,
    bool returnToChat = false,
  }) {
    // SELLER TRUST GATE: Best-effort check against cached data.
    // If the item is cached and seller is inactive, block navigation early.
    // Checkout screen (A3) and backend Guard 6 remain the authoritative checks.
    final listingAsync = ref.read(forSaleDetailProvider(forSaleId));
    final listing = listingAsync.value;
    if (listing != null &&
        listing.sellerTrustLifecycle != ContentLifecycle.active) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text(
            'Penjual tidak aktif — transaksi tidak dapat dilanjutkan',
          ),
        ),
      );
      return;
    }

    final productId = listing?.productId;
    if (productId == null || productId.isEmpty) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'ID produk belum tersedia untuk checkout ini',
        );
      }
      return;
    }

    // **CANONICAL FLOW:** Navigate to checkout with backend-authoritative commerce context
    // Backend will validate agreement and return pricing token for private price
    final queryParams = <String, String>{};
    queryParams['product_id'] = productId;
    if (negotiationId != null) {
      queryParams['negotiation_id'] = negotiationId;
    }
    if (auctionId != null) {
      queryParams['auction_id'] = auctionId;
    }
    // **SHIPPING QUOTE FIX:** Pass shipping quote ID to preserve quote context
    if (shippingQuoteId != null) {
      queryParams['shipping_quote_id'] = shippingQuoteId;
    }
    // Preserve chat context for seamless return
    if (returnToChat) {
      queryParams['return_to_chat'] = widget.chatId;
    }

    final uri = Uri(
      path: '/checkout/$forSaleId',
      queryParameters: queryParams.isEmpty ? null : queryParams,
    );

    context.push(uri.toString());
  }

  void _showMessageOptions(Message message) {
    final isFromUser = message.isFromUser(ref.read(currentUserIdProvider));

    showModalBottomSheet(
      context: context,
      builder: (context) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Report option (shown for messages from other users)
            if (!isFromUser) ...[
              ListTile(
                leading: const Icon(Icons.report, color: AppColors.primaryRed),
                title: const Text(
                  'Report Message',
                  style: TextStyle(color: AppColors.primaryRed),
                ),
                onTap: () {
                  Navigator.pop(context);
                  _handleReportMessage(context, message);
                },
              ),
              const Divider(),
            ],
            if (isFromUser) ...[
              // Edit - disabled: no backend support
              // ListTile(
              //   leading: const Icon(Icons.edit),
              //   title: const Text('Edit'),
              //   onTap: () {
              //     Navigator.pop(context);
              //     _handleEditMessage(message);
              //   },
              // ),
              const Divider(),
            ],
            ListTile(
              leading: const Icon(Icons.copy),
              title: const Text('Copy'),
              onTap: () {
                Navigator.pop(context);
                _handleCopyMessage(message);
              },
            ),
            // Reply - disabled: no backend support
            // ListTile(
            //   leading: const Icon(Icons.reply),
            //   title: const Text('Reply'),
            //   onTap: () {
            //     Navigator.pop(context);
            //     // TODO: Implement reply
            //   },
            // ),
          ],
        ),
      ),
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
        AppSnackBar.showError(context, 'Please login to report messages');
      }
      return;
    }

    // Get chat info for context
    final chat = ref.read(chatDetailProvider(widget.chatId)).chat;
    if (chat == null) return;

    // Navigate to report screen with message context
    // Canonical moderation (SLICE 2): chat_message is NOT a canonical target.
    // The message sender's user profile is the canonical report subject;
    // message context is carried in the report description.
    await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (context) => ReportScreen(
          targetType: ReportTargetType.user.name,
          targetId: message.senderId,
        ),
      ),
    );
  }

  void _handleCopyMessage(Message message) {
    // TODO: Implement copy to clipboard
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text('Copied: ${message.content}')));
  }

  void _showChatInfo(BuildContext context, Chat? chat) {
    if (chat == null) return;

    try {
      final userId = ref.read(currentUserIdProvider);
      final otherUserName = chat.getOtherParticipantName(userId);
      final otherUserHandle = formatChatHandle(otherUserName);
      final otherUserId = chat.getOtherParticipantId(userId);

      showModalBottomSheet(
        context: context,
        builder: (context) => SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ListTile(
                leading: const Icon(Icons.person),
                title: Text(otherUserHandle),
                subtitle: Text('ID: $otherUserId'),
              ),
              ListTile(
                leading: const Icon(Icons.report),
                title: const Text('Report User'),
                onTap: () {
                  Navigator.pop(context);
                  _handleReportUser(context, otherUserId, otherUserHandle);
                },
              ),
              ListTile(
                leading: const Icon(Icons.block, color: AppColors.primaryRed),
                title: const Text(
                  'Block',
                  style: TextStyle(color: AppColors.primaryRed),
                ),
                onTap: () {
                  Navigator.pop(context);
                  _handleBlockUser(context, otherUserId, otherUserHandle);
                },
              ),
            ],
          ),
        ),
      );
    } catch (e) {
      // Failed to show chat info - log and optionally show to user
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('Gagal memuat info chat')));
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
        AppSnackBar.showError(context, 'Please login to report users');
      }
      return;
    }

    // Don't allow reporting yourself
    if (authState.user.id == targetUserId) {
      if (mounted) {
        AppSnackBar.showError(context, 'Cannot report yourself');
      }
      return;
    }

    // Navigate to report screen with user context
    final result = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (context) => ReportScreen(
          targetType: ReportTargetType.user.name,
          targetId: targetUserId,
        ),
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
        icon: const Icon(
          Icons.shield_outlined,
          color: AppColors.primaryRed,
          size: 48,
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
            style: FilledButton.styleFrom(
              backgroundColor: AppColors.primaryRed,
            ),
            child: const Text('Block User'),
          ),
        ],
      ),
    );
  }

  void _showSupportInfo(BuildContext context, Chat? chat) {
    if (chat == null || !chat.isSupportChat) return;

    showModalBottomSheet(
      context: context,
      builder: (context) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              title: Text(
                'Category: ${chat.supportCategory?.name.toUpperCase() ?? 'N/A'}',
              ),
            ),
            ListTile(
              title: Text(
                'Status: ${_getSupportStatusLabel(chat.supportStatus)}',
              ),
            ),
            if (chat.assignedAdminName != null)
              ListTile(title: Text('Agent: ${chat.assignedAdminName}')),
            if (chat.linkedOrderId != null)
              ListTile(title: Text('Order ID: ${chat.linkedOrderId}')),
          ],
        ),
      ),
    );
  }

  String _getSupportStatusLabel(SupportStatus? status) {
    switch (status) {
      case SupportStatus.open:
        return 'OPEN';
      case SupportStatus.inProgress:
        return 'IN PROGRESS';
      case SupportStatus.waitingUser:
        return 'WAITING FOR YOU';
      case SupportStatus.resolved:
        return 'RESOLVED';
      case SupportStatus.closed:
        return 'CLOSED';
      default:
        return 'UNKNOWN';
    }
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
    final confirmed = await BlockConfirmationDialog.show(
      context,
      targetUserId: targetUserId,
      targetDisplayName: targetDisplayName,
    );

    if (confirmed != true || !mounted) return;

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
        return const Icon(
          Icons.verified,
          size: 16,
          color: AppColors.statusInfo,
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

/// Batch Messages Widget
///
/// Resolves all message attachments in one batch call instead of N individual calls.
/// Reduces API calls from N to 2-3 (listings + auctions).
class _MessagesBatchWidget extends ConsumerWidget {
  final List<Message> messages;
  final bool hasMoreMessages;
  final ScrollController scrollController;
  final String currentUserId;
  final Function(Message) onLongPress;
  final Function(ShareReference) onForSaleTap;
  final Function(Message) onNegotiate;
  final Function(Message) onPurchase;

  const _MessagesBatchWidget({
    required this.messages,
    required this.hasMoreMessages,
    required this.scrollController,
    required this.currentUserId,
    required this.onLongPress,
    required this.onForSaleTap,
    required this.onNegotiate,
    required this.onPurchase,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // STEP 1: Collect all ObjectReferences from message attachments
    final references = <ObjectReference>[];
    final messageMap = <String, Message>{};

    for (final message in messages) {
      if (message.objectReference != null) {
        // Convert attachment to ShareReference
        final shareReference = message.objectReference!;
        final ref = ObjectReference(
          type: shareReference.objectType,
          id: shareReference.targetId,
        );
        references.add(ref);
        messageMap[getCacheKey(ref)] = message;
      }
    }

    // STEP 2: Watch batch provider
    final batchPreviewsAsync = ref.watch(
      objectPreviewBatchProvider(references),
    );

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
              padding: EdgeInsets.all(16),
              child: CircularProgressIndicator(),
            ),
          );
        }

        final message = messages[index];
        final nextMessage = index < messages.length - 1
            ? messages[index + 1]
            : null;

        try {
          final isFromUser = message.isFromUser(currentUserId);
          final showAvatar =
              nextMessage == null || nextMessage.senderId != message.senderId;
          final showDateHeader = _shouldShowDateHeader(message, nextMessage);

          // STEP 3: Get pre-resolved data if available
          obj.ObjectPreview? preResolved;
          if (message.objectReference != null) {
            final shareReference = message.objectReference!;
            final cacheKey = getCacheKey(
              ObjectReference(
                type: shareReference.objectType,
                id: shareReference.targetId,
              ),
            );
            preResolved = batchPreviewsAsync.value?[cacheKey];
          }

          return Column(
            children: [
              if (showDateHeader) _buildDateHeader(context, message.createdAt),
              MessageBubble(
                message: message,
                isFromUser: isFromUser,
                showAvatar: showAvatar,
                onLongPress: () => onLongPress(message),
                onTap:
                    message.objectReference?.targetType ==
                            ShareTargetType.forSale &&
                        message.objectReference != null
                    ? () => onForSaleTap(message.objectReference!)
                    : null,
                currentUserId: currentUserId,
                onNegotiate: message.hasAttachment
                    ? () => onNegotiate(message)
                    : null,
                onPurchase: message.hasAttachment
                    ? () => onPurchase(message)
                    : null,
                preResolved: preResolved,
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
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Center(
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
          decoration: BoxDecoration(
            color: AppColors.neutralGray200,
            borderRadius: BorderRadius.circular(12),
          ),
          child: Text(
            _formatDate(date),
            style: const TextStyle(
              fontSize: 12,
              color: AppColors.neutralGray600,
            ),
          ),
        ),
      ),
    );
  }

  bool _shouldShowDateHeader(Message current, Message? next) {
    if (next == null) return false;
    return !current.createdAt.isSameDate(next.createdAt);
  }

  String _formatDate(DateTime dateTime) {
    final now = DateTime.now();
    final diff = now.difference(dateTime);

    if (diff.inDays == 0) {
      return 'Today';
    } else if (diff.inDays == 1) {
      return 'Yesterday';
    } else if (diff.inDays < 7) {
      return '${dateTime.day}/${dateTime.month}/${dateTime.year}';
    } else {
      return '${dateTime.day}/${dateTime.month}/${dateTime.year}';
    }
  }
}
