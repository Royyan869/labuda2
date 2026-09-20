import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'for sale detail screen uses MediaCarouselWidget for gallery display',
    () {
      final source = File(
        'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart',
      ).readAsStringSync();

      expect(source, contains('MediaCarouselWidget('));
      expect(source, contains('media: forSale.media'));
      expect(source, isNot(contains('VideoPlayerController')));
      expect(source, isNot(contains('Chewie')));
    },
  );
}
