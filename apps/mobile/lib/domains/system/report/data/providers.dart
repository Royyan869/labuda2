/// Report Data Layer Providers
///
/// Riverpod providers for report data layer.
/// This file provides all data dependencies for the report feature using pure Riverpod.
///
/// MIGRATION: Repository providers moved from presentation layer to data layer
/// to enforce clean architecture boundaries.
library;

import 'dart:io';

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

/// S3Service provider - will be provided from main app
/// Note: This should be overridden with the actual S3Service implementation
final reportS3ServiceProvider = Provider<S3Service>((ref) {
  throw UnimplementedError('S3Service must be provided from main app');
});

/// Image Uploader provider - wraps S3Service
final reportImageUploaderProvider = Provider<ImageUploader>((ref) {
  final s3Service = ref.watch(reportS3ServiceProvider);
  return _S3ImageUploader(s3Service);
});

/// User Name Provider - for fetching user names
final reportUserNameProviderProvider = Provider<UserNameProvider>((ref) {
  throw UnimplementedError('UserNameProvider must be provided from main app');
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Report Repository provider
///
/// Provides the implementation of ReportRepository.
/// MIGRATION: Previously instantiated directly in presentation layer (violation)
final reportRepositoryProvider = Provider<ReportRepository>((ref) {
  final datasource = ref.watch(reportApiDatasourceProvider);
  final imageUploader = ref.watch(reportImageUploaderProvider);
  return ReportRepositoryImpl(
    datasource: datasource,
    imageUploader: imageUploader,
  );
});

/// Appeal Repository provider
///
/// Provides the implementation of AppealRepository.
/// MIGRATION: Previously instantiated directly in presentation layer (violation)
final appealRepositoryProvider = Provider<AppealRepository>((ref) {
  final datasource = ref.watch(reportApiDatasourceProvider);
  return AppealRepositoryImpl(datasource: datasource);
});

/// Warning Repository provider
///
/// Provides the implementation of WarningRepository.
/// MIGRATION: Previously instantiated directly in presentation layer (violation)
final warningRepositoryProvider = Provider<WarningRepository>((ref) {
  final datasource = ref.watch(reportApiDatasourceProvider);
  final nameProvider = ref.watch(reportUserNameProviderProvider);
  return WarningRepositoryImpl(
    datasource: datasource,
    nameProvider: nameProvider,
  );
});

// =============================================================================
// INTERNAL IMPLEMENTATIONS
// =============================================================================

/// S3 Image Uploader implementation
class _S3ImageUploader implements ImageUploader {
  final S3Service _s3Service;

  _S3ImageUploader(this._s3Service);

  @override
  Future<String> uploadImage({
    required String userId,
    required String filePath,
  }) async {
    final file = File(filePath);
    final result = await _s3Service.uploadImage(file);
    return result.fold(
      (error) => throw Exception('Failed to upload image: $error'),
      (url) => url,
    );
  }
}
