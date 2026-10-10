import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/shared/models/seller_identity_data.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/features/home/presentation/widgets/main_drawer/drawer_header.dart';
import 'package:hishumi/features/home/presentation/widgets/main_drawer/drawer_footer.dart';
import 'package:hishumi/features/home/presentation/widgets/main_drawer/drawer_item.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/seller_state.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';

/// Main drawer widget untuk main screen navigation
///
/// Shows user info, menu items, and app version.
/// Adapts content based on auth state (logged in vs guest).
class MainDrawer extends ConsumerWidget {
  final Function(int) onTabChanged;
  final VoidCallback onNavigateToMessages;
  final VoidCallback onNavigateToNotifications;
  final VoidCallback onHandleSignIn;
  final VoidCallback onHandleSignUp;
  final VoidCallback onHandleSignOut;
  final VoidCallback onHandleSettings;
  final VoidCallback onHandleProfile;

  const MainDrawer({
    super.key,
    required this.onTabChanged,
    required this.onNavigateToMessages,
    required this.onNavigateToNotifications,
    required this.onHandleSignIn,
    required this.onHandleSignUp,
    required this.onHandleSignOut,
    required this.onHandleSettings,
    required this.onHandleProfile,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scheme = Theme.of(context).colorScheme;
    final l10n = AppLocalizations.of(context)!;
    final authState = ref.watch(authControllerProvider);
    final authenticatedUser = ref.watch(authenticatedUserProvider);
    final isLoggedIn = authenticatedUser != null;
    final showPlaceholder =
        authenticatedUser == null && authState is! AuthStateUnauthenticated;
    final sellerIdentityStatus = ref.watch(sellerIdentityStatusProvider);
    final isSeller = sellerIdentityStatus == SellerIdentityStatus.seller;

    return Drawer(
      backgroundColor: scheme.surfaceContainerLow,
      child: SafeArea(
        child: Column(
          children: [
            // Drawer header - conditional based on auth state
            MainDrawerHeader(
              isLoggedIn: isLoggedIn,
              showPlaceholder: showPlaceholder,
              onSignIn: onHandleSignIn,
              onSignUp: onHandleSignUp,
              identity: SellerIdentityData(
                userId: authenticatedUser?.id ?? '',
                username: authenticatedUser?.username,
                avatarUrl: authenticatedUser?.avatarUrl,
                isSeller: isSeller,
                // Store identity comes from the hydrated session snapshot —
                // the same authority as `username` above — so the drawer renders
                // the store name on the first frame. It must never be sourced
                // from a profile stream: that path polls and cannot be instant.
                storeName: authenticatedUser?.storeName,
                storeImageUrl: authenticatedUser?.storeImageUrl,
              ),
              onProfile: onHandleProfile,
            ),

            // Menu items
            Expanded(
              child: ListView(
                padding: EdgeInsets.zero,
                children: [
                  // Seller Dashboard (only for sellers)
                  if (isSeller) ...[
                    MainDrawerItem(
                      icon: Icons.dashboard,
                      title: 'Seller Dashboard',
                      onTap: () {
                        Navigator.pop(context);
                        ref
                            .read(navigationHandlerProvider)
                            .navigateToSellerDashboard();
                      },
                    ),
                  ],

                  const ThemeSelectorTile(
                    contentPadding: EdgeInsets.symmetric(
                      horizontal: AppMetrics.p16,
                      vertical: AppMetrics.p4,
                    ),
                  ),
                  const LanguageSelectorTile(
                    contentPadding: EdgeInsets.symmetric(
                      horizontal: AppMetrics.p16,
                      vertical: AppMetrics.p4,
                    ),
                  ),
                  MainDrawerItem(
                    icon: Icons.settings,
                    title: l10n.settings,
                    onTap: () {
                      Navigator.pop(context);
                      onHandleSettings();
                    },
                  ),
                  MainDrawerItem(
                    icon: Icons.help,
                    title: l10n.helpSupport,
                    onTap: () {
                      final authenticatedUser = ref.read(
                        authenticatedUserProvider,
                      );
                      if (authenticatedUser == null) {
                        // Authentication required: send the guest to the
                        // canonical sign-in route rather than a transient toast.
                        final navigation = ref.read(navigationHandlerProvider);
                        Navigator.pop(context);
                        navigation.navigateToSignIn();
                        return;
                      }
                      Navigator.pop(context);
                      context.push(RoutePaths.helpCenter);
                    },
                  ),
                  if (isLoggedIn) ...[
                    MainDrawerItem(
                      icon: Icons.logout,
                      title: 'Sign Out',
                      onTap: () {
                        Navigator.pop(context);
                        onHandleSignOut();
                      },
                      isDestructive: true,
                    ),
                  ],
                ],
              ),
            ),

            // Footer
            const MainDrawerFooter(),
          ],
        ),
      ),
    );
  }
}


