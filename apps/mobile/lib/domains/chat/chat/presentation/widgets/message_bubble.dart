import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/src/localization/l10n_extension.dart';
// E4.3 — import AppColors directly (not via core/core.dart) because the
// core umbrella re-exports core/src/websocket/chat_websocket_handler.dart
// which also defines `MessageStatus`, clashing with the chat-entities
// MessageStatus consumed by this widget.
import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_identity_display.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_lifecycle_redaction.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/widgets/attachment_widget.dart' as widget_lib;
import 'package:labuda/shared/object/object_preview.dart' as obj;
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';

/// Message Bubble Widget
///
/// Displays a single message in a chat.
/// Supports rendering commerce attachments (listing, quote, negotiation).
///
/// **TRUTH HARDENING:** Automatically fetches live status for commerce attachments
/// to display honest availability/bidding status.
class MessageBubble extends ConsumerWidget {
  final Message message;
  final bool isFromUser;
  final bool showAvatar;
  final VoidCallback? onLongPress;
  final VoidCallback? onTap;
  final VoidCallback? onNegotiate;
  final VoidCallback? onPurchase;
  final String? currentUserId;

  /// Pre-resolved live preview data (from batch provider)
  /// If provided, will be used directly without calling objectPreviewProvider
  final obj.ObjectPreview? preResolved;

  const MessageBubble({
    super.key,
    required this.message,
    required this.isFromUser,
    this.showAvatar = true,
    this.onLongPress,
    this.onTap,
    this.onNegotiate,
    this.onPurchase,
    this.currentUserId,
    this.preResolved,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Align(
      alignment: isFromUser ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        margin: EdgeInsets.only(
          left: isFromUser ? 48 : 8,
          right: isFromUser ? 8 : 48,
          bottom: 4,
          top: 4,
        ),
        child: Column(
          crossAxisAlignment: isFromUser
              ? CrossAxisAlignment.end
              : CrossAxisAlignment.start,
          children: [
            GestureDetector(
              onLongPress: onLongPress,
              child: _buildBubble(context, ref),
            ),
            if (showAvatar) _buildSenderInfo(context),
          ],
        ),
      ),
    );
  }

  Widget _buildBubble(BuildContext context, WidgetRef ref) {
    final backgroundColor = isFromUser
        ? Theme.of(context).colorScheme.primary
        : Colors.grey[200]!;

    final textColor = isFromUser
        ? Theme.of(context).colorScheme.onPrimary
        : Colors.grey[900]!;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(18),
      ),
      constraints: BoxConstraints(
        maxWidth: MediaQuery.of(context).size.width * 0.75,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (message.isHidden)
            _buildHiddenMessage(context, textColor)
          else ...[
            if (message.replyToId != null) _buildReplyPreview(context),
            if (message.hasAttachment) _buildAttachment(context, ref),
            if (message.type == MessageType.text)
              _buildTextMessage(context, textColor)
            else if (message.type == MessageType.image)
              _buildImageMessage(context)
            else if (message.type == MessageType.video)
              _buildVideoMessage(context)
            else if (message.type == MessageType.audio)
              _buildAudioMessage(context)
            else if (message.type == MessageType.file)
              _buildFileMessage(context)
            else if (message.type == MessageType.system)
              _buildSystemMessage(context),
          ],
          _buildMessageFooter(context, textColor),
        ],
      ),
    );
  }

  Widget _buildHiddenMessage(BuildContext context, Color textColor) {
    return Text(
      context.l10n.hiddenMessageByModerator,
      style: TextStyle(
        color: textColor.withValues(alpha: 0.9),
        fontStyle: FontStyle.italic,
      ),
    );
  }

  Widget _buildTextMessage(BuildContext context, Color textColor) {
    return SelectableText(message.content, style: TextStyle(color: textColor));
  }

  Widget _buildImageMessage(BuildContext context) {
    if (message.mediaUrls.isNotEmpty) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(12),
            child: Image.network(
              message.mediaUrls.first,
              width: double.maxFinite,
              fit: BoxFit.cover,
              errorBuilder: (context, error, stackTrace) {
                return Container(
                  width: double.maxFinite,
                  height: 200,
                  color: Colors.grey[300],
                  child: const Icon(Icons.broken_image, size: 48),
                );
              },
            ),
          ),
          if (message.content.isNotEmpty) ...[
            const SizedBox(height: 8),
            _buildTextMessage(context, Colors.white),
          ],
        ],
      );
    }
    return const SizedBox.shrink();
  }

  Widget _buildVideoMessage(BuildContext context) {
    return Container(
      width: double.maxFinite,
      height: 200,
      decoration: BoxDecoration(
        color: Colors.black,
        borderRadius: BorderRadius.circular(12),
      ),
      child: const Center(
        child: Icon(Icons.play_circle_outline, color: Colors.white, size: 48),
      ),
    );
  }

  Widget _buildAudioMessage(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        const Icon(Icons.play_circle_outline, size: 32),
        const SizedBox(width: 8),
        Container(
          width: 100,
          height: 4,
          decoration: BoxDecoration(
            color: Colors.white.withValues(alpha: 0.3),
            borderRadius: BorderRadius.circular(2),
          ),
        ),
        const SizedBox(width: 8),
        Text(
          '0:${message.content.length % 60}',
          style: const TextStyle(fontSize: 12),
        ),
      ],
    );
  }

  Widget _buildFileMessage(BuildContext context) {
    final fileName = message.content.isNotEmpty
        ? message.content
        : 'Attachment';

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        const Icon(Icons.attach_file, size: 20),
        const SizedBox(width: 8),
        Flexible(child: Text(fileName, style: const TextStyle(fontSize: 14))),
        const SizedBox(width: 8),
        const Icon(Icons.download, size: 20),
      ],
    );
  }

  Widget _buildSystemMessage(BuildContext context) {
    return Center(
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: Colors.grey[300],
          borderRadius: BorderRadius.circular(12),
        ),
        child: Text(
          message.content,
          style: TextStyle(
            fontSize: 12,
            color: Colors.grey[700],
            fontStyle: FontStyle.italic,
          ),
        ),
      ),
    );
  }

  Widget _buildReplyPreview(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.all(8),
      decoration: BoxDecoration(
        color: Colors.black.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Container(
            width: 4,
            height: 40,
            decoration: BoxDecoration(
              color: isFromUser
                  ? Colors.white.withValues(alpha: 0.5)
                  : Colors.black.withValues(alpha: 0.2),
              borderRadius: BorderRadius.circular(2),
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  _senderLabelForReplyPreview(),
                  style: TextStyle(
                    fontSize: 11,
                    color: isFromUser ? Colors.white70 : Colors.black54,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  message.content,
                  style: TextStyle(
                    fontSize: 12,
                    color: isFromUser ? Colors.white70 : Colors.black54,
                  ),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildAttachment(BuildContext context, WidgetRef ref) {
    if (!message.hasAttachment) return const SizedBox.shrink();

    // ========================================================================
    // OBJECT RESOLVER INTEGRATION (SAFE MODE)
    // ========================================================================
    // Handle ShareReference (listing, auction, content)
    // ========================================================================
    if (message.objectReference != null) {
      return Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: ObjectPreviewCard(
          reference: message.objectReference!,
          onTap: onTap,
          showTypeBadge: true,
          preResolved: preResolved,
        ),
      );
    }

    // ========================================================================
    // WORKFLOW PAYLOAD ATTACHMENTS
    // ========================================================================
    // Handle Negotiation, Shipping, Location attachments
    // These use the legacy widget_lib.AttachmentWidget
    // ========================================================================

    // Handle Negotiation attachments
    if (message.negotiationOffer != null) {
      return Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: widget_lib.AttachmentWidget(
          attachment: message.negotiationOffer!,
          isFromCurrentUser: isFromUser,
          onTap: onTap,
          onNegotiate: onNegotiate,
          onPurchase: onPurchase,
          currentUserId: currentUserId,
        ),
      );
    }

    if (message.negotiationResult != null) {
      return Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: widget_lib.AttachmentWidget(
          attachment: message.negotiationResult!,
          isFromCurrentUser: isFromUser,
          onTap: onTap,
          onNegotiate: onNegotiate,
          onPurchase: onPurchase,
          currentUserId: currentUserId,
        ),
      );
    }

    // Handle live backend Negotiation Proposal (initial / counter)
    if (message.negotiationProposal != null) {
      return Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: widget_lib.AttachmentWidget(
          attachment: message.negotiationProposal!,
          isFromCurrentUser: isFromUser,
          onTap: onTap,
          currentUserId: currentUserId,
        ),
      );
    }

    // Handle Shipping Quote attachment
    if (message.shippingQuote != null) {
      return Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: widget_lib.AttachmentWidget(
          attachment: message.shippingQuote!,
          isFromCurrentUser: isFromUser,
          onTap: onTap,
          onNegotiate: onNegotiate,
          onPurchase: onPurchase,
          currentUserId: currentUserId,
        ),
      );
    }

    // Handle Location attachment
    if (message.location != null) {
      return Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: widget_lib.AttachmentWidget(
          attachment: message.location!,
          isFromCurrentUser: isFromUser,
          onTap: onTap,
          onNegotiate: onNegotiate,
          onPurchase: onPurchase,
          currentUserId: currentUserId,
        ),
      );
    }

    return const SizedBox.shrink();
  }

  Widget _buildMessageFooter(BuildContext context, Color textColor) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          _formatTime(message.createdAt),
          style: TextStyle(
            fontSize: 10,
            color: textColor.withValues(alpha: 0.7),
          ),
        ),
        const SizedBox(width: 4),
        if (isFromUser) _buildMessageStatusIcon(context, textColor),
      ],
    );
  }

  Widget _buildMessageStatusIcon(BuildContext context, Color textColor) {
    IconData icon;
    Color iconColor = textColor.withValues(alpha: 0.7);

    switch (message.status) {
      case MessageStatus.sending:
        icon = Icons.schedule;
        iconColor = textColor.withValues(alpha: 0.5);
        break;
      case MessageStatus.sent:
        icon = Icons.done;
        break;
      case MessageStatus.delivered:
        icon = Icons.done_all;
        break;
      case MessageStatus.read:
        icon = Icons.done_all;
        iconColor = Colors.blue[300]!;
        break;
      case MessageStatus.failed:
        icon = Icons.error;
        iconColor = Colors.red[300]!;
        break;
    }

    return Icon(icon, size: 14, color: iconColor);
  }

  Widget _buildSenderInfo(BuildContext context) {
    // E4.3 — Message-sender lifecycle redaction. The bubble body remains
    // visible (slot-persistence: messages from removed/suspended users
    // are NOT hidden, only the sender identity is degraded). The footer
    // label switches to the canonical redaction placeholder, rendered
    // italic + muted to match the chat-card and appbar treatments.
    // Active / null / unknown lifecycle falls through to today's
    // rendering.
    final senderDegraded = message.senderLifecycle.isDegraded;
    final displayName = senderDegraded
        ? chatLifecycleRedactionLabel(message.senderLifecycle)
        : _senderLabel();

    return Padding(
      padding: const EdgeInsets.only(left: 4, top: 2),
      child: Text(
        displayName,
        style: TextStyle(
          fontSize: 11,
          color: senderDegraded ? AppColors.neutralGray500 : Colors.grey[600],
          fontStyle: senderDegraded ? FontStyle.italic : FontStyle.normal,
        ),
      ),
    );
  }

  String _senderLabel() {
    if (message.senderUsername.isNotEmpty) {
      return formatChatHandle(message.senderUsername);
    }

    if (message.type == MessageType.system) {
      return message.senderName;
    }

    return '';
  }

  String _senderLabelForReplyPreview() {
    final sender = _senderLabel();
    if (sender.isNotEmpty) return sender;
    if (message.type == MessageType.system) return message.senderName;
    return '';
  }

  String _formatTime(DateTime dateTime) {
    final hour = dateTime.hour.toString().padLeft(2, '0');
    final minute = dateTime.minute.toString().padLeft(2, '0');
    return '$hour:$minute';
  }
}
