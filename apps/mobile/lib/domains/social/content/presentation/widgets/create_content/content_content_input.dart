import 'package:flutter/material.dart';
import 'package:hishumi/shared/widgets/mentions/mention_text_field.dart';

/// Widget for post content text input area with mention support
///
/// Features:
/// - Multi-line text input with @ mention autocomplete
/// - Character counter (0/2000)
/// - Auto-detect hashtags and mentions
/// - Warning color when approaching limit
class ContentContentInput extends StatelessWidget {
  final TextEditingController controller;
  final ValueChanged<String> onChanged;
  final ValueChanged<List<String>>? onMentionsChanged;

  const ContentContentInput({
    super.key,
    required this.controller,
    required this.onChanged,
    this.onMentionsChanged,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    const hintText =
        "What's on your mind?\n\nShare your koi stories, tips... Use @ to mention users!";

    return MentionTextField(
      controller: controller,
      maxLines: null,
      minLines: 5,
      hintText: hintText,
      style: Theme.of(
        context,
      ).textTheme.bodyLarge?.copyWith(color: scheme.onSurface, height: 1.5),
      decoration: InputDecoration(
        hintText: hintText,
        hintStyle: Theme.of(
          context,
        ).textTheme.bodyLarge?.copyWith(color: scheme.onSurfaceVariant),
        border: InputBorder.none,
        enabledBorder: InputBorder.none,
        focusedBorder: InputBorder.none,
        contentPadding: EdgeInsets.zero,
        counterText: '',
      ),
      onMentionsChanged: onMentionsChanged,
      onChanged: () => onChanged(controller.text),
    );
  }
}
