import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/mentions/mention_rich_text.dart';

/// Expandable Text Widget dengan Mention Support (Facebook-style)
///
/// Features:
/// - Menampilkan text dengan maksimal N baris (default 4)
/// - @mention clickable yang bisa navigate ke profil
/// - Tombol "Selengkapnya" / "Lebih sedikit" untuk expand/collapse
/// - Smooth animation untuk expand/collapse
/// - Preserves mention formatting dan style
///
/// Combines:
/// - ExpandableTextWidget (See more/less functionality)
/// - MentionRichText (Clickable @mentions)
class ExpandableMentionTextWidget extends StatefulWidget {
  final String text;
  final TextStyle? style;
  final int maxLines;
  final String? seeMoreText;
  final String? seeLessText;
  final Color? linkColor;

  const ExpandableMentionTextWidget({
    super.key,
    required this.text,
    this.style,
    this.maxLines = 4,
    this.seeMoreText,
    this.seeLessText,
    this.linkColor,
  });

  @override
  State<ExpandableMentionTextWidget> createState() =>
      _ExpandableMentionTextWidgetState();
}

class _ExpandableMentionTextWidgetState
    extends State<ExpandableMentionTextWidget> {
  bool _isExpanded = false;
  bool _hasTextOverflow = false;

  @override
  void initState() {
    super.initState();

    // Check if text overflows after first frame
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _checkTextOverflow();
    });
  }

  void _checkTextOverflow() {
    // Check if widget is still mounted before accessing context
    if (!mounted) return;

    final TextPainter textPainter = TextPainter(
      text: TextSpan(text: widget.text, style: widget.style),
      maxLines: widget.maxLines,
      textDirection: TextDirection.ltr,
    );

    textPainter.layout(
      maxWidth: MediaQuery.of(context).size.width - 32,
    ); // Account for padding

    if (textPainter.didExceedMaxLines && mounted) {
      setState(() {
        _hasTextOverflow = true;
      });
    }
  }

  void _toggleExpanded() {
    setState(() {
      _isExpanded = !_isExpanded;
    });
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final effectiveLinkColor =
        widget.linkColor ??
        (isDark
            ? AppColors.primaryBlue.withValues(alpha: 0.8)
            : AppColors.primaryBlue);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AnimatedSize(
          duration: const Duration(milliseconds: 200),
          curve: Curves.easeInOut,
          child: MentionRichText(
            text: widget.text,
            style: widget.style,
            maxLines: _isExpanded ? null : widget.maxLines,
            overflow: _isExpanded ? null : TextOverflow.ellipsis,
          ),
        ),

        if (_hasTextOverflow) ...[
          const SizedBox(height: 4),
          GestureDetector(
            onTap: _toggleExpanded,
            child: Text(
              _isExpanded
                  ? (widget.seeLessText ?? 'Lebih sedikit')
                  : (widget.seeMoreText ?? 'Selengkapnya'),
              style: TextStyle(
                color: effectiveLinkColor,
                fontSize: widget.style?.fontSize ?? 14,
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
        ],
      ],
    );
  }
}
