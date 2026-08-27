import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Expandable Text Widget seperti Facebook
///
/// Features:
/// - Menampilkan text dengan maksimal 2 baris
/// - Tombol "See more" / "See less" untuk expand/collapse
/// - Smooth animation untuk expand/collapse
/// - Preserves text formatting dan style
class ExpandableTextWidget extends StatefulWidget {
  final String text;
  final TextStyle? style;
  final int maxLines;
  final String? seeMoreText;
  final String? seeLessText;
  final Color? linkColor;
  final bool trimOnTextOverflow;

  const ExpandableTextWidget({
    super.key,
    required this.text,
    this.style,
    this.maxLines = 2,
    this.seeMoreText = 'See more',
    this.seeLessText = 'See less',
    this.linkColor,
    this.trimOnTextOverflow = true,
  });

  @override
  State<ExpandableTextWidget> createState() => _ExpandableTextWidgetState();
}

class _ExpandableTextWidgetState extends State<ExpandableTextWidget>
    with TickerProviderStateMixin {
  bool _isExpanded = false;
  bool _hasTextOverflow = false;
  late AnimationController _animationController;

  @override
  void initState() {
    super.initState();
    _animationController = AnimationController(
      duration: const Duration(milliseconds: 200),
      vsync: this,
    );
    CurvedAnimation(parent: _animationController, curve: Curves.easeInOut);

    // Check if text overflows after first frame
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _checkTextOverflow();
    });
  }

  @override
  void dispose() {
    _animationController.dispose();
    super.dispose();
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
      maxWidth: MediaQuery.of(context).size.width - 24,
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

    if (_isExpanded) {
      _animationController.forward();
    } else {
      _animationController.reverse();
    }
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
          child: Text(
            widget.text,
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
                  ? (widget.seeLessText ?? 'See less')
                  : (widget.seeMoreText ?? 'See more'),
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
