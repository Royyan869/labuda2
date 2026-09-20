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
  static Report toEntity(ReportDto dto) {
    return Report(
      id: dto.id,
      reporterId: dto.reporterId,
      subjectId: dto.subjectId,
      subjectType: ReportTargetTypeExtension.fromString(dto.subjectType),
      reason: ReportReasonTypeExtension.fromString(dto.reasonCode),
      description: dto.reasonNote,
      caseId: dto.caseId,
      createdAt: dto.createdAt,
      caseProjection: dto.caseProjection != null
          ? ReportCaseProjection(
              id: dto.caseProjection!.id,
              status: dto.caseProjection!.status,
              createdAt: dto.caseProjection!.createdAt,
              closedAt: dto.caseProjection!.closedAt,
            )
          : null,
      decisionProjection: dto.decisionProjection != null
          ? ReportDecisionProjection(
              outcome: dto.decisionProjection!.outcome,
              createdAt: dto.decisionProjection!.createdAt,
            )
          : null,
      targetProjection: dto.targetProjection != null
          ? ReportTargetProjection(
              subjectType: dto.targetProjection!.subjectType,
              subjectId: dto.targetProjection!.subjectId,
              title: dto.targetProjection!.title,
            )
          : null,
    );
  }

  /// Map CreateReportRequest to backend CreateReportRequestDto.
  static CreateReportRequestDto toCreateRequestDto(CreateReportRequest request) {
    return CreateReportRequestDto(
      subjectType: request.subjectType.backendValue,
      subjectId: request.subjectId,
      reasonCode: request.reason.backendValue,
      reasonNote: request.description,
    );
  }
}

// =====================
// Appeal Mapper
// =====================

/// Mapper for Appeal entity
class AppealMapper {
  static Appeal toEntity(AppealDto dto) {
    return Appeal(
      id: dto.id,
      userId: '',
      appealType: AppealType.contentRemoval,
      sourceId: dto.decisionId,
      reason: dto.message,
      evidenceDescription: null,
      evidenceUrls: const [],
      status: _mapAppealStatusString(dto.status),
      submittedAt: dto.createdAt,
      reviewerId: dto.reviewedBy,
      reviewerName: dto.reviewedBy,
      reviewedAt: dto.reviewedAt,
      reviewNote: dto.adminResponse,
      decision: null,
    );
  }

  static CreateAppealRequestDto toCreateRequestDto(
    CreateAppealRequest request,
  ) {
    return CreateAppealRequestDto(
      decisionId: request.sourceId ?? '',
      message: request.reason,
    );
  }

  static ReviewAppealRequestDto toReviewRequestDto(
    ReviewAppealRequest request,
  ) {
    return ReviewAppealRequestDto(
      decision: request.decision.value,
      adminResponse: request.reviewNote,
    );
  }

  static AppealStatus _mapAppealStatusString(String value) {
    return AppealStatusExtension.fromString(value);
  }
}

// =====================
// Warning Mapper
// =====================

/// Mapper for UserWarning entity
class WarningMapper {
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

  static WarningLevel _mapLevelString(String value) {
    return WarningLevelExtension.fromString(value);
  }

  static WarningStatus _mapWarningStatusString(String value) {
    return WarningStatusExtension.fromString(value);
  }
}
