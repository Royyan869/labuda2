/// Report Domain Entity
///
/// Pure domain entity for content reporting functionality.
/// Contains all business logic and properties related to user reports.
library;

// =====================
// Enums
// =====================

/// Report Target Type - Jenis konten/user yang bisa dilaporkan
///
/// **Canonical targets (backend contract POST /reports):**
/// content, comment, for_sale, auction, user.
///
/// `chat_message` and `fixed_price_sale` are NOT canonical moderation targets
/// and are rejected by the backend (LABUDA — CANONICAL MODERATION SPEC v1 §12).
enum ReportTargetType {
  content,
  comment,
  user,
  forSale,
  auction,
}

/// Report Reason Code - Alasan pelaporan (backend-owned locked taxonomy)
///
/// Backend contract: reason_code must be one of:
/// scam_or_fraud, prohibited_content, harassment_or_abuse, impersonation,
/// misleading_information, commerce_violation, other.
enum ReportReasonType {
  scamOrFraud,
  prohibitedContent,
  harassmentOrAbuse,
  impersonation,
  misleadingInformation,
  commerceViolation,
  other,
}

/// Report Status - Status laporan (UI display)
///
/// Canonical Report is an immutable historical intake record; it has no
/// decision/enforcement status of its own. The canonical backend contract
/// does not carry a mutable report status — these values remain for UI
/// display and are populated from Case/Decision state in a later slice.
enum ReportStatus { pending, underReview, approved, rejected, resolved }

/// Report Action - Tindakan yang diambil moderator
enum ReportAction {
  none,
  warning,
  contentRemoved,
  userSuspended,
  userBanned,
  dismissed,
}

// =====================
// Extensions
// =====================

extension ReportTargetTypeExtension on ReportTargetType {
  String get value => name;

  /// Backend subject_type string for POST /reports.
  /// Canonical: content | comment | for_sale | auction | user.
  String get backendValue {
    if (this == ReportTargetType.forSale) return 'for_sale';
    return name;
  }

  /// Whether the backend accepts this type via POST /reports.
  /// All canonical targets are supported.
  bool get isBackendSupported => true;

  /// Check if this target type has fully automatic enforcement (soft-delete).
  /// for_sale/auction/user have admin-mediated enforcement via outbox events.
  bool get isV1Supported {
    return this == ReportTargetType.content || this == ReportTargetType.comment;
  }

  /// Whether this type is enabled for the report UI flow.
  bool get isEnabled => isBackendSupported;

  /// Check if this type is reserved for future implementation
  bool get isReserved => !isEnabled;

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

  /// Backend reason_code value for POST /reports (locked taxonomy).
  /// snake_case mapping of the Dart enum names.
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
        return 'Violates commerce / listing rules';
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

extension ReportStatusExtension on ReportStatus {
  String get value => name;

  String get displayName {
    switch (this) {
      case ReportStatus.pending:
        return 'Pending';
      case ReportStatus.underReview:
        return 'Under Review';
      case ReportStatus.approved:
        return 'Approved';
      case ReportStatus.rejected:
        return 'Rejected';
      case ReportStatus.resolved:
        return 'Resolved';
    }
  }

  static ReportStatus fromString(String value) {
    return ReportStatus.values.firstWhere(
      (e) => e.name == value,
      orElse: () => ReportStatus.pending,
    );
  }
}

extension ReportActionExtension on ReportAction {
  String get value => name;

  String get displayName {
    switch (this) {
      case ReportAction.none:
        return 'No Action';
      case ReportAction.warning:
        return 'Warning';
      case ReportAction.contentRemoved:
        return 'Content Removed';
      case ReportAction.userSuspended:
        return 'User Suspended';
      case ReportAction.userBanned:
        return 'User Banned';
      case ReportAction.dismissed:
        return 'Report Dismissed';
    }
  }

  static ReportAction fromString(String value) {
    return ReportAction.values.firstWhere(
      (e) => e.name == value,
      orElse: () => ReportAction.none,
    );
  }
}

// =====================
// Entities
// =====================

/// Report Entity - Domain entity untuk laporan
class Report {
  final String id;
  final String reporterId;
  final String? reporterName;
  final String subjectId;
  final ReportTargetType subjectType;
  final String? targetTitle;
  final ReportReasonType reason;
  final String? description;
  final List<String> evidenceUrls;
  final ReportStatus status;
  final ReportAction action;
  final String? moderatorId;
  final String? moderatorNote;
  final DateTime createdAt;
  final DateTime? reviewedAt;
  final DateTime? resolvedAt;

  const Report({
    required this.id,
    required this.reporterId,
    this.reporterName,
    required this.subjectId,
    required this.subjectType,
    this.targetTitle,
    required this.reason,
    this.description,
    this.evidenceUrls = const [],
    this.status = ReportStatus.pending,
    this.action = ReportAction.none,
    this.moderatorId,
    this.moderatorNote,
    required this.createdAt,
    this.reviewedAt,
    this.resolvedAt,
  });

  /// Check if report is already completed
  bool get isResolved =>
      status == ReportStatus.resolved ||
      status == ReportStatus.approved ||
      status == ReportStatus.rejected;

  /// Check if there is evidence
  bool get hasEvidence => evidenceUrls.isNotEmpty;

  /// Check if report can be reviewed
  bool get canBeReviewed =>
      status == ReportStatus.pending || status == ReportStatus.underReview;

  Report copyWith({
    String? id,
    String? reporterId,
    String? reporterName,
    String? subjectId,
    ReportTargetType? subjectType,
    String? targetTitle,
    ReportReasonType? reason,
    String? description,
    List<String>? evidenceUrls,
    ReportStatus? status,
    ReportAction? action,
    String? moderatorId,
    String? moderatorNote,
    DateTime? createdAt,
    DateTime? reviewedAt,
    DateTime? resolvedAt,
  }) {
    return Report(
      id: id ?? this.id,
      reporterId: reporterId ?? this.reporterId,
      reporterName: reporterName ?? this.reporterName,
      subjectId: subjectId ?? this.subjectId,
      subjectType: subjectType ?? this.subjectType,
      targetTitle: targetTitle ?? this.targetTitle,
      reason: reason ?? this.reason,
      description: description ?? this.description,
      evidenceUrls: evidenceUrls ?? this.evidenceUrls,
      status: status ?? this.status,
      action: action ?? this.action,
      moderatorId: moderatorId ?? this.moderatorId,
      moderatorNote: moderatorNote ?? this.moderatorNote,
      createdAt: createdAt ?? this.createdAt,
      reviewedAt: reviewedAt ?? this.reviewedAt,
      resolvedAt: resolvedAt ?? this.resolvedAt,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is Report && runtimeType == other.runtimeType && id == other.id;

  @override
  int get hashCode => id.hashCode;
}

/// Create Report Request - DTO untuk membuat laporan baru
class CreateReportRequest {
  final String subjectId;
  final ReportTargetType subjectType;
  final String? targetTitle;
  final ReportReasonType reason;
  final String? description;
  final List<String> evidenceUrls;

  const CreateReportRequest({
    required this.subjectId,
    required this.subjectType,
    this.targetTitle,
    required this.reason,
    this.description,
    this.evidenceUrls = const [],
  });

  /// Validate request.
  ///
  /// Only canonical backend-supported types pass validation.
  bool get isValid {
    if (subjectId.isEmpty) return false;
    if (description != null && description!.length > 2000) return false;
    if (!subjectType.isBackendSupported) return false;

    return true;
  }

  /// Check if the target type is supported for reporting
  bool get isTargetTypeSupported => subjectType.isBackendSupported;

  CreateReportRequest copyWith({
    String? subjectId,
    ReportTargetType? subjectType,
    String? targetTitle,
    ReportReasonType? reason,
    String? description,
    List<String>? evidenceUrls,
  }) {
    return CreateReportRequest(
      subjectId: subjectId ?? this.subjectId,
      subjectType: subjectType ?? this.subjectType,
      targetTitle: targetTitle ?? this.targetTitle,
      reason: reason ?? this.reason,
      description: description ?? this.description,
      evidenceUrls: evidenceUrls ?? this.evidenceUrls,
    );
  }
}

/// Report Statistics - Statistik laporan untuk admin dashboard
class ReportStatistics {
  final int totalReports;
  final int pendingReports;
  final int underReviewReports;
  final int resolvedReports;
  final Map<ReportReasonType, int> reportsByReason;
  final Map<ReportTargetType, int> reportsByTarget;
  final DateTime generatedAt;

  const ReportStatistics({
    required this.totalReports,
    required this.pendingReports,
    required this.underReviewReports,
    required this.resolvedReports,
    required this.reportsByReason,
    required this.reportsByTarget,
    required this.generatedAt,
  });

  factory ReportStatistics.empty() => ReportStatistics(
    totalReports: 0,
    pendingReports: 0,
    underReviewReports: 0,
    resolvedReports: 0,
    reportsByReason: {},
    reportsByTarget: {},
    generatedAt: DateTime.now(),
  );

  ReportStatistics copyWith({
    int? totalReports,
    int? pendingReports,
    int? underReviewReports,
    int? resolvedReports,
    Map<ReportReasonType, int>? reportsByReason,
    Map<ReportTargetType, int>? reportsByTarget,
    DateTime? generatedAt,
  }) {
    return ReportStatistics(
      totalReports: totalReports ?? this.totalReports,
      pendingReports: pendingReports ?? this.pendingReports,
      underReviewReports: underReviewReports ?? this.underReviewReports,
      resolvedReports: resolvedReports ?? this.resolvedReports,
      reportsByReason: reportsByReason ?? this.reportsByReason,
      reportsByTarget: reportsByTarget ?? this.reportsByTarget,
      generatedAt: generatedAt ?? this.generatedAt,
    );
  }
}

/// Review Report Request - DTO untuk review laporan (admin)
class ReviewReportRequest {
  final String reportId;
  final ReportStatus status;
  final ReportAction action;
  final String moderatorId;
  final String? moderatorNote;

  const ReviewReportRequest({
    required this.reportId,
    required this.status,
    required this.action,
    required this.moderatorId,
    this.moderatorNote,
  });

  /// Validate request
  bool get isValid {
    if (reportId.isEmpty) return false;
    if (moderatorId.isEmpty) return false;
    return true;
  }

  ReviewReportRequest copyWith({
    String? reportId,
    ReportStatus? status,
    ReportAction? action,
    String? moderatorId,
    String? moderatorNote,
  }) {
    return ReviewReportRequest(
      reportId: reportId ?? this.reportId,
      status: status ?? this.status,
      action: action ?? this.action,
      moderatorId: moderatorId ?? this.moderatorId,
      moderatorNote: moderatorNote ?? this.moderatorNote,
    );
  }
}
