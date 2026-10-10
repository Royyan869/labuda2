import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/social/content/domain/entities/content.dart';
import 'package:hishumi/shared/widgets/app_image.dart';

Widget _wrap(Widget child) {
  return MaterialApp(
    theme: ThemeData.light(),
    home: Scaffold(body: child),
  );
}

MediaEntity _media({
  required String id,
  required String url,
  required int position,
}) {
  return MediaEntity(
    id: id,
    originalUrl: url,
    type: MediaType.image,
    position: position,
    createdAt: DateTime.utc(2026, 1, 1),
  );
}

void main() {
  test('backend CloudFront URL is used as-is', () {
    const url =
        'https://d358tu61i1wrtt.cloudfront.net/images/image.jpg';

    final media = _media(id: 'm1', url: url, position: 0);
    expect(media.originalUrl, url);
  });

  testWidgets('canonical image renders the backend URL and replaces the '
      'error on refresh', (
    tester,
  ) async {
    const firstUrl =
        'https://d358tu61i1wrtt.cloudfront.net/images/first.jpg';
    const secondUrl =
        'https://d358tu61i1wrtt.cloudfront.net/images/second.jpg';

    var imageUrl = firstUrl;

    await tester.pumpWidget(
      _wrap(
        StatefulBuilder(
          builder: (context, setState) {
            return Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                SizedBox(
                  width: 96,
                  height: 96,
                  child: AppImage(
                    imageUrl: imageUrl,
                    errorWidget: const Text('error-state'),
                  ),
                ),
                TextButton(
                  onPressed: () => setState(() {
                    imageUrl = secondUrl;
                  }),
                  child: const Text('Refresh'),
                ),
              ],
            );
          },
        ),
      ),
    );

    await tester.pump();
    expect(find.byType(AppImage), findsOneWidget);
    expect(
      tester.widget<AppImage>(find.byType(AppImage)).imageUrl,
      firstUrl,
    );

    await tester.tap(find.text('Refresh'));
    await tester.pump();

    expect(find.text('error-state'), findsNothing);
    expect(
      tester.widget<AppImage>(find.byType(AppImage)).imageUrl,
      secondUrl,
      reason: 'the refreshed backend URL must reach the canonical widget',
    );
  });
}
