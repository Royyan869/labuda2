import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/preference/seller/data/services/store_photo_upload_service.dart';

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
  String? lastMediaLabel;

  @override
  Future<Result<S3UploadResult>> uploadImageWithFixedKey(
    File imageFile,
    String key, {
    String mediaLabel = 'gambar',
  }) async {
    lastKey = key;
    lastMediaLabel = mediaLabel;
    return Result.success(S3UploadResult(key: key, url: 'https://cdn.example.com/$key'));
  }
}

class _FailingS3Service extends S3Service {
  @override
  Future<Result<S3UploadResult>> uploadImageWithFixedKey(
    File imageFile,
    String key, {
    String mediaLabel = 'gambar',
  }) async {
    return Result.error('backend refused gambar', code: 'INVALID_STORAGE_KEY', statusCode: 400);
  }
}

void main() {
  test('uploadStorePhoto uses the canonical fixed storage key and returns the storage key', () async {
    final tempDir = await Directory.systemTemp.createTemp('store-photo-test');
    addTearDown(() async {
      await tempDir.delete(recursive: true);
    });

    final file = File('${tempDir.path}/store.jpg');
    await file.writeAsBytes(List<int>.filled(16, 7));

    final s3 = _RecordingS3Service();
    final service = StorePhotoUploadService(
      s3Service: s3,
      logger: _RecordingLogger(),
    );

    final result = await service.uploadStorePhoto(
      userId: '62d7e998-f5d8-4486-be84-63d81f9c0e6f',
      imagePath: file.path,
    );

    expect(result.isSuccess, isTrue);
    // Canonical: the caller receives the STORAGE KEY (to persist) plus the
    // read URL (display-only) — never conflated into a single string.
    expect(
      result.data!.storageKey,
      'images/stores/62d7e998-f5d8-4486-be84-63d81f9c0e6f.jpg',
    );
    expect(
      result.data!.displayUrl,
      'https://cdn.example.com/images/stores/62d7e998-f5d8-4486-be84-63d81f9c0e6f.jpg',
    );
    expect(
      s3.lastKey,
      'images/stores/62d7e998-f5d8-4486-be84-63d81f9c0e6f.jpg',
    );
    expect(s3.lastMediaLabel, 'store photo');
  });

  test('uploadStorePhoto surfaces store photo failure text with structured error', () async {
    final tempDir = await Directory.systemTemp.createTemp('store-photo-fail');
    addTearDown(() async {
      await tempDir.delete(recursive: true);
    });

    final file = File('${tempDir.path}/store.jpg');
    await file.writeAsBytes(List<int>.filled(16, 7));

    final service = StorePhotoUploadService(
      s3Service: _FailingS3Service(),
      logger: _RecordingLogger(),
    );

    final result = await service.uploadStorePhoto(
      userId: '62d7e998-f5d8-4486-be84-63d81f9c0e6f',
      imagePath: file.path,
    );

    expect(result.isError, isTrue);
    expect(result.error, contains('store photo'));
    expect(result.errorCode, 'INVALID_STORAGE_KEY');
    expect(result.statusCode, 400);
  });

}
