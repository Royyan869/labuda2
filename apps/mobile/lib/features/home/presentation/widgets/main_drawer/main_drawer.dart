import 'package:labuda/domains/user/profile/profile.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/models/seller_identity_data.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/features/home/presentation/widgets/main_drawer/drawer_header.dart';
import 'package:labuda/features/home/presentation/widgets/main_drawer/drawer_footer.dart';
import 'package:labuda/features/home/presentation/widgets/main_drawer/drawer_item.dart';
import 'package:labuda/domains/system/support/support.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';

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
  final Function(BuildContext, String) onHandleComingSoon;

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
    required this.onHandleComingSoon,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final l10n = AppLocalizations.of(context)!;
    final authState = ref.watch(authControllerProvider);
    final authenticatedUser = ref.watch(authenticatedUserProvider);
    final isLoggedIn = authenticatedUser != null;
    final showPlaceholder =
        authenticatedUser == null && authState is! AuthStateUnauthenticated;
    final sellerIdentityStatus = ref.watch(sellerIdentityStatusProvider);
    final isSeller = sellerIdentityStatus == SellerIdentityStatus.seller;

    return Drawer(
      backgroundColor: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
      child: SafeArea(
        child: Column(
          children: [
            // Drawer header - conditional based on auth state
            MainDrawerHeader(
              isDark: isDark,
              isLoggedIn: isLoggedIn,
              showPlaceholder: showPlaceholder,
              onSignIn: onHandleSignIn,
              onSignUp: onHandleSignUp,
              identity: SellerIdentityData(
                userId: authenticatedUser?.id ?? '',
                username: authenticatedUser?.username,
                avatarUrl: authenticatedUser?.avatarUrl,
                isSeller: isSeller,
                  storeName: ref.watch(profileStreamProvider(authenticatedUser?.id ?? '')).value?.farmInfo?.farmName,
                  storeImageUrl: ref.watch(profileStreamProvider(authenticatedUser?.id ?? '')).value?.farmInfo?.farmPhotoUrl,
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
                      horizontal: 16,
                      vertical: 4,
                    ),
                  ),
                  const LanguageSelectorTile(
                    contentPadding: EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 4,
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
                        // Show login prompt for guest users
                        Navigator.pop(context);
                        AppSnackBar.showError(
                          context,
                          'Please login to access support',
                          duration: const Duration(seconds: 3),
                        );
                        return;
                      }
                      Navigator.pop(context);
                      Navigator.of(context).push(
                        MaterialPageRoute(
                          builder: (context) => HelpCenterScreen(
                            userId: authenticatedUser.id,
                            userName: authenticatedUser.username,
                            userAvatar: authenticatedUser.avatarUrl,
                          ),
                        ),
                      );
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


