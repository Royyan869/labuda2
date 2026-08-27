import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/shared/shared.dart';

class _ScriptedS3Service extends S3Service {
  _ScriptedS3Service(this._uploads, {this.failDeleteFor = const {}})
    : uploadCalls = [],
      deletedKeys = [],
      failedDeleteKeys = [];

  final List<Future<Result<CommerceMediaUploadResult>> Function(File)> _uploads;
  final Set<String> failDeleteFor;
  final List<File> uploadCalls;
  final List<String> deletedKeys;
  final List<String> failedDeleteKeys;

  @override
  Future<Result<CommerceMediaUploadResult>> uploadCommerceMedia(
    File file, {
    int maxVideoDurationMs = AppConstants.maxCommerceVideoDurationMs,
    String domainLabel = 'Commerce',
  }) {
    uploadCalls.add(file);
    if (_uploads.isEmpty) {
      return Future.value(Result.error('unexpected upload'));
    }
    return _uploads.removeAt(0)(file);
  }

  @override
  Future<Result<void>> deleteFile(String storageKey) async {
    deletedKeys.add(storageKey);
    if (failDeleteFor.contains(storageKey)) {
      failedDeleteKeys.add(storageKey);
      return Result.error('delete failed for $storageKey');
    }
    return Result.success(null);
  }
}

File _tempFile(String name, List<int> bytes) {
  final dir = Directory.systemTemp.createTempSync('labuda_local_media_');
  final filePath = '${dir.path}${Platform.pathSeparator}$name';
  final file = File(filePath);
  file.parent.createSync(recursive: true);
  file.writeAsBytesSync(bytes);
  return file;
}

Result<CommerceMediaUploadResult> _imageUpload({
  required String key,
  required String url,
}) {
  return Result.success(
    CommerceMediaUploadResult(key: key, url: url, type: 'image'),
  );
}

Result<CommerceMediaUploadResult> _videoUpload({
  required String key,
  required String url,
  String? thumbnailUrl,
  String? thumbnailStorageKey,
  String? localPosterPath,
}) {
  return Result.success(
    CommerceMediaUploadResult(
      key: key,
      url: url,
      type: 'video',
      thumbnailUrl: thumbnailUrl,
      thumbnailStorageKey: thumbnailStorageKey,
      localPosterPath: localPosterPath,
    ),
  );
}

void main() {
  test(
    'CommerceLocalMediaItem stable id and request dto position are deterministic',
    () {
      final file = _tempFile('alpha.jpg', [1, 2, 3, 4]);
      final item = CommerceLocalMediaItem.fromFile(file, selectionOrder: 7);
      final same = CommerceLocalMediaItem.fromFile(file, selectionOrder: 7);

      expect(item.stableLocalId, same.stableLocalId);
      expect(item.selectionOrder, 7);

      final dto = item
          .copyWith(
            uploadedUrl: 'https://cdn.example.com/alpha.jpg',
            uploadState: CommerceLocalMediaUploadState.uploaded,
          )
          .toRequestDto(position: 3);

      expect(dto.position, 3);
      expect(dto.url, 'https://cdn.example.com/alpha.jpg');
      expect(dto.toJson(), containsPair('position', 3));
    },
  );

  test(
    'stable IDs survive rebuilds, retries, reorder, and sibling removal',
    () {
      final firstPath = _tempFile('first.jpg', [1, 2, 3, 4]);
      final secondPath = _tempFile('second.jpg', [5, 6, 7, 8]);

      final first = CommerceLocalMediaItem.fromFile(
        firstPath,
        selectionOrder: 0,
      );
      final firstRetry = CommerceLocalMediaItem.fromFile(
        firstPath,
        selectionOrder: 0,
      );
      final second = CommerceLocalMediaItem.fromFile(
        secondPath,
        selectionOrder: 1,
      );

      expect(first.stableLocalId, firstRetry.stableLocalId);
      expect(first.stableLocalId, isNot(second.stableLocalId));

      final reordered = [second, first];
      expect(reordered.last.stableLocalId, first.stableLocalId);
      expect(reordered.first.stableLocalId, second.stableLocalId);

      final siblingIds = [
        first,
        second,
      ].map((item) => item.stableLocalId).toList(growable: false);
      final withoutFirst = [second];
      expect(withoutFirst.single.stableLocalId, siblingIds[1]);
    },
  );

  test('same basename with different path still produces distinct IDs', () {
    final first = CommerceLocalMediaItem.fromFile(
      _tempFile('cover.jpg', [1, 2, 3, 4]),
      selectionOrder: 0,
    );
    final second = CommerceLocalMediaItem.fromFile(
      _tempFile('nested${Platform.pathSeparator}cover.jpg', [1, 2, 3, 4]),
      selectionOrder: 0,
    );

    expect(first.stableLocalId, isNot(second.stableLocalId));
  });

  test(
    'same path selected twice yields distinct IDs when selection order changes',
    () {
      final file = _tempFile('duplicate.jpg', [9, 8, 7, 6]);
      final first = CommerceLocalMediaItem.fromFile(file, selectionOrder: 0);
      final second = CommerceLocalMediaItem.fromFile(file, selectionOrder: 1);

      expect(first.stableLocalId, isNot(second.stableLocalId));
    },
  );

  test('stable ID source contract stays pinned to the canonical fingerprint', () {
    final source = File(
      'lib/domains/commerce/catalog/shared/data/models/commerce_local_media_item.dart',
    ).readAsStringSync().replaceAll('\r\n', '\n');

    expect(source, contains(r"replaceAll('\\', '/')"));
    expect(source, contains('file.lengthSync()'));
    expect(source, contains('selectionOrder'));
    expect(source, contains('mediaType'));
    expect(source, isNot(contains('DateTime.now')));
    expect(source, isNot(contains('millisecondsSinceEpoch')));
    expect(source, isNot(contains('UniqueKey')));
  });

  test(
    'image-only success produces a single image DTO and no cleanup',
    () async {
      final service = _ScriptedS3Service([
        (_) async => _imageUpload(
          key: 'images/1_user.jpg',
          url: 'https://cdn.example.com/images/1_user.jpg',
        ),
      ]);
      final coordinator = CommerceMediaUploadCoordinator(s3Service: service);

      final result = await coordinator.uploadTypedMedia([
        CommerceLocalMediaItem.fromFile(
          _tempFile('image-a.jpg', [1, 2, 3, 4]),
          selectionOrder: 0,
        ),
      ]);

      expect(result.isSuccess, isTrue);
      expect(result.data?.cancelled, isFalse);
      expect(result.data?.media, hasLength(1));
      expect(result.data?.media.single.type, 'image');
      expect(service.deletedKeys, isEmpty);
    },
  );

  test(
    'video-only success preserves poster metadata without cleanup',
    () async {
      final service = _ScriptedS3Service([
        (_) async => _videoUpload(
          key: 'videos/1_user.mp4',
          url: 'https://cdn.example.com/videos/1_user.mp4',
          thumbnailUrl: 'https://cdn.example.com/videos/1_user.mp4_poster.jpg',
          thumbnailStorageKey: 'videos/1_user.mp4_poster.jpg',
          localPosterPath: r'C:\tmp\videos_1_user.mp4_poster.jpg',
        ),
      ]);
      final coordinator = CommerceMediaUploadCoordinator(s3Service: service);

      final result = await coordinator.uploadTypedMedia([
        CommerceLocalMediaItem.fromFile(
          _tempFile('video-a.mp4', [1, 2, 3, 4]),
          selectionOrder: 0,
        ),
      ]);

      expect(result.isSuccess, isTrue);
      expect(result.data?.media, hasLength(1));
      expect(result.data?.media.single.type, 'video');
      expect(result.data?.media.single.thumbnailUrl, isNotNull);
      expect(service.deletedKeys, isEmpty);
    },
  );

  test('mixed success preserves ordering and typed positions', () async {
    final service = _ScriptedS3Service([
      (_) async => _imageUpload(
        key: 'images/1_user.jpg',
        url: 'https://cdn.example.com/images/1_user.jpg',
      ),
      (_) async => _videoUpload(
        key: 'videos/2_user.mp4',
        url: 'https://cdn.example.com/videos/2_user.mp4',
        thumbnailUrl: 'https://cdn.example.com/videos/2_user.mp4_poster.jpg',
        thumbnailStorageKey: 'videos/2_user.mp4_poster.jpg',
        localPosterPath: r'C:\tmp\videos_2_user.mp4_poster.jpg',
      ),
    ]);
    final coordinator = CommerceMediaUploadCoordinator(s3Service: service);

    final result = await coordinator.uploadTypedMedia([
      CommerceLocalMediaItem.fromFile(
        _tempFile('image-b.jpg', [1, 2, 3, 4]),
        selectionOrder: 0,
      ),
      CommerceLocalMediaItem.fromFile(
        _tempFile('video-b.mp4', [5, 6, 7, 8]),
        selectionOrder: 1,
      ),
    ]);

    expect(result.isSuccess, isTrue);
    expect(result.data?.media, hasLength(2));
    expect(result.data?.media[0].position, 0);
    expect(result.data?.media[1].position, 1);
    expect(result.data?.media[0].type, 'image');
    expect(result.data?.media[1].type, 'video');
    expect(service.deletedKeys, isEmpty);
  });

  test('first media upload failure does not trigger cleanup', () async {
    final service = _ScriptedS3Service([
      (_) async => Result.error('first upload failed'),
    ]);
    final coordinator = CommerceMediaUploadCoordinator(s3Service: service);

    final result = await coordinator.uploadTypedMedia([
      CommerceLocalMediaItem.fromFile(
        _tempFile('first-fail.jpg', [1, 2, 3, 4]),
        selectionOrder: 0,
      ),
      CommerceLocalMediaItem.fromFile(
        _tempFile('second.jpg', [5, 6, 7, 8]),
        selectionOrder: 1,
      ),
    ]);

    expect(result.isSuccess, isFalse);
    expect(service.deletedKeys, isEmpty);
  });

  test(
    'second upload failure cleans the first uploaded object only once',
    () async {
      final service = _ScriptedS3Service([
        (_) async => _imageUpload(
          key: 'images/cleanup-user.jpg',
          url: 'https://cdn.example.com/images/cleanup-user.jpg',
        ),
        (_) async => Result.error('second upload failed'),
      ]);
      final coordinator = CommerceMediaUploadCoordinator(s3Service: service);

      final result = await coordinator.uploadTypedMedia([
        CommerceLocalMediaItem.fromFile(
          _tempFile('first.jpg', [1, 2, 3, 4]),
          selectionOrder: 0,
        ),
        CommerceLocalMediaItem.fromFile(
          _tempFile('second.jpg', [5, 6, 7, 8]),
          selectionOrder: 1,
        ),
      ]);

      expect(result.isSuccess, isFalse);
      expect(service.deletedKeys, ['images/cleanup-user.jpg']);
    },
  );

  test(
    'poster upload failure after video success cleans video and poster-ledger objects',
    () async {
      final service = _ScriptedS3Service([
        (_) async => _videoUpload(
          key: 'videos/ledger-user.mp4',
          url: 'https://cdn.example.com/videos/ledger-user.mp4',
          thumbnailUrl:
              'https://cdn.example.com/videos/ledger-user.mp4_poster.jpg',
          thumbnailStorageKey: 'videos/ledger-user.mp4_poster.jpg',
          localPosterPath: r'C:\tmp\videos_ledger-user.mp4_poster.jpg',
        ),
        (_) async => Result.error('poster upload failed'),
      ]);
      final coordinator = CommerceMediaUploadCoordinator(s3Service: service);

      final result = await coordinator.uploadTypedMedia([
        CommerceLocalMediaItem.fromFile(
          _tempFile('video-ledger.mp4', [1, 2, 3, 4]),
          selectionOrder: 0,
        ),
        CommerceLocalMediaItem.fromFile(
          _tempFile('video-fail.mp4', [5, 6, 7, 8]),
          selectionOrder: 1,
        ),
      ]);

      expect(result.isSuccess, isFalse);
      expect(
        service.deletedKeys,
        containsAll([
          'videos/ledger-user.mp4',
          'videos/ledger-user.mp4_poster.jpg',
        ]),
      );
    },
  );

  test(
    'cleanup errors do not replace the original upload/publication failure',
    () async {
      final service = _ScriptedS3Service(
        [
          (_) async => _imageUpload(
            key: 'images/original-user.jpg',
            url: 'https://cdn.example.com/images/original-user.jpg',
          ),
          (_) async => Result.error('second upload failed'),
        ],
        failDeleteFor: {'images/original-user.jpg'},
      );
      final coordinator = CommerceMediaUploadCoordinator(s3Service: service);

      final result = await coordinator.uploadTypedMedia([
        CommerceLocalMediaItem.fromFile(
          _tempFile('cleanup.jpg', [1, 2, 3, 4]),
          selectionOrder: 0,
        ),
        CommerceLocalMediaItem.fromFile(
          _tempFile('cleanup-2.jpg', [5, 6, 7, 8]),
          selectionOrder: 1,
        ),
      ]);

      expect(result.isSuccess, isFalse);
      expect(result.error, 'second upload failed');
      expect(service.deletedKeys, ['images/original-user.jpg']);
    },
  );

  test(
    'partial cleanup failure keeps trying later objects and preserves the primary error',
    () async {
      final service = _ScriptedS3Service(
        [
          (_) async => _imageUpload(
            key: 'images/cleanup-a.jpg',
            url: 'https://cdn.example.com/images/cleanup-a.jpg',
          ),
          (_) async => _videoUpload(
            key: 'videos/cleanup-b.mp4',
            url: 'https://cdn.example.com/videos/cleanup-b.mp4',
            thumbnailUrl:
                'https://cdn.example.com/videos/cleanup-b.mp4_poster.jpg',
            thumbnailStorageKey: 'videos/cleanup-b.mp4_poster.jpg',
            localPosterPath: r'C:\tmp\videos_cleanup-b.mp4_poster.jpg',
          ),
          (_) async => _imageUpload(
            key: 'images/cleanup-c.jpg',
            url: 'https://cdn.example.com/images/cleanup-c.jpg',
          ),
          (_) async => Result.error('fourth upload failed'),
        ],
        failDeleteFor: {'images/cleanup-c.jpg'},
      );
      final coordinator = CommerceMediaUploadCoordinator(s3Service: service);

      final result = await coordinator.uploadTypedMedia([
        CommerceLocalMediaItem.fromFile(
          _tempFile('cleanup-a.jpg', [1, 2, 3, 4]),
          selectionOrder: 0,
        ),
        CommerceLocalMediaItem.fromFile(
          _tempFile('cleanup-b.mp4', [5, 6, 7, 8]),
          selectionOrder: 1,
        ),
        CommerceLocalMediaItem.fromFile(
          _tempFile('cleanup-c.jpg', [9, 10, 11, 12]),
          selectionOrder: 2,
        ),
        CommerceLocalMediaItem.fromFile(
          _tempFile('cleanup-d.jpg', [13, 14, 15, 16]),
          selectionOrder: 3,
        ),
      ]);

      expect(result.isSuccess, isFalse);
      expect(result.error, 'fourth upload failed');
      expect(service.failedDeleteKeys, ['images/cleanup-c.jpg']);
      expect(service.deletedKeys, [
        'images/cleanup-c.jpg',
        'videos/cleanup-b.mp4_poster.jpg',
        'videos/cleanup-b.mp4',
        'images/cleanup-a.jpg',
      ]);
    },
  );

  test(
    'successful publication finalizes the ledger and stays inert after late cancel or dispose',
    () async {
      final service = _ScriptedS3Service([
        (_) async => _imageUpload(
          key: 'images/final-a.jpg',
          url: 'https://cdn.example.com/images/final-a.jpg',
        ),
        (_) async => _videoUpload(
          key: 'videos/final-b.mp4',
          url: 'https://cdn.example.com/videos/final-b.mp4',
          thumbnailUrl: 'https://cdn.example.com/videos/final-b.mp4_poster.jpg',
          thumbnailStorageKey: 'videos/final-b.mp4_poster.jpg',
          localPosterPath: r'C:\tmp\videos_final-b.mp4_poster.jpg',
        ),
      ]);
      final coordinator = CommerceMediaUploadCoordinator(s3Service: service);

      final result = await coordinator.uploadTypedMedia([
        CommerceLocalMediaItem.fromFile(
          _tempFile('final-a.jpg', [1, 2, 3, 4]),
          selectionOrder: 0,
        ),
        CommerceLocalMediaItem.fromFile(
          _tempFile('final-b.mp4', [5, 6, 7, 8]),
          selectionOrder: 1,
        ),
      ]);

      coordinator.cancelActiveUploads();
      coordinator.dispose();
      coordinator.cancelActiveUploads();
      coordinator.dispose();

      expect(result.isSuccess, isTrue);
      expect(result.data?.cancelled, isFalse);
      expect(service.deletedKeys, isEmpty);
      expect(service.failedDeleteKeys, isEmpty);
      expect(service.uploadCalls, hasLength(2));
    },
  );

  test(
    'cancellation before upload returns cancelled batch and no cleanup',
    () async {
      final service = _ScriptedS3Service([
        (_) async => _imageUpload(
          key: 'images/owner.jpg',
          url: 'https://cdn.example.com/images/owner.jpg',
        ),
      ]);
      final coordinator = CommerceMediaUploadCoordinator(s3Service: service);

      coordinator.dispose();
      final result = await coordinator.uploadTypedMedia([
        CommerceLocalMediaItem.fromFile(
          _tempFile('cancel-before.jpg', [1, 2, 3, 4]),
          selectionOrder: 0,
        ),
      ]);

      expect(result.isSuccess, isTrue);
      expect(result.data?.cancelled, isTrue);
      expect(service.deletedKeys, isEmpty);
      expect(service.uploadCalls, isEmpty);
    },
  );

  test(
    'stale completion after attempt cancellation is inert and cleans owner uploads',
    () async {
      final secondUpload = Completer<Result<CommerceMediaUploadResult>>();
      final service = _ScriptedS3Service([
        (_) async => _imageUpload(
          key: 'images/stale-owner.jpg',
          url: 'https://cdn.example.com/images/stale-owner.jpg',
        ),
        (_) => secondUpload.future,
      ]);
      final coordinator = CommerceMediaUploadCoordinator(s3Service: service);

      final future = coordinator.uploadTypedMedia([
        CommerceLocalMediaItem.fromFile(
          _tempFile('stale-a.jpg', [1, 2, 3, 4]),
          selectionOrder: 0,
        ),
        CommerceLocalMediaItem.fromFile(
          _tempFile('stale-b.jpg', [5, 6, 7, 8]),
          selectionOrder: 1,
        ),
      ]);

      await Future<void>.delayed(Duration.zero);
      coordinator.cancelActiveUploads();
      secondUpload.complete(
        _imageUpload(
          key: 'images/stale-new.jpg',
          url: 'https://cdn.example.com/images/stale-new.jpg',
        ),
      );

      final result = await future;

      expect(result.isSuccess, isTrue);
      expect(result.data?.cancelled, isTrue);
      expect(service.deletedKeys, ['images/stale-owner.jpg']);
    },
  );

  test(
    'dispose invalidates the active attempt and cleans uploaded objects',
    () async {
      final secondUpload = Completer<Result<CommerceMediaUploadResult>>();
      final service = _ScriptedS3Service([
        (_) async => _imageUpload(
          key: 'images/dispose-owner.jpg',
          url: 'https://cdn.example.com/images/dispose-owner.jpg',
        ),
        (_) => secondUpload.future,
      ]);
      final coordinator = CommerceMediaUploadCoordinator(s3Service: service);

      final future = coordinator.uploadTypedMedia([
        CommerceLocalMediaItem.fromFile(
          _tempFile('dispose-a.jpg', [1, 2, 3, 4]),
          selectionOrder: 0,
        ),
        CommerceLocalMediaItem.fromFile(
          _tempFile('dispose-b.jpg', [5, 6, 7, 8]),
          selectionOrder: 1,
        ),
      ]);

      await Future<void>.delayed(Duration.zero);
      coordinator.dispose();
      secondUpload.complete(
        _imageUpload(
          key: 'images/dispose-new.jpg',
          url: 'https://cdn.example.com/images/dispose-new.jpg',
        ),
      );

      final result = await future;

      expect(result.isSuccess, isTrue);
      expect(result.data?.cancelled, isTrue);
      expect(service.deletedKeys, ['images/dispose-owner.jpg']);
    },
  );
}
