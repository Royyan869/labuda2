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
import 'package:labuda/domains/system/support/domain/domain.dart';
import 'package:labuda/domains/system/support/presentation/providers/support_providers.dart';
import 'package:labuda/shared/shared.dart';

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
  late Future<SupportResult<List<SupportMessage>>> _messagesFuture;
  bool _isSending = false;

  @override
  void initState() {
    super.initState();
    _messagesFuture = _loadMessages();
  }

  @override
  void dispose() {
    _messageController.dispose();
    super.dispose();
  }

  Future<SupportResult<List<SupportMessage>>> _loadMessages() {
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

    if (result.isFailure) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(result.failure?.message ?? 'Gagal mengirim pesan'),
        ),
      );
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

          // Messages List
          Expanded(child: _buildMessagesList(ticketAsync)),

          // Reply composer
          _buildComposer(),
        ],
      ),
    );
  }

  Widget _buildTicketHeader(SupportTicket ticket) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final statusConfig = StatusConfig.get(ticket.status);
    final categoryConfig = CategoryConfig.get(ticket.category);

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralGray100,
        border: Border(
          bottom: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
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
                label: statusConfig.labelId,
                colorValue: statusConfig.colorValue,
              ),
              const SizedBox(width: 8),
              _buildHeaderBadge(
                icon: categoryConfig.icon,
                label: categoryConfig.nameId,
                colorValue: categoryConfig.colorValue,
              ),
            ],
          ),

          // Linked Order (if any)
          if (ticket.linkedOrderId != null) ...[
            const SizedBox(height: 8),
            Row(
              children: [
                Icon(Icons.link, size: 14, color: AppColors.primaryBlue),
                const SizedBox(width: 4),
                Text(
                  'Order #${ticket.linkedOrderId!.substring(0, 8)}...',
                  style: TextStyle(fontSize: 12, color: AppColors.primaryBlue),
                ),
              ],
            ),
          ],

          // Created date
          const SizedBox(height: 8),
          Text(
            'Created ${SupportUtils.formatTimeAgo(ticket.createdAt)}',
            style: TextStyle(
              fontSize: 11,
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray600,
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
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: Color(colorValue).withAlpha(40),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: Color(colorValue).withAlpha(128)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(icon, style: const TextStyle(fontSize: 12)),
          const SizedBox(width: 4),
          Text(
            label,
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.bold,
              color: Color(colorValue),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMessagesList(AsyncValue<SupportTicket?> ticketAsync) {
    return FutureBuilder<SupportResult<List<SupportMessage>>>(
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
                const Icon(
                  Icons.error_outline,
                  size: 48,
                  color: AppColors.error,
                ),
                const SizedBox(height: 16),
                Text(
                  snapshot.error.toString(),
                  textAlign: TextAlign.center,
                  style: const TextStyle(color: AppColors.error),
                ),
              ],
            ),
          );
        }

        if (!snapshot.hasData || snapshot.data!.isFailure) {
          return Center(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const Icon(
                  Icons.error_outline,
                  size: 48,
                  color: AppColors.error,
                ),
                const SizedBox(height: 16),
                Text(
                  snapshot.data?.failure?.message ?? 'Failed to load messages',
                  textAlign: TextAlign.center,
                  style: const TextStyle(color: AppColors.error),
                ),
              ],
            ),
          );
        }

        final messages = snapshot.data!.dataOrThrow;

        // If no messages, show initial placeholder
        if (messages.isEmpty) {
          return _buildEmptyThread(ticketAsync);
        }

        return ListView.builder(
          padding: const EdgeInsets.all(16),
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
          Icon(Icons.mail_outline, size: 64, color: AppColors.neutralGray400),
          const SizedBox(height: 16),
          Text(
            'Ticket berhasil dibuat',
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.bold,
              color: AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Tim support kami akan segera merespon',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 14, color: AppColors.neutralGray600),
          ),
        ],
      ),
    );
  }

  /// Composer: posts the user's reply into the ticket conversation.
  Widget _buildComposer() {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralGray100,
        border: Border(
          top: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
          ),
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          Expanded(
            child: TextField(
              controller: _messageController,
              minLines: 1,
              maxLines: 4,
              textInputAction: TextInputAction.newline,
              decoration: InputDecoration(
                hintText: 'Tulis balasan...',
                isDense: true,
                filled: true,
                fillColor: isDark
                    ? AppColors.darkGray700
                    : AppColors.neutralWhite,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(12),
                  borderSide: BorderSide.none,
                ),
              ),
            ),
          ),
          const SizedBox(width: 8),
          _isSending
              ? const SizedBox(
                  width: 44,
                  height: 44,
                  child: Center(
                    child: SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    ),
                  ),
                )
              : IconButton(
                  onPressed: _sendMessage,
                  icon: const Icon(Icons.send),
                  color: AppColors.primaryRed,
                  tooltip: 'Kirim',
                ),
        ],
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Card(
      margin: const EdgeInsets.only(bottom: 16),
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(
          color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
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
                      ? AppColors.primaryBlue
                      : AppColors.primaryRed,
                  child: Text(
                    isFromUser ? 'Y' : 'S',
                    style: const TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.bold,
                      color: AppColors.neutralWhite,
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
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.bold,
                          color: isDark
                              ? AppColors.neutralWhite
                              : AppColors.neutralGray900,
                        ),
                      ),
                      Text(
                        _getSenderTypeLabel(),
                        style: TextStyle(
                          fontSize: 11,
                          color: isDark
                              ? AppColors.neutralGray500
                              : AppColors.neutralGray600,
                        ),
                      ),
                    ],
                  ),
                ),

                // Timestamp
                Text(
                  _formatTimestamp(message.createdAt),
                  style: TextStyle(
                    fontSize: 11,
                    color: isDark
                        ? AppColors.neutralGray500
                        : AppColors.neutralGray600,
                  ),
                ),
              ],
            ),

            const SizedBox(height: 12),

            // Message Body
            Text(
              message.displayText,
              style: TextStyle(
                fontSize: 14,
                height: 1.5,
                color: isDark
                    ? AppColors.neutralGray200
                    : AppColors.neutralGray800,
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

  String _formatTimestamp(DateTime dateTime) {
    final now = DateTime.now();
    final difference = now.difference(dateTime);

    if (difference.inMinutes < 1) {
      return 'Just now';
    } else if (difference.inHours < 1) {
      return '${difference.inMinutes}m ago';
    } else if (difference.inDays < 1) {
      return '${difference.inHours}h ago';
    } else if (difference.inDays < 7) {
      return '${difference.inDays}d ago';
    } else {
      return '${dateTime.day}/${dateTime.month}/${dateTime.year}';
    }
  }
}
