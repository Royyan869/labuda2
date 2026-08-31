/// Report DTOs for API Integration
///
/// These models match the canonical Go backend /reports endpoints (SLICE 2).
library;

// =====================
// Report DTOs
// =====================

/// Report DTO from backend.
///
/// Backend contract (POST /reports → 201, GET /reports/mine, GET /reports/:id):
///   {id, reporter_id, subject_type, subject_id, reason_code,
///    reason_note?, evidence_snapshot?, case_id?, created_at}
class ReportDto {
  final String id;
  final String reporterId;
  final String subjectType;
  final String subjectId;
  final String reasonCode;
  final String? reasonNote;
  final DateTime createdAt;

  const ReportDto({
    required this.id,
    required this.reporterId,
    required this.subjectType,
    required this.subjectId,
    required this.reasonCode,
    this.reasonNote,
    required this.createdAt,
  });

  /// Parse from POST /reports 201 response or GET /reports/:id response.
  factory ReportDto.fromJson(Map<String, dynamic> json) {
    return ReportDto(
      id: json['id'] as String,
      reporterId: json['reporter_id'] as String,
      subjectType: json['subject_type'] as String,
      subjectId: json['subject_id'] as String,
      reasonCode: json['reason_code'] as String,
      reasonNote: json['reason_note'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}

/// Request DTO for POST /reports.
///
/// Backend contract: {subject_type, subject_id, reason_code, reason_note?}
/// - subject_type: "content" | "comment" | "for_sale" | "auction" | "user"
/// - subject_id: UUID of the entity being reported
/// - reason_code: locked taxonomy value
/// - reason_note: optional free text (1-2000 chars)
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

/// Request to review a report
class ReviewReportRequestDto {
  final String action;
  final String? note;

  const ReviewReportRequestDto({required this.action, this.note});

  Map<String, dynamic> toJson() => {
    'action': action,
    if (note != null) 'note': note,
  };
}
