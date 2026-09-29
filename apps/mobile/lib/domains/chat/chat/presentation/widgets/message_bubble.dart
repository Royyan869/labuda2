import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/src/localization/l10n_extension.dart';
// E4.3 — import AppColors directly (not via core/core.dart) to keep the
// dependency surface explicit. The chat-entities `MessageStatus` consumed by
// this widget must stay the single MessageStatus in scope.
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_identity_display.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_lifecycle_redaction.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_resource_projection_card.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/widgets/attachment_widget.dart' as widget_lib;
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';
import 'package:labuda/shared/widgets/app_image.dart';

/// Message Bubble Widget
///
/// Displays a single message in a chat.
/// Supports rendering commerce attachments (forSale, quote, negotiation).
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

  /// CTA "Beli Sekarang" on the resource projection card. Wired by the chat
  /// screen, which resolves checkout navigation (product id + trust gate).
  final VoidCallback? onProjectionBuy;
  final String? currentUserId;

  const MessageBubble({
    super.key,
    required this.message,
    required this.isFromUser,
    this.showAvatar = true,
    this.onLongPress,
    this.onTap,
    this.onNegotiate,
    this.onPurchase,
    this.onProjectionBuy,
    this.currentUserId,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Align(
      alignment: isFromUser ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        margin: EdgeInsets.only(
          left: isFromUser ? AppMetrics.p48 : AppMetrics.p8,
          right: isFromUser ? AppMetrics.p8 : AppMetrics.p48,
          bottom: AppMetrics.p4,
          top: AppMetrics.p4,
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
    final colorScheme = Theme.of(context).colorScheme;
    final backgroundColor = isFromUser
        ? colorScheme.primary
        : colorScheme.surfaceContainerHigh;

    final textColor = isFromUser
        ? colorScheme.onPrimary
        : colorScheme.onSurface;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p10),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(AppShape.r18),
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
            if (message.resourceProjection != null)
              _buildResourceProjection(context),
            if (message.type == MessageType.text)
              _buildTextMessage(context, textColor)
            else            if (message.type == MessageType.image)
              _buildImageMessage(context, textColor)
            else if (message.type == MessageType.video)
              _buildVideoMessage(context)
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

  Widget _buildImageMessage(BuildContext context, Color textColor) {
    if (message.mediaUrls.isNotEmpty) {
      final colorScheme = Theme.of(context).colorScheme;
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(AppShape.r12),
            child: AppImage(
              imageUrl: message.mediaUrls.first,
              width: double.maxFinite,
              height: 200,
              fit: BoxFit.cover,
              backgroundColor: colorScheme.surfaceContainerHighest,
              errorWidget: Container(
                width: double.maxFinite,
                height: 200,
                color: colorScheme.surfaceContainerHighest,
                child: const Icon(Icons.broken_image, size: 48),
              ),
            ),
          ),
          if (message.content.isNotEmpty) ...[
            const SizedBox(height: 8),
            _buildTextMessage(context, textColor),
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
        color: Theme.of(context).colorScheme.scrim,
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      child: Center(
        child: Icon(
          Icons.play_circle_outline,
          color: Theme.of(context).colorScheme.onPrimary,
          size: 48,
        ),
      ),
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
        Flexible(child: Text(fileName, style: const TextStyle(fontSize: AppType.s14))),
        const SizedBox(width: 8),
        const Icon(Icons.download, size: 20),
      ],
    );
  }

  Widget _buildSystemMessage(BuildContext context) {
    return Center(
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p6),
        decoration: BoxDecoration(
          color: Theme.of(context).colorScheme.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(AppShape.r12),
        ),
        child: Text(
          message.content,
          style: TextStyle(
            fontSize: AppType.s12,
            color: Theme.of(context).colorScheme.onSurfaceVariant,
            fontStyle: FontStyle.italic,
          ),
        ),
      ),
    );
  }

  Widget _buildReplyPreview(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final replyInk = isFromUser
        ? colorScheme.onPrimary.withValues(alpha: 0.7)
        : colorScheme.onSurfaceVariant;
    return Container(
      margin: const EdgeInsets.only(bottom: AppMetrics.p8),
      padding: const EdgeInsets.all(AppMetrics.p8),
      decoration: BoxDecoration(
        color: colorScheme.onSurface.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Row(
        children: [
          Container(
            width: 4,
            height: 40,
            decoration: BoxDecoration(
              color: isFromUser
                  ? colorScheme.onPrimary.withValues(alpha: 0.5)
                  : colorScheme.onSurface.withValues(alpha: 0.2),
              borderRadius: BorderRadius.circular(AppShape.r2),
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
                    fontSize: AppType.s11,
                    color: replyInk,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  message.content,
                  style: TextStyle(
                    fontSize: AppType.s12,
                    color: replyInk,
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
    // Handle ShareReference (forSale, auction, content)
    // ========================================================================
    if (message.objectReference != null) {
      // The canonical authority for a resource-bearing row is the
      // server-resolved projection. When it is present, the projection card is
      // the display and the client-cached transport preview is suppressed
      // (never two representations of the same resource).
      if (message.resourceProjection != null) {
        return const SizedBox.shrink();
      }
      // No projection (e.g. rows persisted before the projection authority
      // existed): render the transport snapshot the message already carries.
      // Display-only — no resolver call, no derived status, no money.
      return Padding(
        padding: const EdgeInsets.only(bottom: AppMetrics.p8),
        child: ObjectPreviewCard(
          reference: message.objectReference!,
          onTap: onTap,
          showTypeBadge: true,
        ),
      );
    }

    // ========================================================================
    // WORKFLOW PAYLOAD ATTACHMENTS
    // ========================================================================
    // Handle Negotiation, Shipping, Location attachments
    // These use the legacy widget_lib.AttachmentWidget
    // ========================================================================

    // Handle live backend Negotiation Proposal (initial / counter)
    // NEGOTIATION ATTACHMENT PURGE (Z3): negotiation_offer and
    // negotiation_result are forbidden legacy types with no producer.
    if (message.negotiationProposal != null) {
      return Padding(
        padding: const EdgeInsets.only(bottom: AppMetrics.p8),
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
        padding: const EdgeInsets.only(bottom: AppMetrics.p8),
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
        padding: const EdgeInsets.only(bottom: AppMetrics.p8),
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

  /// Renders the server-resolved resource projection — the resource this
  /// message is about. Display + navigation only; no Commerce business logic.
  Widget _buildResourceProjection(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppMetrics.p8),
      child: ChatResourceProjectionCard(
        resourceProjection: message.resourceProjection!,
        onBuy: onProjectionBuy,
      ),
    );
  }

  Widget _buildMessageFooter(BuildContext context, Color textColor) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          _formatTime(message.createdAt),
          style: TextStyle(
            fontSize: AppType.s10,
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
        iconColor = context.statusColors.info;
        break;
      case MessageStatus.failed:
        icon = Icons.error;
        iconColor = context.statusColors.error;
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
      padding: const EdgeInsets.only(left: AppMetrics.p4, top: AppMetrics.p2),
      child: Text(
        displayName,
        style: TextStyle(
          fontSize: AppType.s11,
          color: Theme.of(context).colorScheme.onSurfaceVariant,
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
