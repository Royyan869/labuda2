import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/media_preview.dart';

void main() {
  test('stable local media id is deterministic for the same file path', () {
    final first = buildStableLocalMediaId(File(r'C:\tmp\alpha.jpg'));
    final second = buildStableLocalMediaId(File(r'C:\tmp\alpha.jpg'));
    final third = buildStableLocalMediaId(File(r'C:\tmp\bravo.jpg'));

    expect(first, second);
    expect(first, isNot(equals(third)));
    expect(first, isNotEmpty);
  });

  testWidgets('MediaPreview forwards the tapped video index to removal', (
    tester,
  ) async {
    final removed = <int>[];

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: MediaPreview(
            selectedImages: [File(r'C:\tmp\alpha.jpg')],
            selectedVideos: [
              File(r'C:\tmp\video-a.mp4'),
              File(r'C:\tmp\video-b.mp4'),
            ],
            onVideoRemove: removed.add,
            imageHeight: 120,
            imageWidth: 120,
          ),
        ),
      ),
    );

    await tester.pump(const Duration(milliseconds: 250));

    final removeButtons = find.byIcon(Icons.close);
    expect(removeButtons, findsNWidgets(2));

    await tester.tap(removeButtons.at(1));
    await tester.pump();

    expect(removed, [1]);
  });

  test('stable media upload path does not use runtime timestamps or UniqueKey', () {
    final s3Source = File(
      'lib/core/services/s3_service.dart',
    ).readAsStringSync();
    final submissionSource = File(
      'lib/domains/social/content/presentation/widgets/create_content/content_submission_handler.dart',
    ).readAsStringSync();

    for (final source in [s3Source, submissionSource]) {
      expect(source, isNot(contains('DateTime.now()')));
      expect(source, isNot(contains('UniqueKey(')));
    }
  });
}
