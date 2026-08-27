import 'dart:collection';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/src/utils/constants/app_constants.dart';
import 'package:labuda/domains/commerce/catalog/shared/shared.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';
import '../../../../support/queued_image_http_client.dart';

Widget _wrap(Widget child) {
  return MaterialApp(
    theme: ThemeData.light(),
    home: Scaffold(body: child),
  );
}

String _networkUrl(ImageProvider<Object> provider) {
  final resolved = provider is ResizeImage ? provider.imageProvider : provider;
  return (resolved as NetworkImage).url;
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
  test('absolute presigned URL is preserved for network loading', () {
    const url =
        'https://bucket.s3.region.amazonaws.com/path/image.jpg?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=one';

    expect(resolveNetworkImageUrl(url), url);
  });

  test('storage key resolves to the configured CDN base URL', () {
    const storageKey = 'products/user-id/image.jpg';
    final resolved = resolveNetworkImageUrl(storageKey)!;
    expect(resolved, isNot(startsWith(AppConstants.baseUrl)));
    expect(
      resolved,
      startsWith(
        AppConstants.useCloudFront
            ? AppConstants.cdnBaseUrl
            : AppConstants.awsS3BaseUrl,
      ),
    );
    expect(resolved, contains('/products/user-id/image.jpg'));
  });

  testWidgets('stable image failure then refreshed URL replaces the error', (
    tester,
  ) async {
    const firstUrl =
        'https://cdn.example.com/gallery/first.jpg?X-Amz-Signature=one';
    const secondUrl =
        'https://cdn.example.com/gallery/first.jpg?X-Amz-Signature=two';
    final responders = <String, Queue<QueuedImageResponseSpec>>{
      firstUrl: Queue<QueuedImageResponseSpec>.of([
        QueuedImageResponseSpec.failure(),
      ]),
      secondUrl: Queue<QueuedImageResponseSpec>.of([
        QueuedImageResponseSpec.success(onePxPngBytes),
      ]),
    };

    var imageUrl = firstUrl;

    await HttpOverrides.runZoned(() async {
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
                    child: StableNetworkImage(
                      imageUrl: imageUrl,
                      logicalCacheKey: 'detail-media-1',
                      fallback: const Text('error-state'),
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

      await tester.pumpAndSettle();
      await tester.pump(const Duration(seconds: 1));
      await tester.pumpAndSettle();

      expect(find.text('error-state'), findsOneWidget);
      final firstImages = tester.widgetList<Image>(find.byType(Image)).toList();
      expect(firstImages, isNotEmpty);
      expect(_networkUrl(firstImages.first.image), firstUrl);
      expect(responders[firstUrl]!.isEmpty, isTrue);

      await tester.tap(find.text('Refresh'));
      await tester.pump();
      await tester.pumpAndSettle();
      await tester.pump(const Duration(seconds: 1));
      await tester.pumpAndSettle();

      expect(find.text('error-state'), findsNothing);
      final refreshedImages = tester.widgetList<Image>(find.byType(Image)).toList();
      expect(refreshedImages, isNotEmpty);
      expect(responders[secondUrl]!.isEmpty, isTrue);

      expect(
        refreshedImages.map((image) => _networkUrl(image.image)),
        contains(secondUrl),
      );
    }, createHttpClient: (_) => QueuedImageHttpClient(responders));
  });

  testWidgets('empty gallery and failed media do not share the same branch', (
    tester,
  ) async {
    final responders = <String, Queue<QueuedImageResponseSpec>>{
      'https://cdn.example.com/gallery/failed.jpg': Queue<
        QueuedImageResponseSpec
      >.of([QueuedImageResponseSpec.failure()]),
    };

    await HttpOverrides.runZoned(() async {
      await tester.pumpWidget(
        _wrap(
          SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                CommerceDetailMediaGallery(
                  cacheKey: 'empty-gallery',
                  media: const [],
                  logicalCacheKeyBuilder: (media, index) => 'unused',
                ),
                const SizedBox(height: 12),
                CommerceDetailMediaGallery(
                  cacheKey: 'failed-gallery',
                  media: [
                    _media(
                      id: 'failed',
                      url: 'https://cdn.example.com/gallery/failed.jpg',
                      position: 0,
                    ),
                  ],
                  logicalCacheKeyBuilder: (media, index) => media.id,
                  fallback: const Text('failed-gallery'),
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.image_outlined), findsOneWidget);
      expect(find.text('failed-gallery'), findsOneWidget);
      expect(find.byType(StableNetworkImage), findsOneWidget);
    }, createHttpClient: (_) => QueuedImageHttpClient(responders));
  });
}
