import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/shared/shared.dart';

class _RecordingCommerceVideoS3Service extends S3Service {
  int? lastMaxDurationMs;
  String? lastDomainLabel;
  int uploadCalls = 0;

  @override
  Future<Result<CommerceMediaUploadResult>> uploadCommerceMedia(
    File file, {
    int maxVideoDurationMs = AppConstants.maxCommerceVideoDurationMs,
    String domainLabel = 'Commerce',
  }) async {
    uploadCalls += 1;
    lastMaxDurationMs = maxVideoDurationMs;
    lastDomainLabel = domainLabel;
    return Result.success(
      CommerceMediaUploadResult(
        key: 'videos/test.mp4',
        url: 'https://cdn.example.com/videos/test.mp4',
        type: 'video',
        duration: 179999,
      ),
    );
  }
}

Future<File> _tempVideoFile() async {
  final dir = await Directory.systemTemp.createTemp('labuda_commerce_video_');
  final file = File('${dir.path}${Platform.pathSeparator}sample.mp4');
  await file.writeAsBytes(const [1, 2, 3, 4]);
  return file;
}

void main() {
  test(
    'commerce coordinator reuses the same millisecond limit for listing',
    () async {
      final s3 = _RecordingCommerceVideoS3Service();
      final coordinator = CommerceMediaUploadCoordinator(s3Service: s3);
      final file = await _tempVideoFile();
      final item = CommerceLocalMediaItem.fromFile(file, selectionOrder: 0);

      final result = await coordinator.uploadTypedMedia(
        [item],
        maxVideoDurationMs: AppConstants.maxCommerceVideoDurationMs,
        domainLabel: 'Listing',
      );

      expect(result.isSuccess, isTrue);
      expect(s3.uploadCalls, 1);
      expect(s3.lastMaxDurationMs, AppConstants.maxCommerceVideoDurationMs);
      expect(s3.lastDomainLabel, 'Listing');
    },
  );

  test(
    'commerce coordinator reuses the same millisecond limit for auction',
    () async {
      final s3 = _RecordingCommerceVideoS3Service();
      final coordinator = CommerceMediaUploadCoordinator(s3Service: s3);
      final file = await _tempVideoFile();
      final item = CommerceLocalMediaItem.fromFile(file, selectionOrder: 0);

      final result = await coordinator.uploadTypedMedia(
        [item],
        maxVideoDurationMs: AppConstants.maxCommerceVideoDurationMs,
        domainLabel: 'Auction',
      );

      expect(result.isSuccess, isTrue);
      expect(s3.uploadCalls, 1);
      expect(s3.lastMaxDurationMs, AppConstants.maxCommerceVideoDurationMs);
      expect(s3.lastDomainLabel, 'Auction');
    },
  );
}
