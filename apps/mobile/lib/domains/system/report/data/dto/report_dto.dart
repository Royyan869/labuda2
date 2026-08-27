/// Report DTOs for API Integration
///
/// These models match the Go backend moderation/cases endpoints.
library;

// =====================
// Report DTOs
// =====================

/// Moderation case DTO from backend.
///
/// Backend contract (POST /moderation/cases → 201):
///   {case_id, resource_type, resource_id, status, created_at}
///
/// Backend contract (GET /moderation/my-cases, GET /moderation/cases/:id):
///   {id, resource_type, resource_id, status, reported_by, reason,
///    created_at, reviewed_by?, decision_note?, reviewed_at?}
class ModerationCaseDto {
  final String id;
  final String resourceType;
  final String resourceId;
  final String status;
  final String? reportedBy;
  final String? reason;
  final DateTime createdAt;
  final String? reviewedBy;
  final String? decisionNote;
  final DateTime? reviewedAt;

  const ModerationCaseDto({
    required this.id,
    required this.resourceType,
    required this.resourceId,
    required this.status,
    this.reportedBy,
    this.reason,
    required this.createdAt,
    this.reviewedBy,
    this.decisionNote,
    this.reviewedAt,
  });

  /// Parse from POST /moderation/cases 201 response.
  /// Create response uses "case_id" key, not "id".
  factory ModerationCaseDto.fromCreateJson(Map<String, dynamic> json) {
    return ModerationCaseDto(
      id: json['case_id'] as String,
      resourceType: json['resource_type'] as String,
      resourceId: json['resource_id'] as String,
      status: json['status'] as String,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  /// Parse from GET /moderation/my-cases or GET /moderation/cases/:id response.
  factory ModerationCaseDto.fromJson(Map<String, dynamic> json) {
    return ModerationCaseDto(
      id: json['id'] as String,
      resourceType: json['resource_type'] as String,
      resourceId: json['resource_id'] as String,
      status: json['status'] as String,
      reportedBy: json['reported_by'] as String?,
      reason: json['reason'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
      reviewedBy: json['reviewed_by'] as String?,
      decisionNote: json['decision_note'] as String?,
      reviewedAt: json['reviewed_at'] != null
          ? DateTime.parse(json['reviewed_at'] as String)
          : null,
    );
  }
}

/// Request DTO for POST /moderation/cases.
///
/// Backend contract: {entity_type, entity_id, reason}
/// - entity_type: "content" | "comment" | "user" (V1 supported)
/// - entity_id: UUID of the entity being reported
/// - reason: description text (1-500 chars)
class CreateCaseRequestDto {
  final String entityType;
  final String entityId;
  final String reason;

  const CreateCaseRequestDto({
    required this.entityType,
    required this.entityId,
    required this.reason,
  });

  Map<String, dynamic> toJson() => {
    'entity_type': entityType,
    'entity_id': entityId,
    'reason': reason,
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
