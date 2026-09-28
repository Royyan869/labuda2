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
    final sellerCapabilityStatus = ref.watch(sellerCapabilityStatusProvider);
    final isSeller = sellerIdentityStatus == SellerIdentityStatus.seller;

    return Scaffold(
      appBar: AppBarCustom(title: l10n.settings),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.only(top: AppMetrics.p8, bottom: AppMetrics.p8),
          children: [
            // ========================================
            // ROLE-BASED CARDS
            // ========================================

            // Seller Dashboard (only for sellers)
            if (isSeller) _buildSellerDashboardCard(context),

            // Phase 1: Seller shipping setup — global option catalog.
            // For Sale publish requires at least one linked shipping option.
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

            // 📢 Marketing & Promotion Section — seller capability only (hasMarketAuthority)
            if (currentUser != null &&
                sellerCapabilityStatus == SellerCapabilityStatus.active)
              SettingsMarketingSection(
                onNavigate: _handleNavigation,
                userId: currentUser.id,
              ),

            // 🔒 Security & Privacy Section
            SettingsSecurityPrivacySection(onNavigate: _handleNavigation),

            // 🔔 Notifications & Preferences Section
            SettingsAppPreferencesSection(onNavigate: _handleNavigation),

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
    final scheme = Theme.of(context).colorScheme;
    return Container(
      margin: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p8, AppMetrics.p16, AppMetrics.p16),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            context.statusColors.success,
            context.statusColors.success,
          ],
        ),
        borderRadius: BorderRadius.circular(AppShape.r16),
        boxShadow: [
          BoxShadow(
            color: context.statusColors.success.withValues(alpha: 0.3),
            blurRadius: 12,
            offset: const Offset(0, 6),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: () => _navigateToSellerDashboard(context),
          borderRadius: BorderRadius.circular(AppShape.r16),
          child: Padding(
            padding: const EdgeInsets.all(AppMetrics.p20),
            child: Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(AppMetrics.p12),
                  decoration: BoxDecoration(
                    color: scheme.onPrimary.withValues(alpha: 0.2),
                    borderRadius: BorderRadius.circular(AppShape.r12),
                  ),
                  child: Icon(
                    Icons.dashboard,
                    color: scheme.onPrimary,
                    size: 28,
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Seller Dashboard',
                        style: TextStyle(
                          color: scheme.onPrimary,
                          fontSize: AppType.s18,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        'Manage your store and sales',
                        style: TextStyle(
                          color: scheme.onPrimary,
                          fontSize: AppType.s14,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                    ],
                  ),
                ),
                Icon(
                  Icons.arrow_forward_ios,
                  color: scheme.onPrimary,
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
    final scheme = Theme.of(context).colorScheme;
    return Container(
      margin: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p0, AppMetrics.p16, AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: () => context.push(RoutePaths.sellerShipping),
          borderRadius: BorderRadius.circular(AppShape.r12),
          child: Padding(
            padding: const EdgeInsets.all(AppMetrics.p16),
            child: Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(AppMetrics.p10),
                  decoration: BoxDecoration(
                    color: context.statusColors.info.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(AppShape.r10),
                  ),
                  child: Icon(
                    Icons.local_shipping_outlined,
                    color: context.statusColors.info,
                    size: 22,
                  ),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Shipping',
                        style: TextStyle(
                          fontSize: AppType.s15,
                          fontWeight: FontWeight.w600,
                          color: scheme.onSurface,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        'Manage shipping options & rates for your products',
                        style: TextStyle(
                          fontSize: AppType.s12,
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
                Icon(
                  Icons.arrow_forward_ios,
                  size: 16,
                  color: scheme.onSurfaceVariant,
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
            Text(l10n.labudaDescription, style: const TextStyle(fontSize: AppType.s14)),
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
                          padding: const EdgeInsets.all(AppMetrics.p24),
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
              style: TextStyle(
                color: Theme.of(context).colorScheme.primary,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
