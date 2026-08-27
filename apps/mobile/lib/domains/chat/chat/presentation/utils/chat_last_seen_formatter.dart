import 'package:intl/intl.dart';

/// Formats the chat presence "last seen" label using device-local time.
///
/// Rules:
/// - today: localized "Today"/"Hari ini" + localized time
/// - yesterday: localized "Yesterday"/"Kemarin" + localized time
/// - earlier in the current year: localized month/day + localized time
/// - previous years: localized date with year + localized time
String formatChatLastSeen({
  required DateTime lastSeen,
  required DateTime now,
  required String localeName,
}) {
  final localLastSeen = lastSeen.toLocal();
  final localNow = now.toLocal();

  final today = DateTime(localNow.year, localNow.month, localNow.day);
  final yesterday = today.subtract(const Duration(days: 1));
  final localDate = DateTime(
    localLastSeen.year,
    localLastSeen.month,
    localLastSeen.day,
  );

  final time = DateFormat.jm(localeName).format(localLastSeen);

  if (_sameDate(localDate, today)) {
    return '${_todayLabel(localeName)}, $time';
  }

  if (_sameDate(localDate, yesterday)) {
    return '${_yesterdayLabel(localeName)}, $time';
  }

  if (localLastSeen.year == localNow.year) {
    return '${DateFormat.MMMd(localeName).format(localLastSeen)}, $time';
  }

  return '${DateFormat.yMMMd(localeName).format(localLastSeen)}, $time';
}

bool _sameDate(DateTime a, DateTime b) {
  return a.year == b.year && a.month == b.month && a.day == b.day;
}

String _todayLabel(String localeName) {
  final normalized = localeName.toLowerCase();
  if (normalized.startsWith('id')) {
    return 'Hari ini';
  }
  return 'Today';
}

String _yesterdayLabel(String localeName) {
  final normalized = localeName.toLowerCase();
  if (normalized.startsWith('id')) {
    return 'Kemarin';
  }
  return 'Yesterday';
}
