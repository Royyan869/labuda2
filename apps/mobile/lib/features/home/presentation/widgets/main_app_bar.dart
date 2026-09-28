import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/home/presentation/models/main_tab.dart';
import 'package:labuda/domains/system/notification/notification.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_badge_widget.dart';
import 'package:labuda/domains/user/preference/saved_item/saved_item.dart';

/// Main app bar component untuk main screen
///
/// Compact design dengan search bar, hamburger menu, dan action buttons.
/// Shows badges for Chat and Notifications.
///
/// GUEST POLICY (Owner canonical): guest Home keeps public surfaces
/// (Home feed, search) usable. Auth-required actions (Saved, Chat,
/// Notifications) are NOT auto-triggered for guests — badge providers are
/// skipped (no unauthorized fetch / 401 noise) and a tap routes the guest
/// to the explicit Sign In flow instead of bouncing through a private
/// route redirect.
///
/// NOTE: Uses navigationHandlerProvider directly (not MainScreenNavigationHandler)
/// because AppBar actions should NOT call Navigator.pop() - that's for drawer only.
class MainAppBar extends ConsumerWidget implements PreferredSizeWidget {
  final MainTab currentTab;

  const MainAppBar({super.key, required this.currentTab});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scheme = Theme.of(context).colorScheme;
    final navigationHandler = ref.read(navigationHandlerProvider);

    // Get current user ID for notification badge
    final authState = ref.watch(authControllerProvider);
    String userId = '';
    if (authState is AuthStateAuthenticated) {
      userId = authState.user.id;
    }
    final isGuest = userId.isEmpty;

    // Auth-required affordances: guest is sent to the explicit Sign In
    // flow (never to a private route that bounces back to /welcome).
    void guardedAction(VoidCallback authenticatedAction) {
      if (isGuest) {
        navigationHandler.navigateToSignIn();
        return;
      }
      authenticatedAction();
    }

    return AppBar(
      backgroundColor: scheme.surface,
      foregroundColor: scheme.onSurface,
      elevation: AppElevation.none,
      surfaceTintColor: Colors.transparent,
      scrolledUnderElevation: 0,
      leading: IconButton(
        icon: const Icon(Icons.menu),
        onPressed: () => Scaffold.of(context).openDrawer(),
        tooltip: 'Menu',
      ),
      title: _buildSearchBar(context, navigationHandler),
      actions: [
        // Saved Items button (saved For Sale items + watched auctions) with
        // badge. Guest sees the icon without badge fetch and gets the Sign
        // In gate on tap.
        IconButton(
          onPressed: () => guardedAction(navigationHandler.navigateToSavedItems),
          icon: isGuest
              ? Icon(
                  Icons.bookmark_border_outlined,
                  color: scheme.onSurface,
                )
              : SavedItemBadgeWidget(
                  child: Icon(
                    Icons.bookmark_border_outlined,
                    color: scheme.onSurface,
                  ),
                ),
          tooltip:
              'Disimpan (For Sale & Lelang)', // "Saved (For Sale & Auctions)"
        ),
        // Messages with badge
        IconButton(
          onPressed: () => guardedAction(navigationHandler.navigateToChat),
          icon: isGuest
              ? Icon(
                  Icons.chat_bubble_outline,
                  color: scheme.onSurface,
                )
              : ChatBadgeWidget(
                  child: Icon(
                    Icons.chat_bubble_outline,
                    color: scheme.onSurface,
                  ),
                ),
          tooltip: 'Messages',
        ),
        // Notifications with badge
        IconButton(
          onPressed: () =>
              guardedAction(navigationHandler.navigateToNotifications),
          icon: isGuest
              ? Icon(
                  Icons.notifications_outlined,
                  color: scheme.onSurface,
                )
              : NotificationBadgeWidget(
                  userId: userId,
                  child: Icon(
                    Icons.notifications_outlined,
                    color: scheme.onSurface,
                  ),
                ),
          tooltip: 'Notifications',
        ),
        const SizedBox(width: 8),
      ],
    );
  }

  Widget _buildSearchBar(
    BuildContext context,
    NavigationHandler navigationHandler,
  ) {
    final scheme = Theme.of(context).colorScheme;
    return GestureDetector(
      onTap: () => _handleSearchTap(context, navigationHandler),
      child: Container(
        height: 40,
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12),
        decoration: BoxDecoration(
          color: scheme.surfaceContainerHigh,
          borderRadius: BorderRadius.circular(AppShape.r20),
        ),
        child: Row(
          children: [
            Icon(
              Icons.search,
              size: 20,
              color: scheme.onSurfaceVariant,
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                'Search...',
                style: TextStyle(
                  fontSize: AppType.s14,
                  color: scheme.onSurfaceVariant,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _handleSearchTap(
    BuildContext context,
    NavigationHandler navigationHandler,
  ) {
    // Navigate to search screen
    navigationHandler.navigateToSearch();
  }

  @override
  Size get preferredSize => const Size.fromHeight(kToolbarHeight);
}
