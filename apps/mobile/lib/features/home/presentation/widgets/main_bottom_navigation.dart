import 'package:flutter/material.dart';
import 'package:labuda/features/home/presentation/models/main_tab.dart';
import 'package:labuda/core/core.dart';

/// Main bottom navigation widget
///
/// Bottom navigation dengan Create button di posisi tengah.
/// Juga ada tombol Orders dan Settings di posisi terakhir.
class MainBottomNavigation extends StatelessWidget {
  final int currentIndex;
  final bool showMultiFAB;
  final List<MainTab> tabs;
  final Function(int) onTap;

  const MainBottomNavigation({
    super.key,
    required this.currentIndex,
    required this.showMultiFAB,
    required this.tabs,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return BottomNavigationBar(
      type: BottomNavigationBarType.fixed,
      backgroundColor: scheme.surface,
      selectedItemColor: scheme.primary,
      unselectedItemColor: scheme.onSurfaceVariant,
      selectedLabelStyle: TextStyle(
        fontSize: AppType.s12,
        fontWeight: FontWeight.w600,
        color: scheme.primary,
      ),
      unselectedLabelStyle: TextStyle(
        fontSize: AppType.s12,
        fontWeight: FontWeight.w400,
        color: scheme.onSurfaceVariant,
      ),
      currentIndex: (currentIndex >= 0 && currentIndex < tabs.length)
          ? (currentIndex >= 2 ? currentIndex + 1 : currentIndex)
          : 0,
      onTap: onTap,
      elevation: AppElevation.overlay,
      items: _buildBottomNavItems(context),
    );
  }

  List<BottomNavigationBarItem> _buildBottomNavItems(BuildContext context) {
    final items = <BottomNavigationBarItem>[];

    // Add tabs before position 2 (before create button)
    for (int i = 0; i < 2 && i < tabs.length; i++) {
      items.add(
        BottomNavigationBarItem(
          icon: Icon(tabs[i].icon),
          activeIcon: Icon(tabs[i].selectedIcon),
          label: tabs[i].label,
        ),
      );
    }

    // Center Create tab (always at position 2)
    items.add(
      BottomNavigationBarItem(
        icon: Icon(
          showMultiFAB ? Icons.close : Icons.add,
          color: Theme.of(context).colorScheme.primary,
        ),
        activeIcon: Icon(
          showMultiFAB ? Icons.close : Icons.add,
          color: Theme.of(context).colorScheme.primary,
        ),
        label: 'Create',
      ),
    );

    // Add remaining tabs after Create button
    // tabs[2] onwards become nav positions 3, 4, etc.
    for (int i = 2; i < tabs.length; i++) {
      items.add(
        BottomNavigationBarItem(
          icon: Icon(tabs[i].icon),
          activeIcon: Icon(tabs[i].selectedIcon),
          label: tabs[i].label,
        ),
      );
    }

    // Hardcoded Orders button (navigate to separate screen)
    items.add(
      const BottomNavigationBarItem(
        icon: Icon(Icons.shopping_bag_outlined),
        activeIcon: Icon(Icons.shopping_bag),
        label: 'Orders',
      ),
    );

    // Hardcoded Settings button (navigate to separate screen)
    items.add(
      const BottomNavigationBarItem(
        icon: Icon(Icons.settings_outlined),
        activeIcon: Icon(Icons.settings),
        label: 'Settings',
      ),
    );

    return items;
  }
}
