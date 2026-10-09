import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/features/marketplace/presentation/screens/marketplace_screen.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';

/// Main Screen dengan bottom navigation untuk aplikasi LABUDA
///
/// Screen ini menyediakan:
/// - AppBar dengan search bar dan action buttons (Box, Chat, Notifications)
/// - Bottom navigation dengan Create button di tengah
/// - Drawer dengan menu navigasi dan user profile
/// - Tab switching untuk Home, Marketplace, Profile, etc.
class MainScreen extends ConsumerStatefulWidget {
  const MainScreen({super.key});

  @override
  ConsumerState<MainScreen> createState() => _MainScreenState();
}

// CATATAN ARSITEKTUR: pengamat lifecycle (WidgetsBindingObserver) yang
// dulu hidup di screen ini sudah dipindah ke [SessionLifecycleObserver]
// (lib/core/session/session_lifecycle_observer.dart) — pekerjaan resume
// (reconnect socket + re-read backend) milik sesi, jadi harus berjalan di
// route manapun, bukan hanya selama MainScreen ada di stack. Dulu hook itu
// hilang diam-diam pada route yang dicapai lewat `go()`, dan setiap
// focus-blip (screenshot) ikut menjalankan refresh sesi sehingga user
// dilempar loading → splash → home.
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
    final l10n = AppLocalizations.of(context)!;

    // Get navigation registry from provider
    final navigationRegistry = ref.read(navigationRegistryProvider);

    // Listen untuk pending tab switch
    ref.listen(pendingTabSwitchProvider, (previous, next) {
      if (next.hasSwitch && mounted) {
        // `marketplace` is the only target produced by
        // [PendingTabSwitchNotifier.setSwitch] (create For Sale / Auction
        // landing). The former `profile` branch was dead: no producer ever
        // emitted it.
        if (next.target == 'marketplace') {
          final marketplaceTabIndex = _findTabIndexByLabel(
            'Marketplace',
            navigationRegistry,
          );
          if (marketplaceTabIndex >= 0)
            setState(() => _currentIndex = marketplaceTabIndex);
          // NOT cleared here: MarketplaceScreen still has to read the sub-tab.
          // One consumer clears per target — clearing from both sides is what
          // made the sub-tab switch a race.
          return;
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
              label: 'Marketplace',
              icon: Icons.storefront_outlined,
              selectedIcon: Icons.storefront,
              page: const MarketplaceScreen(),
            ),
          ];

    // Ensure _currentIndex is within bounds
    if (_currentIndex >= tabs.length) {
      _currentIndex = 0;
    }

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
          AppSnackBar.showInfo(context, 'Tekan sekali lagi untuk keluar');
        } else {
          // Exit app properly
          SystemNavigator.pop();
        }
      },
      child: Scaffold(
        appBar: const MainAppBar(),
        drawer: _buildDrawer(context),
        body: IndexedStack(
          index: _currentIndex,
          children: tabs.map((tab) => tab.page).toList(),
        ),
        bottomNavigationBar: _buildBottomNavigation(context, tabs),
      ),
    );
  }

  Widget _buildDrawer(BuildContext context) {
    final handler = MainScreenNavigationHandler(ref: ref, context: context);
    final navigation = ref.read(navigationHandlerProvider);

    return MainDrawer(
      onTabChanged: (index) => setState(() => _currentIndex = index),
      onNavigateToMessages: () => navigation.navigateToChat(),
      onNavigateToNotifications: () => navigation.navigateToNotifications(),
      onHandleSignIn: () => navigation.navigateToSignIn(),
      onHandleSignUp: () => navigation.navigateToSignUp(),
      onHandleSignOut: handler.handleSignOut,
      onHandleSettings: () => navigation.navigateToSettings(),
      onHandleProfile: handler.handleProfile,
    );
  }

  Widget _buildBottomNavigation(BuildContext context, List<MainTab> tabs) {
    return MainBottomNavigation(
      currentIndex: _currentIndex,
      showMultiFAB: false,
      tabs: tabs,
      onTap: (index) {
        final navigation = ref.read(navigationHandlerProvider);
        final isGuest =
            ref.read(authControllerProvider) is! AuthStateAuthenticated;

        // GUEST POLICY (Owner canonical): auth-required affordances
        // (Create/Orders/Settings) use an explicit Sign In gate instead of
        // navigating to a private route that bounces back to /welcome.
        if (index == 2) {
          // Create tab (center position)
          if (isGuest) {
            navigation.navigateToSignIn();
            return;
          }
          _showCreateContentModal(context);
          return;
        }

        // Calculate total number of nav items (tabs + Create + Orders + Settings)
        final totalNavItems =
            tabs.length + 3; // +1 for Create, +1 for Orders, +1 for Settings

        // Check if this is Settings button (hardcoded at last position)
        if (index == totalNavItems - 1) {
          // Settings button tapped
          if (isGuest) {
            navigation.navigateToSignIn();
            return;
          }
          navigation.navigateToSettings();
        }
        // Check if this is Orders button (second to last position)
        else if (index == totalNavItems - 2) {
          // Orders button tapped
          if (isGuest) {
            navigation.navigateToSignIn();
            return;
          }
          navigation.navigateToOrders();
        } else {
          // Regular tab - calculate actual tab index
          final actualIndex = index > 2 ? index - 1 : index;

          if (actualIndex >= 0 && actualIndex < tabs.length) {
            setState(() {
              _currentIndex = actualIndex;
            });
          }
        }
      },
    );
  }

  void _showCreateContentModal(BuildContext context) {
    final sellerIdentityStatus = ref.read(sellerIdentityStatusProvider);
    final sellerCapabilityStatus = ref.read(sellerCapabilityStatusProvider);
    // Canonical expiry axis: capability `inactive` alone must not claim expiry.
    final isSubscriptionExpired = ref.read(isSellerSubscriptionExpiredProvider);

    CreateContentBottomSheet.show(
      context: context,
      onCreateContent: () {
        final navigation = ref.read(navigationHandlerProvider);
        navigation.navigateToCreateContent();
      },
      // CREATE FLOW (owner canonical): the bottom-bar sheet pushes the form
      // DIRECTLY. On success the create screens set a pending marketplace
      // switch (for-sale → sub-tab 0, auction → sub-tab 1), so the user lands
      // on the surface where the listing now lives. The marketplace itself
      // carries NO create buttons — this sheet is the single create entry.
      // Comment/chat keep their own direct push: they are pickers that need the
      // create screen to come back to them with a result.
      onCreateForSale: sellerCapabilityStatus == SellerCapabilityStatus.active
          ? () => context.push(RoutePaths.createForSale)
          : null,
      onCreateAuction: sellerCapabilityStatus == SellerCapabilityStatus.active
          ? () => context.push(RoutePaths.createAuction)
          : null,
      onStartSelling: () {
        // Navigate to seller onboarding/subscription
        final navigation = ref.read(navigationHandlerProvider);
        navigation.navigateToSellerUpgrade();
      },
      onRenewSubscription: () {
        final navigation = ref.read(navigationHandlerProvider);
        navigation.navigateToSellerRenewal();
      },
      sellerIdentityStatus: sellerIdentityStatus,
      sellerCapabilityStatus: sellerCapabilityStatus,
      isSubscriptionExpired: isSubscriptionExpired,
    );
  }
}
