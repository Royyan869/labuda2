import 'dart:core';

import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/system/support/presentation/screens/help_center_screen.dart'
    show
        HelpArticle,
        HelpArticleScreen,
        HelpCategory,
        HelpCategoryScreen,
        HelpCenterScreen;
import 'package:labuda/domains/system/support/presentation/screens/support_ticket_thread_screen.dart'
    show SupportTicketThreadScreen;
import 'package:labuda/domains/system/support/presentation/screens/support_tickets_list_screen.dart'
    show SupportTicketsListScreen;
import 'package:labuda/domains/user/identity/authentication/domain/entities/auth_user.dart'
    show AuthUser;
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_controller.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_state.dart';

import 'base_module.dart';

/// Support Module
///
/// User-side support routes only.
/// Admin routes have been removed from mobile app.
///
/// REMOVED:
/// - /support/queue (admin-only)
///
/// Users access support via:
/// - Help center (`/help`) → category (`/help/category/:category`) → article
/// - Settings → "Tiket Saya" → support ticket list (`/support/tickets`)
/// - Ticket thread (`/support/tickets/:ticketId`)
class SupportModule extends BaseModule {
  @override
  List<GoRoute> get routes => [
    GoRoute(
      path: RoutePaths.supportTickets,
      name: RouteNames.supportTickets,
      builder: (context, state) => const SupportTicketsListScreen(),
    ),
    GoRoute(
      path: RoutePaths.supportTicketThread,
      name: RouteNames.supportTicketThread,
      builder: (context, state) {
        final ticketId = state.pathParameters['ticketId'] ?? '';
        return SupportTicketThreadScreen(ticketId: ticketId);
      },
    ),

    // ── Help center ────────────────────────────────────────────────────────
    // The session identity of the signed-in reader is resolved by the route
    // (same pattern as the profile module routes) so that no caller has to
    // hand the screen its own user data.
    GoRoute(
      path: RoutePaths.helpCenter,
      name: RouteNames.helpCenter,
      builder: (context, state) {
        final user = _readerOf(context);
        return HelpCenterScreen(
          userId: user?.id,
          userName: user == null ? null : '@${user.username}',
          userAvatar: user?.avatarUrl,
        );
      },
    ),
    GoRoute(
      path: RoutePaths.helpCategory,
      name: RouteNames.helpCategory,
      builder: (context, state) {
        final raw = state.pathParameters['category'];
        final user = _readerOf(context);
        final category = HelpCategory.values
            .where((value) => value.name == raw)
            .firstOrNull;
        if (category == null) {
          // Unknown category: land on the canonical help entry instead of
          // inventing a new error surface.
          return HelpCenterScreen(
            userId: user?.id,
            userName: user == null ? null : '@${user.username}',
            userAvatar: user?.avatarUrl,
          );
        }
        return HelpCategoryScreen(
          category: category,
          userId: user?.id,
          userName: user == null ? null : '@${user.username}',
          userAvatar: user?.avatarUrl,
        );
      },
    ),
    GoRoute(
      path: RoutePaths.helpArticle,
      name: RouteNames.helpArticle,
      builder: (context, state) {
        final article = state.extra;
        final user = _readerOf(context);
        if (article is! HelpArticle) {
          // The article surface carries localized content with no stable
          // content id yet, so it is only reachable with the article as
          // route extra. Without it, land on the canonical help entry.
          return HelpCenterScreen(
            userId: user?.id,
            userName: user == null ? null : '@${user.username}',
            userAvatar: user?.avatarUrl,
          );
        }
        return HelpArticleScreen(
          article: article,
          userId: user?.id,
          userName: user == null ? null : '@${user.username}',
          userAvatar: user?.avatarUrl,
        );
      },
    ),
  ];

  /// Session identity for the help surfaces. Reads the canonical auth state
  /// once per navigation — the same source the former callers used.
  AuthUser? _readerOf(BuildContext context) {
    final container = ProviderScope.containerOf(context, listen: false);
    final authState = container.read(authControllerProvider);
    return authState is AuthStateAuthenticated ? authState.user : null;
  }

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
