import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/shared/providers/authenticated_account_provider.dart';

/// Permission Helper - Lightweight canonical access interpretation layer
///
/// **OWNERSHIP:**
/// - Single place for access checks based on user roles
/// - Centralizes permission logic to prevent drift
/// - Provides both static methods and extensions for convenience
///
/// **NOT THIS HELPER'S RESPONSIBILITY:**
/// - Route decisions (owned by goRouterProvider)
/// - UI rendering (owned by screens/widgets)
/// - State management (owned by AuthController)
///
/// **SOURCE OF TRUTH:**
/// - All role data comes from AuthUser (PostgreSQL via Backend API /users/me)
/// - This helper only interprets the data, never modifies it
///
/// **SELLER AUTHORITY (S3 ALIGNMENT):**
/// - hasMarketAuthority is the canonical seller state check (from backend)
/// - NO fallback to role-based check - subscription is required for market features
/// - Backend derives hasMarketAuthority from seller_profiles + seller_subscriptions
/// - Active = seller-active
/// - Expired / no subscription = NOT seller-active
///
/// **ROLE VOCABULARY (Canonical):**
/// - guest: Unauthenticated (client-only)
/// - user: Default authenticated user
/// - seller: Can sell items (separate from user)
/// - admin: Administrative privileges (unifies admin/superAdmin/supportAdmin)
/// Moderation authority = admin + moderation.* capabilities (not a role)
///
/// **USAGE:**
/// ```dart
/// // Using static methods
/// if (PermissionHelper.canAccessSellerFeatures(user)) { ... }
///
/// // Using extension on AuthUser
/// if (user.canAccessSellerFeatures) { ... }
///
/// // Using extension on WidgetRef
/// if (ref.canAccessSellerFeatures) { ... }
/// ```
abstract final class PermissionHelper {
  /// Check if user is authenticated (has non-guest role)
  static bool isAuthenticated(AuthUser? user) {
    return user?.isAuthenticated ?? false;
  }

  /// Check if user can access seller features
  ///
  /// **SELLER AUTHORITY (S3 ALIGNMENT):**
  /// - Uses backend-derived hasMarketAuthority (has profile + active subscription)
  /// - NOT using a role check alone
  /// - Seller capability requires active subscription, not just role assignment
  ///
  /// **BUSINESS TRUTH:**
  /// - seller_profiles = identity
  /// - seller_subscriptions = authority
  /// - Active + Grace = seller-active
  /// - Expired / no subscription = NOT seller-active (even if role exists)
  static bool canAccessSellerFeatures(AuthUser? user) {
    // S3: Use hasMarketAuthority (backend-derived) as primary check
    // This combines: hasSellerProfile + sellerSubscriptionStatus == 'active'
    return user?.hasMarketAuthority ?? false;
  }

  /// Get seller subscription status for UI display
  ///
  /// Returns: 'active', 'expired', 'none', or null if not a seller
  static String? getSellerSubscriptionStatus(AuthUser? user) {
    return user?.sellerSubscriptionStatus;
  }

  /// Check if seller has an active subscription
  ///
  /// This is the same as canAccessSellerFeatures() but more explicit
  static bool isSellerSubscriptionActive(AuthUser? user) {
    final status = user?.sellerSubscriptionStatus;
    return status == 'active';
  }

  /// Check if seller subscription has expired
  static bool isSellerSubscriptionExpired(AuthUser? user) {
    return user?.sellerSubscriptionStatus == 'expired';
  }

  /// Check if user has a seller profile (identity)
  static bool hasSellerProfile(AuthUser? user) {
    return user?.hasSellerProfile ?? false;
  }

  /// Check if user can access admin features
  /// Includes unified admin role (admin, super_admin, support_admin from backend)
  static bool canAccessAdminFeatures(AuthUser? user) {
    return user?.isAdmin ?? false;
  }

  /// Check if user is a regular user (buyer/consumer role only, not seller/admin)
  static bool isRegularUser(AuthUser? user) {
    if (user == null) return false;
    return user.roles.contains(UserRole.user) &&
        !user.hasCreatedSellerProfile &&
        !user.isAdmin;
  }

  /// Check if user has a specific role
  static bool hasRole(AuthUser? user, UserRole role) {
    return user?.hasRole(role) ?? false;
  }

  /// Check if user has any of the specified roles
  static bool hasAnyRole(AuthUser? user, List<UserRole> roles) {
    return user?.hasAnyRole(roles) ?? false;
  }

  /// Check if user has all of the specified roles
  static bool hasAllRoles(AuthUser? user, List<UserRole> roles) {
    return user?.hasAllRoles(roles) ?? false;
  }

  /// Check if user's account is active (not suspended, banned, or deleted)
  static bool isAccountActive(AuthUser? user) {
    if (user == null) return false;
    return user.accountStatus == null ||
        user.accountStatus == AccountStatus.active;
  }

  /// Check if user's account is suspended
  static bool isAccountSuspended(AuthUser? user) {
    return user?.accountStatus == AccountStatus.suspended;
  }

  /// Check if user's account is banned
  static bool isAccountBanned(AuthUser? user) {
    return user?.accountStatus == AccountStatus.banned;
  }

  /// Check if user's account is deleted
  static bool isAccountDeleted(AuthUser? user) {
    return user?.accountStatus == AccountStatus.deleted;
  }

  /// Check if user has any active penalty points
  static bool hasActivePenalties(AuthUser? user) {
    final activePoints = user?.activePenaltyPoints;
    return activePoints != null && activePoints > 0;
  }

  /// Check if user is ID verified
  static bool isIdVerified(AuthUser? user) {
    return user?.isIdVerified ?? false;
  }

  /// Check if user is farm verified
  static bool isFarmVerified(AuthUser? user) {
    return user?.isFarmVerified ?? false;
  }

  /// Check if user is phone verified
  static bool isPhoneVerified(AuthUser? user) {
    return user?.isPhoneVerified ?? false;
  }
}

/// Extension on AuthUser for permission checks
///
/// This provides a more fluent API when you already have an AuthUser instance.
///
/// **AUTH ALIGNMENT:** These extensions use backend-derived authority fields.
/// For seller features, `hasMarketAuthority` is the canonical check.
extension AuthUserPermissionExtension on AuthUser {
  /// Check if user can access seller features
  ///
  /// **AUTH ALIGNMENT:** Uses backend-derived `hasMarketAuthority` directly.
  /// This is consistent with PermissionHelper.canAccessSellerFeatures().
  bool get canAccessSellerFeatures => hasMarketAuthority ?? false;

  /// Check if user can access admin features (unified admin role)
  bool get canAccessAdminFeatures => isAdmin;

  /// Check if account is active
  bool get isAccountActive =>
      accountStatus == null || accountStatus == AccountStatus.active;

  /// Check if account is suspended
  bool get isAccountSuspended => accountStatus == AccountStatus.suspended;

  /// Check if account is banned
  bool get isAccountBanned => accountStatus == AccountStatus.banned;

  /// Check if account is deleted
  bool get isAccountDeleted => accountStatus == AccountStatus.deleted;

  /// Check if has active penalties
  bool get hasActivePenalties {
    final activePoints = activePenaltyPoints;
    return activePoints != null && activePoints > 0;
  }
}

/// Extension on WidgetRef for permission checks
///
/// This provides convenient access to permission checks from widgets.
extension WidgetRefPermissionExtension on WidgetRef {
  /// Get authenticated user or null
  AuthUser? get authenticatedUser => watch(authenticatedUserProvider);

  /// Check if current user is authenticated
  bool get isAuthenticated =>
      PermissionHelper.isAuthenticated(authenticatedUser);

  /// Check if current user can access seller features
  bool get canAccessSellerFeatures =>
      PermissionHelper.canAccessSellerFeatures(authenticatedUser);

  /// Check if current user can access admin features
  bool get canAccessAdminFeatures =>
      PermissionHelper.canAccessAdminFeatures(authenticatedUser);

  /// Check if current user is a regular user
  bool get isRegularUser => PermissionHelper.isRegularUser(authenticatedUser);

  /// Check if current user has a specific role
  bool hasRole(UserRole role) =>
      PermissionHelper.hasRole(authenticatedUser, role);

  /// Check if current user has any of the specified roles
  bool hasAnyRole(List<UserRole> roles) =>
      PermissionHelper.hasAnyRole(authenticatedUser, roles);

  /// Check if current user has all of the specified roles
  bool hasAllRoles(List<UserRole> roles) =>
      PermissionHelper.hasAllRoles(authenticatedUser, roles);

  /// Check if current user's account is active
  bool get isAccountActive =>
      PermissionHelper.isAccountActive(authenticatedUser);
}
