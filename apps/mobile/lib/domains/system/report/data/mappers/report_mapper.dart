/// Report Mapper
///
/// Maps between domain entities and DTOs.
library;

import '../../domain/entities/entities.dart';
import '../dto/dto.dart';

// =====================
// Report Mapper
// =====================

/// Mapper for Report entity
class ReportMapper {
  /// Map backend ReportDto to domain Report entity.
  ///
  /// Backend returns subject_type / subject_id / reason_code / reason_note.
  static Report toEntity(ReportDto dto) {
    return Report(
      id: dto.id,
      reporterId: dto.reporterId,
      subjectId: dto.subjectId,
      subjectType: ReportTargetTypeExtension.fromString(dto.subjectType),
      reason: ReportReasonTypeExtension.fromString(dto.reasonCode),
      description: dto.reasonNote,
      evidenceUrls: const [],
      status: ReportStatus.pending,
      action: ReportAction.none,
      createdAt: dto.createdAt,
    );
  }

  /// Map CreateReportRequest to backend CreateReportRequestDto.
  ///
  /// Serializes:
  ///   subjectType → subject_type (content/comment/for_sale/auction/user)
  ///   subjectId   → subject_id
  ///   reason      → reason_code (locked taxonomy)
  ///   description → reason_note (optional free text)
  static CreateReportRequestDto toCreateRequestDto(CreateReportRequest request) {
    return CreateReportRequestDto(
      subjectType: request.subjectType.backendValue,
      subjectId: request.subjectId,
      reasonCode: request.reason.backendValue,
      reasonNote: request.description,
    );
  }

  /// Map Review Request to DTO
  static ReviewReportRequestDto toReviewRequestDto(
    ReviewReportRequest request,
  ) {
    return ReviewReportRequestDto(
      action: _actionToString(request.action),
      note: request.moderatorNote,
    );
  }

  // =====================
  // Private Helpers
  // =====================

  static String _actionToString(ReportAction action) {
    return action.value;
  }
}

// =====================
// Appeal Mapper
// =====================

/// Mapper for Appeal entity
class AppealMapper {
  /// Map DTO to Domain Entity.
  ///
  /// Backend contract (V1): {id, case_id, status, message, created_at,
  ///                          admin_response?, reviewed_by?, reviewed_at?}
  /// Domain mapping: caseId→sourceId, message→reason,
  ///                 appealType defaults to contentRemoval (V1 only).
  static Appeal toEntity(AppealDto dto) {
    return Appeal(
      id: dto.id,
      userId: '', // backend create response omits user_id; populated on read
      appealType: AppealType.contentRemoval, // V1: content/comment only
      sourceId: dto.caseId,
      reason: dto.message,
      evidenceDescription: null,
      evidenceUrls: const [],
      status: _mapAppealStatusString(dto.status),
      submittedAt: dto.createdAt,
      reviewerId: dto.reviewedBy,
      reviewerName: dto.reviewedBy,
      reviewedAt: dto.reviewedAt,
      reviewNote: dto.adminResponse,
      decision: null, // backend does not expose a decision enum field
    );
  }

  /// Map Domain Entity to DTO (for creating request).
  ///
  /// Backend contract: {case_id (uuid), message (string)}.
  /// sourceId on CreateAppealRequest holds the moderation case UUID.
  static CreateAppealRequestDto toCreateRequestDto(
    CreateAppealRequest request,
  ) {
    return CreateAppealRequestDto(
      caseId: request.sourceId ?? '',
      message: request.reason,
    );
  }

  /// Map Review Request to DTO (admin only).
  static ReviewAppealRequestDto toReviewRequestDto(
    ReviewAppealRequest request,
  ) {
    return ReviewAppealRequestDto(
      decision: request.decision.value,
      adminResponse: request.reviewNote,
    );
  }

  // =====================
  // Private Helpers
  // =====================

  static AppealStatus _mapAppealStatusString(String value) {
    return AppealStatusExtension.fromString(value);
  }
}

// =====================
// Warning Mapper
// =====================

/// Mapper for UserWarning entity
///
/// Maps between backend DTO contract and domain entity.
/// V1: Contract-aligned, no misleading transformations.
class WarningMapper {
  /// Map DTO to Domain Entity
  ///
  /// Backend fields -> Domain fields:
  /// - id -> id
  /// - user_id -> userId
  /// - level -> level (direct mapping: info/warning/severe)
  /// - reason -> reason
  /// - issued_by -> adminId
  /// - created_at -> createdAt
  /// - is_active -> isActive
  /// - status -> status
  /// - expires_at -> expiresAt
  /// - revoked_at -> revokedAt
  /// - revoked_by -> revokedBy
  /// - adminName (resolved separately from adminId)
  static UserWarning toEntity(UserWarningDto dto, {required String adminName}) {
    return UserWarning(
      id: dto.id,
      userId: dto.userId,
      level: _mapLevelString(dto.level),
      reason: dto.reason,
      adminId: dto.issuedBy,
      adminName: adminName,
      createdAt: dto.createdAt,
      isActive: dto.isActive,
      status: _mapWarningStatusString(dto.status),
      expiresAt: dto.expiresAt,
      revokedAt: dto.revokedAt,
      revokedBy: dto.revokedBy,
    );
  }

  /// Map Domain Entity to DTO (for creating request)
  ///
  /// Matches backend CreateWarningRequest exactly:
  /// - user_id (required, uuid)
  /// - level (required, one of: info, warning, severe)
  /// - reason (required, string 1-500 chars)
  /// - expires_at (optional, Unix timestamp)
  static IssueWarningRequestDto toCreateRequestDto(
    CreateWarningRequest request,
  ) {
    return IssueWarningRequestDto(
      userId: request.userId,
      level: request.level.value, // Direct string mapping
      reason: request.reason,
      expiresAt: request.expiresAtUnix,
    );
  }

  // =====================
  // Private Helpers
  // =====================

  static WarningLevel _mapLevelString(String value) {
    return WarningLevelExtension.fromString(value);
  }

  static WarningStatus _mapWarningStatusString(String value) {
    return WarningStatusExtension.fromString(value);
  }
}

// =====================
// Statistics Mapper
// =====================

/// Mapper for Report Statistics
class ReportStatisticsMapper {
  /// Map API response to Domain Entity
  static ReportStatistics fromJson(Map<String, dynamic> json) {
    final reports = json['reports'] as List<dynamic>? ?? [];
    final byReason = <ReportReasonType, int>{};
    final byTarget = <ReportTargetType, int>{};

    int pending = 0;
    int underReview = 0;
    int resolved = 0;

    for (final report in reports) {
      final data = report as Map<String, dynamic>;
      final statusStr = data['status'] as String? ?? 'pending';
      final reasonStr = data['reason_code'] as String? ?? 'other';
      final targetStr = data['subject_type'] as String? ?? 'content';

      // Count by status
      final status = ReportStatusExtension.fromString(statusStr);
      switch (status) {
        case ReportStatus.pending:
          pending++;
          break;
        case ReportStatus.underReview:
          underReview++;
          break;
        case ReportStatus.resolved:
        case ReportStatus.approved:
        case ReportStatus.rejected:
          resolved++;
          break;
      }

      // Count by reason
      final reason = ReportReasonTypeExtension.fromString(reasonStr);
      byReason[reason] = (byReason[reason] ?? 0) + 1;

      // Count by target type
      final target = ReportTargetTypeExtension.fromString(targetStr);
      byTarget[target] = (byTarget[target] ?? 0) + 1;
    }

    return ReportStatistics(
      totalReports: json['total'] as int? ?? reports.length,
      pendingReports: json['pending'] as int? ?? pending,
      underReviewReports: json['under_review'] as int? ?? underReview,
      resolvedReports: json['resolved'] as int? ?? resolved,
      reportsByReason: byReason,
      reportsByTarget: byTarget,
      generatedAt: DateTime.now(),
    );
  }
}
