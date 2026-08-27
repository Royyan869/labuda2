/// Account status of a user in LABUDA platform
///
/// **BACKEND AUTHORITY:** This enum MUST match the Go backend exactly.
/// Source: backend/internal/domain/user/domain/entity/user_enums.go
///
/// Values are synchronized with backend:
/// - active: User can perform all actions
/// - suspended: Temporarily restricted (e.g., due to violations)
/// - banned: Permanently banned from the platform
/// - deleted: Account has been deleted
enum AccountStatus {
  /// User account is active and can perform all actions
  /// API value: "active"
  active,

  /// User account is temporarily suspended
  /// API value: "suspended"
  /// Typically due to accumulated penalty points
  suspended,

  /// User account is permanently banned
  /// API value: "banned"
  /// Cannot be reversed without admin intervention
  banned,

  /// User account has been deleted
  /// API value: "deleted"
  /// Soft delete in backend (deleted_at is set)
  deleted;

  /// Convert from API value (snake_case) to AccountStatus enum
  /// Returns [AccountStatus.active] as default fallback for unknown values
  static AccountStatus fromApiValue(String value) {
    return AccountStatus.values.firstWhere(
      (status) => status.apiValue == value,
      orElse: () => AccountStatus.active,
    );
  }

  /// Get API value (snake_case) for backend communication
  String get apiValue {
    switch (this) {
      case AccountStatus.active:
        return 'active';
      case AccountStatus.suspended:
        return 'suspended';
      case AccountStatus.banned:
        return 'banned';
      case AccountStatus.deleted:
        return 'deleted';
    }
  }

  /// Check if user can perform actions (active only)
  bool get isActive => this == AccountStatus.active;

  /// Check if user is restricted (suspended, banned, or deleted)
  bool get isRestricted =>
      this == AccountStatus.suspended ||
      this == AccountStatus.banned ||
      this == AccountStatus.deleted;

  /// Check if user is banned (permanent)
  bool get isBanned => this == AccountStatus.banned;

  /// Check if user is suspended (temporary)
  bool get isSuspended => this == AccountStatus.suspended;

  /// Display name for UI
  String get displayName {
    switch (this) {
      case AccountStatus.active:
        return 'Active';
      case AccountStatus.suspended:
        return 'Suspended';
      case AccountStatus.banned:
        return 'Banned';
      case AccountStatus.deleted:
        return 'Deleted';
    }
  }
}

/// Extension for AccountStatus with additional helpers
extension AccountStatusExtension on AccountStatus {
  /// Check if user can perform commerce actions (buy/sell)
  bool get canPerformCommerceActions => isActive;

  /// Check if user can message others
  bool get canMessage => isActive;

  /// Check if user can create listings
  bool get canCreateListings => isActive;

  /// Check if user can place bids
  bool get canPlaceBids => isActive;
}
