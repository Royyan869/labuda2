import 'dart:io';
import 'package:labuda/core/core.dart';

/// Store Photo Upload Service - Upload farm logo to AWS S3
///
/// **OWNER:** Seller Domain
/// **PREVIOUSLY:** shared/services/store_photo_upload_service.dart
///
/// Storage Structure (Fixed filename - Auto-replace on re-upload):
/// - images/stores/{userId}.jpg         ← Always same filename (auto-overwrite)
///
/// Benefits:
/// - No duplicate files when seller re-uploads store logo
/// - Auto-cleanup of old files (S3 auto-deletes on overwrite)
/// - Consistent URLs
/// - Lower storage costs
/// - NO CONFLICT with user avatar (different path: images/avatars/{userId}.jpg)
///
/// **REALIGNMENT NOTE:** Although this service is used by both seller and profile domains
/// (for store photo upload in profile editing), it is conceptually owned by Seller domain
/// as it manages seller-specific assets.
class StorePhotoUploadService {
  final S3Service _s3Service;
  final ILoggerService _logger;

  static const String _storageFolder = 'images/stores';

  StorePhotoUploadService({
    required S3Service s3Service,
    required ILoggerService logger,
  }) : _s3Service = s3Service,
       _logger = logger;

  /// Upload store photo/logo
  /// Uses fixed filename '{userId}.jpg' - will auto-replace old file if exists
  Future<Result<String>> uploadStorePhoto({
    required String userId,
    required String imagePath,
  }) async {
    try {
      final file = File(imagePath);
      if (!await file.exists()) {
        return Result.error('File not found: $imagePath');
      }

      _logger.info(
        'Uploading store photo',
        extra: {'userId': userId, 'imagePath': imagePath},
      );

      // Upload to S3 with fixed key (auto-replaces existing file)
      final key = '$_storageFolder/$userId.jpg';
      final result = await _s3Service.uploadImageWithKey(file, key);

      if (result.isSuccess) {
        _logger.info(
          'Store photo uploaded successfully',
          extra: {'userId': userId, 'url': result.data},
        );
        return Result.success(result.data!);
      } else {
        _logger.error(
          'Failed to upload store photo',
          extra: {'userId': userId, 'error': result.error},
        );
        return Result.error('Failed to upload store photo: ${result.error}');
      }
    } catch (e, stackTrace) {
      _logger.error('Failed to upload store photo', stackTrace: stackTrace);
      return Result.error('Failed to upload store photo: ${e.toString()}');
    }
  }

  /// Get store photo URL from AWS S3 (with CloudFront CDN)
  static String getStorePhotoUrl(String userId) {
    final baseUrl = AppConstants.useCloudFront
        ? AppConstants.cdnBaseUrl
        : AppConstants.awsS3BaseUrl;

    // Fixed filename strategy: {userId}.jpg
    return '$baseUrl/$_storageFolder/$userId.jpg';
  }

  /// Delete store photo from AWS S3
  Future<Result<void>> deleteStorePhoto(String userId) async {
    try {
      final photoUrl = getStorePhotoUrl(userId);

      _logger.info(
        'Deleting store photo',
        extra: {'userId': userId, 'url': photoUrl},
      );

      final result = await _s3Service.deleteFile(photoUrl);

      if (result.isSuccess) {
        _logger.info(
          'Store photo deleted successfully',
          extra: {'userId': userId},
        );
        return Result.success(null);
      } else {
        return Result.error('Failed to delete store photo: ${result.error}');
      }
    } catch (e, stackTrace) {
      _logger.error('Failed to delete store photo', stackTrace: stackTrace);
      return Result.error('Failed to delete store photo: ${e.toString()}');
    }
  }
}
