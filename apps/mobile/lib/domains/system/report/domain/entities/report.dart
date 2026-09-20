/// Report Domain Entity
///
/// Pure domain entity for content reporting functionality.
/// Canonical model: Report has NO persisted status of its own.
/// Presentation state is derived strictly from Case.status + Decision.outcome.
library;

// =====================
// Enums
// =====================

/// Report Target Type - Jenis konten/user yang bisa dilaporkan
///
/// Canonical targets: content, comment, for_sale, auction, user.
enum ReportTargetType {
  content,
  comment,
  user,
  forSale,
  auction,
}

/// Report Reason Code - Alasan pelaporan (backend-owned locked taxonomy)
enum ReportReasonType {
  scamOrFraud,
  prohibitedContent,
  harassmentOrAbuse,
  impersonation,
  misleadingInformation,
  commerceViolation,
  other,
}

/// Presentation Display State (UI-only derivation, NEVER persisted)
///
/// Derived from correlated Case + Decision state:
/// - open + no decision → underReview
/// - resolved + no_violation → reviewedNoViolation
/// - resolved + violation → reviewedViolation
/// - fallback (no case info yet) → submitted
enum ReportDisplayState {
  submitted,
  underReview,
  reviewedNoViolation,
  reviewedViolation,
}

// =====================
// Extensions
// =====================

extension ReportTargetTypeExtension on ReportTargetType {
  String get value => name;

  String get backendValue {
    if (this == ReportTargetType.forSale) return 'for_sale';
    return name;
  }

  bool get isBackendSupported => true;
  bool get isV1Supported => true;
  bool get isEnabled => true;

  String get displayName {
    switch (this) {
      case ReportTargetType.user:
        return 'User';
      case ReportTargetType.content:
        return 'Content';
      case ReportTargetType.forSale:
        return 'For Sale';
      case ReportTargetType.auction:
        return 'Auction';
      case ReportTargetType.comment:
        return 'Comment';
    }
  }

  static ReportTargetType fromString(String value) {
    return ReportTargetType.values.firstWhere(
      (e) => e.name == value || e.backendValue == value,
      orElse: () => ReportTargetType.user,
    );
  }
}

extension ReportReasonTypeExtension on ReportReasonType {
  String get value => name;

  String get backendValue {
    switch (this) {
      case ReportReasonType.scamOrFraud:
        return 'scam_or_fraud';
      case ReportReasonType.prohibitedContent:
        return 'prohibited_content';
      case ReportReasonType.harassmentOrAbuse:
        return 'harassment_or_abuse';
      case ReportReasonType.impersonation:
        return 'impersonation';
      case ReportReasonType.misleadingInformation:
        return 'misleading_information';
      case ReportReasonType.commerceViolation:
        return 'commerce_violation';
      case ReportReasonType.other:
        return 'other';
    }
  }

  String get displayName {
    switch (this) {
      case ReportReasonType.scamOrFraud:
        return 'Scam / Fraud';
      case ReportReasonType.prohibitedContent:
        return 'Prohibited Content';
      case ReportReasonType.harassmentOrAbuse:
        return 'Harassment / Abuse';
      case ReportReasonType.impersonation:
        return 'Impersonation';
      case ReportReasonType.misleadingInformation:
        return 'Misleading Information';
      case ReportReasonType.commerceViolation:
        return 'Commerce Violation';
      case ReportReasonType.other:
        return 'Other';
    }
  }

  String get description {
    switch (this) {
      case ReportReasonType.scamOrFraud:
        return 'Fraud attempts or suspicious activity';
      case ReportReasonType.prohibitedContent:
        return 'Content that violates platform rules';
      case ReportReasonType.harassmentOrAbuse:
        return 'Intimidating or harassing behavior';
      case ReportReasonType.impersonation:
        return 'Pretending to be someone else';
      case ReportReasonType.misleadingInformation:
        return 'Misleading or false information';
      case ReportReasonType.commerceViolation:
        return 'Violates commerce rules';
      case ReportReasonType.other:
        return 'Other reasons not listed above';
    }
  }

  static ReportReasonType fromString(String value) {
    return ReportReasonType.values.firstWhere(
      (e) => e.name == value || e.backendValue == value,
      orElse: () => ReportReasonType.other,
    );
  }
}

extension ReportDisplayStateExtension on ReportDisplayState {
  String get displayName {
    switch (this) {
      case ReportDisplayState.submitted:
        return 'Submitted';
      case ReportDisplayState.underReview:
        return 'Under Review';
      case ReportDisplayState.reviewedNoViolation:
        return 'No Violation Found';
      case ReportDisplayState.reviewedViolation:
        return 'Violation Confirmed';
    }
  }
}

// =====================
// User-Facing Projections
// =====================

/// User-facing Case projection
class ReportCaseProjection {
  final String id;
  final String status; // open | resolved
  final DateTime createdAt;
  final DateTime? closedAt;

  const ReportCaseProjection({
    required this.id,
    required this.status,
    required this.createdAt,
    this.closedAt,
  });

  bool get isOpen => status == 'open';
  bool get isResolved => status == 'resolved';
}

/// User-facing Decision projection
class ReportDecisionProjection {
  final String outcome; // no_violation | violation
  final DateTime createdAt;

  const ReportDecisionProjection({
    required this.outcome,
    required this.createdAt,
  });

  bool get isViolation => outcome == 'violation';
  bool get isNoViolation => outcome == 'no_violation';
}

/// User-facing Target projection
class ReportTargetProjection {
  final String subjectType;
  final String subjectId;
  final String title;

  const ReportTargetProjection({
    required this.subjectType,
    required this.subjectId,
    required this.title,
  });
}

// =====================
// Entity
// =====================

/// Report Entity - Immutable domain intake record
class Report {
  final String id;
  final String reporterId;
  final String subjectId;
  final ReportTargetType subjectType;
  final ReportReasonType reason;
  final String? description;
  final String? caseId;
  final DateTime createdAt;

  final ReportCaseProjection? caseProjection;
  final ReportDecisionProjection? decisionProjection;
  final ReportTargetProjection? targetProjection;

  const Report({
    required this.id,
    required this.reporterId,
    required this.subjectId,
    required this.subjectType,
    required this.reason,
    this.description,
    this.caseId,
    required this.createdAt,
    this.caseProjection,
    this.decisionProjection,
    this.targetProjection,
  });

  /// Derived display state for UI consumption.
  /// Never persisted — calculated on the fly.
  ReportDisplayState get displayState {
    if (caseProjection == null) {
      return ReportDisplayState.submitted;
    }
    if (caseProjection!.isOpen) {
      return ReportDisplayState.underReview;
    }
    if (caseProjection!.isResolved) {
      if (decisionProjection?.isViolation == true) {
        return ReportDisplayState.reviewedViolation;
      }
      return ReportDisplayState.reviewedNoViolation;
    }
    return ReportDisplayState.submitted;
  }

  /// Title for display (uses target title if present, otherwise target type name)
  String get targetTitle => targetProjection?.title ?? subjectType.displayName;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is Report && runtimeType == other.runtimeType && id == other.id;

  @override
  int get hashCode => id.hashCode;
}

/// Create Report Request
class CreateReportRequest {
  final String subjectId;
  final ReportTargetType subjectType;
  final String? targetTitle;
  final ReportReasonType reason;
  final String? description;

  const CreateReportRequest({
    required this.subjectId,
    required this.subjectType,
    this.targetTitle,
    required this.reason,
    this.description,
  });

  bool get isValid {
    if (subjectId.isEmpty) return false;
    if (description != null && description!.length > 2000) return false;
    return subjectType.isBackendSupported;
  }
}
