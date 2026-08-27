/// Profile Data Providers - Riverpod providers for profile data layer
///
/// This file provides all data dependencies for the profile feature using pure Riverpod.
/// Replaces the GetIt-based ProfileApiDI dependency injection.
///
/// MIGRATION STATUS: Migrated from profile_api_di.dart (GetIt) to Riverpod
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/address_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/repositories/address_repository_api.dart';
import 'package:labuda/domains/user/profile/data/repositories/profile_repository_api.dart';
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/domains/user/profile/data/services/avatar_upload_service.dart';
import 'package:labuda/domains/user/profile/data/services/cover_photo_upload_service.dart';
import 'package:labuda/domains/user/profile/data/services/user_lookup_service.dart';
import 'package:labuda/domains/user/profile/data/services/user_sync_service.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_address_repository.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_profile_repository.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// User API Datasource Provider
final userApiDatasourceProvider = Provider<UserApiDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return UserApiDatasource(apiClient, logger: logger);
});

/// Address API Datasource Provider
final addressApiDatasourceProvider = Provider<AddressApiDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return AddressApiDatasource(apiClient, logger: logger);
});

// =============================================================================
// SERVICE PROVIDERS
// =============================================================================

/// User Sync Service Provider
final userSyncServiceProvider = Provider<UserSyncService>((ref) {
  final datasource = ref.watch(userApiDatasourceProvider);
  final logger = ref.watch(loggerServiceProvider);
  final ILocalStorageService localStorage = ref.watch(
    localStorageServiceProvider,
  );
  return UserSyncService(
    datasource: datasource,
    logger: logger,
    localStorage: localStorage,
  );
});

/// **R2.3 PROFILE DOMAIN EXTRACTION:**
/// User Lookup Service Provider - Lightweight user identity lookup.
/// Replaces UserSearchApiService from shared (deprecated).
///
/// Use for: TaggedUsersChips, user selection, mention resolution.
final userLookupServiceProvider = Provider<UserLookupService>((ref) {
  final datasource = ref.watch(userApiDatasourceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return UserLookupService(datasource: datasource, logger: logger);
});

/// **R2.3 PROFILE DOMAIN EXTRACTION:**
/// Avatar Cache Service Provider - Avatar URL caching and fetching.
/// Replaces UserAvatarApiService from shared (deprecated).
///
/// Use for: HybridAvatar, any component needing avatar URLs.
final avatarCacheServiceProvider = Provider<AvatarCacheService>((ref) {
  final datasource = ref.watch(userApiDatasourceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return AvatarCacheService(datasource: datasource, logger: logger);
});

/// Avatar Upload Service Provider - Upload profile avatar to AWS S3
final avatarUploadServiceProvider = Provider<AvatarUploadService>((ref) {
  final s3Service = ref.watch(s3ServiceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return AvatarUploadService(s3Service: s3Service, logger: logger);
});

/// Cover Photo Upload Service Provider - Upload profile cover to AWS S3
final coverPhotoUploadServiceProvider = Provider<CoverPhotoUploadService>((
  ref,
) {
  final s3Service = ref.watch(s3ServiceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return CoverPhotoUploadService(s3Service: s3Service, logger: logger);
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Profile Repository Provider
///
/// Provides the API implementation of IProfileRepository.
/// This replaces the GetIt-based ProfileApiDI.profileRepository.
///
/// MIGRATION: Previously accessed via `ProfileApiDI.profileRepository` or `sl<IProfileRepository>()`
final profileRepositoryProvider = Provider<IProfileRepository>((ref) {
  final datasource = ref.watch(userApiDatasourceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return ProfileRepositoryApi(datasource: datasource, logger: logger);
});

/// Address Repository Provider
///
/// Provides the API implementation of IAddressRepository.
/// This replaces the GetIt-based ProfileApiDI.addressRepository.
///
/// MIGRATION: Previously accessed via `ProfileApiDI.addressRepository` or `sl<IAddressRepository>()`
final addressRepositoryProvider = Provider<IAddressRepository>((ref) {
  final datasource = ref.watch(addressApiDatasourceProvider);
  return AddressRepositoryApi(datasource);
});
