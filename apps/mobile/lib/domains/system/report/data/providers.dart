/// Report Data Layer Providers
///
/// Riverpod providers for report data layer.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/report/data/data.dart';
import 'package:labuda/domains/system/report/domain/repositories/report_repository.dart';
import 'package:labuda/domains/system/report/domain/repositories/appeal_repository.dart';
import 'package:labuda/domains/system/report/domain/repositories/warning_repository.dart';
import 'package:labuda/domains/system/report/data/repositories/report_repository_impl.dart';
import 'package:labuda/domains/system/report/data/repositories/appeal_repository_impl.dart';
import 'package:labuda/domains/system/report/data/repositories/warning_repository_impl.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// Report ApiDatasource provider
final reportApiDatasourceProvider = Provider<ReportApiDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return ReportApiDatasourceImpl(apiClient);
});

// =============================================================================
// INFRASTRUCTURE PROVIDERS
// =============================================================================

/// User Name Provider - for fetching user names
final reportUserNameProviderProvider = Provider<UserNameProvider>((ref) {
  throw UnimplementedError('UserNameProvider must be provided from main app');
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Report Repository provider
final reportRepositoryProvider = Provider<ReportRepository>((ref) {
  final datasource = ref.watch(reportApiDatasourceProvider);
  return ReportRepositoryImpl(
    datasource: datasource,
  );
});

/// Appeal Repository provider
final appealRepositoryProvider = Provider<AppealRepository>((ref) {
  final datasource = ref.watch(reportApiDatasourceProvider);
  return AppealRepositoryImpl(datasource: datasource);
});

/// Warning Repository provider
final warningRepositoryProvider = Provider<WarningRepository>((ref) {
  final datasource = ref.watch(reportApiDatasourceProvider);
  final nameProvider = ref.watch(reportUserNameProviderProvider);
  return WarningRepositoryImpl(
    datasource: datasource,
    nameProvider: nameProvider,
  );
});
