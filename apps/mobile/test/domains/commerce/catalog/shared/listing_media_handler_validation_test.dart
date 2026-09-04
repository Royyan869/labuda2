import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_media_handler.dart';

File _tempFile(String name, {int byteCount = 1}) {
  final dir = Directory.systemTemp.createTempSync('labuda_media_validation_');
  final file = File('${dir.path}${Platform.pathSeparator}$name');
  file.writeAsBytesSync(List<int>.filled(byteCount, 1));
  return file;
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
}
