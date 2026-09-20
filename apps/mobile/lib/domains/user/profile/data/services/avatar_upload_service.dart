import 'dart:io';
import 'package:labuda/core/core.dart';

/// Avatar Upload Service - Upload profile avatar to AWS S3
///
/// Storage Structure (Fixed filename - Auto-replace on re-upload):
/// - images/avatars/{userId}.jpg         <- Always same filename (auto-overwrite)
///
/// Benefits:
/// - No duplicate files when user re-uploads avatar
/// - Auto-cleanup of old files (S3 auto-deletes on overwrite)
/// - Consistent URLs
/// - Lower storage costs
class AvatarUploadService {
  final S3Service _s3Service;
  final ILoggerService _logger;

  static const String _storageFolder = 'images/avatars';

  AvatarUploadService({
    required S3Service s3Service,
    required ILoggerService logger,
  }) : _s3Service = s3Service,
       _logger = logger;

  /// Upload avatar photo
  /// Uses fixed filename '{userId}.jpg' - will auto-replace old file if exists
  Future<Result<String>> uploadAvatar({
    required String userId,
    required String imagePath,
  }) async {
    try {
      final file = File(imagePath);
      if (!await file.exists()) {
        return Result.error('File not found: $imagePath');
      }

      _logger.info(
        'Uploading avatar',
        extra: {'userId': userId, 'imagePath': imagePath},
      );

      // Upload to S3 with canonical fixed key (verified ownership, overwrite).
      final key = '$_storageFolder/$userId.jpg';
      final result = await _s3Service.uploadImageWithFixedKey(
        file,
        key,
        mediaLabel: 'avatar',
      );

      if (result.isSuccess) {
        // Persist/display uses the canonical read_url from S3UploadResult.
        final readUrl = result.data!.url;
        _logger.info(
          'Avatar uploaded successfully',
          extra: {'userId': userId, 'url': readUrl, 'storageKey': result.data!.key},
        );
        return Result.success(readUrl);
      } else {
        _logger.error(
          'Failed to upload avatar',
          extra: {'userId': userId, 'error': result.error, 'code': result.errorCode},
        );
        return Result.error(
          'Failed to upload avatar: ${result.error}',
          code: result.errorCode,
          statusCode: result.statusCode,
        );
      }
    } catch (e, stackTrace) {
      _logger.error('Failed to upload avatar', stackTrace: stackTrace);
      return Result.error('Failed to upload avatar: ${e.toString()}');
    }
  }

  /// Avatar removal clears the DB reference (photoUrl = null) — no S3 delete.
  /// This method is retained as a no-op success for call sites that expect it,
  /// per locked delete decision (no /media/delete-url, no DeleteObject).
  Future<Result<void>> deleteAvatar(String userId) async {
    _logger.info('Avatar removal — DB reference clear only (no S3 delete)', extra: {'userId': userId});
    return Result.success(null);
  }
}
