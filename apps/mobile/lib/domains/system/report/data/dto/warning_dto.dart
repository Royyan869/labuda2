/// Warning DTOs for API Integration
///
/// These models match the Go backend DTOs exactly.
/// Warnings are passive moderation records, not enforcement mechanisms.
library;

// =====================
// Warning DTOs
// =====================

/// User Warning DTO from API
///
/// Matches backend response from:
/// - GET /api/v1/warnings/:id
/// - GET /api/v1/warnings
/// - GET /api/v1/users/:id/warnings
/// - GET /api/v1/users/:id/warnings/active
class UserWarningDto {
  final String id;
  final String userId;
  final String
  level; // 'info', 'warning', or 'severe' - severity, not escalation
  final String reason; // Explanation for the warning
  final bool isActive; // Whether warning is currently active
  final String
  status; // 'active', 'revoked', or 'expired' - computed from state
  final DateTime createdAt;
  final DateTime? expiresAt; // Optional expiration date
  final DateTime? revokedAt; // When warning was revoked (if applicable)
  final String? revokedBy; // Admin ID who revoked (if applicable)
  final String issuedBy; // Admin ID who issued

  const UserWarningDto({
    required this.id,
    required this.userId,
    required this.level,
    required this.reason,
    required this.isActive,
    required this.status,
    required this.createdAt,
    this.expiresAt,
    this.revokedAt,
    this.revokedBy,
    required this.issuedBy,
  });

  /// Parse from backend JSON response
  factory UserWarningDto.fromJson(Map<String, dynamic> json) {
    // Parse issued_by from backend, map to issuedBy
    final issuedBy =
        json['issued_by'] as String? ?? json['issuedBy'] as String?;

    return UserWarningDto(
      id: json['id'] as String,
      userId: json['user_id'] as String,
      level: json['level'] as String,
      reason: json['reason'] as String,
      isActive: json['is_active'] as bool? ?? json['isActive'] as bool? ?? true,
      status: json['status'] as String,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
      expiresAt: json['expires_at'] != null
          ? DateTime.parse(json['expires_at'] as String)
          : null,
      revokedAt: json['revoked_at'] != null
          ? DateTime.parse(json['revoked_at'] as String)
          : null,
      revokedBy: json['revoked_by'] as String?,
      issuedBy: issuedBy ?? '',
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'user_id': userId,
    'level': level,
    'reason': reason,
    'is_active': isActive,
    'status': status,
    'created_at': createdAt.toIso8601String(),
    if (expiresAt != null) 'expires_at': expiresAt!.toIso8601String(),
    if (revokedAt != null) 'revoked_at': revokedAt!.toIso8601String(),
    if (revokedBy != null) 'revoked_by': revokedBy,
    'issued_by': issuedBy,
  };
}
