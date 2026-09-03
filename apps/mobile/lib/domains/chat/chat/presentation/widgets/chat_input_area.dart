import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/domain/entities/negotiation.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';

/// Chat Input Area Widget
///
/// Provides text input and attachment options for sending messages.
/// Shows commerce action buttons when fixed-price sale context exists.
///
/// **CV2:** Enhanced with pending deal visibility, CTA clarity, and next-step guidance.
class ChatInputArea extends ConsumerStatefulWidget {
  final String chatId;
  final TextEditingController messageController;
  final Future<void> Function(String content, {MessageType type}) onSendMessage;
  final VoidCallback onAttachmentTap;
  final VoidCallback? onSendQuote;
  final VoidCallback? onStartNegotiation;
  final VoidCallback? onBuyNow;

  const ChatInputArea({
    super.key,
    required this.chatId,
    required this.messageController,
    required this.onSendMessage,
    required this.onAttachmentTap,
    this.onSendQuote,
    this.onStartNegotiation,
    this.onBuyNow,
  });

  @override
  ConsumerState<ChatInputArea> createState() => _ChatInputAreaState();
}

class _ChatInputAreaState extends ConsumerState<ChatInputArea> {
  bool _isTyping = false;
  String? _replyToMessageId;

  @override
  void initState() {
    super.initState();
    widget.messageController.addListener(_onTextChanged);
  }

  @override
  void dispose() {
    widget.messageController.removeListener(_onTextChanged);
    super.dispose();
  }

  void _onTextChanged() {
    final isTyping = widget.messageController.text.isNotEmpty;
    if (_isTyping != isTyping) {
      setState(() {
        _isTyping = isTyping;
      });
    }
  }

  Future<void> _handleSendMessage() async {
    final content = widget.messageController.text.trim();
    if (content.isEmpty) return;

    await widget.onSendMessage(content);

    if (mounted) {
      widget.messageController.clear();
      setState(() {
        _isTyping = false;
      });
      _clearReply();
    }
  }

  void _handleAttachmentTap() {
    widget.onAttachmentTap();
  }

  void _clearReply() {
    setState(() {
      _replyToMessageId = null;
    });
  }

  @override
  Widget build(BuildContext context) {
    final chatDetailState = ref.watch(chatDetailProvider(widget.chatId));
    final chat = chatDetailState.chat;
    final canSend = chat?.status == ChatStatus.active;

    // Room-level context was removed from the backend.
    // Commerce actions that depended on chat.context are now message-level.

    // Watch negotiation state for pending deals
    // Negotiation state is now managed by NegotiationNotifier (domain entry point)
    final negotiationState = ref.watch(negotiationNotifierProvider);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 8),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.05),
            blurRadius: 4,
            offset: const Offset(0, -2),
          ),
        ],
      ),
      child: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            // Pending Deal Indicator (shown above commerce actions)
            if (negotiationState.currentNegotiation != null)
              _buildPendingDealIndicator(
                context,
                negotiationState.currentNegotiation!,
                isSeller: false,
              ),
            // Commerce actions gated by room-level context were removed;
            // per-message resource projections are the canonical source.
            if (_replyToMessageId != null) _buildReplyPreview(context),
            Row(
              children: [
                _buildAttachmentButton(context),
                const SizedBox(width: 8),
                Expanded(child: _buildTextField(context, canSend)),
                const SizedBox(width: 8),
                _buildSendButton(context, canSend),
              ],
            ),
          ],
        ),
      ),
    );
  }

  /// **EXECUTION WAVE CV2:** Pending Deal Indicator
  ///
  /// Shows a visual indicator when there's an active negotiation in progress.
  /// This keeps transaction momentum visible and reminds users of next steps.
  ///
  /// **CV2 HYGIENE:** Only shows for canonical active/accepted states.
  /// Hides indicator for terminal states (cancelled, expired).
  Widget _buildPendingDealIndicator(
    BuildContext context,
    Negotiation negotiation, {
    required bool isSeller,
  }) {
    final String statusLabel;
    final String nextStepHint;
    final Color statusColor;

    // **CANONICAL STATUS CHECK:** Use enum comparison directly
    // NegotiationStatus.active: negotiation in progress (can accept counter offers)
    // NegotiationStatus.accepted: seller accepted, ready for checkout
    // NegotiationStatus.cancelled/expired: terminal, hide indicator
    final status = negotiation.status;

    if (status.isTerminal) {
      // Terminal states: cancelled, expired - don't show indicator
      return const SizedBox.shrink();
    }

    if (status == NegotiationStatus.active) {
      statusLabel = isSeller
          ? 'Menunggu Respons Anda'
          : 'Menunggu Penjual Menjawab';
      nextStepHint = isSeller
          ? '• Terima atau tolak tawaran pembeli'
          : '• Tunggu respons penjual\n• Barang belum dikunci';
      statusColor = AppColors.coinPrimary;
    } else if (status == NegotiationStatus.accepted) {
      statusLabel = 'Harga Disetujui!';
      nextStepHint = '• Segera checkout untuk mengunci barang';
      statusColor = AppColors.successGreen;
    } else {
      return const SizedBox.shrink();
    }

    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: statusColor.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(
          color: statusColor.withValues(alpha: 0.4),
          width: 1.2,
        ),
      ),
      child: Row(
        children: [
          Container(
            width: 8,
            height: 8,
            decoration: BoxDecoration(
              color: statusColor,
              shape: BoxShape.circle,
              boxShadow: [
                BoxShadow(
                  color: statusColor.withValues(alpha: 0.4),
                  blurRadius: 4,
                  spreadRadius: 1,
                ),
              ],
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(
                      Icons.handshake_outlined,
                      size: 13,
                      color: statusColor,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      statusLabel,
                      style: TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w700,
                        color: statusColor,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 3),
                Text(
                  nextStepHint,
                  style: TextStyle(
                    fontSize: 10,
                    color: AppColors.neutralGray600,
                    height: 1.3,
                  ),
                ),
              ],
            ),
          ),
          Icon(
            Icons.chevron_right,
            size: 16,
            color: statusColor.withValues(alpha: 0.6),
          ),
        ],
      ),
    );
  }

  Widget _buildReplyPreview(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.all(8),
      decoration: BoxDecoration(
        color: Colors.grey[200],
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Container(
            width: 4,
            height: 40,
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.primary,
              borderRadius: BorderRadius.circular(2),
            ),
          ),
          const SizedBox(width: 8),
          const Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Replying to...',
                  style: TextStyle(fontSize: 11, fontWeight: FontWeight.bold),
                ),
                Text(
                  'Message content preview...',
                  style: TextStyle(fontSize: 12),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          IconButton(
            icon: const Icon(Icons.close, size: 16),
            onPressed: _clearReply,
            padding: EdgeInsets.zero,
            constraints: const BoxConstraints(),
          ),
        ],
      ),
    );
  }

  /// Enhanced Commerce Actions
  ///
  /// Shows fixed-price sale context with improved CTA clarity:
  /// - Visual prioritization of actions
  Widget _buildAttachmentButton(BuildContext context) {
    return IconButton(
      icon: Icon(
        Icons.add_circle,
        color: Theme.of(context).colorScheme.primary,
        size: 28,
      ),
      onPressed: _handleAttachmentTap,
    );
  }

  Widget _buildTextField(BuildContext context, bool canSend) {
    return Container(
      decoration: BoxDecoration(
        color: Colors.grey[100],
        borderRadius: BorderRadius.circular(24),
      ),
      child: TextField(
        controller: widget.messageController,
        maxLines: 4,
        minLines: 1,
        enabled: canSend,
        decoration: const InputDecoration(
          hintText: 'Type a message...',
          border: InputBorder.none,
          contentPadding: EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        ),
        textCapitalization: TextCapitalization.sentences,
        onSubmitted: canSend ? (_) => _handleSendMessage() : null,
      ),
    );
  }

  Widget _buildSendButton(BuildContext context, bool canSend) {
    return IconButton(
      icon: Icon(
        _isTyping ? Icons.send : Icons.mic,
        color: canSend ? Theme.of(context).colorScheme.primary : Colors.grey,
        size: 28,
      ),
      onPressed: canSend && _isTyping ? _handleSendMessage : null,
    );
  }
}
