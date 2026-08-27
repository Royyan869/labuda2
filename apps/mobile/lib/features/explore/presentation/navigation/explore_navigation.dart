import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import '../screens/explore_screen.dart';

/// Register Explore tab to navigation registry
///
/// This function should be called during app initialization
/// to add the Explore tab to the bottom navigation.
///
/// Refactored: Clean navigation registration
void registerExploreTab(INavigationRegistry registry) {
  registry.registerTab(
    NavigationTab(
      id: 'explore',
      label: 'Explore',
      icon: Icons.explore_outlined,
      selectedIcon: Icons.explore,
      order:
          1, // Position: Home (0), Explore (1), Create (2-center), Others (3+)
      pageBuilder: () => const ExploreScreen(),
    ),
  );
}
