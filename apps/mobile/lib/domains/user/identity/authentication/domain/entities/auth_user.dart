import 'package:labuda/core/core.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'account_status.dart';
import 'seller_tier.dart';

/// Authentication user entity.
///
/// This is the domain entity for authenticated users in the LABUDA platform.
///
/// **BACKEND AUTHORITY:** All user status, badges, and penalty fields come from backend.
/// Client does NOT calculate or derive these values - purely for display.
///
/// **SOURCE OF TRUTH (Single Source):**
/// - Identity (email, password, provider): Firebase Auth
/// - Profile Data (username, bio, avatarUrl): PostgreSQL via Backend API /users/me
/// - Roles & Permissions: PostgreSQL via Backend API /users/me
/// - Account Status, Badge, Penalties: PostgreSQL via Backend API /users/me
///
/// **Multiple Roles Support:**
/// - User can have multiple roles (e.g., user + admin)
/// - Primary role (first in list) used for default behavior
///
/// **SELLER AUTHORITY (CRITICAL):**
/// Use `hasMarketAuthority` or the `isSeller` extension getter instead.
/// Seller MARKET capability requires ACTIVE subscription, not just role assignment.
/// Workspace access (dashboard, view orders) only requires hasSellerProfile.
/// See the AuthUserRoleExtension below for canonical seller state checks.
class AuthUser extends BaseEntity {
  // === Core Identity ===
  final String email;
  final String username;
  final String? avatarUrl;
  final String? bio;
  final bool isEmailVerified;
  final String? phoneNumber;
  final DateTime? phoneVerifiedAt;
  final DateTime? dateOfBirth;

  // === Account Status (BACKEND AUTHORITY) ===
  final AccountStatus? accountStatus; // active, suspended, banned, deleted
  // SellerBadge removed in PHASE 1A remediation - use sellerTier instead
  final SellerTier?
  sellerTier; // basic, pro, elite (Canonical performance tier, NOT a role)

  // === Seller State (BACKEND AUTHORITY) ===
  // These fields provide honest seller state from backend - no guessing from role
  final bool?
  hasSellerProfile; // Has created a seller profile (workspace identity)
  final String? sellerSubscriptionStatus; // active, expired, none
  final bool?
  hasMarketAuthority; // MARKET authority: has profile + active subscription
  // NOTE: Workspace access uses hasSellerProfile; market features use hasMarketAuthority

  // === Penalty Points (BACKEND AUTHORITY) ===
  final int? totalPenaltyPoints; // Lifetime penalty points
  final int? activePenaltyPoints; // Currently active penalty points

  // === Verification Flags (BACKEND AUTHORITY) ===
  // All verification flags come from backend API (Single Source of Truth)
  final bool? isPhoneVerified; // Phone verification status (from backend)
  final bool? isIdVerified; // KYC verification status
  final bool? isFarmVerified; // Farm/livestock seller verification
  final DateTime? idVerifiedAt; // KYC verification timestamp
  final DateTime? farmVerifiedAt; // Farm verification timestamp
  // Note: phoneVerifiedAt is in Core Identity section above

  // === Legacy Fields ===
  final List<UserRole> roles;
  final AuthProvider provider;

  // === Public Lifecycle (BACKEND AUTHORITY) ===
  //
  // E5.2 — Canonical public lifecycle string sourced from
  // `response.identity.lifecycle` on GET /users/:id (server coarsens
  // users.account_status + users.deleted_at via viewercontext.CoarsenLifecycle
  // and surfaces only {active, unavailable, removed}). Defaults to
  // ContentLifecycle.active when the wire omits `identity` (e.g. /users/me
  // and other surfaces that still return the legacy flat UserDTO). The
  // mobile layer MUST NOT derive this from the raw `accountStatus` field —
  // doing so would replicate the coarsening rule client-side and violate
  // ADR-006 §11 "Frontend-rendered user identity is forbidden".
  final ContentLifecycle lifecycle;

  const AuthUser({
    required super.id,
    required super.createdAt,
    required super.updatedAt,
    required this.email,
    required this.username,
    this.avatarUrl,
    this.bio,
    required this.isEmailVerified,
    this.phoneNumber,
    this.isPhoneVerified,
    this.phoneVerifiedAt,
    this.dateOfBirth,
    this.accountStatus,
    this.sellerTier,
    this.hasSellerProfile,
    this.sellerSubscriptionStatus,
    this.hasMarketAuthority,
    this.totalPenaltyPoints,
    this.activePenaltyPoints,
    this.isIdVerified,
    this.isFarmVerified,
    this.idVerifiedAt,
    this.farmVerifiedAt,
    required this.roles,
    required this.provider,
    this.lifecycle = ContentLifecycle.active,
  });

  /// Primary role (first role in the list)
  UserRole get role => roles.isNotEmpty ? roles.first : UserRole.guest;

  /// Check if user has a specific role
  bool hasRole(UserRole role) => roles.contains(role);

  /// Check if user has any of the specified roles
  bool hasAnyRole(List<UserRole> checkRoles) =>
      roles.any((role) => checkRoles.contains(role));

  /// Check if user has all of the specified roles
  bool hasAllRoles(List<UserRole> checkRoles) =>
      checkRoles.every((role) => roles.contains(role));

  @override
  List<Object?> get props => [
    ...super.props,
    email,
    username,
    avatarUrl,
    bio,
    isEmailVerified,
    phoneNumber,
    isPhoneVerified,
    phoneVerifiedAt,
    dateOfBirth,
    accountStatus,
    sellerTier,
    hasSellerProfile,
    sellerSubscriptionStatus,
    hasMarketAuthority,
    totalPenaltyPoints,
    activePenaltyPoints,
    isIdVerified,
    isFarmVerified,
    idVerifiedAt,
    farmVerifiedAt,
    roles,
    provider,
    lifecycle,
  ];

  AuthUser copyWith({
    String? id,
    DateTime? createdAt,
    DateTime? updatedAt,
    String? email,
    String? username,
    String? avatarUrl,
    String? bio,
    bool? isEmailVerified,
    String? phoneNumber,
    bool? isPhoneVerified,
    DateTime? phoneVerifiedAt,
    DateTime? dateOfBirth,
    AccountStatus? accountStatus,
    SellerTier? sellerTier,
    bool? hasSellerProfile,
    String? sellerSubscriptionStatus,
    bool? hasMarketAuthority,
    int? totalPenaltyPoints,
    int? activePenaltyPoints,
    bool? isIdVerified,
    bool? isFarmVerified,
    DateTime? idVerifiedAt,
    DateTime? farmVerifiedAt,
    List<UserRole>? roles,
    AuthProvider? provider,
    ContentLifecycle? lifecycle,
  }) {
    return AuthUser(
      id: id ?? this.id,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      email: email ?? this.email,
      username: username ?? this.username,
      avatarUrl: avatarUrl ?? this.avatarUrl,
      bio: bio ?? this.bio,
      isEmailVerified: isEmailVerified ?? this.isEmailVerified,
      phoneNumber: phoneNumber ?? this.phoneNumber,
      isPhoneVerified: isPhoneVerified ?? this.isPhoneVerified,
      phoneVerifiedAt: phoneVerifiedAt ?? this.phoneVerifiedAt,
      dateOfBirth: dateOfBirth ?? this.dateOfBirth,
      accountStatus: accountStatus ?? this.accountStatus,
      sellerTier: sellerTier ?? this.sellerTier,
      hasSellerProfile: hasSellerProfile ?? this.hasSellerProfile,
      sellerSubscriptionStatus:
          sellerSubscriptionStatus ?? this.sellerSubscriptionStatus,
      hasMarketAuthority: hasMarketAuthority ?? this.hasMarketAuthority,
      totalPenaltyPoints: totalPenaltyPoints ?? this.totalPenaltyPoints,
      activePenaltyPoints: activePenaltyPoints ?? this.activePenaltyPoints,
      isIdVerified: isIdVerified ?? this.isIdVerified,
      isFarmVerified: isFarmVerified ?? this.isFarmVerified,
      idVerifiedAt: idVerifiedAt ?? this.idVerifiedAt,
      farmVerifiedAt: farmVerifiedAt ?? this.farmVerifiedAt,
      roles: roles ?? this.roles,
      provider: provider ?? this.provider,
      lifecycle: lifecycle ?? this.lifecycle,
    );
  }

  /// Get social handle for mentions and casual context.
  ///
  /// OWNER TRUTH: public identity is `username` only. `fullName` is private
  /// (KYC/forms/receipts) and must NOT surface as a public identity label.
  String get socialHandle => '@$username';
}

/// Extension untuk AuthUser
///
/// **IMPORTANT:** These helpers use the canonical UserRole enum from core/src/auth/app_role.dart
/// Seller tiers (basic, pro, elite) are NOT roles - use SellerTier enum instead.
///
/// **SELLER AUTHORITY (S3 ALIGNMENT):**
/// - hasMarketAuthority is the canonical MARKET capability check (from backend)
/// - NO fallback to role-based check - subscription is required for MARKET features
/// - Backend derives hasMarketAuthority from seller_profiles + seller_subscriptions
///
/// **BUSINESS TRUTH:**
/// - seller_profiles (hasSellerProfile) = workspace identity
/// - seller_subscriptions (hasMarketAuthority) = MARKET authority
/// - Workspace access: requires hasSellerProfile (expired sellers can view)
/// - Market features: require hasMarketAuthority (active subscription needed)
/// - Active = hasMarketAuthority
/// - Expired / no subscription = NOT hasMarketAuthority (even if role exists)
extension AuthUserRoleExtension on AuthUser {
  /// Canonical MARKET capability check - uses backend-derived hasMarketAuthority
  ///
  /// **S3 HARDENING:** No longer falls back to role check.
  /// - Returns true ONLY when hasMarketAuthority == true (has profile + active subscription)
  /// - Returns false if hasMarketAuthority is null or false
  bool get isSeller => hasMarketAuthority ?? false;

  /// Check if seller has an active subscription
  bool get isSellerWithActiveSubscription =>
      sellerSubscriptionStatus == 'active';

  /// Check if seller has created a seller profile
  bool get hasCreatedSellerProfile => hasSellerProfile ?? false;

  /// Check if seller subscription has expired
  bool get isSellerSubscriptionExpired => sellerSubscriptionStatus == 'expired';

  bool get isAdmin => roles.any((r) => r == UserRole.admin);
  bool get isGuest => roles.isEmpty || roles.first == UserRole.guest;
  bool get isAuthenticated => !isGuest;

  /// **DEPRECATED: Semantically incorrect**
  /// Sellers and admins can also buy things - "buyer" is not an exclusive identity.
  /// Use `isAuthenticated` or `PermissionHelper.isRegularUser()` instead.
  /// This getter returns true only for users who are NOT sellers or admins.
  @Deprecated(
    'Buyer is not exclusive - sellers/admins can also buy. Use isAuthenticated',
  )
  bool get isBuyer => roles.contains(UserRole.user) && !isSeller && !isAdmin;
}

/// Authentication provider yang digunakan
enum AuthProvider { email, google, apple, phone }
