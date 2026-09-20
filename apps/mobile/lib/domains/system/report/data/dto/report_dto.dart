/// Report DTOs for API Integration
///
/// Models match the canonical Go backend /reports endpoints.
library;

/// Case projection DTO from user-facing report endpoint.
class ReportCaseProjectionDto {
  final String id;
  final String status;
  final DateTime createdAt;
  final DateTime? closedAt;

  const ReportCaseProjectionDto({
    required this.id,
    required this.status,
    required this.createdAt,
    this.closedAt,
  });

  factory ReportCaseProjectionDto.fromJson(Map<String, dynamic> json) {
    return ReportCaseProjectionDto(
      id: json['id'] as String,
      status: json['status'] as String,
      createdAt: DateTime.parse(json['created_at'] as String),
      closedAt: json['closed_at'] != null ? DateTime.parse(json['closed_at'] as String) : null,
    );
  }
}

/// Decision projection DTO from user-facing report endpoint.
class ReportDecisionProjectionDto {
  final String outcome;
  final DateTime createdAt;

  const ReportDecisionProjectionDto({
    required this.outcome,
    required this.createdAt,
  });

  factory ReportDecisionProjectionDto.fromJson(Map<String, dynamic> json) {
    return ReportDecisionProjectionDto(
      outcome: json['outcome'] as String,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}

/// Safe target projection DTO from user-facing report endpoint.
class ReportTargetProjectionDto {
  final String subjectType;
  final String subjectId;
  final String title;

  const ReportTargetProjectionDto({
    required this.subjectType,
    required this.subjectId,
    required this.title,
  });

  factory ReportTargetProjectionDto.fromJson(Map<String, dynamic> json) {
    return ReportTargetProjectionDto(
      subjectType: json['subject_type'] as String,
      subjectId: json['subject_id'] as String,
      title: json['title'] as String,
    );
  }
}

/// Report DTO from backend.
///
/// Backend contract (POST /reports → 201, GET /reports/mine, GET /reports/:id):
///   {id, reporter_id, subject_type, subject_id, reason_code, reason_note?, case_id?, created_at, case?, decision?, target?}
class ReportDto {
  final String id;
  final String reporterId;
  final String subjectType;
  final String subjectId;
  final String reasonCode;
  final String? reasonNote;
  final String? caseId;
  final DateTime createdAt;
  final ReportCaseProjectionDto? caseProjection;
  final ReportDecisionProjectionDto? decisionProjection;
  final ReportTargetProjectionDto? targetProjection;

  const ReportDto({
    required this.id,
    required this.reporterId,
    required this.subjectType,
    required this.subjectId,
    required this.reasonCode,
    this.reasonNote,
    this.caseId,
    required this.createdAt,
    this.caseProjection,
    this.decisionProjection,
    this.targetProjection,
  });

  factory ReportDto.fromJson(Map<String, dynamic> json) {
    return ReportDto(
      id: json['id'] as String,
      reporterId: json['reporter_id'] as String? ?? '',
      subjectType: json['subject_type'] as String,
      subjectId: json['subject_id'] as String,
      reasonCode: json['reason_code'] as String,
      reasonNote: json['reason_note'] as String?,
      caseId: json['case_id'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
      caseProjection: json['case'] != null ? ReportCaseProjectionDto.fromJson(json['case'] as Map<String, dynamic>) : null,
      decisionProjection: json['decision'] != null ? ReportDecisionProjectionDto.fromJson(json['decision'] as Map<String, dynamic>) : null,
      targetProjection: json['target'] != null ? ReportTargetProjectionDto.fromJson(json['target'] as Map<String, dynamic>) : null,
    );
  }
}

/// Request DTO for POST /reports.
class CreateReportRequestDto {
  final String subjectType;
  final String subjectId;
  final String reasonCode;
  final String? reasonNote;

  const CreateReportRequestDto({
    required this.subjectType,
    required this.subjectId,
    required this.reasonCode,
    this.reasonNote,
  });

  Map<String, dynamic> toJson() => {
    'subject_type': subjectType,
    'subject_id': subjectId,
    'reason_code': reasonCode,
    if (reasonNote != null && reasonNote!.isNotEmpty) 'reason_note': reasonNote,
  };
}
