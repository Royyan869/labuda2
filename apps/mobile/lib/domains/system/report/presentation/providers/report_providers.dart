/// Report Module Providers
///
/// Riverpod providers for Report module presentation layer.
/// This is the main entry point for the presentation layer.
///
/// MIGRATION STATUS: Migrated from report_api_di.dart (GetIt) to Riverpod
/// Repository providers are now sourced from data layer (no boundary violation)
library;

import 'package:riverpod/riverpod.dart';

// Domain
import 'package:labuda/domains/system/report/domain/repositories/report_repository.dart';
import 'package:labuda/domains/system/report/domain/repositories/appeal_repository.dart';
import 'package:labuda/domains/system/report/domain/repositories/warning_repository.dart';

// Data Layer Providers - use prefix to avoid naming conflicts
import 'package:labuda/domains/system/report/data/providers.dart' as data;

// Presentation Providers
import 'package:labuda/domains/system/report/presentation/providers/report/report_state.dart';
import 'package:labuda/domains/system/report/presentation/providers/report/report_notifier.dart';
import 'package:labuda/domains/system/report/presentation/providers/appeal/appeal_state.dart';
import 'package:labuda/domains/system/report/presentation/providers/appeal/appeal_notifier.dart';

// Re-export data layer infrastructure providers for main app override
export 'package:labuda/domains/system/report/data/providers.dart'
    show reportS3ServiceProvider, reportUserNameProviderProvider;

// =====================
// Infrastructure Providers (Re-export from data layer)
// =====================

/// Current User ID provider - should be overridden with auth provider
///
/// Example: overrideWithProvider(reportCurrentUserIdProvider, authStateProvider.select((s) => s.user?.id))
final reportCurrentUserIdProvider = Provider<String?>((ref) => null);

// =====================
// Repository Providers (Sourced from data layer)
// =====================

/// Report Repository provider - sourced from data layer
/// No direct impl dependency in presentation layer
final reportRepositoryProvider = Provider<ReportRepository>((ref) {
  return ref.watch(data.reportRepositoryProvider);
});

/// Appeal Repository provider - sourced from data layer
/// No direct impl dependency in presentation layer
final appealRepositoryProvider = Provider<AppealRepository>((ref) {
  return ref.watch(data.appealRepositoryProvider);
});

/// Warning Repository provider - sourced from data layer
/// No direct impl dependency in presentation layer
final warningRepositoryProvider = Provider<WarningRepository>((ref) {
  return ref.watch(data.warningRepositoryProvider);
});

// =====================
// Report Notifier Providers
// =====================

/// Report Actions Notifier provider
final reportActionsNotifierProvider =
    NotifierProvider<ReportActionsNotifier, ReportActionsState>(() {
      return ReportActionsNotifier();
    });

/// Report List Notifier provider
final reportListNotifierProvider =
    NotifierProvider<ReportListNotifier, ReportListState>(() {
      return ReportListNotifier();
    });

// =====================
// Appeal Notifier Providers
// =====================

/// Appeal Actions Notifier provider
final appealActionsNotifierProvider =
    NotifierProvider<AppealActionsNotifier, AppealActionsState>(() {
      return AppealActionsNotifier();
    });

/// User Appeal List Notifier provider
final userAppealListNotifierProvider =
    NotifierProvider<UserAppealListNotifier, AppealListState>(() {
      return UserAppealListNotifier();
    });
