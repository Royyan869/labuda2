import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/core/media/media_upload_config.dart';
import 'package:hishumi/core/media/media_upload_orchestrator.dart';
import 'package:hishumi/core/providers/core_providers.dart';
import 'package:hishumi/core/services/s3_service.dart';

// ============================================================================
// PER-FILE UPLOAD PROGRESS CONTRACT
//
// A batch is judged FILE BY FILE. The old loop collapsed five files into one
// opaque boolean: a failure on file 3 stopped the loop, stranded files 4 and 5
// behind one generic snackbar, and the composer had no way to say WHICH file
// was stuck. This gates the per-file truth: state transitions, byte progress,
// independence (later files still run after an earlier failure), and a retry
// that re-runs only what failed.
// ============================================================================

class _FakeS3 implements S3Service {
  final Map<String, int> uploadCounts = {};
  String? failPath;
  bool reportsProgress = false;

  /// Explicit so the orchestrator's callback is stored, not swallowed by
  /// `noSuchMethod`.
  @override
  void Function(int sent, int total)? onPutProgress;

  @override
  Future<Result<String>> uploadImage(File file, {required String folder}) async {
    uploadCounts[file.path] = (uploadCounts[file.path] ?? 0) + 1;
    if (reportsProgress) onPutProgress?.call(50, 100);
    if (file.path == failPath) return Result.error('server menolak file ini');
    return Result.success('https://cdn.labuda/${file.path}');
  }

  @override
  Future<Result<String>> uploadVideo(
    File videoFile, {
    required String folder,
  }) async => uploadImage(videoFile, folder: folder);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  late _FakeS3 fake;
  late BuildContext context;

  List<MediaPendingItem> makeItems() => [
    MediaPendingItem(File('/tmp/satu.jpg')),
    MediaPendingItem(File('/tmp/dua.jpg')),
    MediaPendingItem(File('/tmp/tiga.jpg')),
  ];

  Future<void> pumpHost(WidgetTester tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [s3ServiceProvider.overrideWithValue(fake)],
        child: MaterialApp(
          // The orchestrator's failure snackbar needs a Scaffold — the composer
          // always sits inside one.
          home: Scaffold(
            body: Builder(
              builder: (ctx) {
                context = ctx;
                return const SizedBox();
              },
            ),
          ),
        ),
      ),
    );
  }

  setUp(() => fake = _FakeS3());

  testWidgets(
    'a failure on one file does not strand the ones after it',
    (tester) async {
      await pumpHost(tester);
      final items = makeItems();
      fake.reportsProgress = true;
      fake.failPath = items[1].file.path;

      // Phase + progress of each item AT THE MOMENT it changed. The progress
      // must be read here: a failed file resets it afterwards.
      final snapshots = <(List<MediaUploadPhase>, double)>[];
      final ok = await MediaUploadOrchestrator.uploadPending(
        context: context,
        config: MediaUploadConfig.forComment,
        items: items,
        onChanged: () => snapshots.add((
          items.map((e) => e.phase).toList(growable: false),
          items[1].progress,
        )),
      );

      expect(ok, isFalse);
      expect(items[0].phase, MediaUploadPhase.uploaded);
      expect(items[0].url, isNotNull);
      // The batch did NOT stop at the failure: the third file still ran.
      expect(items[2].phase, MediaUploadPhase.uploaded);
      expect(items[1].phase, MediaUploadPhase.failed);
      expect(items[1].error, 'server menolak file ini');
      expect(items[1].url, isNull);
      expect(fake.uploadCounts, {
        '/tmp/satu.jpg': 1,
        '/tmp/dua.jpg': 1,
        '/tmp/tiga.jpg': 1,
      });

      // Byte progress reached the item that was actually uploading.
      expect(
        snapshots.any(
          (snapshot) =>
              snapshot.$1[1] == MediaUploadPhase.uploading &&
              snapshot.$2 >= 0.5,
        ),
        isTrue,
      );

      // The progress hook never outlives the batch it belongs to.
      expect(fake.onPutProgress, isNull);
    },
  );

  testWidgets('a retry re-runs ONLY the file that failed', (tester) async {
    await pumpHost(tester);
    final items = makeItems();
    fake.failPath = items[1].file.path;

    await MediaUploadOrchestrator.uploadPending(
      context: context,
      config: MediaUploadConfig.forComment,
      items: items,
    );
    expect(items[1].phase, MediaUploadPhase.failed);

    // Same call again — the loop skips everything that already holds a URL.
    fake.failPath = null;
    final ok = await MediaUploadOrchestrator.uploadPending(
      context: context,
      config: MediaUploadConfig.forComment,
      items: items,
    );

    expect(ok, isTrue);
    expect(items[1].phase, MediaUploadPhase.uploaded);
    expect(items.every((item) => item.uploaded), isTrue);
    expect(fake.uploadCounts['/tmp/satu.jpg'], 1);
    expect(fake.uploadCounts['/tmp/dua.jpg'], 2);
    expect(fake.uploadCounts['/tmp/tiga.jpg'], 1);
  });
}
