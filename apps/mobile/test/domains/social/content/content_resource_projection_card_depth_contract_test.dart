import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_resource_projection_card.dart';

Map<String, dynamic> _contentProjectionJson({
  String resourceId = 'content-b',
  String nestedResourceType = 'profile',
  String nestedResourceId = 'profile-c',
}) {
  return <String, dynamic>{
    'state': 'LIVE',
    'resource_type': 'content',
    'resource_id': resourceId,
    'content': <String, dynamic>{
      'caption': 'Content B',
      'media': <Map<String, dynamic>>[],
      'lifecycle': 'active',
      'created_at': '2026-07-23T00:00:00.000Z',
      'author': <String, dynamic>{
        'id': 'author-1',
        'username': 'alice',
        'avatar_url': 'https://example.com/alice.png',
        'lifecycle': 'active',
      },
      'nested_resource': <String, dynamic>{
        'resource_type': nestedResourceType,
        'resource_id': nestedResourceId,
      },
    },
  };
}

Widget _wrap(
  ContentResourceProjection projection, {
  required ValueChanged<String> onRouteBuilt,
}) {
  return MaterialApp(
    home: Scaffold(
      body: SingleChildScrollView(
        child: Center(
          child: ContentResourceProjectionCard(
            resourceProjection: projection,
            onTap: () => onRouteBuilt(projection.canonicalPath),
          ),
        ),
      ),
    ),
  );
}

void main() {
  testWidgets(
    'nested_resource renders one nested badge and routes only the primary content path',
    (tester) async {
      final projection = ContentResourceProjection.fromJson(
        _contentProjectionJson(),
      );
      String? navigatedPath;

      await tester.pumpWidget(
        _wrap(projection, onRouteBuilt: (path) => navigatedPath = path),
      );
      await tester.pumpAndSettle();

      expect(projection.content!.nestedResource, isNotNull);
      expect(
        projection.content!.nestedResource!.resourceType,
        ContentResourceProjectionType.profile,
      );
      expect(projection.content!.nestedResource!.resourceId, 'profile-c');

      expect(find.byType(ContentResourceProjectionCard), findsOneWidget);

      final badges = tester
          .widgetList<CommerceMarketplaceCardBadge>(
            find.byType(CommerceMarketplaceCardBadge),
          )
          .toList();
      expect(
        badges.where((badge) => badge.label == projection.nestedResourceLabel),
        hasLength(1),
      );
      expect(
        badges.where((badge) => badge.label == 'LIVE'),
        hasLength(1),
      );

      await tester.tap(find.byType(ContentResourceProjectionCard));
      await tester.pumpAndSettle();

      expect(navigatedPath, projection.canonicalPath);
      expect(find.byType(ContentResourceProjectionCard), findsOneWidget);
    },
  );
}
