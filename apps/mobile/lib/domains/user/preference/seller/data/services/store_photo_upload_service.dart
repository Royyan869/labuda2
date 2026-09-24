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
/// Outcome of a successful store photo upload.
///
/// [storageKey] is the canonical persisted form (`images/stores/{userId}.jpg`)
/// — the ONLY value that may be written to the backend. [displayUrl] is the
/// resolved read URL for immediate UI rendering; never persist it.
class StorePhotoUploadOutcome {
  final String storageKey;
  final String displayUrl;

  const StorePhotoUploadOutcome({
    required this.storageKey,
    required this.displayUrl,
  });
}

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
  ///
  /// Returns the canonical [StorePhotoUploadOutcome]: the STORAGE KEY to
  /// persist plus the read URL for display-only rendering.
  Future<Result<StorePhotoUploadOutcome>> uploadStorePhoto({
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

      // Upload to S3 with canonical fixed key (verified ownership, overwrite).
      final key = '$_storageFolder/$userId.jpg';
      final result = await _s3Service.uploadImageWithFixedKey(
        file,
        key,
        mediaLabel: 'store photo',
      );

      if (result.isSuccess) {
        // Persist the canonical STORAGE KEY (images/stores/{userId}.jpg); the
        // read URL is display-only and is re-resolved server-side via
        // mediaresolve on hydration.
        final storageKey = result.data!.key;
        _logger.info(
          'Store photo uploaded successfully',
          extra: {'userId': userId, 'storageKey': storageKey, 'readUrl': result.data!.url},
        );
        return Result.success(
          StorePhotoUploadOutcome(storageKey: storageKey, displayUrl: result.data!.url),
        );
      } else {
        _logger.error(
          'Failed to upload store photo',
          extra: {'userId': userId, 'error': result.error, 'code': result.errorCode},
        );
        return Result.error(
          'Failed to upload store photo: ${result.error}',
          code: result.errorCode,
          statusCode: result.statusCode,
        );
      }
    } catch (e, stackTrace) {
      _logger.error('Failed to upload store photo', stackTrace: stackTrace);
      return Result.error('Failed to upload store photo: ${e.toString()}');
    }
  }

  // NOTE: There is intentionally NO deleteStorePhoto method. Canonical store
  // photo removal is a backend write — PATCH /seller/profile with
  // store_image_url: "" clears the DB reference (see edit_profile_save_handler).
  // A client-side delete stub would imply a parallel removal authority that
  // does not exist.
}
