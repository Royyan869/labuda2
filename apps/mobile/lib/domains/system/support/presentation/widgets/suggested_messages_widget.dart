library;

/// Suggested Messages Widget (Refactored)
/// UI-only widget for suggested admin replies
/// Presentation layer - pure UI, no business logic

import 'package:flutter/material.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';

// ============================================
// WIDGET
// ============================================

/// Suggested Messages Widget
/// Horizontal scrollable list of suggested messages for support admins
/// Shows greeting templates and quick replies that can be tapped to fill input
class SuggestedMessagesWidgetRefactored extends StatelessWidget {
  final Function(String) onMessageSelected;

  const SuggestedMessagesWidgetRefactored({
    super.key,
    required this.onMessageSelected,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    // Combine all suggested messages
    final List<SuggestedMessage> messages = [
      // Greetings (hardcoded names for privacy)
      ...GreetingTemplates.friendly.map(
        (template) => SuggestedMessage(
          text: template,
          category: 'Greeting',
          icon: Icons.waving_hand,
          color: Colors.green,
        ),
      ),

      // Quick replies
      ...QuickReplies.get('acknowledgment').map(
        (text) => SuggestedMessage(
          text: text,
          category: 'Acknowledgment',
          icon: Icons.check_circle_outline,
          color: Colors.blue,
        ),
      ),

      ...QuickReplies.get('resolved').map(
        (text) => SuggestedMessage(
          text: text,
          category: 'Resolved',
          icon: Icons.task_alt,
          color: Colors.purple,
        ),
      ),

      ...QuickReplies.get('follow_up').map(
        (text) => SuggestedMessage(
          text: text,
          category: 'Follow Up',
          icon: Icons.help_outline,
          color: Colors.orange,
        ),
      ),

      ...QuickReplies.get('closing').map(
        (text) => SuggestedMessage(
          text: text,
          category: 'Closing',
          icon: Icons.thumb_up,
          color: Colors.teal,
        ),
      ),
    ];

    return Container(
      height: 120,
      padding: const EdgeInsets.symmetric(vertical: 8),
      decoration: BoxDecoration(
        color: isDark ? Colors.grey[900] : Colors.grey[100],
        border: Border(
          bottom: BorderSide(
            color: isDark ? Colors.grey[800]! : Colors.grey[300]!,
            width: 1,
          ),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: Row(
              children: [
                Icon(
                  Icons.tips_and_updates_outlined,
                  size: 14,
                  color: isDark ? Colors.grey[400] : Colors.grey[600],
                ),
                const SizedBox(width: 6),
                Text(
                  'Suggested Messages',
                  style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                    color: isDark ? Colors.grey[400] : Colors.grey[600],
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 8),

          // Horizontal scroll list
          Expanded(
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 16),
              itemCount: messages.length,
              separatorBuilder: (context, index) => const SizedBox(width: 8),
              itemBuilder: (context, index) {
                final message = messages[index];
                return _buildMessageChip(context, message, isDark);
              },
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMessageChip(
    BuildContext context,
    SuggestedMessage message,
    bool isDark,
  ) {
    return InkWell(
      onTap: () => onMessageSelected(message.text),
      borderRadius: BorderRadius.circular(20),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
        decoration: BoxDecoration(
          color: isDark ? Colors.grey[800] : Colors.white,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(
            color: message.color.withValues(alpha: 0.3),
            width: 1.5,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(message.icon, size: 18, color: message.color),
            const SizedBox(width: 8),
            ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 280),
              child: Text(
                message.text,
                style: TextStyle(
                  fontSize: 14,
                  color: isDark ? Colors.grey[200] : Colors.grey[800],
                ),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ============================================
// HELPER CLASS
// ============================================

/// Suggested Message Model
class SuggestedMessage {
  final String text;
  final String category;
  final IconData icon;
  final Color color;

  const SuggestedMessage({
    required this.text,
    required this.category,
    required this.icon,
    required this.color,
  });
}
