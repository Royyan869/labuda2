/// User Warning Domain Entity
///
/// Pure domain entity for user warnings.
/// Warnings are PASSIVE MODERATION RECORDS, not enforcement mechanisms.
///
/// V1 DEFINITION:
/// - Warnings are admin-issued records of policy violations
/// - Warnings do NOT trigger automatic suspensions or restrictions
/// - Warning levels indicate SEVERITY, not escalation step
/// - Warnings can be active, revoked, or expired
library;

// =====================
// Enums
// =====================

/// Warning severity levels - matches backend WarningLevel exactly
enum WarningLevel {
  /// Minor policy reminder
  info,

  /// Moderate policy violation
  warning,

  /// Serious policy violation
  severe,
}

/// Warning status - matches backend WarningStatus exactly
enum WarningStatus {
  /// Warning is currently active
  active,

  /// Warning has been revoked by admin
  revoked,

  /// Warning has passed its expiration date
  expired,
}

// =====================
// Extensions
// =====================

extension WarningLevelExtension on WarningLevel {
  String get value => name;

  String get displayName {
    switch (this) {
      case WarningLevel.info:
        return 'Info';
      case WarningLevel.warning:
        return 'Warning';
      case WarningLevel.severe:
        return 'Severe';
    }
  }

  /// Parse from backend string value
  static WarningLevel fromString(String value) {
    return WarningLevel.values.firstWhere(
      (e) => e.name == value,
      orElse: () => WarningLevel.info,
    );
  }
}

extension WarningStatusExtension on WarningStatus {
  String get value => name;

  String get displayName {
    switch (this) {
      case WarningStatus.active:
        return 'Active';
      case WarningStatus.revoked:
        return 'Revoked';
      case WarningStatus.expired:
        return 'Expired';
    }
  }

  bool get isActive => this == WarningStatus.active;
  bool get isResolved =>
      this == WarningStatus.expired || this == WarningStatus.revoked;

  /// Parse from backend string value
  static WarningStatus fromString(String value) {
    return WarningStatus.values.firstWhere(
      (e) => e.name == value,
      orElse: () => WarningStatus.active,
    );
  }
}

// =====================
// Entities
// =====================

/// User Warning Entity - Track warnings issued to users
///
/// V1: Passive record only. No enforcement, no escalation, no suspensions.
class UserWarning {
  final String id;
  final String userId;
  final WarningLevel level; // Severity: info, warning, severe
  final String reason; // Explanation for the warning
  final String adminId; // Admin ID who issued the warning
  final String adminName; // Resolved admin name (for display)
  final DateTime createdAt; // When warning was issued
  final DateTime? expiresAt; // Optional expiration date
  final WarningStatus status; // active, revoked, or expired
  final bool isActive; // Direct from backend is_active field
  final DateTime? revokedAt; // When warning was revoked
  final String? revokedBy; // Admin ID who revoked

  const UserWarning({
    required this.id,
    required this.userId,
    required this.level,
    required this.reason,
    required this.adminId,
    required this.adminName,
    required this.createdAt,
    this.expiresAt,
    this.status = WarningStatus.active,
    this.isActive = true,
    this.revokedAt,
    this.revokedBy,
  });

  /// Check if warning is currently active (not expired and not revoked)
  bool get isCurrentlyActive {
    if (!isActive) return false;
    if (status != WarningStatus.active) return false;
    if (expiresAt != null && DateTime.now().isAfter(expiresAt!)) {
      return false;
    }
    return true;
  }

  /// Check if warning can be revoked (only active warnings)
  bool get canBeRevoked => status == WarningStatus.active;

  /// Check if warning has expired
  bool get isExpired => expiresAt != null && DateTime.now().isAfter(expiresAt!);

  /// Get days until expiry
  int? get daysUntilExpiry {
    if (expiresAt == null) return null;
    final diff = expiresAt!.difference(DateTime.now()).inDays;
    return diff > 0 ? diff : 0;
  }

  UserWarning copyWith({
    String? id,
    String? userId,
    WarningLevel? level,
    String? reason,
    String? adminId,
    String? adminName,
    DateTime? createdAt,
    DateTime? expiresAt,
    WarningStatus? status,
    bool? isActive,
    DateTime? revokedAt,
    String? revokedBy,
  }) {
    return UserWarning(
      id: id ?? this.id,
      userId: userId ?? this.userId,
      level: level ?? this.level,
      reason: reason ?? this.reason,
      adminId: adminId ?? this.adminId,
      adminName: adminName ?? this.adminName,
      createdAt: createdAt ?? this.createdAt,
      expiresAt: expiresAt ?? this.expiresAt,
      status: status ?? this.status,
      isActive: isActive ?? this.isActive,
      revokedAt: revokedAt ?? this.revokedAt,
      revokedBy: revokedBy ?? this.revokedBy,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is UserWarning &&
          runtimeType == other.runtimeType &&
          id == other.id;

  @override
  int get hashCode => id.hashCode;
}

/// Create Warning Request - for admins to issue new warnings
///
/// Matches backend CreateWarningRequest requirements:
/// - userId (required, uuid)
/// - level (required, one of: info, warning, severe)
/// - reason (required, string 1-500 chars)
/// - expiresAt (optional, Unix timestamp)
class CreateWarningRequest {
  final String userId;
  final WarningLevel level;
  final String reason;
  final DateTime? expiresAt;

  const CreateWarningRequest({
    required this.userId,
    required this.level,
    required this.reason,
    this.expiresAt,
  });

  /// Validate request against backend requirements
  bool get isValid {
    if (userId.isEmpty) return false;
    if (reason.isEmpty || reason.length > 500) return false;
    return true;
  }

  /// Convert expiresAt to Unix timestamp for backend
  int? get expiresAtUnix {
    if (expiresAt == null) return null;
    return expiresAt!.millisecondsSinceEpoch ~/ 1000;
  }

  CreateWarningRequest copyWith({
    String? userId,
    WarningLevel? level,
    String? reason,
    DateTime? expiresAt,
  }) {
    return CreateWarningRequest(
      userId: userId ?? this.userId,
      level: level ?? this.level,
      reason: reason ?? this.reason,
      expiresAt: expiresAt ?? this.expiresAt,
    );
  }
}
