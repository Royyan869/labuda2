library;

/// Support Ticket Card Widget (Refactored)
/// UI-only widget for displaying support ticket in queue
/// Presentation layer - pure UI, delegates actions to providers

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';

// ============================================
// WIDGET
// ============================================

/// Support Ticket Card Widget
/// Displays support ticket info in user's list
/// Shows: category, priority, status, user info, last message, time ago
class SupportTicketCardRefactored extends ConsumerWidget {
  final SupportTicket ticket;
  final VoidCallback? onTap;

  const SupportTicketCardRefactored({
    super.key,
    required this.ticket,
    this.onTap,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);

    final categoryConfig = CategoryConfig.get(ticket.category);
    final priorityConfig = PriorityConfig.get(ticket.priority);
    final statusConfig = StatusConfig.get(ticket.status);

    // Time ago
    final timeAgo = ticket.lastMessageAt != null
        ? SupportUtils.formatTimeAgo(ticket.lastMessageAt!)
        : SupportUtils.formatTimeAgo(ticket.createdAt);

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Header: Priority + Category + Time
              Row(
                children: [
                  // Priority Badge
                  Flexible(
                    child: _buildBadge(
                      icon: priorityConfig.icon,
                      label: priorityConfig.labelId,
                      colorValue: priorityConfig.colorValue,
                    ),
                  ),
                  const SizedBox(width: 8),

                  // Category Badge
                  Flexible(
                    child: _buildBadge(
                      icon: categoryConfig.icon,
                      label: categoryConfig.nameId,
                      colorValue: categoryConfig.colorValue,
                    ),
                  ),

                  const SizedBox(width: 8),

                  // Time ago
                  Text(
                    timeAgo,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: Colors.grey[600],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),

              // User Info
              Row(
                children: [
                  // User Avatar
                  CircleAvatar(
                    radius: 20,
                    backgroundImage: ticket.userAvatar != null
                        ? NetworkImage(ticket.userAvatar!)
                        : null,
                    child: ticket.userAvatar == null
                        ? Text(
                            ticket.userName.isNotEmpty
                                ? ticket.userName[0].toUpperCase()
                                : '?',
                            style: const TextStyle(fontWeight: FontWeight.bold),
                          )
                        : null,
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          ticket.userName,
                          style: theme.textTheme.titleSmall?.copyWith(
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                        if (ticket.linkedOrderId != null) ...[
                          const SizedBox(height: 2),
                          Row(
                            children: [
                              Icon(
                                Icons.link,
                                size: 12,
                                color: Colors.blue[700],
                              ),
                              const SizedBox(width: 4),
                              Flexible(
                                child: Text(
                                  'Order #${ticket.linkedOrderId!.substring(0, 8)}...',
                                  style: TextStyle(
                                    fontSize: 11,
                                    color: Colors.blue[700],
                                  ),
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ),
                            ],
                          ),
                        ],
                      ],
                    ),
                  ),

                  // Status Badge
                  Flexible(
                    child: _buildBadge(
                      icon: statusConfig.icon,
                      label: statusConfig.labelId,
                      colorValue: statusConfig.colorValue,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),

              // Last Message Preview
              if (ticket.lastMessage != null) ...[
                Text(
                  ticket.lastMessage!,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: Colors.grey[700],
                  ),
                ),
                const SizedBox(height: 12),
              ],

              // View Ticket Button (always shown for users)
              _buildViewTicketButton(context, 'View Ticket'),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildBadge({
    required String icon,
    required String label,
    required int colorValue,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: Color(colorValue).withAlpha(40),
        borderRadius: BorderRadius.circular(6),
        border: Border.all(color: Color(colorValue).withAlpha(128)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(icon, style: const TextStyle(fontSize: 12)),
          const SizedBox(width: 4),
          Flexible(
            child: Text(
              label,
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.bold,
                color: Color(colorValue),
              ),
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildViewTicketButton(BuildContext context, String label) {
    return SizedBox(
      width: double.infinity,
      child: OutlinedButton.icon(
        onPressed: onTap,
        icon: const Icon(Icons.mail_outline, size: 18),
        label: Text(label),
        style: OutlinedButton.styleFrom(
          padding: const EdgeInsets.symmetric(vertical: 12),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        ),
      ),
    );
  }
}
