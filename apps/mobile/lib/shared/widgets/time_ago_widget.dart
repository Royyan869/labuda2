import 'package:flutter/material.dart';
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';

/// Shared Time Ago Widget untuk menampilkan relative time dengan konsisten
///
/// Delegates to the canonical [TimeFormatService] (Indonesian relative
/// progression). Styling presets only — NOT a mandatory timestamp widget;
/// callers choose presentation per context.
/// REFACTORED: Widget is now pure UI - delegates to domain service
class TimeAgoWidget extends StatelessWidget {
  final DateTime dateTime;
  final TextStyle? style;
  final Color? color;
  final double? fontSize;
  final FontWeight? fontWeight;
  final bool showFullDate;

  /// Optional single-line bound for compact compositions. Null preserves
  /// the legacy unbounded behavior for existing callers.
  final int? maxLines;
  final TextOverflow? overflow;

  static const _timeService = TimeFormatService();

  const TimeAgoWidget({
    super.key,
    required this.dateTime,
    this.style,
    this.color,
    this.fontSize,
    this.fontWeight,
    this.showFullDate = false,
    this.maxLines,
    this.overflow,
  });

  /// Compact time ago for inline content (12sp default).
  const TimeAgoWidget.instagram({
    super.key,
    required this.dateTime,
    this.style,
    this.color,
    this.fontSize = 12,
    this.fontWeight,
    this.maxLines,
    this.overflow,
  }) : showFullDate = false;

  /// Larger time ago for detail surfaces (13sp default).
  const TimeAgoWidget.facebook({
    super.key,
    required this.dateTime,
    this.style,
    this.color,
    this.fontSize = 13,
    this.fontWeight,
    this.maxLines,
    this.overflow,
  }) : showFullDate = false;

  /// Compact format untuk card headers
  const TimeAgoWidget.compact({
    super.key,
    required this.dateTime,
    this.style,
    this.color,
    this.fontSize = 12,
    this.fontWeight = FontWeight.w400,
    this.maxLines,
    this.overflow,
  }) : showFullDate = false;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    final defaultColor =
        color ?? scheme.onSurfaceVariant;

    final textStyle =
        style ??
        theme.textTheme.bodySmall?.copyWith(
          color: defaultColor,
          fontSize: fontSize,
          fontWeight: fontWeight,
        );

    // Delegate to domain service
    final formattedText = _timeService.formatTimeAgo(
      dateTime,
      showFullDate: showFullDate,
    );

    return Text(
      formattedText,
      style: textStyle,
      maxLines: maxLines,
      overflow: overflow,
    );
  }
}
