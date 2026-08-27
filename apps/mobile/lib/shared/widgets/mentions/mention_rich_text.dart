import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/utils/mention_parser.dart';
import 'package:labuda/features/search/search/search.dart'; // **R2.2 MIGRATED**: Import from search domain

/// Widget untuk display text dengan clickable mentions
///
/// **R2.2 MIGRATED**: Now uses mentionResolverProvider from search domain
/// instead of shared/providers.
///
/// Supports:
/// - Clickable @username mentions (navigate to profile)
/// - Special mentions (@everyone, @admins) dengan warna berbeda
/// - Custom styling untuk mentions
/// - Automatic username to userId resolution for navigation
///
/// Usage:
/// ```dart
/// MentionRichText(
///   text: "Hi @john, check this out!",
///   onMentionTap: (username) {
///     // Navigate to profile
///   },
/// )
/// ```
class MentionRichText extends ConsumerWidget {
  final String text;
  final TextStyle? style;
  final TextStyle? mentionStyle;
  final TextStyle? specialMentionStyle;
  final Function(String username)? onMentionTap;
  final int? maxLines;
  final TextOverflow? overflow;
  final TextAlign? textAlign;

  const MentionRichText({
    super.key,
    required this.text,
    this.style,
    this.mentionStyle,
    this.specialMentionStyle,
    this.onMentionTap,
    this.maxLines,
    this.overflow,
    this.textAlign,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final segments = MentionParser.parseText(text);

    // Default styles
    final defaultStyle =
        style ??
        TextStyle(
          color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
          fontSize: 14,
        );

    final defaultMentionStyle =
        mentionStyle ??
        const TextStyle(
          color: AppColors.primaryBlue,
          fontWeight: FontWeight.w600,
        );

    final defaultSpecialMentionStyle =
        specialMentionStyle ??
        const TextStyle(
          color: AppColors.primaryRed,
          fontWeight: FontWeight.w700,
        );

    return RichText(
      text: TextSpan(
        style: defaultStyle,
        children: segments.map((segment) {
          if (segment.isMention) {
            return TextSpan(
              text: segment.text,
              style: segment.isSpecialMention
                  ? defaultSpecialMentionStyle
                  : defaultMentionStyle,
              recognizer: TapGestureRecognizer()
                ..onTap = () => _handleMentionTap(context, ref, segment),
            );
          } else {
            return TextSpan(text: segment.text);
          }
        }).toList(),
      ),
      maxLines: maxLines,
      overflow: overflow ?? TextOverflow.clip,
      textAlign: textAlign ?? TextAlign.start,
    );
  }

  Future<void> _handleMentionTap(
    BuildContext context,
    WidgetRef ref,
    MentionSegment segment,
  ) async {
    if (segment.isSpecialMention) {
      // Special mentions don't navigate
      // Could show a dialog or tooltip instead
      return;
    }

    if (onMentionTap != null) {
      onMentionTap!(segment.username!);
    } else {
      // Default behavior: navigate to user profile
      // Resolve username to userId first
      final resolver = ref.read(mentionResolverProvider);
      final userId = await resolver.resolveUsername(segment.username!);

      if (userId != null && context.mounted) {
        ref.read(navigationHandlerProvider).navigateToUserProfile(userId);
      }
    }
  }
}

/// Simplified version for displaying text with mentions
/// without navigation (read-only)
class MentionText extends ConsumerWidget {
  final String text;
  final TextStyle? style;
  final TextStyle? mentionStyle;
  final int? maxLines;
  final TextOverflow? overflow;

  const MentionText({
    super.key,
    required this.text,
    this.style,
    this.mentionStyle,
    this.maxLines,
    this.overflow,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return MentionRichText(
      text: text,
      style: style,
      mentionStyle: mentionStyle,
      maxLines: maxLines,
      overflow: overflow,
      onMentionTap: null, // No tap action
    );
  }
}
