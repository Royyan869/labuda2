/// LABUDA Canonical Role Vocabulary
///
/// **SINGLE SOURCE OF TRUTH for Flutter role vocabulary**
///
/// This enum defines the canonical role names used across the Flutter app.
/// All role checks, guards, and permissions must use this vocabulary.
///
/// **MAPPING TO BACKEND (Go/PostgreSQL):**
/// - DB "user" ↔ Flutter UserRole.user (default authenticated role)
/// - DB "admin" ↔ Flutter UserRole.admin (includes both admin and super_admin from backend)
///
/// **SPECIAL ROLE:**
/// - guest (client-only, not stored in DB)
///
/// **MIGRATION NOTES:**
/// - buyer → user (canonical: all authenticated users are "users")
/// - supportAdmin → admin (canonical: merged into admin)
/// - superAdmin → admin (canonical: merged into admin)
/// - verifier → REMOVED (ghost role, no backend equivalent)
/// - moderator → REMOVED (not a DB role; moderation = admin + moderation.* capabilities)
///
/// **IMPORTANT:**
/// - Seller tiers (basic, pro, elite) are NOT roles - use SellerTier enum
/// - Backend is the FINAL AUTHORITY for all role assignments
/// - This enum is for UI convenience and compile-time safety only
library;

/// Canonical user roles in LABUDA platform
///
/// **ROLE DEFINITIONS:**
/// - guest: Unauthenticated user (client-side only, never sent to backend)
/// - user: Default authenticated user role (buyer, consumer)
/// - admin: User has administrative privileges (includes admin, super_admin, support_admin)
///
/// **MODERATION:**
/// Moderation authority is NOT a role. It is granted via admin + moderation.* capabilities.
enum UserRole {
  /// Guest - unauthenticated user (client-side only)
  /// Never sent to backend API
  guest,

  /// Default authenticated user role
  /// Represents buyers/consumers in the platform
  /// Backend: "user" or "buyer"
  user,

  /// Admin role - user has administrative privileges
  /// Backend: "admin", "super_admin", "support_admin" all map to this
  /// This is a unified admin role for UI simplicity
  admin,
}

/// Extension for backward compatibility during migration
///
/// **DEPRECATED:** Use canonical role names directly
/// These helpers will be removed in future versions
extension UserRoleMigrationExtension on UserRole {
  /// **DEPRECATED:** Use `== UserRole.admin` instead
  bool get isAdmin => this == UserRole.admin;

  /// **DEPRECATED:** Use `== UserRole.guest` instead
  bool get isGuest => this == UserRole.guest;

  /// **DEPRECATED:** Use `this != UserRole.guest` instead
  bool get isAuthenticated => this != UserRole.guest;

  /// **DEPRECATED:** Use `== UserRole.user` instead
  @Deprecated('Use UserRole.user instead - buyer is now canonical "user"')
  bool get isBuyer => this == UserRole.user;

  /// **DEPRECATED:** All admin roles are now unified
  @Deprecated(
    'Use UserRole.admin instead - superAdmin is now canonical "admin"',
  )
  bool get isSuperAdmin => this == UserRole.admin;

  /// **DEPRECATED:** All admin roles are now unified
  @Deprecated(
    'Use UserRole.admin instead - supportAdmin is now canonical "admin"',
  )
  bool get isSupportAdmin => this == UserRole.admin;
}

/// Extension for API mapping (backend ↔ Flutter)
extension UserRoleApiMapping on UserRole {
  /// Convert Flutter role to backend API string
  String toApiValue() {
    switch (this) {
      case UserRole.guest:
        return 'guest'; // Client-only, typically not sent
      case UserRole.user:
        return 'user'; // Maps to backend "user" or "buyer"
      case UserRole.admin:
        return 'admin'; // Maps to backend "admin", "super_admin", "support_admin"
    }
  }
}

/// Helper class for UserRole API parsing
///
/// Provides static methods for converting between backend API strings
/// and canonical UserRole enum values.
class UserRoleParser {
  /// Parse backend API string to UserRole
  /// Handles legacy/alias role names for backward compatibility
  static UserRole fromApiValue(String value) {
    final normalized = value.toLowerCase().trim();

    switch (normalized) {
      // Canonical mappings
      case 'guest':
        return UserRole.guest;
      case 'user':
        return UserRole.user;
      case 'admin':
        return UserRole.admin;

      // Legacy/alias mappings - these map to canonical roles
      case 'buyer':
      case 'authenticated':
        return UserRole.user; // Legacy "buyer" → canonical "user"

      case 'super_admin':
      case 'superadmin':
      case 'support_admin':
      case 'supportadmin':
      case 'staff':
        return UserRole.admin; // All admin variants → canonical "admin"

      // Ghost roles - fallback to regular user
      case 'verifier':
      case 'moderator':
        return UserRole.user;

      default:
        // Safe fallback for unknown roles
        return UserRole.user;
    }
  }

  /// Check if backend role string represents an admin
  static bool isAdminRole(String value) {
    return fromApiValue(value) == UserRole.admin;
  }
}

/// Extension for display names (UI only)
extension UserRoleDisplay on UserRole {
  /// Get display name for UI
  String get displayName {
    switch (this) {
      case UserRole.guest:
        return 'Guest';
      case UserRole.user:
        return 'User';
      case UserRole.admin:
        return 'Administrator';
    }
  }

  /// Get short display name for badges/chips
  String get shortLabel {
    switch (this) {
      case UserRole.guest:
        return 'Guest';
      case UserRole.user:
        return 'User';
      case UserRole.admin:
        return 'Admin';
    }
  }
}
