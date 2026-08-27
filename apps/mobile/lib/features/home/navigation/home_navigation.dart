import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/home/presentation/screens/home_screen.dart';

/// Register Home tab to navigation registry
///
/// This function should be called during app initialization
/// to add the Home tab to the bottom navigation.
void registerHomeTab(INavigationRegistry registry) {
  registry.registerTab(
    NavigationTab(
      id: 'home',
      label: 'Home',
      icon: Icons.home_outlined,
      selectedIcon: Icons.home,
      order:
          0, // Position: Home (0), Explore (1), Create (2-center), Others (3+)
      pageBuilder: () => const HomeScreen(),
    ),
  );
}
