/// Discount Data Providers - Riverpod providers for discount data layer
///
/// This file provides all data dependencies for the discount feature using pure Riverpod.
/// Replaces the GetIt-based DiscountApiDI dependency injection.
///
/// MIGRATION STATUS: Migrated from discount_api_di.dart (GetIt) to Riverpod
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/discount/data/datasources/discount_api_datasource.dart';
import 'package:labuda/domains/commerce/pricing/discount/data/repositories/discount_api_repository_impl.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/repositories/i_discount_repository.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// Discount API Datasource Provider
final discountApiDatasourceProvider = Provider<DiscountApiDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return DiscountApiDatasource(apiClient);
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Discount Repository Provider
///
/// Provides the API implementation of IDiscountRepository.
/// This replaces the GetIt-based DiscountApiDI.discountRepository.
///
/// MIGRATION: Previously accessed via `DiscountApiDI.discountRepository` or `sl<IDiscountRepository>()`
final discountRepositoryProvider = Provider<IDiscountRepository>((ref) {
  final datasource = ref.watch(discountApiDatasourceProvider);
  return DiscountApiRepositoryImpl(datasource);
});
