import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/notification.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_badge_widget.dart';
import 'package:labuda/domains/user/preference/saved_item/saved_item.dart';

class MainAppBar extends ConsumerWidget implements PreferredSizeWidget {
  const MainAppBar({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scheme = Theme.of(context).colorScheme;
    final navigation = ref.read(navigationHandlerProvider);
    final authState = ref.watch(authControllerProvider);
    final userId = authState is AuthStateAuthenticated ? authState.user.id : '';
    final isGuest = userId.isEmpty;

    void guarded(VoidCallback action) {
      if (isGuest) {
        navigation.navigateToSignIn();
      } else {
        action();
      }
    }

    Widget action({
      required VoidCallback onPressed,
      required Widget icon,
      required String tooltip,
    }) => SizedBox.square(
      dimension: kToolbarHeight,
      child: IconButton(onPressed: onPressed, icon: icon, tooltip: tooltip),
    );

    return AppBar(
      leadingWidth: 0,
      titleSpacing: 0,
      title: SizedBox(
        height: kToolbarHeight,
        child: Row(
          children: [
            action(
              onPressed: () => Scaffold.of(context).openDrawer(),
              icon: const Icon(Icons.menu),
              tooltip: 'Menu',
            ),
            const Spacer(),
            action(
              onPressed: navigation.navigateToSearch,
              icon: Icon(Icons.search, color: scheme.onSurface),
              tooltip: 'Search',
            ),
            const Spacer(),
            action(
              onPressed: () => guarded(navigation.navigateToSavedItems),
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
              tooltip: 'Disimpan (For Sale & Lelang)',
            ),
            const Spacer(),
            action(
              onPressed: () => guarded(navigation.navigateToMyBids),
              icon: Icon(Icons.gavel_outlined, color: scheme.onSurface),
              tooltip: 'My Bids / Bid Saya',
            ),
            const Spacer(),
            action(
              onPressed: () => guarded(navigation.navigateToChat),
              icon: isGuest
                  ? Icon(Icons.chat_bubble_outline, color: scheme.onSurface)
                  : ChatBadgeWidget(
                      child: Icon(
                        Icons.chat_bubble_outline,
                        color: scheme.onSurface,
                      ),
                    ),
              tooltip: 'Messages',
            ),
            const Spacer(),
            action(
              onPressed: () => guarded(navigation.navigateToNotifications),
              icon: isGuest
                  ? Icon(Icons.notifications_outlined, color: scheme.onSurface)
                  : NotificationBadgeWidget(
                      userId: userId,
                      child: Icon(
                        Icons.notifications_outlined,
                        color: scheme.onSurface,
                      ),
                    ),
              tooltip: 'Notifications',
            ),
          ],
        ),
      ),
      actions: const [SizedBox(width: 8)],
    );
  }

  @override
  Size get preferredSize => const Size.fromHeight(kToolbarHeight);
}
