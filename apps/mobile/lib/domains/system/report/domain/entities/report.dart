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
/// **All 6 types are backend-supported via POST /moderation/cases.**
/// Backend resource types: content, comment, for_sale, auction, user, chat_message.
///
/// Enforcement per type:
///   content / comment -> soft-delete (automatic on admin enforce)
///   for_sale          -> Withdraw() via moderation event (admin enforce)
///   auction           -> CancelForModeration() via moderation event (admin enforce)
///   user              -> account_status='suspended' (admin enforce)
///   message           -> SoftHideForModeration() for chat_message (admin enforce)
enum ReportTargetType {
  content,
  comment,
  user,
  message, // Backend value: chat_message
  forSale,
  auction,
}

/// Report Reason Type - Alasan pelaporan
enum ReportReasonType {
  spam,
  harassment,
  inappropriateContent,
  scam,
  fakeProduct,
  copyrightViolation,
  violence,
  hateSpeech,
  falseInformation,
  other,
}

/// Report Status - Status laporan
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

  /// Backend entity_type string for POST /moderation/cases.
  /// Only valid for backend-supported types.
  String get backendValue {
    if (this == ReportTargetType.message) return 'chat_message';
    if (this == ReportTargetType.forSale) return 'for_sale';
    return name;
  }

  /// Whether the backend accepts this type via POST /moderation/cases.
  /// All 6 types are supported. Backend moderation_resource_enum includes:
  /// content, comment, for_sale, auction, user, chat_message.
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
      case ReportTargetType.message:
        return 'Message';
    }
  }

  static ReportTargetType fromString(String value) {
    return ReportTargetType.values.firstWhere(
      (e) => e.name == value,
      orElse: () => ReportTargetType.user,
    );
  }
}

extension ReportReasonTypeExtension on ReportReasonType {
  String get value => name;

  String get displayName {
    switch (this) {
      case ReportReasonType.spam:
        return 'Spam';
      case ReportReasonType.harassment:
        return 'Harassment / Bullying';
      case ReportReasonType.inappropriateContent:
        return 'Inappropriate Content';
      case ReportReasonType.scam:
        return 'Scam';
      case ReportReasonType.fakeProduct:
        return 'Fake Product';
      case ReportReasonType.copyrightViolation:
        return 'Copyright Violation';
      case ReportReasonType.violence:
        return 'Violence';
      case ReportReasonType.hateSpeech:
        return 'Hate Speech';
      case ReportReasonType.falseInformation:
        return 'False Information';
      case ReportReasonType.other:
        return 'Other';
    }
  }

  String get description {
    switch (this) {
      case ReportReasonType.spam:
        return 'Repetitive content, excessive promotion, or irrelevant content';
      case ReportReasonType.harassment:
        return 'Intimidating or harassing behavior towards other users';
      case ReportReasonType.inappropriateContent:
        return 'Adult, vulgar, or unsuitable content';
      case ReportReasonType.scam:
        return 'Fraud attempts or suspicious activity';
      case ReportReasonType.fakeProduct:
        return 'Product does not match description or is fake';
      case ReportReasonType.copyrightViolation:
        return 'Using photos or content belonging to others';
      case ReportReasonType.violence:
        return 'Content containing violence';
      case ReportReasonType.hateSpeech:
        return 'Attacking specific groups or individuals';
      case ReportReasonType.falseInformation:
        return 'Misleading or false information';
      case ReportReasonType.other:
        return 'Other reasons not listed above';
    }
  }

  static ReportReasonType fromString(String value) {
    return ReportReasonType.values.firstWhere(
      (e) => e.name == value,
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
  final String targetId;
  final ReportTargetType targetType;
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
    required this.targetId,
    required this.targetType,
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
    String? targetId,
    ReportTargetType? targetType,
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
      targetId: targetId ?? this.targetId,
      targetType: targetType ?? this.targetType,
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
  final String targetId;
  final ReportTargetType targetType;
  final String? targetTitle;
  final ReportReasonType reason;
  final String? description;
  final List<String> evidenceUrls;

  const CreateReportRequest({
    required this.targetId,
    required this.targetType,
    this.targetTitle,
    required this.reason,
    this.description,
    this.evidenceUrls = const [],
  });

  /// Validate request.
  ///
  /// Only backend-supported types (content, comment, user) pass validation.
  bool get isValid {
    if (targetId.isEmpty) return false;
    if (description != null && description!.length > 500) return false;
    if (!targetType.isBackendSupported) return false;

    return true;
  }

  /// Check if the target type is supported for reporting
  bool get isTargetTypeSupported => targetType.isBackendSupported;

  CreateReportRequest copyWith({
    String? targetId,
    ReportTargetType? targetType,
    String? targetTitle,
    ReportReasonType? reason,
    String? description,
    List<String>? evidenceUrls,
  }) {
    return CreateReportRequest(
      targetId: targetId ?? this.targetId,
      targetType: targetType ?? this.targetType,
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
    // Can only approve/reject/resolve from pending or under_review
    if (status == ReportStatus.approved ||
        status == ReportStatus.rejected ||
        status == ReportStatus.resolved) {
      return action != ReportAction.none;
    }
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
