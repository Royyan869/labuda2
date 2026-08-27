import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_colors.dart';

/// Reusable widget untuk hashtag display dan management
/// Extracted from create_content_screen.dart untuk reusability
class HashtagInputWidget extends StatelessWidget {
  final List<String> hashtags;
  final Function(String) onRemoveHashtag;
  final bool isDark;

  const HashtagInputWidget({
    super.key,
    required this.hashtags,
    required this.onRemoveHashtag,
    this.isDark = false,
  });

  @override
  Widget build(BuildContext context) {
    if (hashtags.isEmpty) return const SizedBox.shrink();

    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Icon(Icons.tag, size: 18, color: AppColors.warningYellow),
        const SizedBox(width: 8),
        Expanded(
          child: Wrap(
            spacing: 8,
            runSpacing: 8,
            children: hashtags.map((hashtag) {
              return Chip(
                label: Text('#$hashtag'),
                deleteIcon: const Icon(Icons.close, size: 16),
                onDeleted: () => onRemoveHashtag(hashtag),
                backgroundColor: AppColors.warningYellow.withValues(alpha: 0.1),
                labelStyle: TextStyle(color: AppColors.warningYellow),
                side: BorderSide.none,
                elevation: 0,
              );
            }).toList(),
          ),
        ),
      ],
    );
  }
}
