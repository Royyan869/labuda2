/// Content Data Providers - Riverpod providers for content data layer
///
/// This file provides all data dependencies for the content feature using pure Riverpod.
/// Replaces the GetIt-based ContentApiDI dependency injection.
///
/// MIGRATION STATUS: Migrated from content_api_di.dart (GetIt) to Riverpod
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/data/content_repository_impl.dart';
import 'package:labuda/domains/social/content/data/remote/content_api_datasource.dart';
import 'package:labuda/domains/social/content/domain/repositories/content_repository.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// Content API Datasource Provider
final contentApiDatasourceProvider = Provider<ContentApiDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return ContentApiDatasource(apiClient);
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Content Repository Provider
///
/// Provides the API implementation of ContentRepository.
/// This replaces the GetIt-based ContentApiDI.contentRepository.
///
/// MIGRATION: Previously accessed via `ContentApiDI.contentRepository` or `sl<ContentRepository>()`
final contentRepositoryProvider = Provider<ContentRepository>((ref) {
  final datasource = ref.watch(contentApiDatasourceProvider);
  final apiClient = ref.watch(apiClientProvider);
  return ContentRepositoryImpl(datasource, apiClient);
});
