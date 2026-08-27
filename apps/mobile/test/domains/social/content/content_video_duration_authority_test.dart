import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_submission_handler.dart';

class _RecordingVideoUploadS3Service extends S3Service {
  int? lastMaxDurationMs;
  String? lastDomainLabel;
  int uploadCalls = 0;

  @override
  Future<Result<String>> uploadVideo(
    File videoFile, {
    int? maxDurationMs,
    String? domainLabel,
  }) async {
    uploadCalls += 1;
    lastMaxDurationMs = maxDurationMs;
    lastDomainLabel = domainLabel;
    return Result.success('https://cdn.example.com/content-video.mp4');
  }
}

Future<File> _tempVideoFile() async {
  final dir = await Directory.systemTemp.createTemp('labuda_content_video_');
  final file = File('${dir.path}${Platform.pathSeparator}content.mp4');
  await file.writeAsBytes(const [1, 2, 3, 4]);
  return file;
}

void main() {
  test(
    'content video uploads use the canonical 10-minute limit in ms',
    () async {
      final s3 = _RecordingVideoUploadS3Service();
      final video = await _tempVideoFile();

      final media = await ContentSubmissionHandler.uploadMedia(const [], [
        video,
      ], s3);

      expect(media, hasLength(1));
      expect(media.single.type, MediaType.video);
      expect(s3.uploadCalls, 1);
      expect(s3.lastMaxDurationMs, AppConstants.maxContentVideoDurationMs);
      expect(s3.lastDomainLabel, 'Content');
    },
  );
}
