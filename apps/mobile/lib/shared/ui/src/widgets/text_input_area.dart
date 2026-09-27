import 'package:flutter/material.dart';
import 'package:labuda/shared/widgets/mentions/mention_text_field.dart';

/// Configuration for TextInputArea features
class TextInputConfig {
  final bool enableQuickActions;
  final String hintText;
  final int maxLines;

  const TextInputConfig({
    this.enableQuickActions = true,
    this.hintText = 'Type a message...',
    this.maxLines = 5,
  });

  static const TextInputConfig chat = TextInputConfig(
    hintText: 'Type a message...',
  );

  static const TextInputConfig comment = TextInputConfig(
    hintText: 'Write a comment...',
    maxLines: 3,
  );
}

/// Text Input Area Widget - Generic main input area with send button and mention support
class TextInputArea extends StatelessWidget {
  final TextEditingController messageController;
  final FocusNode focusNode;
  final bool hasTextContent;
  final List<String> selectedMediaUrls;
  final TextInputConfig config;
  final bool showQuickActions;
  final VoidCallback? onToggleQuickActions;
  final VoidCallback onSendMessage;
  final Function(List<String> mentionedUserIds)? onMentionsChanged;

  const TextInputArea({
    super.key,
    required this.messageController,
    required this.focusNode,
    required this.hasTextContent,
    required this.selectedMediaUrls,
    required this.config,
    this.showQuickActions = false,
    this.onToggleQuickActions,
    required this.onSendMessage,
    this.onMentionsChanged,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final hasContent = hasTextContent || selectedMediaUrls.isNotEmpty;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          // Text input with mention support
          Expanded(
            child: Container(
              decoration: BoxDecoration(
                color: scheme.surfaceContainerHigh,
                borderRadius: BorderRadius.circular(20),
              ),
              child: MentionTextField(
                controller: messageController,
                focusNode: focusNode,
                maxLines: config.maxLines,
                minLines: 1,
                hintText: config.hintText,
                decoration: InputDecoration(
                  hintText: config.hintText,
                  hintStyle: TextStyle(
                    color: scheme.onSurfaceVariant,
                    fontSize: 16,
                  ),
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(20),
                    borderSide: BorderSide.none,
                  ),
                  enabledBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(20),
                    borderSide: BorderSide.none,
                  ),
                  focusedBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(20),
                    borderSide: BorderSide.none,
                  ),
                  contentPadding: const EdgeInsets.symmetric(
                    horizontal: 16,
                    vertical: 12,
                  ),
                  suffixIcon:
                      config.enableQuickActions && onToggleQuickActions != null
                      ? IconButton(
                          onPressed: onToggleQuickActions,
                          icon: Icon(
                            showQuickActions
                                ? Icons.keyboard_arrow_up
                                : Icons.add,
                            color: scheme.onSurfaceVariant,
                            size: 20,
                          ),
                          padding: const EdgeInsets.all(8),
                        )
                      : null,
                ),
                style: TextStyle(
                  color: scheme.onSurface,
                  fontSize: 16,
                ),
                onMentionsChanged: onMentionsChanged,
              ),
            ),
          ),

          const SizedBox(width: 8),

          // Send button
          GestureDetector(
            onTap: hasContent ? onSendMessage : null,
            child: Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: hasContent
                    ? scheme.primary
                    : scheme.surfaceContainerHighest,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.send,
                color: hasContent
                    ? scheme.onPrimary
                    : scheme.onSurfaceVariant,
                size: 20,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
