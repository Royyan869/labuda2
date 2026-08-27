import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';

/// Shared Time Ago Widget untuk menampilkan relative time dengan konsisten
///
/// Features:
/// - Instagram-style time formatting (1m, 1h, 1d, etc.)
/// - Responsive text styling dengan theme integration
/// - Support untuk different time formats
/// - Consistent styling across app
/// - Auto-refresh capability (optional)
///
/// REFACTORED: Widget is now pure UI - delegates to domain service
class TimeAgoWidget extends StatelessWidget {
  final DateTime dateTime;
  final TextStyle? style;
  final Color? color;
  final double? fontSize;
  final FontWeight? fontWeight;
  final TimeFormat format;
  final bool showFullDate;

  static const _timeService = TimeFormatService();

  const TimeAgoWidget({
    super.key,
    required this.dateTime,
    this.style,
    this.color,
    this.fontSize,
    this.fontWeight,
    this.format = TimeFormat.short,
    this.showFullDate = false,
  });

  /// Instagram-style time ago (12m, 2h, 3d)
  const TimeAgoWidget.instagram({
    super.key,
    required this.dateTime,
    this.style,
    this.color,
    this.fontSize = 12,
    this.fontWeight,
  }) : format = TimeFormat.short,
       showFullDate = false;

  /// Facebook-style time ago (12 minutes ago, 2 hours ago)
  const TimeAgoWidget.facebook({
    super.key,
    required this.dateTime,
    this.style,
    this.color,
    this.fontSize = 13,
    this.fontWeight,
  }) : format = TimeFormat.long,
       showFullDate = false;

  /// Compact format untuk card headers
  const TimeAgoWidget.compact({
    super.key,
    required this.dateTime,
    this.style,
    this.color,
    this.fontSize = 12,
    this.fontWeight = FontWeight.w400,
  }) : format = TimeFormat.short,
       showFullDate = false;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    final defaultColor =
        color ?? (isDark ? AppColors.neutralGray400 : AppColors.neutralGray600);

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
      format: format,
      showFullDate: showFullDate,
    );

    return Text(formattedText, style: textStyle);
  }
}
