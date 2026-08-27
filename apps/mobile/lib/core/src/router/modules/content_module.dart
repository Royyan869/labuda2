import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/social/comment/comment.dart';
import 'package:labuda/domains/social/content/content.dart';

import 'base_module.dart';

/// ContentModule untuk universal Content creation and detail surfaces
///
/// Module ini mengelola routes untuk:
/// - Content detail view
/// - Discussion/Comment surface (full screen)
/// - Content creation (universal composer)
/// - Content management
class ContentModule extends BaseModule {
  @override
  String get moduleName => 'ContentModule';

  @override
  List<GoRoute> get routes => [
    // Content detail route (works for universal content rows)
    GoRoute(
      path: '/content/:contentId',
      name: 'content-detail',
      builder: (context, state) {
        final contentId = state.pathParameters['contentId'] ?? '';
        return ContentDetailScreen(contentId: contentId);
      },
    ),

    // Discussion/Comment route - full screen comment surface
    // This is the canonical V1 discussion surface for all content
    GoRoute(
      path: '/comment/content/:contentId',
      name: 'content-discussion',
      builder: (context, state) {
        final contentId = state.pathParameters['contentId'] ?? '';
        // Optional query parameters for context
        final contentTitle = state.uri.queryParameters['title'];
        return DiscussionScreen(
          contentId: contentId,
          contentTitle: contentTitle,
        );
      },
    ),

    // Create content route
    GoRoute(
      path: '/create/content',
      name: 'create-content',
      pageBuilder: (context, state) {
        return MaterialPage(
          key: state.pageKey,
          child: const CreateContentScreen(),
        );
      },
    ),
  ];

  @override
  Future<void> initialize() async {
    try {
      // TODO: Register repositories dan use cases ketika sudah ready
      // For now, just mark as initialized
    } catch (e) {
      rethrow;
    }
  }

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {
    // Cleanup resources if needed
  }
}
