/// Content Removal Domain Entity
///
/// Pure domain entity for content removal tracking.
/// Contains all business logic and properties related to content removals.
library;

import 'report.dart'; // for ReportTargetType

// =====================
// Enums
// =====================

/// Content removal status
enum RemovalStatus { active, restored, permanentlyDeleted }

// =====================
// Extensions
// =====================

extension RemovalStatusExtension on RemovalStatus {
  String get value => name;

  String get displayName {
    switch (this) {
      case RemovalStatus.active:
        return 'Removed';
      case RemovalStatus.restored:
        return 'Restored';
      case RemovalStatus.permanentlyDeleted:
        return 'Permanently Deleted';
    }
  }

  bool get isActive => this == RemovalStatus.active;
  bool get isRestored => this == RemovalStatus.restored;

  static RemovalStatus fromString(String value) {
    return RemovalStatus.values.firstWhere(
      (e) => e.name == value,
      orElse: () => RemovalStatus.active,
    );
  }
}

// =====================
// Entities
// =====================

/// Content Removal Entity - Track removed content
class ContentRemoval {
  final String id;
  final String targetId;
  final ReportTargetType targetType;
  final String? targetTitle;
  final String? reportId;
  final String reason;
  final String adminId;
  final String adminName;
  final DateTime removedAt;
  final RemovalStatus status;
  final bool isRestored;
  final DateTime? restoredAt;
  final String? restoredBy;
  final String? restoreReason;

  const ContentRemoval({
    required this.id,
    required this.targetId,
    required this.targetType,
    this.targetTitle,
    this.reportId,
    required this.reason,
    required this.adminId,
    required this.adminName,
    required this.removedAt,
    this.status = RemovalStatus.active,
    this.isRestored = false,
    this.restoredAt,
    this.restoredBy,
    this.restoreReason,
  });

  /// Check if removal is currently active
  bool get isActive => status == RemovalStatus.active && !isRestored;

  /// Check if removal can be restored
  bool get canBeRestored => status == RemovalStatus.active && !isRestored;

  /// Get days since removal
  int get daysSinceRemoval {
    return DateTime.now().difference(removedAt).inDays;
  }

  ContentRemoval copyWith({
    String? id,
    String? targetId,
    ReportTargetType? targetType,
    String? targetTitle,
    String? reportId,
    String? reason,
    String? adminId,
    String? adminName,
    DateTime? removedAt,
    RemovalStatus? status,
    bool? isRestored,
    DateTime? restoredAt,
    String? restoredBy,
    String? restoreReason,
  }) {
    return ContentRemoval(
      id: id ?? this.id,
      targetId: targetId ?? this.targetId,
      targetType: targetType ?? this.targetType,
      targetTitle: targetTitle ?? this.targetTitle,
      reportId: reportId ?? this.reportId,
      reason: reason ?? this.reason,
      adminId: adminId ?? this.adminId,
      adminName: adminName ?? this.adminName,
      removedAt: removedAt ?? this.removedAt,
      status: status ?? this.status,
      isRestored: isRestored ?? this.isRestored,
      restoredAt: restoredAt ?? this.restoredAt,
      restoredBy: restoredBy ?? this.restoredBy,
      restoreReason: restoreReason ?? this.restoreReason,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ContentRemoval &&
          runtimeType == other.runtimeType &&
          id == other.id;

  @override
  int get hashCode => id.hashCode;
}

/// Create Content Removal Request - DTO untuk membuat content removal baru (admin)
class CreateContentRemovalRequest {
  final String targetId;
  final ReportTargetType targetType;
  final String? targetTitle;
  final String? reportId;
  final String reason;
  final String adminId;

  const CreateContentRemovalRequest({
    required this.targetId,
    required this.targetType,
    this.targetTitle,
    this.reportId,
    required this.reason,
    required this.adminId,
  });

  /// Validate request
  bool get isValid {
    if (targetId.isEmpty) return false;
    if (adminId.isEmpty) return false;
    if (reason.isEmpty) return false;
    if (reason.length > 500) return false;
    return true;
  }

  CreateContentRemovalRequest copyWith({
    String? targetId,
    ReportTargetType? targetType,
    String? targetTitle,
    String? reportId,
    String? reason,
    String? adminId,
  }) {
    return CreateContentRemovalRequest(
      targetId: targetId ?? this.targetId,
      targetType: targetType ?? this.targetType,
      targetTitle: targetTitle ?? this.targetTitle,
      reportId: reportId ?? this.reportId,
      reason: reason ?? this.reason,
      adminId: adminId ?? this.adminId,
    );
  }
}

/// Restore Content Request - DTO untuk restore konten
class RestoreContentRequest {
  final String removalId;
  final String adminId;
  final String? restoreReason;

  const RestoreContentRequest({
    required this.removalId,
    required this.adminId,
    this.restoreReason,
  });

  /// Validate request
  bool get isValid {
    if (removalId.isEmpty) return false;
    if (adminId.isEmpty) return false;
    if (restoreReason != null && restoreReason!.length > 500) return false;
    return true;
  }

  RestoreContentRequest copyWith({
    String? removalId,
    String? adminId,
    String? restoreReason,
  }) {
    return RestoreContentRequest(
      removalId: removalId ?? this.removalId,
      adminId: adminId ?? this.adminId,
      restoreReason: restoreReason ?? this.restoreReason,
    );
  }
}
