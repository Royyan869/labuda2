/// Follow Data Providers - Riverpod providers for follow data layer
///
/// This file provides all data dependencies for the follow feature using pure Riverpod.
/// Replaces the GetIt-based FollowApiDI dependency injection.
///
/// MIGRATION STATUS: Migrated from follow_api_di.dart (GetIt) to Riverpod
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/follow/data/datasources/follow_api_datasource.dart';
import 'package:labuda/domains/social/follow/data/repositories/api/follow_repository_api.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// Follow API Datasource Provider
final followApiDatasourceProvider = Provider<FollowApiDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return FollowApiDatasource(apiClient, logger: logger);
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Follow Repository Provider
///
/// Provides the API implementation of IFollowRepository.
/// This replaces the GetIt-based FollowApiDI repository.
///
/// MIGRATION: Previously accessed via `FollowApiDI` or `sl<IFollowRepository>()`
final followRepositoryProvider = Provider<IFollowRepository>((ref) {
  final datasource = ref.watch(followApiDatasourceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return FollowRepositoryApi(datasource, logger: logger);
});
