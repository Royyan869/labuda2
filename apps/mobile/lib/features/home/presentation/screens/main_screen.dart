import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/features/explore/presentation/screens/explore_screen.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';

/// Main Screen dengan bottom navigation untuk aplikasi LABUDA
///
/// Screen ini menyediakan:
/// - AppBar dengan search bar dan action buttons (Box, Chat, Notifications)
/// - Bottom navigation dengan Create button di tengah
/// - Drawer dengan menu navigasi dan user profile
/// - Tab switching untuk Home, Explore, Profile, etc.
class MainScreen extends ConsumerStatefulWidget {
  const MainScreen({super.key});

  @override
  ConsumerState<MainScreen> createState() => _MainScreenState();
}

class _MainScreenState extends ConsumerState<MainScreen> {
  int _currentIndex = 0;
  DateTime? _lastBackPressed;

  /// Helper untuk find tab index by label
  int _findTabIndexByLabel(String label, INavigationRegistry registry) {
    final registeredTabs = registry.getRegisteredTabs();

    for (int i = 0; i < registeredTabs.length; i++) {
      if (registeredTabs[i].label.toLowerCase() == label.toLowerCase()) {
        return i;
      }
    }
    return -1; // Not found
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final l10n = AppLocalizations.of(context)!;

    // Get navigation registry from provider
    final navigationRegistry = ref.read(navigationRegistryProvider);

    // Listen untuk pending tab switch
    ref.listen(pendingTabSwitchProvider, (previous, next) {
      if (next.hasSwitch && mounted) {
        if (next.target == 'profile') {
          // Find Profile tab index
          final profileTabIndex = _findTabIndexByLabel(
            'Profile',
            navigationRegistry,
          );
          if (profileTabIndex >= 0) {
            setState(() => _currentIndex = profileTabIndex);
          }
        } else if (next.target == 'explore') {
          // Find Explore tab index
          final exploreTabIndex = _findTabIndexByLabel(
            'Explore',
            navigationRegistry,
          );
          if (exploreTabIndex >= 0) {
            setState(() => _currentIndex = exploreTabIndex);
          }
        }
        // Clear pending switch
        Future.microtask(() {
          if (mounted) {
            ref.read(pendingTabSwitchProvider.notifier).clear();
          }
        });
      }
    });

    // Get tabs dari navigation registry
    final registeredTabs = navigationRegistry.getRegisteredTabs();

    // Fallback jika belum ada tabs yang diregister
    final tabs = registeredTabs.isNotEmpty
        ? registeredTabs
              .map(
                (navTab) => MainTab(
                  label: navTab.label,
                  icon: navTab.icon,
                  selectedIcon: navTab.selectedIcon,
                  page: navTab.pageBuilder(),
                ),
              )
              .toList()
        : [
            MainTab(
              label: l10n.home,
              icon: Icons.home_outlined,
              selectedIcon: Icons.home,
              page: const HomeScreen(),
            ),
            MainTab(
              label: 'Explore',
              icon: Icons.explore_outlined,
              selectedIcon: Icons.explore,
              page: const ExploreScreen(),
            ),
          ];

    // Ensure _currentIndex is within bounds
    if (_currentIndex >= tabs.length) {
      _currentIndex = 0;
    }

    final currentTab = tabs.isNotEmpty && _currentIndex < tabs.length
        ? tabs[_currentIndex]
        : tabs.isNotEmpty
        ? tabs[0]
        : MainTab(
            label: l10n.home,
            icon: Icons.home_outlined,
            selectedIcon: Icons.home,
            page: const HomeScreen(),
          );

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;

        final now = DateTime.now();
        final backButtonHasNotBeenPressedOrSnackBarHasBeenClosed =
            _lastBackPressed == null ||
            now.difference(_lastBackPressed!) > const Duration(seconds: 2);

        if (backButtonHasNotBeenPressedOrSnackBarHasBeenClosed) {
          _lastBackPressed = now;
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text('Press back again to exit'),
              duration: Duration(seconds: 3),
            ),
          );
        } else {
          // Exit app properly
          SystemNavigator.pop();
        }
      },
      child: Scaffold(
        appBar: MainAppBar(currentTab: currentTab),
        drawer: _buildDrawer(context, isDark),
        body: IndexedStack(
          index: _currentIndex,
          children: tabs.map((tab) => tab.page).toList(),
        ),
        bottomNavigationBar: _buildBottomNavigation(context, isDark, tabs),
      ),
    );
  }

  Widget _buildDrawer(BuildContext context, bool isDark) {
    final handler = MainScreenNavigationHandler(ref: ref, context: context);

    return MainDrawer(
      onTabChanged: (index) => setState(() => _currentIndex = index),
      onNavigateToMessages: handler.navigateToMessages,
      onNavigateToNotifications: handler.navigateToNotifications,
      onHandleSignIn: handler.handleSignIn,
      onHandleSignUp: handler.handleSignUp,
      onHandleSignOut: handler.handleSignOut,
      onHandleSettings: handler.handleSettings,
      onHandleProfile: handler.handleProfile,
      onHandleComingSoon: handler.handleComingSoon,
    );
  }

  Widget _buildBottomNavigation(
    BuildContext context,
    bool isDark,
    List<MainTab> tabs,
  ) {
    return MainBottomNavigation(
      currentIndex: _currentIndex,
      showMultiFAB: false,
      tabs: tabs,
      onTap: (index) {
        if (index == 2) {
          // Create tab (center position) - show modal bottom sheet
          _showCreateContentModal(context);
        } else {
          // Calculate total number of nav items (tabs + Create + Orders + Settings)
          final totalNavItems =
              tabs.length + 3; // +1 for Create, +1 for Orders, +1 for Settings

          // Check if this is Settings button (hardcoded at last position)
          if (index == totalNavItems - 1) {
            // Settings button tapped - navigate to Settings screen
            final handler = MainScreenNavigationHandler(
              ref: ref,
              context: context,
            );
            handler.handleSettings(closeDrawer: false);
          }
          // Check if this is Orders button (second to last position)
          else if (index == totalNavItems - 2) {
            // Orders button tapped - navigate to Orders screen
            ref.read(navigationHandlerProvider).navigateToOrders();
          } else {
            // Regular tab - calculate actual tab index
            final actualIndex = index > 2 ? index - 1 : index;

            if (actualIndex >= 0 && actualIndex < tabs.length) {
              setState(() {
                _currentIndex = actualIndex;
              });
            }
          }
        }
      },
    );
  }

  void _showCreateContentModal(BuildContext context) {
    final sellerIdentityStatus = ref.read(sellerIdentityStatusProvider);
    final sellerCapabilityStatus = ref.read(sellerCapabilityStatusProvider);

    CreateContentBottomSheet.show(
      context: context,
      onCreateContent: () {
        final navigation = ref.read(navigationHandlerProvider);
        navigation.navigateToCreateContent();
      },
      onCreateListing: sellerCapabilityStatus == SellerCapabilityStatus.active
          ? () {
              final navigation = ref.read(navigationHandlerProvider);
              navigation.navigateToCreateForSale();
            }
          : null,
      onCreateAuction: sellerCapabilityStatus == SellerCapabilityStatus.active
          ? () {
              final navigation = ref.read(navigationHandlerProvider);
              navigation.navigateToCreateAuction();
            }
          : null,
      onStartSelling: () {
        // Navigate to seller onboarding/subscription
        final navigation = ref.read(navigationHandlerProvider);
        navigation.navigateToSellerUpgrade();
      },
      onRenewSubscription: () {
        // Navigate to subscription renewal (uses seller upgrade flow)
        final navigation = ref.read(navigationHandlerProvider);
        navigation.navigateToSellerUpgrade();
      },
      sellerIdentityStatus: sellerIdentityStatus,
      sellerCapabilityStatus: sellerCapabilityStatus,
    );
  }
}
