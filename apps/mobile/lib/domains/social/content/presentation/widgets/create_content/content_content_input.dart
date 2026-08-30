import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/mentions/mention_text_field.dart';

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
  final bool isDark;

  const ContentContentInput({
    super.key,
    required this.controller,
    required this.onChanged,
    this.onMentionsChanged,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    const hintText =
        "What's on your mind?\n\nShare your koi stories, tips... Use @ to mention users!";

    return MentionTextField(
      controller: controller,
      maxLines: null,
      minLines: 5,
      hintText: hintText,
      style: TextStyle(
        fontSize: 16,
        color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray800,
        height: 1.5,
      ),
      decoration: InputDecoration(
        hintText: hintText,
        hintStyle: TextStyle(
          color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
          fontSize: 16,
        ),
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
