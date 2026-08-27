import 'package:flutter/material.dart';

/// Interface untuk navigation registry yang memungkinkan feature registration
/// tanpa direct dependencies antar features
abstract interface class INavigationRegistry {
  /// Register a tab untuk main navigation
  void registerTab(NavigationTab tab);

  /// Get all registered tabs
  List<NavigationTab> getRegisteredTabs();

  /// Clear all registered tabs
  void clearTabs();
}

/// Model untuk navigation tab
class NavigationTab {
  final String id;
  final String label;
  final IconData icon;
  final IconData selectedIcon;
  final Widget Function() pageBuilder;
  final int order;

  const NavigationTab({
    required this.id,
    required this.label,
    required this.icon,
    required this.selectedIcon,
    required this.pageBuilder,
    this.order = 0,
  });
}
