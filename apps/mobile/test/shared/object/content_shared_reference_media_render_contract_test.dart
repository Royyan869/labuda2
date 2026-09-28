/// CONTENT SHARED-REFERENCE MEDIA RENDER CONTRACT
///
/// Pins the canonical media path for a Content share reference rendered by
/// `ObjectPreviewCard`:
///   backend preview image URL
///     → `ShareTargetType.content`
///     → `AppImage` (URL as-is).
///
/// NEGATIVE PROOF: mobile builds no readable URL — every reference type
/// renders the cached snapshot URL unchanged through the single widget.
///
/// The shell has no live branch any more (see
/// `reference_attachment_live_fetch_purge_test.dart`): every case below is the
/// snapshot path.
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/object/object_preview.dart';
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';
import 'package:labuda/shared/widgets/app_image.dart';

ShareReference _contentReference({String? imageUrl}) {
  return ShareReference(
    targetType: ShareTargetType.content,
    targetId: 'content-1',
    preview: ObjectPreview(
      id: 'content-1',
      type: 'content',
      title: 'Konten utama',
      imageUrl: imageUrl,
      status: 'available',
    ),
  );
}

Widget _card(ShareReference reference) {
  return MaterialApp(
    home: Scaffold(
      body: SingleChildScrollView(child: ObjectPreviewCard(reference: reference)),
    ),
  );
}

void main() {
  testWidgets(
    'content reference renders the backend URL as-is',
    (tester) async {
      const backendUrl =
          'https://d358tu61i1wrtt.cloudfront.net/images/1749600000003_shared.jpg';

      await tester.pumpWidget(
        _card(_contentReference(imageUrl: backendUrl)),
      );
      await tester.pump();

      expect(find.byType(AppImage), findsOneWidget);
      final appImage = tester.widget<AppImage>(find.byType(AppImage));
      expect(
        appImage.imageUrl,
        backendUrl,
        reason: 'the backend URL must reach the canonical widget unchanged',
      );
    },
  );

  testWidgets('content reference without a preview image renders no image', (
    tester,
  ) async {
    await tester.pumpWidget(_card(_contentReference()));
    await tester.pump();

    expect(find.byType(AppImage), findsNothing);
  });

  testWidgets(
    'commerce reference renders through the same canonical widget',
    (tester) async {
      final reference = ShareReference.forSale(
        forSaleId: 'sale-1',
        title: 'Koi Premium',
        imageUrl:
            'https://d358tu61i1wrtt.cloudfront.net/images/sale.jpg',
      );

      await tester.pumpWidget(_card(reference));
      await tester.pump();

      expect(find.byType(AppImage), findsOneWidget);
      final appImage = tester.widget<AppImage>(find.byType(AppImage));
      expect(
        appImage.imageUrl,
        'https://d358tu61i1wrtt.cloudfront.net/images/sale.jpg',
      );
    },
  );
}
