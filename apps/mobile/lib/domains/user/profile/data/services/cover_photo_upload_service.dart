import 'dart:io';
import 'package:labuda/core/core.dart';

/// Cover Photo Upload Service - Upload profile cover via the canonical
/// backend media authority (STAGE 4F-1/4F-2).
///
/// Canonical storage key (fixed, owner-scoped, backend-validated):
/// - images/profile-covers/{userId}.jpg
///
/// The service:
/// 1. Requests a presigned PUT for the canonical fixed key from
///    POST /media/upload-url (backend validates caller ownership; legacy
///    images/covers/ keys are rejected).
/// 2. PUTs the file bytes.
/// 3. Returns the backend-confirmed STORAGE KEY (what must be persisted)
///    plus the resolved read URL (what the UI displays).
///
/// Removal is NOT an S3 delete: clearing the cover is a PATCH of
/// cover_photo_url = "" (backend converts to NULL). There is no delete
/// endpoint by design.
class CoverPhotoUploadService {
  final S3Service _s3Service;
  final ILoggerService _logger;

  static const String storageFolder = 'images/profile-covers';

  CoverPhotoUploadService({
    required S3Service s3Service,
    required ILoggerService logger,
  }) : _s3Service = s3Service,
       _logger = logger;

  /// Canonical fixed storage key for a user's cover photo.
  static String storageKeyFor(String userId) =>
      '$storageFolder/$userId.jpg';

  /// Upload cover photo to the canonical fixed key.
  ///
  /// Returns the STORAGE KEY on success (the value the backend persists).
  /// The read URL is available via [CoverUploadResult.readUrl] for immediate
  /// display without waiting for a refetch.
  Future<Result<CoverUploadResult>> uploadCoverPhoto({
    required String userId,
    required String imagePath,
  }) async {
    try {
      final file = File(imagePath);
      if (!await file.exists()) {
        return Result.error('File not found: $imagePath');
      }

      _logger.info(
        'Uploading cover photo',
        extra: {'userId': userId, 'imagePath': imagePath},
      );

      final key = storageKeyFor(userId);
      final result = await _s3Service.uploadImageWithFixedKey(
        file,
        key,
        mediaLabel: 'cover photo',
      );

      if (result.isSuccess) {
        final upload = result.data!;
        _logger.info(
          'Cover photo uploaded successfully',
          extra: {'userId': userId, 'storageKey': upload.key},
        );
        return Result.success(
          CoverUploadResult(storageKey: upload.key, readUrl: upload.url),
        );
      } else {
        _logger.error(
          'Failed to upload cover photo',
          extra: {'userId': userId, 'error': result.error},
        );
        return Result.error('Failed to upload cover photo: ${result.error}');
      }
    } catch (e, stackTrace) {
      _logger.error('Failed to upload cover photo', stackTrace: stackTrace);
      return Result.error('Failed to upload cover photo: ${e.toString()}');
    }
  }
}

/// Result of a canonical cover upload: the storage key to persist and the
/// read URL to display.
class CoverUploadResult {
  final String storageKey;
  final String readUrl;

  const CoverUploadResult({required this.storageKey, required this.readUrl});
}
