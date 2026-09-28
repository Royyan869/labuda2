import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/store_photo_preview.dart';
import 'package:labuda/shared/widgets/app_image.dart';

Widget _wrap(Widget child) {
  return MaterialApp(home: Scaffold(body: Center(child: child)));
}

void main() {
  testWidgets('empty renders the store fallback icon', (tester) async {
    await tester.pumpWidget(_wrap(const StorePhotoPreview(size: 120)));
    await tester.pump();

    expect(find.byIcon(Icons.store_outlined), findsOneWidget);
    expect(find.byType(AppImage), findsNothing);
  });

  testWidgets('local selection renders the local file', (tester) async {
    await tester.pumpWidget(
      _wrap(
        const StorePhotoPreview(
          localPath: '/tmp/store.jpg',
          size: 120,
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(Image), findsOneWidget);
    expect(find.byType(AppImage), findsNothing);
  });

  testWidgets('uploaded server truth wins over the stale local file',
      (tester) async {
    const displayUrl =
        'https://d358tu61i1wrtt.cloudfront.net/images/stores/user.jpg';

    await tester.pumpWidget(
      _wrap(
        const StorePhotoPreview(
          localPath: '/tmp/store.jpg',
          displayUrl: displayUrl,
          size: 120,
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(AppImage), findsOneWidget);
    expect(
      tester.widget<AppImage>(find.byType(AppImage)).imageUrl,
      displayUrl,
    );
  });

  testWidgets('uploading shows a progress veil over the local file',
      (tester) async {
    await tester.pumpWidget(
      _wrap(
        const StorePhotoPreview(
          localPath: '/tmp/store.jpg',
          isUploading: true,
          size: 120,
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsOneWidget);
  });

  test('crop dialog catch must never pop the caller screen', () {
    final source = File(
      'lib/shared/widgets/avatar_editor_widget.dart',
    ).readAsStringSync();
    final catchBlock = source.substring(source.indexOf('} catch (e) {'));
    expect(
      catchBlock.contains('.pop('),
      isFalse,
      reason:
          'the pick dialog is already popped before picking; an extra pop in '
          'catch closes the caller route (P1: wizard died after crop)',
    );
  });
}
