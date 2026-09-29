import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/domain/entities/negotiation.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/shared/widgets/composer_action_buttons.dart';
import 'package:labuda/shared/widgets/pending_media_strip.dart';

/// Chat Input Area Widget
///
/// Provides text input and attachment options for sending messages.
/// Composer commerce CTAs were removed together with the room-level
/// context — the composer never decides commerce actions (O4: chat is a
/// display layer only; enforced by the purge contract test in test/).
///
/// **CV2:** Enhanced with pending deal visibility, CTA clarity, and next-step guidance.
class ChatInputArea extends ConsumerStatefulWidget {
  final String chatId;
  final TextEditingController messageController;
  final Future<void> Function(String content, {MessageType type}) onSendMessage;
  final VoidCallback onAttachmentTap;

  /// True while a send is in flight, uploads included. Drives the send button's
  /// visible state: a locked, spinning button instead of a silent guard that
  /// swallowed the taps it was meant to prevent.
  final bool isSending;

  /// Re-runs the uploads the strip marks as failed. Only those files are
  /// re-uploaded — anything that already holds an asset id is skipped.
  final VoidCallback? onRetryUpload;

  /// True while the composer holds a pending commerce attachment. Enables
  /// resource-only sends: the always-visible send button stays enabled and
  /// submits even with an empty draft (attachment + optional text = one
  /// message).
  final bool hasPendingAttachment;

  /// Local attachments waiting to be uploaded at Send. Each item carries its own
  /// lifecycle, so the strip shows which file is uploading, done, or failed.
  final List<MediaPendingItem> pendingMedia;

  /// Removes one pending media item by index.
  final void Function(int index)? onRemovePendingMedia;

  const ChatInputArea({
    super.key,
    required this.chatId,
    required this.messageController,
    required this.onSendMessage,
    required this.onAttachmentTap,
    this.isSending = false,
    this.onRetryUpload,
    this.hasPendingAttachment = false,
    this.pendingMedia = const [],
    this.onRemovePendingMedia,
  });

  @override
  ConsumerState<ChatInputArea> createState() => _ChatInputAreaState();
}

class _ChatInputAreaState extends ConsumerState<ChatInputArea> {
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

  // Rebuild on every draft change so the always-visible send button can
  // toggle its disabled state (canonical composer action row).
  void _onTextChanged() {
    if (mounted) setState(() {});
  }

  Future<void> _handleSendMessage() async {
    final content = widget.messageController.text.trim();
    if (content.isEmpty && !widget.hasPendingAttachment) return;

    await widget.onSendMessage(content);

    if (mounted) {
      widget.messageController.clear();
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
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p8),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        boxShadow: [
          BoxShadow(
            color: Theme.of(
              context,
            ).colorScheme.shadow.withValues(alpha: 0.05),
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
            if (widget.pendingMedia.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(bottom: AppMetrics.p8),
                child: PendingMediaStrip(
                  items: widget.pendingMedia,
                  onRemove: (i) => widget.onRemovePendingMedia?.call(i),
                  onRetry: widget.onRetryUpload,
                ),
              ),
            // Canonical action row: [pill] [+] [send]. The `+` lives to the
            // right of the textarea (never left), send is always visible.
            Row(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Expanded(child: _buildTextField(context, canSend)),
                const SizedBox(width: 8),
                _buildAttachmentButton(context),
                const SizedBox(width: 8),
                _buildSendButton(canSend),
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
      statusColor = context.statusColors.success;
    } else {
      return const SizedBox.shrink();
    }

    return Container(
      margin: const EdgeInsets.only(bottom: AppMetrics.p8),
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p10),
      decoration: BoxDecoration(
        color: statusColor.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(AppShape.r10),
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
                        fontSize: AppType.s12,
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
                    fontSize: AppType.s10,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
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
      margin: const EdgeInsets.only(bottom: AppMetrics.p8),
      padding: const EdgeInsets.all(AppMetrics.p8),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainerHigh,
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Row(
        children: [
          Container(
            width: 4,
            height: 40,
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.primary,
              borderRadius: BorderRadius.circular(AppShape.r2),
            ),
          ),
          const SizedBox(width: 8),
          const Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Replying to...',
                  style: TextStyle(fontSize: AppType.s11, fontWeight: FontWeight.bold),
                ),
                Text(
                  'Message content preview...',
                  style: TextStyle(fontSize: AppType.s12),
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

  /// Attach entry (`+`) of the canonical composer action row — opens the
  /// media/commerce attachment sheet owned by the chat screen.
  Widget _buildAttachmentButton(BuildContext context) {
    return ComposerAddButton(onPressed: _handleAttachmentTap);
  }

  Widget _buildTextField(BuildContext context, bool canSend) {
    return TextField(
      controller: widget.messageController,
      maxLines: 4,
      minLines: 1,
      enabled: canSend,
      decoration: AppTheme.composerDecoration(
        Theme.of(context).colorScheme,
        hintText: 'Type a message...',
      ),
      textCapitalization: TextCapitalization.sentences,
      onSubmitted: canSend ? (_) => _handleSendMessage() : null,
    );
  }

  /// Canonical send: always visible, disabled until the draft holds content
  /// (or a pending attachment). Voice messages do not exist (kill order).
  Widget _buildSendButton(bool canSend) {
    final hasDraft = widget.messageController.text.trim().isNotEmpty;
    return ComposerSendButton(
      // Locked + spinner while in flight: the first tap must LOOK like it did
      // something, and a second must be impossible rather than swallowed.
      loading: widget.isSending,
      onPressed: (canSend && (hasDraft || widget.hasPendingAttachment))
          ? _handleSendMessage
          : null,
    );
  }
}
