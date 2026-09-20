import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_media_handler.dart';

File _tempFile(String name, {int byteCount = 1}) {
  final dir = Directory.systemTemp.createTempSync('labuda_media_validation_');
  final file = File('${dir.path}${Platform.pathSeparator}$name');
  file.writeAsBytesSync(List<int>.filled(byteCount, 1));
  return file;
}

/// Canonical test seam mirrored from
/// `test/domains/social/content/content_media_failure_semantics_test.dart`:
/// a fake [S3Service] injected through [s3ServiceProvider] so the production
/// handler path is exercised without any network or platform picker.
class _FakeS3Service extends S3Service {
  _FakeS3Service({this.baseUrl = 'https://fake-s3.example.com'});

  final String baseUrl;

  final List<String> imageUploads = [];
  final List<String> videoUploads = [];

  /// Number of upcoming image uploads that must fail.
  int failingImageUploads = 0;

  @override
  Future<Result<String>> uploadImage(File imageFile) async {
    imageUploads.add(imageFile.path);
    if (failingImageUploads > 0) {
      failingImageUploads--;
      return Result.error('fake image upload failed');
    }
    return Result.success('$baseUrl/images/${_baseName(imageFile.path)}');
  }

  @override
  Future<Result<String>> uploadVideo(File videoFile) async {
    videoUploads.add(videoFile.path);
    return Result.success('$baseUrl/videos/${_baseName(videoFile.path)}');
  }
}

String _baseName(String path) => path.split(Platform.pathSeparator).last;

/// Pumps the canonical widget tree — [ProviderScope] (with the S3 override)
/// → [MaterialApp] → [Scaffold] — and returns a mounted [BuildContext] that is
/// downstream of the scope, matching the real call sites that pass a screen
/// context into [ForSaleMediaHandler.uploadMedia].
Future<BuildContext> _pumpScopedContext(
  WidgetTester tester,
  _FakeS3Service s3,
) async {
  late BuildContext scopedContext;
  await tester.pumpWidget(
    ProviderScope(
      overrides: [s3ServiceProvider.overrideWithValue(s3)],
      child: MaterialApp(
        home: Scaffold(
          body: Builder(
            builder: (context) {
              scopedContext = context;
              return const SizedBox.shrink();
            },
          ),
        ),
      ),
    ),
  );
  return scopedContext;
}

/// Drains the snackbar that `uploadMedia` shows so the test does not end with
/// a pending auto-dismiss timer.
Future<void> _drainSnackBar(WidgetTester tester) async {
  await tester.pumpAndSettle();
  await tester.pump(const Duration(seconds: 10));
  await tester.pumpAndSettle();
}

void _deleteTempFile(File file) {
  final dir = file.parent;
  if (dir.existsSync()) dir.deleteSync(recursive: true);
}

void main() {
  group('ForSaleMediaHandler validation', () {
    test('exposes canonical media limits', () {
      expect(ForSaleMediaHandler.maxMedia, 10);
      expect(ForSaleMediaHandler.maxImageSizeMb, 10);
      final handler = ForSaleMediaHandler();
      expect(handler, isA<ForSaleMediaHandler>());
    });

    test('handler can be instantiated and has expected gallery/camera entry points', () {
      final handler = ForSaleMediaHandler();
      expect(handler.pickMediaFromGallery, isA<Function>());
      expect(handler.openCamera, isA<Function>());
      expect(ForSaleMediaHandler.showMediaPicker, isA<Function>());
    });

    test('temp file helper creates files within limits', () {
      final file = _tempFile('picked.jpg');
      addTearDown(() async {
        if (await file.parent.exists()) {
          await file.parent.delete(recursive: true);
        }
      });
      expect(file.existsSync(), isTrue);
      expect(file.lengthSync(), greaterThan(0));
    });
  });

  group('ForSaleMediaHandler runtime S3 dependency', () {
    testWidgets('handler context resolves s3ServiceProvider to the override', (
      tester,
    ) async {
      final s3 = _FakeS3Service();
      final context = await _pumpScopedContext(tester, s3);

      final resolved = ProviderScope.containerOf(
        context,
        listen: false,
      ).read(s3ServiceProvider);

      expect(
        identical(resolved, s3),
        isTrue,
        reason: 'the context handed to the handler must see the override',
      );
    });

    testWidgets('uploadMedia routes image + video through the injected fake S3', (
      tester,
    ) async {
      final photo = _tempFile('photo.jpg');
      final video = _tempFile('clip.mp4');
      addTearDown(() {
        _deleteTempFile(photo);
        _deleteTempFile(video);
      });

      final s3 = _FakeS3Service();
      final context = await _pumpScopedContext(tester, s3);

      final urls = await ForSaleMediaHandler().uploadMedia(
        context: context,
        files: [photo, video],
      );
      await _drainSnackBar(tester);

      expect(s3.imageUploads, [photo.path]);
      expect(s3.videoUploads, [video.path]);
      expect(urls, [
        'https://fake-s3.example.com/images/photo.jpg',
        'https://fake-s3.example.com/videos/clip.mp4',
      ]);
    });

    testWidgets('a second override is honored — S3 is read per call', (
      tester,
    ) async {
      final photo = _tempFile('photo.jpg');
      addTearDown(() => _deleteTempFile(photo));

      final first = _FakeS3Service(baseUrl: 'https://first-fake.example.com');
      final firstContext = await _pumpScopedContext(tester, first);
      final firstUrls = await ForSaleMediaHandler().uploadMedia(
        context: firstContext,
        files: [photo],
      );
      await _drainSnackBar(tester);

      final second = _FakeS3Service(baseUrl: 'https://second-fake.example.com');
      final secondContext = await _pumpScopedContext(tester, second);
      final secondUrls = await ForSaleMediaHandler().uploadMedia(
        context: secondContext,
        files: [photo],
      );
      await _drainSnackBar(tester);

      expect(firstUrls, ['https://first-fake.example.com/images/photo.jpg']);
      expect(secondUrls, ['https://second-fake.example.com/images/photo.jpg']);
      expect(first.imageUploads, hasLength(1));
      expect(second.imageUploads, hasLength(1));
      expect(first.videoUploads, isEmpty);
    });

    testWidgets('failed fake upload yields no URL and reports the failure', (
      tester,
    ) async {
      final photo = _tempFile('photo.jpg');
      addTearDown(() => _deleteTempFile(photo));

      final s3 = _FakeS3Service()..failingImageUploads = 1;
      final context = await _pumpScopedContext(tester, s3);

      final urls = await ForSaleMediaHandler().uploadMedia(
        context: context,
        files: [photo],
      );
      await tester.pump();

      expect(s3.imageUploads, [photo.path]);
      expect(urls, isEmpty);
      expect(find.text('0 media berhasil diupload, 1 gagal'), findsOneWidget);
      await _drainSnackBar(tester);
    });
  });
}
