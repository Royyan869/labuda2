import 'dart:io';

import 'package:labuda/core/core.dart';

/// Upload result containing successful and failed files
/// W14-B3: Prevents silent partial success - caller is aware of all upload outcomes
class UploadResult {
  final List<String> succeeded;
  final List<UploadFailure> failed;

  const UploadResult({this.succeeded = const [], this.failed = const []});

  bool get hasFailures => failed.isNotEmpty;
  bool get hasSuccesses => succeeded.isNotEmpty;
  bool get isComplete => !hasFailures;
  int get totalAttempted => succeeded.length + failed.length;
}

/// Represents a single failed upload with diagnostic info
class UploadFailure {
  final String fileName;
  final String error;
  final UploadFileType type;

  const UploadFailure({
    required this.fileName,
    required this.error,
    required this.type,
  });
}

/// Type of file being uploaded
enum UploadFileType { image, video }

/// Media upload service for universal content
///
/// MIGRATED from Firebase Storage to AWS S3
/// NOTE: CreateContentUseCase dependency removed - use case not available
///
/// W14-B3: Fixed silent upload failure - now returns structured UploadResult
/// with both successes and failures, preventing partial success from going unnoticed.
class MediaUploadService {
  final ILoggerService _logger;
  final S3Service _s3Service;

  MediaUploadService({required ILoggerService logger, S3Service? s3Service})
    : _logger = logger,
      _s3Service = s3Service ?? S3Service();

  /// Uploads media files and returns structured result with successes and failures.
  /// W14-B3: No longer silent on failures - all outcomes are tracked and reported.
  ///
  /// Returns [UploadResult] containing:
  /// - [succeeded]: List of URLs for successfully uploaded files
  /// - [failed]: List of [UploadFailure] with file name, error, and type
  Future<UploadResult> uploadMedia({
    required List<File> images,
    required List<File> videos,
    required String userId,
    required String contentType,
  }) async {
    final List<String> succeeded = [];
    final List<UploadFailure> failed = [];

    // Upload images using S3 service
    for (final image in images) {
      final fileName = _getFileName(image);
      final result = await _s3Service.uploadImage(image);

      if (result.isSuccess && result.data != null) {
        succeeded.add(result.data!);
      } else {
        failed.add(
          UploadFailure(
            fileName: fileName,
            error: result.error ?? 'Unknown upload error',
            type: UploadFileType.image,
          ),
        );
        _logger.warning(
          'Image upload failed: $fileName',
          extra: {'error': result.error},
        );
      }
    }

    // Upload videos using S3 service
    for (final video in videos) {
      final fileName = _getFileName(video);
      final result = await _s3Service.uploadVideo(video);

      if (result.isSuccess && result.data != null) {
        succeeded.add(result.data!);
      } else {
        failed.add(
          UploadFailure(
            fileName: fileName,
            error: result.error ?? 'Unknown upload error',
            type: UploadFileType.video,
          ),
        );
        _logger.warning(
          'Video upload failed: $fileName',
          extra: {'error': result.error},
        );
      }
    }

    final result = UploadResult(succeeded: succeeded, failed: failed);

    // Log summary for monitoring
    _logger.info(
      'Upload complete: ${succeeded.length} succeeded, ${failed.length} failed',
      extra: {
        'total_attempted': result.totalAttempted,
        'has_failures': result.hasFailures,
        'user_id': userId,
        'content_type': contentType,
      },
    );

    return result;
  }

  /// Extract file name from File path for error reporting
  String _getFileName(File file) {
    try {
      return file.path.split(RegExp(r'[/\\]')).last;
    } catch (_) {
      return 'unknown_file';
    }
  }

  Future<void> submitContent(Map<String, dynamic> data) async {
    final mode = data['mode'] as String;

    switch (mode.toLowerCase()) {
      case 'content':
        await _submitContent(data);
        break;
      default:
        throw Exception('Mode $mode belum tersedia');
    }
  }

  Future<void> _submitContent(Map<String, dynamic> data) async {
    // NOTE: CreateContentUseCase not available - feature disabled
    // Content submission is not available through this service
    throw Exception(
      'Content submission not available - use content creation flow directly',
    );
  }
}
