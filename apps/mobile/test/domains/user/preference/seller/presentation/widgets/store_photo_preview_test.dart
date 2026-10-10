import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/widgets/wizard/store_photo_preview.dart';
import 'package:hishumi/shared/widgets/app_image.dart';

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

  test('crop failure paths must never pop the caller screen', () {
    // Crop authority lives in AvatarImageProcessor (AvatarEditorWidget only
    // opens the pick sheet and delegates). Scan every catch block THERE.
    final source = File(
      'lib/shared/widgets/avatar_image_processor.dart',
    ).readAsStringSync();

    final catchBlocks = <String>[];
    for (final match in RegExp(r'catch\s*\([^)]*\)\s*\{').allMatches(source)) {
      var depth = 1;
      var cursor = match.end;
      while (cursor < source.length && depth > 0) {
        final char = source[cursor];
        if (char == '{') depth++;
        if (char == '}') depth--;
        cursor++;
      }
      catchBlocks.add(source.substring(match.end, cursor));
    }

    expect(
      catchBlocks.length,
      greaterThanOrEqualTo(2),
      reason:
          'anti-vacuum: crop + temp-file saves each own a failure path; a run '
          'that finds none is scanning the wrong file (the marker moved once '
          'already — that is what broke this test)',
    );
    for (final block in catchBlocks) {
      expect(
        block.contains('.pop('),
        isFalse,
        reason:
            'the crop route pops itself on success; an extra pop in catch '
            'closes the caller route (P1: wizard died after crop)',
      );
    }
  });
}
