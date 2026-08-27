/// Report DI Helper
///
/// Helper functions for initializing the report module dependencies.
///
/// MIGRATION STATUS: Migrated from report_api_di.dart (GetIt) to Riverpod
library;

import 'package:labuda/core/services/s3_service.dart';
import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart';

/// Report DI
///
/// Helper class for initializing report module dependencies using pure Riverpod.
class ReportDI {
  /// Create provider overrides for report module
  ///
  /// Use this in ProviderScope to provide S3Service:
  /// ```dart
  /// ProviderScope(
  ///   overrides: [
  ///     ...ReportDI.overrides(
  ///       s3Service: s3ServiceInstance,
  ///       currentUserId: getCurrentUserId(),
  ///     ),
  ///   ],
  ///   child: MyApp(),
  /// )
  /// ```
  ///
  /// MIGRATION: ApiClient now comes from core.providers.apiClientProvider
  static List overrides({required S3Service s3Service, String? currentUserId}) {
    return [
      reportS3ServiceProvider.overrideWithValue(s3Service),
      if (currentUserId != null)
        reportCurrentUserIdProvider.overrideWithValue(currentUserId),
    ];
  }
}
