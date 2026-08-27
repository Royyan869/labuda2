/// Like Data Providers - Riverpod providers for like data layer
///
/// This file provides all data dependencies for the like feature using pure Riverpod.
/// Replaces the GetIt-based LikeApiDI dependency injection.
///
/// MIGRATION STATUS: Migrated from like_api_di.dart (GetIt) to Riverpod
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/like/data/like_repository_impl.dart';
import 'package:labuda/domains/social/like/data/remote/like_api_datasource.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// Like API Datasource Provider
final likeApiDatasourceProvider = Provider<LikeApiDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return LikeApiDatasource(apiClient, logger: logger);
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Like Repository Provider
///
/// Provides the API implementation of LikeRepository.
/// This replaces the GetIt-based LikeApiDI.likeRepository.
///
/// MIGRATION: Previously accessed via `LikeApiDI.likeRepository` or `sl<LikeRepository>()`
final likeRepositoryProvider = Provider<LikeRepository>((ref) {
  final datasource = ref.watch(likeApiDatasourceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return LikeRepositoryImpl(datasource, logger: logger);
});
