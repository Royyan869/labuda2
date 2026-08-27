import 'dart:core';

import 'package:go_router/go_router.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/system/support/support.dart';

import 'base_module.dart';

/// Support Module
///
/// User-side support routes only.
/// Admin routes have been removed from mobile app.
///
/// REMOVED:
/// - /support/queue (admin-only)
/// - /support/tickets/:id (admin-only detail screen)
///
/// Users access support via:
/// - Settings -> "Tiket Saya" -> support list / thread entry points
/// - Pre-chat form for creating new tickets
class SupportModule extends BaseModule {
  @override
  List<GoRoute> get routes => [
    GoRoute(
      path: RoutePaths.supportTicketThread,
      name: RouteNames.supportTicketThread,
      builder: (context, state) {
        final ticketId = state.pathParameters['ticketId'] ?? '';
        return SupportTicketThreadScreen(ticketId: ticketId);
      },
    ),
  ];

  @override
  Future<void> initialize() async {
    // Support module initialization if needed
  }

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {
    // Cleanup support module resources
  }

  @override
  String get moduleName => 'SupportModule';
}
