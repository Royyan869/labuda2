/// Time formatting domain service — THE canonical relative-time authority.
///
/// Provides business logic for formatting time-related data.
/// This service is pure domain logic with no Flutter dependencies.
///
/// Canonical Indonesian relative progression (Owner-locked):
///
///   baru saja → N menit lalu → N jam lalu → N hari lalu →
///   N minggu lalu → N bulan lalu → N tahun lalu
///
/// Boundaries: sub-minute → `baru saja` (including future timestamps);
/// `<60m` → minutes; `<24h` → hours; `<7d` → days; `<30d` → weeks
/// (`inDays ~/ 7`); `<365d` → months (`inDays ~/ 30`); otherwise years
/// (`inDays ~/ 365`). Indonesian has no singular/plural distinction, so
/// `1 menit lalu` and `5 menit lalu` share the same shape.
///
/// `Today`/`Yesterday` are presentation-specific conventions and are NOT
/// outputs of this engine. Absolute calendar rendering belongs to
/// `AppFormatters` (intl, `id_ID`).
library;

class TimeFormatService {
  const TimeFormatService();

  /// Format canonical relative time from [dateTime] to [now].
  ///
  /// [now] defaults to `DateTime.now()`; tests pass an explicit reference
  /// so boundaries stay deterministic. `DateTime.difference` compares
  /// absolute instants, so mixed local/UTC inputs resolve correctly.
  /// When [showFullDate] is true, the full calendar date is returned
  /// instead of a relative string.
  String formatTimeAgo(
    DateTime dateTime, {
    DateTime? now,
    bool showFullDate = false,
  }) {
    if (showFullDate) {
      return _formatFullDate(dateTime);
    }

    final reference = now ?? DateTime.now();
    final difference = reference.difference(dateTime);

    if (difference.isNegative || difference.inMinutes < 1) {
      return 'baru saja';
    } else if (difference.inHours < 1) {
      return '${difference.inMinutes} menit lalu';
    } else if (difference.inDays < 1) {
      return '${difference.inHours} jam lalu';
    } else if (difference.inDays < 7) {
      return '${difference.inDays} hari lalu';
    } else if (difference.inDays < 30) {
      return '${difference.inDays ~/ 7} minggu lalu';
    } else if (difference.inDays < 365) {
      return '${difference.inDays ~/ 30} bulan lalu';
    } else {
      return '${difference.inDays ~/ 365} tahun lalu';
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
}
