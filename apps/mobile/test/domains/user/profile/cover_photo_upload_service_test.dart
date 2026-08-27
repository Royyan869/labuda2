import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/services/cover_photo_upload_service.dart';

class _RecordingLogger extends Fake implements ILoggerService {
  @override
  Future<Result<void>> debug(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result<void>.success(null));

  @override
  Future<Result<void>> info(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result<void>.success(null));

  @override
  Future<Result<void>> warning(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result<void>.success(null));

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => Future.value(Result<void>.success(null));

  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => Future.value(Result<void>.success(null));
}

class _RecordingS3Service extends S3Service {
  String? lastKey;

  @override
  Future<Result<S3UploadResult>> uploadImageWithFixedKey(
    File imageFile,
    String key, {
    String mediaLabel = 'gambar',
  }) async {
    lastKey = key;
    return Result.success(
      S3UploadResult(
        key: key,
        url: 'https://cdn.example.com/media/$key',
      ),
    );
  }
}

class _FailingS3Service extends S3Service {
  @override
  Future<Result<S3UploadResult>> uploadImageWithFixedKey(
    File imageFile,
    String key, {
    String mediaLabel = 'gambar',
  }) async {
    return Result.error('backend refused $mediaLabel');
  }
}

void main() {
  test(
    'uploadCoverPhoto uses the canonical fixed storage key and returns storage key + read URL',
    () async {
      final tempDir = await Directory.systemTemp.createTemp(
        'cover-upload-test',
      );
      addTearDown(() async {
        await tempDir.delete(recursive: true);
      });

      final file = File('${tempDir.path}/cover.jpg');
      await file.writeAsBytes(List<int>.filled(16, 3));

      final s3 = _RecordingS3Service();
      final service = CoverPhotoUploadService(
        s3Service: s3,
        logger: _RecordingLogger(),
      );
      const userId = '62d7e998-f5d8-4486-be84-63d81f9c0e6f';

      final result = await service.uploadCoverPhoto(
        userId: userId,
        imagePath: file.path,
      );

      expect(result.isSuccess, isTrue);
      // Persistence value MUST be the canonical storage key (not a URL).
      expect(result.data!.storageKey, 'images/profile-covers/$userId.jpg');
      // Read URL is available for display.
      expect(
        result.data!.readUrl,
        'https://cdn.example.com/media/images/profile-covers/$userId.jpg',
      );
      expect(s3.lastKey, 'images/profile-covers/$userId.jpg');
      // No legacy prefix anywhere in the canonical contract.
      expect(
        result.data!.storageKey.contains('images/covers/'),
        isFalse,
      );
    },
  );

  test('uploadCoverPhoto surfaces cover-specific failure text', () async {
    final tempDir = await Directory.systemTemp.createTemp('cover-upload-fail');
    addTearDown(() async {
      await tempDir.delete(recursive: true);
    });

    final file = File('${tempDir.path}/cover.jpg');
    await file.writeAsBytes(List<int>.filled(16, 3));

    final service = CoverPhotoUploadService(
      s3Service: _FailingS3Service(),
      logger: _RecordingLogger(),
    );

    final result = await service.uploadCoverPhoto(
      userId: '62d7e998-f5d8-4486-be84-63d81f9c0e6f',
      imagePath: file.path,
    );

    expect(result.isError, isTrue);
    expect(result.error, contains('cover photo'));
  });
}
