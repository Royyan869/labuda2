/// Rating Data Providers - Riverpod providers for rating data layer
///
/// This file provides all data dependencies for the rating feature using pure Riverpod.
/// Replaces the GetIt-based RatingApiDI dependency injection.
///
/// MIGRATION STATUS: Migrated from rating_api_di.dart (GetIt) to Riverpod
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/rating/data/datasources/rating_api_datasource.dart';
import 'package:labuda/domains/social/rating/data/repositories/api/rating_repository_api.dart';
import 'package:labuda/domains/social/rating/domain/repositories/i_rating_repository.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// Rating API Datasource Provider
final ratingApiDatasourceProvider = Provider<RatingApiDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return RatingApiDatasource(apiClient);
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Rating Repository Provider
///
/// Provides the API implementation of IRatingRepository.
/// This replaces the GetIt-based RatingApiDI.ratingRepository.
///
/// MIGRATION: Previously accessed via `RatingApiDI.ratingRepository` or `sl<IRatingRepository>()`
final ratingRepositoryProvider = Provider<IRatingRepository>((ref) {
  final datasource = ref.watch(ratingApiDatasourceProvider);
  return RatingRepositoryApi(datasource);
});
