/// Permission Guard Widgets
///
/// Reusable widgets for protecting screens and UI elements based on
/// authentication state and user roles.
///
/// **ROLE VOCABULARY (Canonical):**
/// - guest: Unauthenticated (client-only)
/// - user: Default authenticated user
/// - admin: Administrative privileges (unified admin role)
/// - seller capability: backend-derived state, not a role
/// Moderation authority = admin + moderation.* capabilities (not a role)
///
/// SOURCE OF TRUTH: PostgreSQL (Backend API /users/me)
///
/// Widgets:
/// - [AuthGuard] - Protects screens requiring authentication
/// - [RoleGuard] - Protects screens requiring specific roles
/// - [AdminGuard] - Convenience guard for admin-only screens
/// - [SellerGuard] - Convenience guard for seller-only screens
/// - [AuthAware] - Conditional rendering based on auth state
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';
import 'package:labuda/shared/providers/authenticated_account_provider.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

/// Auth Guard Widget - Protects screens that require authentication
///
/// SOURCE OF TRUTH: PostgreSQL (Backend API /users/me)
///
/// **STATE MACHINE AWARE:**
/// - Shows loading screen during backend sync (SyncingWithBackend)
/// - Shows error screen if auth error occurs
/// - Shows child when fully authenticated (AuthStateAuthenticated)
/// - Shows login required screen if unauthenticated
class AuthGuard extends ConsumerWidget {
  final Widget child;
  final String? redirectMessage;
  final bool showLoadingForUnauthenticated;

  const AuthGuard({
    super.key,
    required this.child,
    this.redirectMessage,
    this.showLoadingForUnauthenticated = false,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final currentUser = ref.watch(authenticatedUserProvider);
    final isSyncing = ref.watch(isSyncingWithBackendProvider);
    final authError = ref.watch(authErrorProvider);
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Backend sync in progress - show loading screen
    if (isSyncing) {
      return _buildSyncingScreen(isDark);
    }

    // Auth error occurred - show error screen with retry
    if (authError != null) {
      return _buildErrorScreen(context, ref, authError, isDark);
    }

    // User authenticated with valid backend data
    if (currentUser != null) {
      return child;
    }

    // User not authenticated
    if (showLoadingForUnauthenticated) {
      return _buildLoadingScreen(isDark);
    }

    return _buildAuthRequiredScreen(context, ref, isDark);
  }

  Widget _buildLoadingScreen(bool isDark) {
    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralWhite,
      body: const Center(child: CircularProgressIndicator()),
    );
  }

  /// Backend syncing screen - shown while fetching data from backend API
  /// SOURCE OF TRUTH: PostgreSQL (Backend API /users/me)
  Widget _buildSyncingScreen(bool isDark) {
    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralWhite,
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const CircularProgressIndicator(),
            const SizedBox(height: 24),
            Text(
              'Loading profile...',
              style: TextStyle(
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
                fontSize: 14,
              ),
            ),
          ],
        ),
      ),
    );
  }

  /// Error screen - shown when auth error occurs
  /// SOURCE OF TRUTH: PostgreSQL (Backend API /users/me)
  Widget _buildErrorScreen(
    BuildContext context,
    WidgetRef ref,
    String error,
    bool isDark,
  ) {
    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralWhite,
      appBar: AppBar(
        title: const Text('Connection Error'),
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
        backgroundColor: isDark
            ? AppColors.darkGray800
            : AppColors.neutralWhite,
        foregroundColor: isDark
            ? AppColors.neutralWhite
            : AppColors.darkGray800,
        elevation: 0,
      ),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Container(
                padding: const EdgeInsets.all(24),
                decoration: BoxDecoration(
                  color: AppColors.statusError.withValues(alpha: 0.1),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  Icons.cloud_off,
                  size: 64,
                  color: AppColors.statusError,
                ),
              ),
              const SizedBox(height: 32),
              Text(
                'Unable to Load Profile',
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.darkGray800,
                ),
              ),
              const SizedBox(height: 16),
              Text(
                'Please check your internet connection and try again.',
                style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                  color: isDark
                      ? AppColors.neutralGray300
                      : AppColors.neutralGray600,
                  height: 1.5,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 16),
              Text(
                error,
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: isDark
                      ? AppColors.neutralGray500
                      : AppColors.neutralGray500,
                ),
                textAlign: TextAlign.center,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
              const SizedBox(height: 40),
              ElevatedButton.icon(
                onPressed: () {
                  // Retry: trigger force refresh
                  ref
                      .read(authControllerProvider.notifier)
                      .forceRefreshAuthState();
                },
                icon: const Icon(Icons.refresh),
                label: const Text('Retry'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primaryRed,
                  foregroundColor: AppColors.light,
                  padding: const EdgeInsets.symmetric(
                    horizontal: 32,
                    vertical: 12,
                  ),
                ),
              ),
              const SizedBox(height: 16),
              TextButton.icon(
                onPressed: () => Navigator.of(context).pop(),
                icon: const Icon(Icons.arrow_back),
                label: const Text('Back'),
                style: TextButton.styleFrom(
                  foregroundColor: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildAuthRequiredScreen(
    BuildContext context,
    WidgetRef ref,
    bool isDark,
  ) {
    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralWhite,
      appBar: AppBar(
        title: const Text('Access Limited'),
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
        backgroundColor: isDark
            ? AppColors.darkGray800
            : AppColors.neutralWhite,
        foregroundColor: isDark
            ? AppColors.neutralWhite
            : AppColors.darkGray800,
        elevation: 0,
      ),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Container(
                padding: const EdgeInsets.all(24),
                decoration: BoxDecoration(
                  color: AppColors.primaryRed.withValues(alpha: 0.1),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  Icons.lock_outline,
                  size: 64,
                  color: AppColors.primaryRed,
                ),
              ),
              const SizedBox(height: 32),
              Text(
                'Login Required',
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.darkGray800,
                ),
              ),
              const SizedBox(height: 16),
              Text(
                redirectMessage ??
                    'You must login first to access this feature. '
                        'Please login or register to continue.',
                style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                  color: isDark
                      ? AppColors.neutralGray300
                      : AppColors.neutralGray600,
                  height: 1.5,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 40),
              Row(
                children: [
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: () {
                        ref.read(navigationHandlerProvider).navigateToSignUp();
                      },
                      icon: const Icon(Icons.person_add),
                      label: const Text('Register'),
                      style: OutlinedButton.styleFrom(
                        foregroundColor: AppColors.primaryRed,
                        side: BorderSide(color: AppColors.primaryRed),
                        padding: const EdgeInsets.symmetric(vertical: 12),
                      ),
                    ),
                  ),
                  const SizedBox(width: 16),
                  Expanded(
                    child: ElevatedButton.icon(
                      onPressed: () {
                        ref.read(navigationHandlerProvider).navigateToSignIn();
                      },
                      icon: const Icon(Icons.login),
                      label: const Text('Login'),
                      style: ElevatedButton.styleFrom(
                        backgroundColor: AppColors.primaryRed,
                        foregroundColor: AppColors.light,
                        padding: const EdgeInsets.symmetric(vertical: 12),
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 24),
              TextButton.icon(
                onPressed: () => Navigator.of(context).pop(),
                icon: const Icon(Icons.arrow_back),
                label: const Text('Back'),
                style: TextButton.styleFrom(
                  foregroundColor: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Role Guard - Protects screens that require specific user roles
///
/// SOURCE OF TRUTH: PostgreSQL (Backend API /users/me)
///
/// **STATE MACHINE AWARE:**
/// - Shows loading screen during backend sync (via AuthGuard)
/// - Shows access denied if user doesn't have required role
/// - Shows child if authenticated with valid role
class RoleGuard extends ConsumerWidget {
  final Widget child;
  final List<UserRole> allowedRoles;
  final String? accessDeniedMessage;

  const RoleGuard({
    super.key,
    required this.child,
    required this.allowedRoles,
    this.accessDeniedMessage,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authenticatedUser = ref.watch(authenticatedUserProvider);
    final isAuthenticated = ref.watch(isAuthenticatedProvider);
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Not authenticated or syncing - let AuthGuard handle
    if (authenticatedUser == null || !isAuthenticated) {
      return AuthGuard(
        redirectMessage: 'You must login to access this feature.',
        child: child,
      );
    }

    // Authenticated with valid role - check if has required role
    if (authenticatedUser.hasAnyRole(allowedRoles)) {
      return child;
    }

    // Authenticated but doesn't have required role
    return _buildAccessDeniedScreen(context, isDark);
  }

  Widget _buildAccessDeniedScreen(BuildContext context, bool isDark) {
    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralWhite,
      appBar: AppBar(
        title: const Text('Access Denied'),
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
        backgroundColor: isDark
            ? AppColors.darkGray800
            : AppColors.neutralWhite,
        foregroundColor: isDark
            ? AppColors.neutralWhite
            : AppColors.darkGray800,
        elevation: 0,
      ),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Container(
                padding: const EdgeInsets.all(24),
                decoration: BoxDecoration(
                  color: AppColors.statusWarning.withValues(alpha: 0.1),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  Icons.block,
                  size: 64,
                  color: AppColors.statusWarning,
                ),
              ),
              const SizedBox(height: 32),
              Text(
                'Access Denied',
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.darkGray800,
                ),
              ),
              const SizedBox(height: 16),
              Text(
                accessDeniedMessage ??
                    'You do not have permission to access this feature. '
                        'Contact administrator if you think this is an error.',
                style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                  color: isDark
                      ? AppColors.neutralGray300
                      : AppColors.neutralGray600,
                  height: 1.5,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 40),
              ElevatedButton.icon(
                onPressed: () => Navigator.of(context).pop(),
                icon: const Icon(Icons.arrow_back),
                label: const Text('Back'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primaryRed,
                  foregroundColor: AppColors.light,
                  padding: const EdgeInsets.symmetric(
                    horizontal: 32,
                    vertical: 12,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Admin Guard - Convenience wrapper for admin-only screens
///
/// Uses canonical UserRole.admin (unified admin role).
/// This includes backend "admin", "super_admin", and "support_admin".
///
/// PASS 2C: currently unused — mobile has no admin screens/routes today;
/// the admin panel lives in the separate `apps/admin` React app. Kept here
/// as ready-to-use scaffolding in case mobile ever grows an admin surface,
/// not because anything currently depends on it.
class AdminGuard extends StatelessWidget {
  final Widget child;
  final String? accessDeniedMessage;

  const AdminGuard({super.key, required this.child, this.accessDeniedMessage});

  @override
  Widget build(BuildContext context) {
    return RoleGuard(
      allowedRoles: const [UserRole.admin],
      accessDeniedMessage:
          accessDeniedMessage ??
          'This page can only be accessed by Administrators.',
      child: child,
    );
  }
}

/// Seller Guard - Convenience wrapper for seller-only screens
///
/// **SELLER AUTHORITY (S3 ALIGNMENT):**
/// - Uses backend-derived `hasMarketAuthority` (has profile + active subscription)
/// - Not using role checks for seller access
/// - Seller capability requires active subscription, not just role assignment
///
/// **BUSINESS TRUTH:**
/// - seller_profiles = identity
/// - seller_subscriptions = authority
/// - Active = seller-active
/// - Expired / no subscription = NOT seller-active (even if identity exists)
class SellerGuard extends ConsumerWidget {
  final Widget child;
  final String? accessDeniedMessage;

  const SellerGuard({super.key, required this.child, this.accessDeniedMessage});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authenticatedUser = ref.watch(authenticatedUserProvider);
    final isSyncing = ref.watch(isSyncingWithBackendProvider);
    final sellerIdentityStatus = ref.watch(sellerIdentityStatusProvider);
    final sellerCapabilityStatus = ref.watch(sellerCapabilityStatusProvider);
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Backend sync in progress - show loading screen
    if (isSyncing) {
      return _buildSyncingScreen(isDark);
    }

    // No authenticated user
    if (authenticatedUser == null) {
      return _buildAccessDeniedScreen(
        context,
        isDark,
        'Login required to access seller features.',
      );
    }

    if (sellerIdentityStatus != SellerIdentityStatus.seller) {
      final message =
          accessDeniedMessage ??
          'This page can only be accessed by Sellers with an active profile.';
      return _buildAccessDeniedScreen(context, isDark, message);
    }

    if (sellerCapabilityStatus != SellerCapabilityStatus.active) {
      final message =
          accessDeniedMessage ??
          (sellerCapabilityStatus == SellerCapabilityStatus.inactive
              ? 'Your seller subscription has expired. Please renew to access seller features.'
              : 'This page can only be accessed by Sellers with an active subscription.');
      return _buildAccessDeniedScreen(context, isDark, message);
    }

    // Seller is active - allow access
    return child;
  }

  Widget _buildSyncingScreen(bool isDark) {
    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralWhite,
      body: const Center(child: CircularProgressIndicator()),
    );
  }

  Widget _buildAccessDeniedScreen(
    BuildContext context,
    bool isDark,
    String message,
  ) {
    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralWhite,
      appBar: AppBar(
        title: const Text('Seller Access Required'),
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
        backgroundColor: isDark
            ? AppColors.darkGray800
            : AppColors.neutralWhite,
        foregroundColor: isDark
            ? AppColors.neutralWhite
            : AppColors.darkGray800,
        elevation: 0,
      ),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Container(
                padding: const EdgeInsets.all(24),
                decoration: BoxDecoration(
                  color: AppColors.statusWarning.withValues(alpha: 0.1),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  Icons.store_outlined,
                  size: 64,
                  color: AppColors.statusWarning,
                ),
              ),
              const SizedBox(height: 32),
              Text(
                'Seller Feature',
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.darkGray800,
                ),
              ),
              const SizedBox(height: 16),
              Text(
                message,
                style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                  color: isDark
                      ? AppColors.neutralGray300
                      : AppColors.neutralGray600,
                  height: 1.5,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 40),
              ElevatedButton.icon(
                onPressed: () => Navigator.of(context).pop(),
                icon: const Icon(Icons.arrow_back),
                label: const Text('Back'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primaryRed,
                  foregroundColor: AppColors.light,
                  padding: const EdgeInsets.symmetric(
                    horizontal: 32,
                    vertical: 12,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Auth-aware wrapper for conditional rendering based on auth state
class AuthAware extends ConsumerWidget {
  final Widget authenticated;
  final Widget? unauthenticated;
  final Widget? syncing;

  const AuthAware({
    super.key,
    required this.authenticated,
    this.unauthenticated,
    this.syncing,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authenticatedUser = ref.watch(authenticatedUserProvider);
    final isSyncing = ref.watch(isSyncingWithBackendProvider);

    if (isSyncing) {
      return syncing ?? _buildSyncingWidget(context);
    }

    if (authenticatedUser != null) {
      return authenticated;
    }

    return unauthenticated ?? const SizedBox.shrink();
  }

  Widget _buildSyncingWidget(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const CircularProgressIndicator(),
          const SizedBox(height: 16),
          Text(
            'Syncing...',
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
        ],
      ),
    );
  }
}

/// Extension for easy auth checks on WidgetRef
extension PermissionExtensions on WidgetRef {
  /// Check if current user is authenticated
  bool get isAuthenticated => watch(authenticatedUserProvider) != null;

  /// Get authenticated user or null
  AuthUser? get authenticatedUser => watch(authenticatedUserProvider);

  /// Check if current user has specific role
  bool hasRole(UserRole role) {
    return authenticatedUser?.hasRole(role) ?? false;
  }

  /// Check if current user has any of the specified roles
  bool hasAnyRole(List<UserRole> roles) {
    final user = authenticatedUser;
    return user != null && user.hasAnyRole(roles);
  }

  /// Check if current user is admin (unified admin role)
  bool get isAdmin {
    return hasRole(UserRole.admin);
  }

  /// Check if current user is seller
  ///
  /// **SELLER AUTHORITY (S3 ALIGNMENT):**
  /// - Uses backend-derived hasMarketAuthority (has profile + active subscription)
  /// - Not using role checks for seller access
  /// - Seller capability requires active subscription, not just role assignment
  ///
  /// For feature access, rely on hasMarketAuthority instead of role checks.
  bool get isSeller {
    // S3: Use hasMarketAuthority (backend-derived) without role fallback
    return authenticatedUser?.hasMarketAuthority ?? false;
  }

  /// Check if current user is a regular user (not seller/admin)
  bool get isRegularUser {
    final user = authenticatedUser;
    if (user == null) return false;
    return user.roles.contains(UserRole.user) && !isSeller && !isAdmin;
  }
}
