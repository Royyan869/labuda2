import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'unified_edit_profile_screen.dart';
import 'package:labuda/domains/user/preference/seller/seller.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/settings_profile_identity_section.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/settings_security_privacy_section.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/settings_app_preferences_section.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/settings_support_section.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/settings_account_management_section.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/settings_marketing_section.dart';

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
          padding: const EdgeInsets.only(
            top: AppMetrics.p8,
            bottom: AppMetrics.p8,
          ),
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

            // 🪙 Labuda Coins (loyalty) — canonical entry point
            _buildCoinsTile(context),

            // 📢 Marketing & Promotion Section — seller capability only (hasMarketAuthority)
            if (currentUser != null &&
                sellerCapabilityStatus == SellerCapabilityStatus.active)
              SettingsMarketingSection(onNavigate: _handleNavigation),

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
      case 'discount':
        _navigateToDiscountManagement(context);
        break;
      case 'promotion':
        context.push(RoutePaths.sellerCanonicalPromotions);
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

  void _navigateToDiscountManagement(BuildContext context) {
    context.push(RoutePaths.sellerDiscounts);
  }

  Future<void> _navigateToUpgradeSeller(BuildContext context) async {
    await context.push(RoutePaths.sellerUpgrade);

    // Refresh auth state when returning from upgrade screen
    // This ensures upgrade card visibility is correctly updated (even on cancel)
    if (mounted) {
      await ref.read(authControllerProvider.notifier).forceRefreshAuthState();
    }
  }

  void _navigateToSecurity(BuildContext context) {
    context.push(RoutePaths.security);
  }

  void _navigateToAddress(BuildContext context) {
    context.push(RoutePaths.addresses);
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
    context.push(RoutePaths.termsOfService);
  }

  void _navigateToPrivacyPolicy(BuildContext context) {
    context.push(RoutePaths.privacyPolicy);
  }

  void _navigateToBlockedUsers(BuildContext context) {
    context.push(RoutePaths.blockedUsers);
  }

  void _navigateToMyReports(BuildContext context) {
    context.push(RoutePaths.myReports);
  }

  void _showContactSupport(BuildContext context) {
    // Navigate to Help Center first (self-help layer). The route resolves the
    // reader identity from the session, so no user data is passed here — only
    // the auth gate below remains local to this affordance.
    final currentUser = ref.read(authenticatedUserProvider);

    if (currentUser == null) {
      ref.read(navigationHandlerProvider).navigateToSignIn();
      return;
    }

    context.push(RoutePaths.helpCenter);
  }

  Widget _buildSellerDashboardCard(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      margin: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p8,
        AppMetrics.p16,
        AppMetrics.p16,
      ),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [context.statusColors.success, context.statusColors.success],
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
            padding: const EdgeInsets.all(AppMetrics.p24),
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
                    size: AppIconSize.emphasis,
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Seller Dashboard',
                        style: context.typeRoles.titleSection.copyWith(
                          color: scheme.onPrimary,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        'Manage your store and sales',
                        style: context.typeRoles.bodyDense.copyWith(
                          color: scheme.onPrimary,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                    ],
                  ),
                ),
                Icon(
                  Icons.arrow_forward_ios,
                  color: scheme.onPrimary,
                  size: AppIconSize.action,
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
      margin: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p0,
        AppMetrics.p16,
        AppMetrics.p12,
      ),
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
                  padding: const EdgeInsets.all(AppMetrics.p12),
                  decoration: BoxDecoration(
                    color: context.statusColors.info.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(AppShape.r10),
                  ),
                  child: Icon(
                    Icons.local_shipping_outlined,
                    color: context.statusColors.info,
                    size: AppIconSize.header,
                  ),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Shipping',
                        style: context.typeRoles.titleCompact.copyWith(
                          fontWeight: FontWeight.w600,
                          color: scheme.onSurface,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        'Manage shipping options & rates for your products',
                        style: context.typeRoles.labelMicro.copyWith(
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
                Icon(
                  Icons.arrow_forward_ios,
                  size: AppIconSize.inlineGlyph,
                  color: scheme.onSurfaceVariant,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  /// Labuda Coins (loyalty points) — canonical entry point to the `/coins`
  /// surface. Coins are loyalty points, not wallet/payment.
  Widget _buildCoinsTile(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return ListTile(
      leading: Icon(Icons.toll_outlined, color: scheme.onSurfaceVariant),
      title: Text('Labuda Coins', style: TextStyle(color: scheme.onSurface)),
      subtitle: Text(
        'Lihat saldo dan riwayat koin',
        style: context.typeRoles.bodyDense.copyWith(
          color: scheme.onSurfaceVariant,
        ),
      ),
      trailing: Icon(
        Icons.arrow_forward_ios,
        size: AppIconSize.inlineGlyph,
        color: scheme.onSurfaceVariant,
      ),
      onTap: () => context.push(RoutePaths.coins),
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
            Text(l10n.labudaDescription, style: context.typeRoles.bodyDense),
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
                      'Gagal keluar. Coba lagi.',
                      duration: const Duration(seconds: 4),
                    );
                  }
                }
              }
            },
            child: Text(
              l10n.signOut,
              style: TextStyle(color: Theme.of(context).colorScheme.primary),
            ),
          ),
        ],
      ),
    );
  }
}
