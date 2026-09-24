import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import '../screens/marketplace_screen.dart';

void registerMarketplaceTab(INavigationRegistry registry) {
  registry.registerTab(
    NavigationTab(
      id: 'marketplace',
      label: 'Marketplace',
      icon: Icons.storefront_outlined,
      selectedIcon: Icons.storefront,
      order: 1,
      pageBuilder: () => const MarketplaceScreen(),
    ),
  );
}
