import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/services/avatar_upload_service.dart';

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
  Future<Result<String>> uploadImageWithKey(
    File imageFile,
    String key,
  ) async {
    lastKey = key;
    return Result.success('https://d358tu61i1wrtt.cloudfront.net/$key');
  }
}

class _FailingS3Service extends S3Service {
  @override
  Future<Result<String>> uploadImageWithKey(
    File imageFile,
    String key,
  ) async {
    return Result.error('backend refused');
  }
}

void main() {
  test(
    'uploadAvatar uses the canonical storage key and returns the profile URL',
    () async {
      final tempDir = await Directory.systemTemp.createTemp(
        'avatar-upload-test',
      );
      addTearDown(() async {
        await tempDir.delete(recursive: true);
      });

      final file = File('${tempDir.path}/avatar.jpg');
      await file.writeAsBytes(List<int>.filled(16, 9));

      final s3 = _RecordingS3Service();
      final service = AvatarUploadService(
        s3Service: s3,
        logger: _RecordingLogger(),
      );
      const userId = '62d7e998-f5d8-4486-be84-63d81f9c0e6f';

      final result = await service.uploadAvatar(
        userId: userId,
        imagePath: file.path,
      );

      expect(result.isSuccess, isTrue);
      expect(result.data, AvatarUploadService.getAvatarUrl(userId));
      expect(s3.lastKey, 'images/avatars/$userId.jpg');
    },
  );

  test('uploadAvatar surfaces avatar-specific failure text', () async {
    final tempDir = await Directory.systemTemp.createTemp('avatar-upload-fail');
    addTearDown(() async {
      await tempDir.delete(recursive: true);
    });

    final file = File('${tempDir.path}/avatar.jpg');
    await file.writeAsBytes(List<int>.filled(16, 9));

    final service = AvatarUploadService(
      s3Service: _FailingS3Service(),
      logger: _RecordingLogger(),
    );

    final result = await service.uploadAvatar(
      userId: '62d7e998-f5d8-4486-be84-63d81f9c0e6f',
      imagePath: file.path,
    );

    expect(result.isError, isTrue);
    expect(result.error, contains('avatar'));
  });
}
