import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Reply message data for reply preview
class ReplyData {
  final String senderName;
  final String content;
  final List<String> mediaUrls;

  const ReplyData({
    required this.senderName,
    required this.content,
    this.mediaUrls = const [],
  });
}

/// Text Input Reply Preview Widget - Generic reply preview for text inputs
class TextInputReplyPreview extends StatelessWidget {
  final ReplyData replyingTo;
  final VoidCallback? onCancelReply;
  final bool isDark;

  const TextInputReplyPreview({
    super.key,
    required this.replyingTo,
    this.onCancelReply,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
        border: Border(left: BorderSide(color: AppColors.primaryRed, width: 4)),
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Replying to ${replyingTo.senderName.isNotEmpty ? replyingTo.senderName : "User"}',
                  style: TextStyle(
                    color: AppColors.primaryRed,
                    fontSize: 12,
                    fontWeight: FontWeight.w500,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  replyingTo.content.isNotEmpty
                      ? replyingTo.content
                      : replyingTo.mediaUrls.isNotEmpty
                      ? '📷 Media'
                      : 'Message',
                  style: TextStyle(
                    color: isDark
                        ? AppColors.neutralGray300
                        : AppColors.neutralGray600,
                    fontSize: 14,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          if (onCancelReply != null)
            GestureDetector(
              onTap: onCancelReply,
              child: Container(
                padding: const EdgeInsets.all(4),
                child: Icon(
                  Icons.close,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                  size: 20,
                ),
              ),
            ),
        ],
      ),
    );
  }
}
