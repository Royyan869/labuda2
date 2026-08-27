/// Appeal Domain Entity
///
/// Pure domain entity for appeal functionality.
/// Contains all business logic and properties related to user appeals.
library;

// =====================
// Enums
// =====================

/// Type of appeal
///
/// V1 NOTE: The backend does not use AppealType — the resource_type is inferred
/// from the moderation case. These enum values are local UI scaffolding only.
/// Do not send to backend.
enum AppealType { warning, suspension, ban, contentRemoval, penalty }

/// Appeal status
enum AppealStatus { pending, underReview, approved, rejected, cancelled }

/// Appeal decision
///
/// V1 NOTE: Backend V1 returns only "approved" or "rejected" (AppealStatus).
/// AppealDecision is future scaffolding — not returned by any V1 backend endpoint.
enum AppealDecision {
  upheld, // Original action maintained
  reversed, // Action reversed
  modified, // Action modified (e.g., reduced suspension)
}

// =====================
// Extensions
// =====================

extension AppealTypeExtension on AppealType {
  String get value => name;

  String get displayName {
    switch (this) {
      case AppealType.warning:
        return 'Warning Appeal';
      case AppealType.suspension:
        return 'Suspension Appeal';
      case AppealType.ban:
        return 'Ban Appeal';
      case AppealType.contentRemoval:
        return 'Content Removal Appeal';
      case AppealType.penalty:
        return 'Penalty Appeal';
    }
  }

  String get description {
    switch (this) {
      case AppealType.warning:
        return 'Submit appeal for issued warning';
      case AppealType.suspension:
        return 'Submit appeal for account suspension';
      case AppealType.ban:
        return 'Submit appeal for permanent ban';
      case AppealType.contentRemoval:
        return 'Submit appeal for content removal';
      case AppealType.penalty:
        return 'Submit appeal for trust score reduction';
    }
  }

  static AppealType fromString(String value) {
    return AppealType.values.firstWhere(
      (e) => e.name == value,
      orElse: () => AppealType.warning,
    );
  }
}

extension AppealStatusExtension on AppealStatus {
  String get value => name;

  String get displayName {
    switch (this) {
      case AppealStatus.pending:
        return 'Pending';
      case AppealStatus.underReview:
        return 'Under Review';
      case AppealStatus.approved:
        return 'Approved';
      case AppealStatus.rejected:
        return 'Rejected';
      case AppealStatus.cancelled:
        return 'Cancelled';
    }
  }

  bool get isResolved =>
      this == AppealStatus.approved ||
      this == AppealStatus.rejected ||
      this == AppealStatus.cancelled;

  bool get isActive =>
      this == AppealStatus.pending || this == AppealStatus.underReview;

  static AppealStatus fromString(String value) {
    return AppealStatus.values.firstWhere(
      (e) => e.name == value,
      orElse: () => AppealStatus.pending,
    );
  }
}

extension AppealDecisionExtension on AppealDecision {
  String get value => name;

  String get displayName {
    switch (this) {
      case AppealDecision.upheld:
        return 'Decision Upheld';
      case AppealDecision.reversed:
        return 'Decision Reversed';
      case AppealDecision.modified:
        return 'Decision Modified';
    }
  }

  static AppealDecision fromString(String value) {
    return AppealDecision.values.firstWhere(
      (e) => e.name == value,
      orElse: () => AppealDecision.upheld,
    );
  }
}

// =====================
// Entities
// =====================

/// Appeal Entity - User appeal for moderation actions
class Appeal {
  final String id;
  final String userId;
  final AppealType appealType;
  final String? sourceId; // warningId, removalId, etc.
  final String reason;
  final String? evidenceDescription;
  final List<String> evidenceUrls;
  final AppealStatus status;
  final DateTime submittedAt;
  final String? reviewerId;
  final String? reviewerName;
  final DateTime? reviewedAt;
  final String? reviewNote;
  // V1 NOTE: decision field not returned by backend V1. Backend returns
  // appeal status (approved/rejected) instead. Reserved for future use.
  final AppealDecision? decision;

  const Appeal({
    required this.id,
    required this.userId,
    required this.appealType,
    this.sourceId,
    required this.reason,
    this.evidenceDescription,
    this.evidenceUrls = const [],
    this.status = AppealStatus.pending,
    required this.submittedAt,
    this.reviewerId,
    this.reviewerName,
    this.reviewedAt,
    this.reviewNote,
    this.decision,
  });

  /// Check if appeal can be cancelled
  bool get canBeCancelled => status == AppealStatus.pending;

  /// Check if appeal is resolved
  bool get isResolved => status.isResolved;

  /// Check if appeal is active
  bool get isActive => status.isActive;

  /// Check if appeal was approved
  bool get isApproved => status == AppealStatus.approved;

  /// Check if appeal was rejected
  bool get isRejected => status == AppealStatus.rejected;

  /// Check if has evidence
  bool get hasEvidence =>
      evidenceUrls.isNotEmpty || evidenceDescription != null;

  Appeal copyWith({
    String? id,
    String? userId,
    AppealType? appealType,
    String? sourceId,
    String? reason,
    String? evidenceDescription,
    List<String>? evidenceUrls,
    AppealStatus? status,
    DateTime? submittedAt,
    String? reviewerId,
    String? reviewerName,
    DateTime? reviewedAt,
    String? reviewNote,
    AppealDecision? decision,
  }) {
    return Appeal(
      id: id ?? this.id,
      userId: userId ?? this.userId,
      appealType: appealType ?? this.appealType,
      sourceId: sourceId ?? this.sourceId,
      reason: reason ?? this.reason,
      evidenceDescription: evidenceDescription ?? this.evidenceDescription,
      evidenceUrls: evidenceUrls ?? this.evidenceUrls,
      status: status ?? this.status,
      submittedAt: submittedAt ?? this.submittedAt,
      reviewerId: reviewerId ?? this.reviewerId,
      reviewerName: reviewerName ?? this.reviewerName,
      reviewedAt: reviewedAt ?? this.reviewedAt,
      reviewNote: reviewNote ?? this.reviewNote,
      decision: decision ?? this.decision,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is Appeal && runtimeType == other.runtimeType && id == other.id;

  @override
  int get hashCode => id.hashCode;
}

/// Create Appeal Request - DTO untuk membuat appeal baru
class CreateAppealRequest {
  final String userId;
  final AppealType appealType;
  final String? sourceId;
  final String reason;
  final String? evidenceDescription;
  final List<String> evidenceUrls;

  const CreateAppealRequest({
    required this.userId,
    required this.appealType,
    this.sourceId,
    required this.reason,
    this.evidenceDescription,
    this.evidenceUrls = const [],
  });

  /// Validate request
  bool get isValid {
    if (userId.isEmpty) return false;
    if (reason.isEmpty) return false;
    if (reason.length > 1000) return false;
    if (evidenceDescription != null && evidenceDescription!.length > 1000) {
      return false;
    }
    if (evidenceUrls.length > 5) return false;
    return true;
  }

  CreateAppealRequest copyWith({
    String? userId,
    AppealType? appealType,
    String? sourceId,
    String? reason,
    String? evidenceDescription,
    List<String>? evidenceUrls,
  }) {
    return CreateAppealRequest(
      userId: userId ?? this.userId,
      appealType: appealType ?? this.appealType,
      sourceId: sourceId ?? this.sourceId,
      reason: reason ?? this.reason,
      evidenceDescription: evidenceDescription ?? this.evidenceDescription,
      evidenceUrls: evidenceUrls ?? this.evidenceUrls,
    );
  }
}

/// Review Appeal Request - DTO untuk review appeal (admin)
class ReviewAppealRequest {
  final String appealId;
  final AppealStatus status;
  final AppealDecision decision;
  final String reviewerId;
  final String reviewerName;
  final String? reviewNote;

  const ReviewAppealRequest({
    required this.appealId,
    required this.status,
    required this.decision,
    required this.reviewerId,
    required this.reviewerName,
    this.reviewNote,
  });

  /// Validate request
  bool get isValid {
    if (appealId.isEmpty) return false;
    if (reviewerId.isEmpty) return false;
    if (reviewerName.isEmpty) return false;
    // Can only approve/reject from pending or under_review
    if (!status.isResolved) return false;
    if (decision == AppealDecision.upheld && status == AppealStatus.approved) {
      return false; // Can't uphold AND approve
    }
    return true;
  }

  ReviewAppealRequest copyWith({
    String? appealId,
    AppealStatus? status,
    AppealDecision? decision,
    String? reviewerId,
    String? reviewerName,
    String? reviewNote,
  }) {
    return ReviewAppealRequest(
      appealId: appealId ?? this.appealId,
      status: status ?? this.status,
      decision: decision ?? this.decision,
      reviewerId: reviewerId ?? this.reviewerId,
      reviewerName: reviewerName ?? this.reviewerName,
      reviewNote: reviewNote ?? this.reviewNote,
    );
  }
}
