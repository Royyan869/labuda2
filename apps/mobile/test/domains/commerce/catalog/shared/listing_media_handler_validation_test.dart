import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/listing/presentation/widgets/listing_media_handler.dart';

File _tempFile(String name, {int byteCount = 1}) {
  final dir = Directory.systemTemp.createTempSync('labuda_media_validation_');
  final file = File('${dir.path}${Platform.pathSeparator}$name');
  file.writeAsBytesSync(List<int>.filled(byteCount, 1));
  return file;
}

void main() {
  group('ListingMediaHandler validation', () {
    test('rejects oversized images with photo-specific copy', () {
      final handler = ListingMediaHandler();
      final file = _tempFile('picked.jpg');
      addTearDown(() async {
        if (await file.parent.exists()) {
          await file.parent.delete(recursive: true);
        }
      });

      final message = handler.mediaSizeValidationMessage(
        file,
        imageSizeLimitMbOverride: 0,
      );

      expect(message, 'Ukuran foto maksimal 0MB');
      expect(ListingMediaHandler.isVideoFile(file), isFalse);
    });

    test('rejects oversized videos with video-specific copy', () {
      final handler = ListingMediaHandler();
      final file = _tempFile('picked.mp4');
      addTearDown(() async {
        if (await file.parent.exists()) {
          await file.parent.delete(recursive: true);
        }
      });

      final message = handler.mediaSizeValidationMessage(
        file,
        videoSizeLimitMbOverride: 0,
      );

      expect(message, 'Ukuran video maksimal 0MB');
      expect(ListingMediaHandler.isVideoFile(file), isTrue);
    });

    test('allows files within the configured limits', () {
      final handler = ListingMediaHandler();
      final file = _tempFile('picked.jpg');
      addTearDown(() async {
        if (await file.parent.exists()) {
          await file.parent.delete(recursive: true);
        }
      });

      final message = handler.mediaSizeValidationMessage(
        file,
        imageSizeLimitMbOverride: 1,
      );

      expect(message, isNull);
    });
  });
}
