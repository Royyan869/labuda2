library;

/// Support Ticket Card Widget (Refactored)
/// UI-only widget for displaying support ticket in queue
/// Presentation layer - pure UI, delegates actions to providers

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/domains/system/support/domain/domain.dart';
import 'package:hishumi/domains/system/shared/domain/services/time_format_service.dart';
import 'package:hishumi/domains/system/support/presentation/utils/support_category_label.dart';
import 'package:hishumi/domains/system/support/presentation/utils/support_priority_label.dart';
import 'package:hishumi/domains/system/support/presentation/utils/support_status_label.dart';
import 'package:hishumi/core/src/localization/l10n_extension.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

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

    // Time ago — canonical relative-time authority.
    final timeAgo = ticket.lastMessageAt != null
        ? const TimeFormatService().formatTimeAgo(ticket.lastMessageAt!)
        : const TimeFormatService().formatTimeAgo(ticket.createdAt);

    return Card(
      margin: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p16,
        vertical: AppMetrics.p8,
      ),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppShape.r12),
        child: Padding(
          padding: const EdgeInsets.all(AppMetrics.p16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Header: Priority + Category + Time
              // Relative last-activity timestamp is secondary metadata:
              // Flexible + single-line ellipsis (same compact strategy as
              // the canonical support list / notification timestamp rows).
              // Formatter authority remains TimeFormatService.
              Row(
                children: [
                  // Priority Badge
                  Flexible(
                    child: _buildBadge(
                      context,
                      icon: priorityConfig.icon,
                      label: ticket.priority.label(context.l10n),
                      colorValue: priorityConfig.colorValue,
                    ),
                  ),
                  const SizedBox(width: 8),

                  // Category Badge
                  Flexible(
                    child: _buildBadge(
                      context,
                      icon: categoryConfig.icon,
                      label: ticket.category.label(context.l10n),
                      colorValue: categoryConfig.colorValue,
                    ),
                  ),

                  const SizedBox(width: 8),

                  // Time ago
                  Flexible(
                    child: Text(
                      timeAgo,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
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
                                size: AppIconSize.inlineGlyph,
                                color: Theme.of(context).colorScheme.secondary,
                              ),
                              const SizedBox(width: 4),
                              Flexible(
                                child: Text(
                                  'Order #${ticket.linkedOrderId!.substring(0, 8)}...',
                                  style: context.typeRoles.labelMicro.copyWith(
                                    color: Theme.of(
                                      context,
                                    ).colorScheme.secondary,
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
                      context,
                      icon: statusConfig.icon,
                      label: ticket.status.label(context.l10n),
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
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
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

  Widget _buildBadge(
    BuildContext context, {
    required String icon,
    required String label,
    required int colorValue,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p8,
        vertical: AppMetrics.p4,
      ),
      decoration: BoxDecoration(
        color: Color(colorValue).withAlpha(40),
        borderRadius: BorderRadius.circular(AppShape.r6),
        border: Border.all(color: Color(colorValue).withAlpha(128)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(icon, style: context.typeRoles.labelMicro),
          const SizedBox(width: 4),
          Flexible(
            child: Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: context.typeRoles.labelMicro.copyWith(
                fontWeight: FontWeight.bold,
                color: Color(colorValue),
              ),
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
        icon: const Icon(Icons.mail_outline, size: AppIconSize.action),
        label: Text(label),
        style: OutlinedButton.styleFrom(
          padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12),
        ),
      ),
    );
  }
}
