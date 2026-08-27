/// Seller Data Providers - Riverpod providers for seller data layer
///
/// This file provides all data dependencies for the seller feature using pure Riverpod.
/// Replaces the GetIt-based SellerApiDI dependency injection.
///
/// MIGRATION STATUS: Migrated from seller_api_di.dart (GetIt) to Riverpod
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/providers/core_providers.dart'
    show apiClientProvider, loggerServiceProvider, s3ServiceProvider;
import 'package:labuda/domains/user/preference/seller/data/remote/seller_remote_datasource.dart';
import 'package:labuda/domains/user/preference/seller/data/repositories/seller_repository_impl.dart';
import 'package:labuda/domains/user/preference/seller/data/services/store_photo_upload_service.dart';
import 'package:labuda/domains/user/preference/seller/domain/repositories/seller_repository.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// Seller Remote Datasource Provider
final sellerRemoteDatasourceProvider = Provider<SellerRemoteDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return SellerRemoteDatasource(apiClient: apiClient);
});

// =============================================================================
// SERVICE PROVIDERS
// =============================================================================

/// Store Photo Upload Service Provider - Upload farm/store logo to AWS S3
final storePhotoUploadServiceProvider = Provider<StorePhotoUploadService>((
  ref,
) {
  final s3Service = ref.watch(s3ServiceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return StorePhotoUploadService(s3Service: s3Service, logger: logger);
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Seller Repository Provider
///
/// Provides the implementation of SellerRepository with all dependencies.
/// This replaces the GetIt-based sellerRepository.
///
/// MIGRATION: Previously accessed via `sl<SellerRepository>()`
/// CONTEST DOMAIN REMOVED: Contest dependencies removed as Contest domain has been sunset
final sellerRepositoryProvider = Provider<SellerRepository>((ref) {
  final remoteDatasource = ref.watch(sellerRemoteDatasourceProvider);
  return SellerRepositoryImpl(remoteDatasource: remoteDatasource);
});
