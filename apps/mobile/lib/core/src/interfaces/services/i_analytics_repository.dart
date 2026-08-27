import 'package:labuda/core/common/result.dart';

/// Repository interface untuk analytics dan tracking data.
///
/// Mengikuti interface-first design pattern sesuai ARCHITECTURE.md.
/// Digunakan untuk tracking user behavior, circumvention attempts,
/// dan analytics data untuk business intelligence.
abstract class IAnalyticsRepository {
  /// Log analytics event dengan parameters.
  ///
  /// **Parameters:**
  /// - [eventName]: Nama event yang di-track
  /// - [parameters]: Data tambahan untuk event
  /// - [userId]: ID user yang melakukan event (optional)
  ///
  /// **Returns:**
  /// [Result<void>] success atau error.
  Future<Result<void>> logEvent(
    String eventName, {
    Map<String, dynamic>? parameters,
    String? userId,
  });

  /// Log user action untuk tracking behavior.
  ///
  /// **Parameters:**
  /// - [action]: Action yang dilakukan user
  /// - [userId]: ID user
  /// - [extra]: Data tambahan (optional)
  ///
  /// **Returns:**
  /// [Result<void>] success atau error.
  Future<Result<void>> logUserAction(
    String action,
    String userId, {
    Map<String, dynamic>? extra,
  });

  /// Log circumvention attempt dengan detail lengkap.
  ///
  /// **Parameters:**
  /// - [content]: Konten yang mengandung violation
  /// - [userId]: ID user yang melakukan attempt
  /// - [extra]: Data tambahan seperti violation type, pattern, etc
  ///
  /// **Returns:**
  /// [Result<void>] success atau error.
  Future<Result<void>> logCircumventionAttempt(
    String content,
    String userId, {
    Map<String, dynamic>? extra,
  });

  /// Set user properties untuk segmentasi.
  ///
  /// **Parameters:**
  /// - [properties]: Map dari property name ke value
  ///
  /// **Returns:**
  /// [Result<void>] success atau error.
  Future<Result<void>> setUserProperties(Map<String, dynamic> properties);

  /// Track engagement dengan konten.
  ///
  /// **Parameters:**
  /// - [userId]: ID user
  /// - [contentId]: ID konten
  /// - [contentType]: Jenis konten (post, product, collection)
  /// - [engagementType]: Jenis engagement (view, like, share, comment)
  /// - [duration]: Durasi engagement dalam seconds (optional)
  ///
  /// **Returns:**
  /// [Result<void>] success atau error.
  Future<Result<void>> trackEngagement({
    required String userId,
    required String contentId,
    required String contentType,
    required String engagementType,
    int? duration,
  });

  /// Mendapatkan circumvention statistics.
  ///
  /// **Parameters:**
  /// - [startDate]: Tanggal mulai
  /// - [endDate]: Tanggal akhir
  /// - [userId]: Filter berdasarkan user (optional)
  /// - [violationType]: Filter berdasarkan jenis violation (optional)
  ///
  /// **Returns:**
  /// [Result<AnalyticsCircumventionStats>] dengan statistik circumvention.
  Future<Result<AnalyticsCircumventionStats>> getCircumventionStats({
    required DateTime startDate,
    required DateTime endDate,
    String? userId,
    String? violationType,
  });

  /// Flush pending analytics data ke server.
  ///
  /// **Returns:**
  /// [Result<void>] success atau error.
  Future<Result<void>> flush();
}

/// Data model untuk analytics circumvention statistics
class AnalyticsCircumventionStats {
  final int totalAttempts;
  final int uniqueUsers;
  final Map<String, int> violationTypes;
  final Map<String, int> dailyAttempts;
  final double averageConfidence;
  final int blockedAttempts;
  final int filteredAttempts;

  const AnalyticsCircumventionStats({
    required this.totalAttempts,
    required this.uniqueUsers,
    required this.violationTypes,
    required this.dailyAttempts,
    required this.averageConfidence,
    required this.blockedAttempts,
    required this.filteredAttempts,
  });

  /// Business logic: Get block rate
  double get blockRate {
    if (totalAttempts == 0) return 0.0;
    return blockedAttempts / totalAttempts;
  }

  /// Business logic: Get filter rate
  double get filterRate {
    if (totalAttempts == 0) return 0.0;
    return filteredAttempts / totalAttempts;
  }

  /// Business logic: Get most common violation type
  String? get mostCommonViolation {
    if (violationTypes.isEmpty) return null;

    String? mostCommon;
    int maxCount = 0;

    for (final entry in violationTypes.entries) {
      if (entry.value > maxCount) {
        maxCount = entry.value;
        mostCommon = entry.key;
      }
    }

    return mostCommon;
  }
}
