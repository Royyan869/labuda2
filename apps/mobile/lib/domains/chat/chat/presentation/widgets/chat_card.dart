import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_identity_display.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_lifecycle_redaction.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';
import 'package:labuda/shared/widgets/profile_avatar.dart';
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
    final unreadCount = chat.roomUnreadCount;
    final colorScheme = Theme.of(context).colorScheme;

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
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p12),
        decoration: BoxDecoration(
          border: Border(
            bottom: BorderSide(color: colorScheme.outlineVariant, width: 0.5),
          ),
        ),
        child: Row(
          children: [
            _buildAvatar(
              otherUserAvatar,
              otherUserName,
              otherUserId,
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
    final unreadCount = chat.roomUnreadCount;
    final colorScheme = Theme.of(context).colorScheme;

    return InkWell(
      onTap: onTap,
      onLongPress: onLongPress,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p12),
        decoration: BoxDecoration(
          color: chat.supportStatus == SupportStatus.open
              ? colorScheme.secondary.withValues(alpha: 0.08)
              : null,
          border: Border(
            bottom: BorderSide(color: colorScheme.outlineVariant, width: 0.5),
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
    String userName,
    String userId, {
    bool degraded = false,
  }) {
    // E4.3 — Degraded participants always render the neutral fallback
    // (no NetworkImage of a redacted account, no initials anywhere).
    // Canonical avatar: user photo or user icon.
    return ProfileAvatar(
      userId: userId,
      size: 56,
      imageUrl: (!degraded && avatarUrl != null && avatarUrl.isNotEmpty)
          ? avatarUrl
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
    final colorScheme = Theme.of(context).colorScheme;
    return Row(
      children: [
        Expanded(
          child: Text(
            userName,
            style: TextStyle(
              fontWeight: FontWeight.w600,
              fontSize: AppType.s16,
              // E4.3 — Degraded identity: italic + muted color so the
              // redaction placeholder is visually distinct from a real
              // username. Matches the E3.1 comment-author treatment.
              fontStyle: degraded ? FontStyle.italic : FontStyle.normal,
              color: degraded ? colorScheme.onSurfaceVariant : null,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        if (chat.updatedAt != null) _buildTimestamp(context),
      ],
    );
  }

  Widget _buildSupportHeader(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
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
                  fontSize: AppType.s16,
                ),
              ),
              if (chat.assignedAdminName != null)
                Text(
                  'Agent: ${chat.assignedAdminName}',
                  style: TextStyle(fontSize: AppType.s12, color: colorScheme.onSurfaceVariant),
                ),
            ],
          ),
        ),
        if (chat.updatedAt != null) _buildTimestamp(context),
      ],
    );
  }

  Widget _buildTimestamp(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final timeStr = chat.updatedAt != null
        ? timeago.format(chat.updatedAt!)
        : timeago.format(chat.createdAt);

    return Text(
      timeStr,
      style: TextStyle(fontSize: AppType.s12, color: colorScheme.onSurfaceVariant),
    );
  }

  Widget _buildLastMessage(BuildContext context, String currentUserId) {
    final colorScheme = Theme.of(context).colorScheme;
    if (chat.lastMessage == null) {
      return Text(
        'No messages yet',
        style: TextStyle(fontSize: AppType.s14, color: colorScheme.onSurfaceVariant),
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
      );
    }

    final message = chat.lastMessage!;
    if (message.isHidden) {
      return Text(
        context.l10n.hiddenMessageByModerator,
        style: TextStyle(fontSize: AppType.s14, color: colorScheme.onSurfaceVariant),
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
      );
    }

    final prefix = message.isFromUser(currentUserId) ? 'You: ' : '';

    return Text(
      '$prefix${_getMessagePreview(message)}',
      style: TextStyle(fontSize: AppType.s14, color: colorScheme.onSurface),
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
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4),
      decoration: BoxDecoration(
        color: _getSupportCategoryColor(context).withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r4),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            _getSupportCategoryIcon(),
            size: 14,
            color: _getSupportCategoryColor(context),
          ),
          const SizedBox(width: 4),
          Text(
            _getSupportCategoryLabel(),
            style: TextStyle(fontSize: AppType.s12, color: _getSupportCategoryColor(context)),
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
            padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4),
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.primary,
              shape: BoxShape.circle,
            ),
            constraints: const BoxConstraints(minWidth: 20, minHeight: 20),
            child: Center(
              child: Text(
                unreadCount > 99 ? '99+' : unreadCount.toString(),
                style: TextStyle(
                  color: Theme.of(context).colorScheme.onPrimary,
                  fontSize: AppType.s11,
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
            color: Theme.of(context).colorScheme.outline,
          ),
      ],
    );
  }

  Color _getSupportCategoryColor(BuildContext context) {
    return switch (chat.supportCategory) {
      SupportCategory.paymentIssue => context.statusColors.success,
      SupportCategory.refundRequest => context.statusColors.success,
      SupportCategory.orderIssue => context.statusColors.warning,
      SupportCategory.shippingIssue => context.statusColors.warning,
      SupportCategory.accountIssue => AppColors.primaryPurple,
      SupportCategory.listingIssue => context.statusColors.success,
      SupportCategory.dispute => context.statusColors.error,
      SupportCategory.technicalIssue => context.statusColors.info,
      SupportCategory.other || null =>
        Theme.of(context).colorScheme.onSurfaceVariant,
    };
  }

  IconData _getSupportCategoryIcon() {
    return switch (chat.supportCategory) {
      SupportCategory.paymentIssue => Icons.payment,
      SupportCategory.refundRequest => Icons.currency_exchange,
      SupportCategory.orderIssue => Icons.shopping_bag,
      SupportCategory.shippingIssue => Icons.local_shipping,
      SupportCategory.accountIssue => Icons.account_circle,
      SupportCategory.listingIssue => Icons.sell,
      SupportCategory.dispute => Icons.gavel,
      SupportCategory.technicalIssue => Icons.bug_report,
      SupportCategory.other || null => Icons.help_outline,
    };
  }

  String _getSupportCategoryLabel() {
    return chat.supportCategory?.wireValue.toUpperCase() ?? 'OTHER';
  }
}
