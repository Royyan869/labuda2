library;

/// Support Ticket Thread Screen
///
/// Email-like thread view for support tickets (NO chat behavior)
/// - Vertical list of messages with sender labels + timestamps
/// - No typing indicators, no "online/offline", no chat bubbles
/// - Simple card-based UI like email thread

import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter/material.dart' as flutter show ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';
import 'package:labuda/domains/system/support/presentation/providers/support_providers.dart';
import 'package:labuda/domains/system/support/presentation/utils/support_category_label.dart';
import 'package:labuda/domains/system/support/presentation/utils/support_status_label.dart';
import 'package:labuda/domains/system/support/presentation/widgets/support_activity_timeline.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/widgets/composer_action_buttons.dart';

class SupportTicketThreadScreen extends ConsumerStatefulWidget {
  final String ticketId;

  const SupportTicketThreadScreen({super.key, required this.ticketId});

  @override
  ConsumerState<SupportTicketThreadScreen> createState() =>
      _SupportTicketThreadScreenState();
}

class _SupportTicketThreadScreenState
    extends ConsumerState<SupportTicketThreadScreen> {
  final TextEditingController _messageController = TextEditingController();
  late Future<Result<List<SupportMessage>>> _messagesFuture;
  bool _isSending = false;

  @override
  void initState() {
    super.initState();
    _messagesFuture = _loadMessages();
    _messageController.addListener(_handleComposerChanged);
  }

  // Rebuild on every draft change so the always-visible send button can
  // toggle its disabled state (canonical composer action row).
  void _handleComposerChanged() {
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    _messageController.removeListener(_handleComposerChanged);
    _messageController.dispose();
    super.dispose();
  }

  Future<Result<List<SupportMessage>>> _loadMessages() {
    return ref.read(supportRepositoryProvider).getMessages(widget.ticketId);
  }

  void _reloadMessages() {
    setState(() {
      _messagesFuture = _loadMessages();
    });
  }

  /// Send the authenticated user's reply through the Support API. Sender
  /// identity is derived server-side from the session — the client never
  /// supplies it.
  Future<void> _sendMessage() async {
    final text = _messageController.text.trim();
    if (text.isEmpty || _isSending) return;

    setState(() => _isSending = true);
    final result = await ref
        .read(supportRepositoryProvider)
        .sendMessage(ticketId: widget.ticketId, message: text);

    if (!mounted) return;
    setState(() => _isSending = false);

    if (result.isError) {
      AppSnackBar.showError(context, result.error ?? 'Gagal mengirim pesan');
      return;
    }

    _messageController.clear();
    _reloadMessages();
  }

  @override
  Widget build(BuildContext context) {
    final ticketAsync = ref.watch(supportTicketProvider(widget.ticketId));

    return Scaffold(
      appBar: AppBarCustom(title: 'Support Ticket'),
      body: Column(
        children: [
          // Ticket Context Header
          ticketAsync.when(
            data: (ticket) {
              if (ticket == null) {
                return const SizedBox.shrink();
              }
              return _buildTicketHeader(ticket);
            },
            loading: () => const SizedBox.shrink(),
            error: (_, _) => const SizedBox.shrink(),
          ),

          // Read-only activity timeline: the canonical ticket event history
          // from the owner-only Support events contract.
          SupportActivityTimeline(ticketId: widget.ticketId),

          // Messages List
          Expanded(child: _buildMessagesList(ticketAsync)),

          // Reply composer
          _buildComposer(),
        ],
      ),
    );
  }

  Widget _buildTicketHeader(SupportTicket ticket) {
    final statusConfig = StatusConfig.get(ticket.status);
    final categoryConfig = CategoryConfig.get(ticket.category);

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainer,
        border: Border(
          bottom: BorderSide(
            color: Theme.of(context).colorScheme.outlineVariant,
          ),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Status and Category
          Row(
            children: [
              _buildHeaderBadge(
                icon: statusConfig.icon,
                label: ticket.status.label(context.l10n),
                colorValue: statusConfig.colorValue,
              ),
              const SizedBox(width: 8),
              _buildHeaderBadge(
                icon: categoryConfig.icon,
                label: ticket.category.label(context.l10n),
                colorValue: categoryConfig.colorValue,
              ),
            ],
          ),

          // Linked Order (if any)
          if (ticket.linkedOrderId != null) ...[
            const SizedBox(height: 8),
            Row(
              children: [
                Icon(
                  Icons.link,
                  size: AppIconSize.inlineGlyph,
                  color: Theme.of(context).colorScheme.secondary,
                ),
                const SizedBox(width: 4),
                Text(
                  'Order #${ticket.linkedOrderId!.substring(0, 8)}...',
                  style: context.typeRoles.labelMicro.copyWith(
                    color: Theme.of(context).colorScheme.secondary,
                  ),
                ),
              ],
            ),
          ],

          // Created date
          const SizedBox(height: 8),
          Text(
            'Created ${const TimeFormatService().formatTimeAgo(ticket.createdAt)}',
            style: context.typeRoles.labelMicro.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildHeaderBadge({
    required String icon,
    required String label,
    required int colorValue,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p12,
        vertical: AppMetrics.p4,
      ),
      decoration: BoxDecoration(
        color: Color(colorValue).withAlpha(40),
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: Color(colorValue).withAlpha(128)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(icon, style: context.typeRoles.labelMicro),
          const SizedBox(width: 4),
          Text(
            label,
            style: context.typeRoles.labelMicro.copyWith(
              fontWeight: FontWeight.bold,
              color: Color(colorValue),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMessagesList(AsyncValue<SupportTicket?> ticketAsync) {
    return FutureBuilder<Result<List<SupportMessage>>>(
      future: _messagesFuture,
      builder: (context, snapshot) {
        if (snapshot.connectionState == flutter.ConnectionState.waiting) {
          return const Center(child: CircularProgressIndicator());
        }

        if (snapshot.hasError) {
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
                  snapshot.error.toString(),
                  textAlign: TextAlign.center,
                  style: TextStyle(color: context.statusColors.error),
                ),
              ],
            ),
          );
        }

        if (!snapshot.hasData || snapshot.data!.isError) {
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
                  snapshot.data?.error ?? 'Failed to load messages',
                  textAlign: TextAlign.center,
                  style: TextStyle(color: context.statusColors.error),
                ),
              ],
            ),
          );
        }

        final messages = snapshot.data!.data!;

        // If no messages, show initial placeholder
        if (messages.isEmpty) {
          return _buildEmptyThread(ticketAsync);
        }

        return ListView.builder(
          padding: const EdgeInsets.all(AppMetrics.p16),
          itemCount: messages.length,
          itemBuilder: (context, index) {
            final message = messages[index];
            // Sender identity comes from the persisted canonical sender_type
            // on the message — never guessed from a UUID or local state.
            final isFromUser = message.senderType == SupportSenderType.user;

            return _ThreadMessageCard(message: message, isFromUser: isFromUser);
          },
        );
      },
    );
  }

  Widget _buildEmptyThread(AsyncValue<SupportTicket?> ticketAsync) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.mail_outline,
            size: AppIconSize.display,
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
          const SizedBox(height: 16),
          Text(
            'Ticket berhasil dibuat',
            style: context.typeRoles.titleProminent.copyWith(
              fontWeight: FontWeight.bold,
              color: Theme.of(context).colorScheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Tim support kami akan segera merespon',
            textAlign: TextAlign.center,
            style: context.typeRoles.bodyDense.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

  /// Composer: posts the user's reply into the ticket conversation.
  ///
  /// SAFE-AREA-14: the composer touches the bottom edge, so this `SafeArea`
  /// is THE live system-bottom-inset authority — the same contract as the
  /// chat (`ChatInputArea`) and comment (`CommentInputWithCommerceReference`)
  /// composers. The keyboard stays with `Scaffold.resizeToAvoidBottomInset`
  /// (body resize), never with hand-rolled inset arithmetic here.
  Widget _buildComposer() {
    return Container(
      padding: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p12,
        AppMetrics.p16,
        AppMetrics.p16,
      ),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainer,
        border: Border(
          top: BorderSide(color: Theme.of(context).colorScheme.outlineVariant),
        ),
      ),
      child: SafeArea(
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Expanded(
              child: TextField(
                controller: _messageController,
                minLines: 1,
                maxLines: 4,
                textInputAction: TextInputAction.newline,
                decoration: AppTheme.composerDecoration(
                  Theme.of(context).colorScheme,
                  hintText: 'Tulis balasan...',
                ),
              ),
            ),
            const SizedBox(width: 8),
            // Canonical action row: send always visible — disabled while the
            // draft is empty, spinner while the send is in flight.
            ComposerSendButton(
              loading: _isSending,
              onPressed: _messageController.text.trim().isNotEmpty
                  ? _sendMessage
                  : null,
            ),
          ],
        ),
      ),
    );
  }
}

/// Thread Message Card
///
/// Email-like message display (NOT chat bubble)
/// Shows: sender label, timestamp, message body
class _ThreadMessageCard extends StatelessWidget {
  final SupportMessage message;
  final bool isFromUser;

  const _ThreadMessageCard({required this.message, required this.isFromUser});

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: AppMetrics.p16),
      elevation: AppElevation.none,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppShape.r12),
        side: BorderSide(color: Theme.of(context).colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Sender Label & Timestamp
            Row(
              children: [
                // Sender avatar placeholder
                CircleAvatar(
                  radius: 16,
                  backgroundColor: isFromUser
                      ? Theme.of(context).colorScheme.secondary
                      : Theme.of(context).colorScheme.primary,
                  child: Text(
                    isFromUser ? 'Y' : 'S',
                    style: context.typeRoles.labelMicro.copyWith(
                      fontWeight: FontWeight.bold,
                      color: Theme.of(context).colorScheme.onPrimary,
                    ),
                  ),
                ),
                const SizedBox(width: 12),

                // Sender name
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        isFromUser ? 'You' : 'Support Team',
                        style: context.typeRoles.bodyDense.copyWith(
                          fontWeight: FontWeight.bold,
                          color: Theme.of(context).colorScheme.onSurface,
                        ),
                      ),
                      Text(
                        _getSenderTypeLabel(),
                        style: context.typeRoles.labelMicro.copyWith(
                          color: Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),

                // Timestamp — canonical relative-time authority
                // (TimeFormatService). Message age follows the same Indonesian
                // product locale as the ticket-created header and support
                // list/card surfaces.
                Text(
                  const TimeFormatService().formatTimeAgo(message.createdAt),
                  style: context.typeRoles.labelMicro.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),

            const SizedBox(height: 12),

            // Message Body
            Text(
              message.displayText,
              style: context.typeRoles.bodyDense.copyWith(
                height: 1.5,
                color: Theme.of(context).colorScheme.onSurface,
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _getSenderTypeLabel() {
    switch (message.senderType) {
      case SupportSenderType.user:
        return 'Customer';
      case SupportSenderType.admin:
        return 'Support Agent';
      case SupportSenderType.system:
        return 'System';
    }
  }
}
