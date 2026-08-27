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

      // Upload to S3 with fixed key (auto-replaces existing file)
      final key = '$_storageFolder/$userId.jpg';
      final result = await _s3Service.uploadImageWithKey(file, key);

      if (result.isSuccess) {
        _logger.info(
          'Avatar uploaded successfully',
          extra: {'userId': userId, 'url': result.data},
        );
        return Result.success(result.data!);
      } else {
        _logger.error(
          'Failed to upload avatar',
          extra: {'userId': userId, 'error': result.error},
        );
        return Result.error('Failed to upload avatar: ${result.error}');
      }
    } catch (e, stackTrace) {
      _logger.error('Failed to upload avatar', stackTrace: stackTrace);
      return Result.error('Failed to upload avatar: ${e.toString()}');
    }
  }

  /// Get avatar URL from AWS S3 (with CloudFront CDN)
  static String getAvatarUrl(String userId) {
    final baseUrl = AppConstants.useCloudFront
        ? AppConstants.cdnBaseUrl
        : AppConstants.awsS3BaseUrl;

    // Fixed filename strategy: {userId}.jpg
    return '$baseUrl/$_storageFolder/$userId.jpg';
  }

  /// Delete avatar from AWS S3
  Future<Result<void>> deleteAvatar(String userId) async {
    try {
      final photoUrl = getAvatarUrl(userId);

      _logger.info(
        'Deleting avatar',
        extra: {'userId': userId, 'url': photoUrl},
      );

      final result = await _s3Service.deleteFile(photoUrl);

      if (result.isSuccess) {
        _logger.info('Avatar deleted successfully', extra: {'userId': userId});
        return Result.success(null);
      } else {
        return Result.error('Failed to delete avatar: ${result.error}');
      }
    } catch (e, stackTrace) {
      _logger.error('Failed to delete avatar', stackTrace: stackTrace);
      return Result.error('Failed to delete avatar: ${e.toString()}');
    }
  }
}
