/// Report Module Providers
///
/// Riverpod providers for Report module presentation layer.
library;

import 'package:riverpod/riverpod.dart';

// Domain
import 'package:labuda/domains/system/report/domain/repositories/report_repository.dart';
import 'package:labuda/domains/system/report/domain/repositories/appeal_repository.dart';
import 'package:labuda/domains/system/report/domain/repositories/warning_repository.dart';

// Data Layer Providers
import 'package:labuda/domains/system/report/data/providers.dart' as data;

// Presentation Providers
import 'package:labuda/domains/system/report/presentation/providers/report/report_state.dart';
import 'package:labuda/domains/system/report/presentation/providers/report/report_notifier.dart';
import 'package:labuda/domains/system/report/presentation/providers/appeal/appeal_state.dart';
import 'package:labuda/domains/system/report/presentation/providers/appeal/appeal_notifier.dart';

export 'package:labuda/domains/system/report/data/providers.dart'
    show reportUserNameProviderProvider;

// =====================
// Infrastructure Providers
// =====================

/// Current User ID provider - should be overridden with auth provider
final reportCurrentUserIdProvider = Provider<String?>((ref) => null);

// =====================
// Repository Providers
// =====================

final reportRepositoryProvider = Provider<ReportRepository>((ref) {
  return ref.watch(data.reportRepositoryProvider);
});

final appealRepositoryProvider = Provider<AppealRepository>((ref) {
  return ref.watch(data.appealRepositoryProvider);
});

final warningRepositoryProvider = Provider<WarningRepository>((ref) {
  return ref.watch(data.warningRepositoryProvider);
});

// =====================
// Report Notifier Providers
// =====================

final reportActionsNotifierProvider =
    NotifierProvider<ReportActionsNotifier, ReportActionsState>(() {
      return ReportActionsNotifier();
    });

final reportListNotifierProvider =
    NotifierProvider<ReportListNotifier, ReportListState>(() {
      return ReportListNotifier();
    });

// =====================
// Appeal Notifier Providers
// =====================

final appealActionsNotifierProvider =
    NotifierProvider<AppealActionsNotifier, AppealActionsState>(() {
      return AppealActionsNotifier();
    });

final userAppealListNotifierProvider =
    NotifierProvider<UserAppealListNotifier, AppealListState>(() {
      return UserAppealListNotifier();
    });
