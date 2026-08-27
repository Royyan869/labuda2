import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  group('Create content route contract', () {
    test('content module keeps only the universal composer route', () {
      final source = File(
        'lib/core/src/router/modules/content_module.dart',
      ).readAsStringSync();

      expect(source, contains('/create/content'));
      expect(source, isNot(contains('/create/request')));
      expect(source, isNot(contains('/create/post')));
      expect(source, contains('CreateContentScreen'));
    });

    test('router entry points only target the universal composer', () {
      final source = File(
        'lib/core/src/router/app_router.dart',
      ).readAsStringSync();

      expect(source, contains('navigateToCreateContent()'));
      expect(source, contains('RoutePaths.createContent'));
      expect(source, isNot(contains('navigateToCreateRequest()')));
      expect(source, isNot(contains('navigateToCreatePost()')));
      expect(source, isNot(contains('initialType')));
    });

    test('main screen create flow only passes the universal callback', () {
      final source = File(
        'lib/features/home/presentation/screens/main_screen.dart',
      ).readAsStringSync();

      expect(source, contains('onCreateContent:'));
      expect(source, isNot(contains('onCreateRequest')));
      expect(source, isNot(contains('initialType')));
    });

    test('bottom sheet exposes the universal composer entry', () {
      final source = File(
        'lib/shared/widgets/create_content_bottom_sheet.dart',
      ).readAsStringSync();

      expect(source, contains('Buat Konten'));
      expect(source, isNot(contains('onCreateRequest')));
      expect(source, isNot(contains('Minta Koi (Request)')));
    });
  });
}
