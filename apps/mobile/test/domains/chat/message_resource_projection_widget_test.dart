import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_resource_projection.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_resource_projection_card.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';

void main() {
  testWidgets(
    'ChatResourceProjectionCard renders canonical profile projection',
    (tester) async {
      final projection = ChatLiveResourceProjection(
        state: ChatResourceProjectionState.live,
        resourceType: ChatResourceType.profile,
        viewerCapabilities: const ChatResourceViewerCapabilities.live(
          canInteract: false,
        ),
        resourceId: 'user-widget-1',
        canonicalUrl: '/user/user-widget-1',
        commerceActions: null,
        payload: const ChatResourceProfileLivePayload(
          username: 'alice',
          avatarUrl: null,
          storeName: 'Toko Alice',
          isSeller: true,
          lifecycle: 'active',
        ),
      );

      final router = GoRouter(
        initialLocation: '/',
        routes: [
          GoRoute(
            path: '/',
            builder: (context, state) => Scaffold(
              body: SingleChildScrollView(
                child: ChatResourceProjectionCard(
                  resourceProjection: projection,
                ),
              ),
            ),
          ),
          GoRoute(
            path: '/user/user-widget-1',
            builder: (context, state) =>
                const Scaffold(body: Text('profile destination')),
          ),
        ],
      );

      await tester.pumpWidget(MaterialApp.router(routerConfig: router));
      await tester.pumpAndSettle();

      expect(find.text('@alice'), findsOneWidget);
      expect(find.text('Toko Alice'), findsOneWidget);
      expect(find.text('Profil'), findsWidgets);
      expect(find.text('LIVE'), findsWidgets);

      await tester.tap(find.byType(CommerceMarketplaceCardShell));
      await tester.pumpAndSettle();

      expect(find.text('profile destination'), findsOneWidget);
    },
  );
}
