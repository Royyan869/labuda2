import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_identity_display.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_lifecycle_redaction.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';
import 'package:timeago/timeago.dart' as timeago;

/// Chat Card Widget
///
/// Displays a single chat item in the chat list.
class ChatCard extends ConsumerWidget {
  final Chat chat;
  final VoidCallback onTap;
  final VoidCallback? onLongPress;

  const ChatCard({
    super.key,
    required this.chat,
    required this.onTap,
    this.onLongPress,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final currentUserId = ref.watch(currentUserIdProvider);

    if (chat.isSupportChat) {
      return _buildSupportChatCard(context, currentUserId);
    }

    return _buildPrivateChatCard(context, currentUserId);
  }

  Widget _buildPrivateChatCard(BuildContext context, String currentUserId) {
    final otherUserId = chat.getOtherParticipantId(currentUserId);
    final otherUserName = chat.getOtherParticipantName(currentUserId);
    final otherUserHandle = formatChatHandle(otherUserName);
    final otherUserAvatar = chat.participantAvatars[otherUserId];
    final unreadCount = chat.getUnreadCount(currentUserId);

    // E4.3 — Chat-participant lifecycle redaction. Slot-persistence is
    // preserved: the chat room remains tappable (the InkWell still opens
    // the conversation), only the participant identity collapses to the
    // redaction placeholder + neutral avatar + muted styling. Active /
    // null / unknown lifecycle falls through to today's rendering.
    final otherLifecycle = chat.getOtherParticipantLifecycle(currentUserId);
    final participantDegraded = otherLifecycle.isDegraded;
    final participantDisplayName = participantDegraded
        ? chatLifecycleRedactionLabel(otherLifecycle)
        : otherUserHandle;

    return InkWell(
      onTap: onTap,
      onLongPress: onLongPress,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        decoration: BoxDecoration(
          border: Border(
            bottom: BorderSide(color: Colors.grey[200]!, width: 0.5),
          ),
        ),
        child: Row(
          children: [
            _buildAvatar(
              otherUserAvatar,
              otherUserName,
              degraded: participantDegraded,
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _buildHeader(
                    context,
                    participantDisplayName,
                    degraded: participantDegraded,
                  ),
                  const SizedBox(height: 4),
                  _buildLastMessage(context, currentUserId),

                ],
              ),
            ),
            const SizedBox(width: 8),
            _buildTrailing(context, unreadCount),
          ],
        ),
      ),
    );
  }

  Widget _buildSupportChatCard(BuildContext context, String currentUserId) {
    final unreadCount = chat.unreadCounts.values.fold(
      0,
      (sum, count) => sum + count,
    );

    return InkWell(
      onTap: onTap,
      onLongPress: onLongPress,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        decoration: BoxDecoration(
          color: chat.supportStatus == SupportStatus.open
              ? Colors.blue[50]
              : null,
          border: Border(
            bottom: BorderSide(color: Colors.grey[200]!, width: 0.5),
          ),
        ),
        child: Row(
          children: [
            _buildSupportAvatar(context),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _buildSupportHeader(context),
                  const SizedBox(height: 4),
                  _buildLastMessage(context, currentUserId),
                  if (chat.supportCategory != null) ...[
                    const SizedBox(height: 4),
                    _buildSupportCategoryChip(context),
                  ],
                ],
              ),
            ),
            const SizedBox(width: 8),
            _buildTrailing(context, unreadCount),
          ],
        ),
      ),
    );
  }

  Widget _buildAvatar(
    String? avatarUrl,
    String userName, {
    bool degraded = false,
  }) {
    // E4.3 — Degraded participants always render the neutral fallback
    // (no NetworkImage of a redacted account, no initials derived from
    // the redaction placeholder). Matches the E3.1 comment-author and
    // E2.1 feed-author redaction visuals.
    if (degraded) {
      return const CircleAvatar(
        radius: 28,
        backgroundColor: AppColors.neutralGray200,
        child: Icon(
          Icons.person_off_outlined,
          color: AppColors.neutralGray500,
          size: 28,
        ),
      );
    }

    final initial = userName.isNotEmpty ? userName[0].toUpperCase() : '?';

    return CircleAvatar(
      radius: 28,
      backgroundColor: Colors.blue[100],
      backgroundImage: avatarUrl != null && avatarUrl.isNotEmpty
          ? NetworkImage(avatarUrl)
          : null,
      child: avatarUrl == null || avatarUrl.isEmpty
          ? Text(
              initial,
              style: TextStyle(
                color: Colors.blue[800],
                fontWeight: FontWeight.bold,
                fontSize: 20,
              ),
            )
          : null,
    );
  }

  Widget _buildSupportAvatar(BuildContext context) {
    return CircleAvatar(
      radius: 28,
      backgroundColor: Theme.of(context).colorScheme.primaryContainer,
      child: Icon(
        Icons.support_agent,
        color: Theme.of(context).colorScheme.onPrimaryContainer,
      ),
    );
  }

  Widget _buildHeader(
    BuildContext context,
    String userName, {
    bool degraded = false,
  }) {
    return Row(
      children: [
        Expanded(
          child: Text(
            userName,
            style: TextStyle(
              fontWeight: FontWeight.w600,
              fontSize: 16,
              // E4.3 — Degraded identity: italic + muted color so the
              // redaction placeholder is visually distinct from a real
              // username. Matches the E3.1 comment-author treatment.
              fontStyle: degraded ? FontStyle.italic : FontStyle.normal,
              color: degraded ? AppColors.neutralGray500 : null,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        if (chat.updatedAt != null) _buildTimestamp(),
      ],
    );
  }

  Widget _buildSupportHeader(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Support',
                style: const TextStyle(
                  fontWeight: FontWeight.w600,
                  fontSize: 16,
                ),
              ),
              if (chat.assignedAdminName != null)
                Text(
                  'Agent: ${chat.assignedAdminName}',
                  style: TextStyle(fontSize: 12, color: Colors.grey[600]),
                ),
            ],
          ),
        ),
        if (chat.updatedAt != null) _buildTimestamp(),
      ],
    );
  }

  Widget _buildTimestamp() {
    final timeStr = chat.updatedAt != null
        ? timeago.format(chat.updatedAt!)
        : timeago.format(chat.createdAt);

    return Text(
      timeStr,
      style: TextStyle(fontSize: 12, color: Colors.grey[600]),
    );
  }

  Widget _buildLastMessage(BuildContext context, String currentUserId) {
    if (chat.lastMessage == null) {
      return Text(
        'No messages yet',
        style: TextStyle(fontSize: 14, color: Colors.grey[500]),
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
      );
    }

    final message = chat.lastMessage!;
    if (message.isHidden) {
      return Text(
        context.l10n.hiddenMessageByModerator,
        style: TextStyle(fontSize: 14, color: Colors.grey[500]),
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
      );
    }

    final prefix = message.isFromUser(currentUserId) ? 'You: ' : '';

    return Text(
      '$prefix${_getMessagePreview(message)}',
      style: TextStyle(fontSize: 14, color: Colors.grey[700]),
      maxLines: 2,
      overflow: TextOverflow.ellipsis,
    );
  }

  String _getMessagePreview(Message message) {
    switch (message.type) {
      case MessageType.image:
        return '📷 Photo';
      case MessageType.video:
        return '🎥 Video';
      case MessageType.audio:
        return '🎤 Audio';
      case MessageType.file:
        return '📎 File';
      case MessageType.system:
        return message.content;
      case MessageType.negotiationProposal:
        return '💰 Nego';
      case MessageType.shippingQuote:
        return '🚚 Ongkir';
      case MessageType.text:
        return message.content;
    }
  }


  Widget _buildSupportCategoryChip(BuildContext context) {
    if (chat.supportCategory == null) return const SizedBox.shrink();

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: _getSupportCategoryColor().withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            _getSupportCategoryIcon(),
            size: 14,
            color: _getSupportCategoryColor(),
          ),
          const SizedBox(width: 4),
          Text(
            _getSupportCategoryLabel(),
            style: TextStyle(fontSize: 12, color: _getSupportCategoryColor()),
          ),
        ],
      ),
    );
  }

  Widget _buildTrailing(BuildContext context, int unreadCount) {
    return Column(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        if (unreadCount > 0)
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.primary,
              shape: BoxShape.circle,
            ),
            constraints: const BoxConstraints(minWidth: 20, minHeight: 20),
            child: Center(
              child: Text(
                unreadCount > 99 ? '99+' : unreadCount.toString(),
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 11,
                  fontWeight: FontWeight.bold,
                ),
              ),
            ),
          )
        else
          Icon(
            chat.isSupportChat && chat.supportStatus == SupportStatus.resolved
                ? Icons.check_circle
                : Icons.chevron_right,
            size: 20,
            color: Colors.grey[400],
          ),
      ],
    );
  }


  Color _getSupportCategoryColor() {
    switch (chat.supportCategory) {
      case SupportCategory.payment:
        return Colors.green;
      case SupportCategory.order:
        return Colors.orange;
      case SupportCategory.technical:
        return Colors.blue;
      case SupportCategory.account:
        return Colors.purple;
      case SupportCategory.general:
      default:
        return Colors.grey;
    }
  }

  IconData _getSupportCategoryIcon() {
    switch (chat.supportCategory) {
      case SupportCategory.payment:
        return Icons.payment;
      case SupportCategory.order:
        return Icons.shopping_bag;
      case SupportCategory.technical:
        return Icons.bug_report;
      case SupportCategory.account:
        return Icons.account_circle;
      case SupportCategory.general:
      default:
        return Icons.help_outline;
    }
  }

  String _getSupportCategoryLabel() {
    return chat.supportCategory?.name.toUpperCase() ?? 'GENERAL';
  }
}
