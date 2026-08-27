import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/home/presentation/models/main_tab.dart';

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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return BottomNavigationBar(
      type: BottomNavigationBarType.fixed,
      backgroundColor: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
      selectedItemColor: AppColors.primaryRed,
      unselectedItemColor: isDark
          ? AppColors.neutralGray400
          : AppColors.neutralGray600,
      selectedLabelStyle: const TextStyle(
        fontSize: 12,
        fontWeight: FontWeight.w600,
        color: AppColors.primaryRed,
      ),
      unselectedLabelStyle: TextStyle(
        fontSize: 12,
        fontWeight: FontWeight.w400,
        color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
      ),
      currentIndex: (currentIndex >= 0 && currentIndex < tabs.length)
          ? (currentIndex >= 2 ? currentIndex + 1 : currentIndex)
          : 0,
      onTap: onTap,
      elevation: 8,
      items: _buildBottomNavItems(),
    );
  }

  List<BottomNavigationBarItem> _buildBottomNavItems() {
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
          color: AppColors.primaryRed,
        ),
        activeIcon: Icon(
          showMultiFAB ? Icons.close : Icons.add,
          color: AppColors.primaryRed,
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
