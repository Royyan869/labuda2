/// Share Providers - Riverpod providers for share module
///
/// This file provides all dependencies for the share feature using pure Riverpod.
/// Replaces the GetIt-based ShareApiDI dependency injection.
///
/// MIGRATION STATUS: Migrated from share_api_di.dart (GetIt) to Riverpod
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import '../../domain/domain.dart';
import '../../data/data.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// Native Share Service Provider
final nativeShareServiceProvider = Provider<NativeShareService>((ref) {
  return NativeShareServiceImpl();
});

/// Share API Datasource Provider
///
/// **MIGRATED to Go Backend API**
final shareApiDatasourceProvider = Provider<ShareApiDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return ShareApiDatasource(apiClient);
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Share Repository Provider
///
/// Provides the API implementation of ShareRepository.
/// This replaces the GetIt-based ShareApiDI.repository.
///
/// MIGRATION: Previously accessed via `ShareApiDI.repository` or `sl<ShareRepository>()`
final shareRepositoryProvider = Provider<ShareRepository>((ref) {
  final datasource = ref.watch(shareApiDatasourceProvider);
  final nativeShareService = ref.watch(nativeShareServiceProvider);

  return ShareRepositoryApi(
    datasource: datasource,
    nativeShareService: nativeShareService,
  );
});
