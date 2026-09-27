import 'package:flutter/material.dart';

/// Reusable widget untuk hashtag display dan management
/// Extracted from create_content_screen.dart untuk reusability
class HashtagInputWidget extends StatelessWidget {
  final List<String> hashtags;
  final Function(String) onRemoveHashtag;

  const HashtagInputWidget({
    super.key,
    required this.hashtags,
    required this.onRemoveHashtag,
  });

  @override
  Widget build(BuildContext context) {
    if (hashtags.isEmpty) return const SizedBox.shrink();
    final scheme = Theme.of(context).colorScheme;

    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Icon(Icons.tag, size: 18, color: scheme.secondary),
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
                backgroundColor: scheme.secondary.withValues(alpha: 0.1),
                labelStyle: TextStyle(color: scheme.secondary),
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
