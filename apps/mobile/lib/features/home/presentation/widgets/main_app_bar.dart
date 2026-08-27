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
/// NOTE: Uses navigationHandlerProvider directly (not MainScreenNavigationHandler)
/// because AppBar actions should NOT call Navigator.pop() - that's for drawer only.
class MainAppBar extends ConsumerWidget implements PreferredSizeWidget {
  final MainTab currentTab;

  const MainAppBar({super.key, required this.currentTab});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final navigationHandler = ref.read(navigationHandlerProvider);

    // Get current user ID for notification badge
    final authState = ref.watch(authControllerProvider);
    String userId = '';
    if (authState is AuthStateAuthenticated) {
      userId = authState.user.id;
    }

    return AppBar(
      backgroundColor: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
      foregroundColor: isDark
          ? AppColors.neutralGray400
          : AppColors.neutralGray900,
      elevation: 0,
      surfaceTintColor: Colors.transparent,
      scrolledUnderElevation: 0,
      leading: IconButton(
        icon: const Icon(Icons.menu),
        onPressed: () => Scaffold.of(context).openDrawer(),
        tooltip: 'Menu',
      ),
      title: _buildSearchBar(context, isDark, navigationHandler),
      actions: [
        // Saved Items button (saved items + watched auctions) with badge
        IconButton(
          onPressed: () => navigationHandler.navigateToSavedItems(),
          icon: SavedItemBadgeWidget(
            child: Icon(
              Icons.bookmark_border_outlined,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray900,
            ),
          ),
          tooltip:
              'Disimpan (Listing & Lelang)', // "Saved (Listings & Auctions)"
        ),
        // Messages with badge
        IconButton(
          onPressed: () => navigationHandler.navigateToChat(),
          icon: ChatBadgeWidget(
            child: Icon(
              Icons.chat_bubble_outline,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray900,
            ),
          ),
          tooltip: 'Messages',
        ),
        // Notifications with badge
        IconButton(
          onPressed: () => navigationHandler.navigateToNotifications(),
          icon: NotificationBadgeWidget(
            userId: userId,
            child: Icon(
              Icons.notifications_outlined,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray900,
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
    bool isDark,
    NavigationHandler navigationHandler,
  ) {
    return GestureDetector(
      onTap: () => _handleSearchTap(context, navigationHandler),
      child: Container(
        height: 40,
        padding: const EdgeInsets.symmetric(horizontal: 12),
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
          borderRadius: BorderRadius.circular(20),
        ),
        child: Row(
          children: [
            Icon(
              Icons.search,
              size: 20,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                'Search...',
                style: TextStyle(
                  fontSize: 14,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray500,
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
