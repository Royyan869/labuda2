import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/system/support/support.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'unified_edit_profile_screen.dart';
import 'security_screen.dart';
import 'address_list_screen.dart';
import 'terms_of_service_screen.dart';
import 'privacy_policy_screen.dart';
import 'blocked_users_screen.dart';
import 'package:labuda/domains/user/preference/seller/seller.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/settings_profile_identity_section.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/settings_security_privacy_section.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/settings_app_preferences_section.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/settings_support_section.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/settings_account_management_section.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/settings_marketing_section.dart';
import 'package:labuda/domains/system/report/presentation/screens/my_reports_screen.dart';

/// Unified Settings Screen (Personal + Business Management)
class SettingsScreen extends ConsumerStatefulWidget {
  const SettingsScreen({super.key});

  @override
  ConsumerState<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends ConsumerState<SettingsScreen> {
  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    // Use centralized providers (TANGGUNG_JAWAB_MODUL compliance)
    final currentUser = ref.watch(authenticatedUserProvider);
    final sellerIdentityStatus = ref.watch(sellerIdentityStatusProvider);
    final isSeller = sellerIdentityStatus == SellerIdentityStatus.seller;

    // Watch presence state for online status toggle
    final presenceState = ref.watch(presenceManagerProvider);
    final showOnlineStatus = presenceState.isEnabled;

    return Scaffold(
      appBar: AppBarCustom(title: l10n.settings),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.only(top: 8, bottom: 8),
          children: [
            // ========================================
            // ROLE-BASED CARDS
            // ========================================

            // Seller Dashboard (only for sellers)
            if (isSeller) _buildSellerDashboardCard(context),

            // Phase 1: Seller shipping setup — global option catalog.
            // Listing publish requires at least one linked shipping option.
            if (isSeller) _buildSellerShippingTile(context),

            // Upgrade Seller Card (only for non-sellers)
            if (sellerIdentityStatus == SellerIdentityStatus.nonSeller)
              SettingsUpgradeCard(
                onUpgrade: () => _navigateToUpgradeSeller(context),
              ),

            // ========================================
            // GENERAL SETTINGS (All users)
            // ========================================

            // 👤 Profile & Identity Section
            SettingsProfileIdentitySection(onNavigate: _handleNavigation),

            // 📢 Marketing & Promotion Section
            if (currentUser != null)
              SettingsMarketingSection(
                onNavigate: _handleNavigation,
                userId: currentUser.id,
              ),

            // 🔒 Security & Privacy Section
            SettingsSecurityPrivacySection(
              onNavigate: _handleNavigation,
              showOnlineStatus: showOnlineStatus,
              onShowOnlineStatusChanged: (value) =>
                  ref.read(presenceManagerProvider.notifier).setEnabled(value),
            ),

            // 🔔 Notifications & Preferences Section
            const SettingsAppPreferencesSection(),

            // ========================================
            // FOOTER
            // ========================================

            // Support & Legal Section
            SettingsSupportSection(onNavigate: _handleNavigation),

            // Account Management Section
            SettingsAccountManagementSection(
              onSignOut: () => _showSignOutDialog(context, l10n),
            ),
          ],
        ),
      ),
    );
  }

  void _handleNavigation(String route) {
    final l10n = AppLocalizations.of(context)!;

    switch (route) {
      case 'editProfile':
        _navigateToEditProfile(context);
        break;
      case 'personalInformation':
        _navigateToPersonalInformation(context);
        break;
      case 'address':
        _navigateToAddress(context);
        break;
      case 'security':
        _navigateToSecurity(context);
        break;
      case 'notifications':
        _navigateToNotificationSettings(context);
        break;
      case 'businessProfile':
        _navigateToBusinessProfile(context);
        break;
      case 'helpSupport':
        _showContactSupport(context);
        break;
      case 'termsOfService':
        _navigateToTermsOfService(context);
        break;
      case 'privacyPolicy':
        _navigateToPrivacyPolicy(context);
        break;
      case 'blockedUsers':
        _navigateToBlockedUsers(context);
        break;
      case 'myReports':
        _navigateToMyReports(context);
        break;
      // Coins feature removed for BI compliance - no longer available
      // case 'coins':
      //   _navigateToCoins(context);
      //   break;
      case 'about':
        _showAboutDialog(context, l10n);
        break;
    }
  }

  void _navigateToEditProfile(BuildContext context) {
    // Use centralized provider (TANGGUNG_JAWAB_MODUL compliance)
    final currentUser = ref.read(authenticatedUserProvider);
    if (currentUser != null) {
      context.push(
        RoutePaths.editProfile,
        extra: UnifiedEditProfileSection.personal,
      );
    }
  }

  void _navigateToPersonalInformation(BuildContext context) {
    context.push(RoutePaths.personalInformation);
  }

  void _navigateToBusinessProfile(BuildContext context) {
    // Use centralized provider (TANGGUNG_JAWAB_MODUL compliance)
    final currentUser = ref.read(authenticatedUserProvider);
    if (currentUser != null) {
      context.push(
        RoutePaths.editProfile,
        extra: UnifiedEditProfileSection.business,
      );
    }
  }

  Future<void> _navigateToUpgradeSeller(BuildContext context) async {
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (context) => const SellerUpgradeWizardScreen(),
      ),
    );

    // Refresh auth state when returning from upgrade screen
    // This ensures upgrade card visibility is correctly updated (even on cancel)
    if (mounted) {
      await ref.read(authControllerProvider.notifier).forceRefreshAuthState();
    }
  }

  void _navigateToSecurity(BuildContext context) {
    Navigator.of(
      context,
    ).push(MaterialPageRoute(builder: (context) => const SecurityScreen()));
  }

  void _navigateToAddress(BuildContext context) {
    Navigator.of(
      context,
    ).push(MaterialPageRoute(builder: (context) => const AddressListScreen()));
  }

  void _navigateToNotificationSettings(BuildContext context) {
    // Use NavigationHandler for centralized navigation (per ROUTING_AND_NAVIGATION_GUIDE)
    ref.read(navigationHandlerProvider).navigateToNotificationSettings();
  }

  void _navigateToSellerDashboard(BuildContext context) {
    final navigation = ref.read(navigationHandlerProvider);
    navigation.navigateToSellerDashboard();
  }

  void _navigateToTermsOfService(BuildContext context) {
    Navigator.of(context).push(
      MaterialPageRoute(builder: (context) => const TermsOfServiceScreen()),
    );
  }

  void _navigateToPrivacyPolicy(BuildContext context) {
    Navigator.of(context).push(
      MaterialPageRoute(builder: (context) => const PrivacyPolicyScreen()),
    );
  }

  void _navigateToBlockedUsers(BuildContext context) {
    Navigator.of(
      context,
    ).push(MaterialPageRoute(builder: (context) => const BlockedUsersScreen()));
  }

  void _navigateToMyReports(BuildContext context) {
    Navigator.of(
      context,
    ).push(MaterialPageRoute(builder: (context) => const MyReportsScreen()));
  }

  void _showContactSupport(BuildContext context) {
    // Navigate to Help Center first (self-help layer)
    // Use centralized provider (TANGGUNG_JAWAB_MODUL compliance)
    final currentUser = ref.read(authenticatedUserProvider);

    if (currentUser == null) {
      AppSnackBar.showError(
        context,
        'Please login to access support',
        duration: const Duration(seconds: 4),
      );
      return;
    }

    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (context) => HelpCenterScreen(
          userId: currentUser.id,
          userName: '@${currentUser.username}',
          userAvatar: currentUser.avatarUrl,
        ),
      ),
    );
  }

  Widget _buildSellerDashboardCard(BuildContext context) {
    return Container(
      margin: const EdgeInsets.fromLTRB(16, 8, 16, 16),
      decoration: BoxDecoration(
        gradient: const LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            Color(0xFF10B981), // Emerald green
            Color(0xFF059669), // Darker emerald
          ],
        ),
        borderRadius: BorderRadius.circular(16),
        boxShadow: [
          BoxShadow(
            color: const Color(0xFF10B981).withValues(alpha: 0.3),
            blurRadius: 12,
            offset: const Offset(0, 6),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: () => _navigateToSellerDashboard(context),
          borderRadius: BorderRadius.circular(16),
          child: Padding(
            padding: const EdgeInsets.all(20),
            child: Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: AppColors.neutralWhite.withValues(alpha: 0.2),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: const Icon(
                    Icons.dashboard,
                    color: AppColors.neutralWhite,
                    size: 28,
                  ),
                ),
                const SizedBox(width: 16),
                const Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Seller Dashboard',
                        style: TextStyle(
                          color: AppColors.neutralWhite,
                          fontSize: 18,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      SizedBox(height: 4),
                      Text(
                        'Manage your store and sales',
                        style: TextStyle(
                          color: AppColors.neutralWhite,
                          fontSize: 14,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                    ],
                  ),
                ),
                const Icon(
                  Icons.arrow_forward_ios,
                  color: AppColors.neutralWhite,
                  size: 18,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  /// Phase 1: Pengiriman entry — opens the seller global shipping options screen.
  Widget _buildSellerShippingTile(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      margin: const EdgeInsets.fromLTRB(16, 0, 16, 12),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
        ),
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: () => context.push(RoutePaths.sellerShipping),
          borderRadius: BorderRadius.circular(12),
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(10),
                  decoration: BoxDecoration(
                    color: AppColors.statusInfo.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: const Icon(
                    Icons.local_shipping_outlined,
                    color: AppColors.statusInfo,
                    size: 22,
                  ),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Pengiriman',
                        style: TextStyle(
                          fontSize: 15,
                          fontWeight: FontWeight.w600,
                          color: isDark
                              ? AppColors.neutralWhite
                              : AppColors.neutralGray900,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        'Atur opsi & tarif pengiriman untuk listing Anda',
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.neutralGray600,
                        ),
                      ),
                    ],
                  ),
                ),
                Icon(
                  Icons.arrow_forward_ios,
                  size: 16,
                  color: AppColors.neutralGray400,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _showAboutDialog(BuildContext context, AppLocalizations l10n) {
    final currentYear = DateTime.now().year.toString();

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(l10n.aboutLABUDA),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(l10n.koiSocialCommercePlatform),
            const SizedBox(height: 8),
            Text(l10n.version),
            const SizedBox(height: 8),
            Text(l10n.copyrightLabudaTeam(currentYear)),
            const SizedBox(height: 16),
            Text(l10n.labudaDescription, style: const TextStyle(fontSize: 14)),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: Text(l10n.close),
          ),
        ],
      ),
    );
  }

  void _showSignOutDialog(BuildContext context, AppLocalizations l10n) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(l10n.signOut),
        content: Text(l10n.signOutConfirm),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: Text(l10n.cancel),
          ),
          TextButton(
            onPressed: () async {
              Navigator.of(context).pop(); // Close confirmation dialog

              if (context.mounted) {
                // Show loading dialog
                showDialog(
                  context: context,
                  barrierDismissible: false,
                  builder: (context) => PopScope(
                    canPop: false,
                    child: Center(
                      child: Card(
                        child: Padding(
                          padding: const EdgeInsets.all(24),
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              const CircularProgressIndicator(),
                              const SizedBox(height: 16),
                              Text(
                                'Signing out...',
                                style: Theme.of(context).textTheme.bodyLarge,
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ),
                );

                try {
                  // Perform sign out
                  final authController = ref.read(
                    authControllerProvider.notifier,
                  );
                  await authController.signOut();

                  if (mounted && context.mounted) {
                    // Close loading dialog
                    Navigator.of(context).pop();

                    // Navigate to welcome screen and clear navigation stack
                    ref.read(navigationHandlerProvider).navigateToWelcome();

                    // Show success message
                    AppSnackBar.showSuccess(
                      context,
                      l10n.signedOutSuccessfully,
                      duration: const Duration(seconds: 3),
                    );
                  }
                } catch (e) {
                  if (mounted && context.mounted) {
                    // Close loading dialog
                    Navigator.of(context).pop();

                    // Show error message
                    AppSnackBar.showError(
                      context,
                      'Failed to sign out. Please try again.',
                      duration: const Duration(seconds: 4),
                    );
                  }
                }
              }
            },
            child: Text(
              l10n.signOut,
              style: TextStyle(color: AppColors.primaryRed),
            ),
          ),
        ],
      ),
    );
  }
}
