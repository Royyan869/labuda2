/// Time formatting domain service
///
/// Provides business logic for formatting time-related data.
/// This service is pure domain logic with no Flutter dependencies.
library;

/// Time format options
enum TimeFormat {
  /// Short format: 1m, 1h, 1d (Instagram style)
  short,

  /// Medium format: 1m ago, 1h ago, 1d ago
  medium,

  /// Long format: 1 minute ago, 1 hour ago, 1 day ago (Facebook style)
  long,
}

/// Time formatting service
class TimeFormatService {
  const TimeFormatService();

  /// Format time ago from now
  String formatTimeAgo(
    DateTime dateTime, {
    TimeFormat format = TimeFormat.short,
    bool showFullDate = false,
  }) {
    if (showFullDate) {
      return _formatFullDate(dateTime);
    }

    final now = DateTime.now();
    final difference = now.difference(dateTime);

    // Show full date for posts older than 7 days
    if (difference.inDays > 7) {
      return _formatMonthDay(dateTime);
    }

    switch (format) {
      case TimeFormat.short:
        return _formatShort(difference);
      case TimeFormat.long:
        return _formatLong(difference);
      case TimeFormat.medium:
        return _formatMedium(difference);
    }
  }

  /// Instagram-style time ago (1m, 1h, 1d)
  String formatShort(DateTime dateTime) {
    final difference = DateTime.now().difference(dateTime);
    return _formatShort(difference);
  }

  /// Facebook-style time ago (1 minute ago, 1 hour ago)
  String formatLong(DateTime dateTime) {
    final difference = DateTime.now().difference(dateTime);
    return _formatLong(difference);
  }

  /// Medium format (1m ago, 1h ago, 1d ago)
  String formatMedium(DateTime dateTime) {
    final difference = DateTime.now().difference(dateTime);
    return _formatMedium(difference);
  }

  String _formatShort(Duration difference) {
    if (difference.inDays > 0) {
      return '${difference.inDays}d';
    } else if (difference.inHours > 0) {
      return '${difference.inHours}h';
    } else if (difference.inMinutes > 0) {
      return '${difference.inMinutes}m';
    } else {
      return 'now';
    }
  }

  String _formatLong(Duration difference) {
    if (difference.inDays > 0) {
      final days = difference.inDays;
      return '$days ${days == 1 ? 'day' : 'days'} ago';
    } else if (difference.inHours > 0) {
      final hours = difference.inHours;
      return '$hours ${hours == 1 ? 'hour' : 'hours'} ago';
    } else if (difference.inMinutes > 0) {
      final minutes = difference.inMinutes;
      return '$minutes ${minutes == 1 ? 'minute' : 'minutes'} ago';
    } else {
      return 'just now';
    }
  }

  String _formatMedium(Duration difference) {
    if (difference.inDays > 0) {
      return '${difference.inDays}d ago';
    } else if (difference.inHours > 0) {
      return '${difference.inHours}h ago';
    } else if (difference.inMinutes > 0) {
      return '${difference.inMinutes}m ago';
    } else {
      return 'now';
    }
  }

  String _formatFullDate(DateTime dateTime) {
    // Using basic formatting to avoid intl dependency in domain
    final months = [
      'Jan',
      'Feb',
      'Mar',
      'Apr',
      'May',
      'Jun',
      'Jul',
      'Aug',
      'Sep',
      'Oct',
      'Nov',
      'Dec',
    ];
    return '${months[dateTime.month - 1]} ${dateTime.day}, ${dateTime.year}';
  }

  String _formatMonthDay(DateTime dateTime) {
    final months = [
      'Jan',
      'Feb',
      'Mar',
      'Apr',
      'May',
      'Jun',
      'Jul',
      'Aug',
      'Sep',
      'Oct',
      'Nov',
      'Dec',
    ];
    return '${months[dateTime.month - 1]} ${dateTime.day}';
  }
}
