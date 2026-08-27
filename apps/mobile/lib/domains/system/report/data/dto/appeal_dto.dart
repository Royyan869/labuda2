/// Appeal DTOs for API Integration
///
/// These models match the Go backend API contract.
/// Backend fields: id, case_id, status, message, created_at,
///                 admin_response?, reviewed_by?, reviewed_at?
library;

// =====================
// Appeal DTOs
// =====================

/// Appeal DTO from API — matches appealToResponse shape in appeal_handler.go
class AppealDto {
  final String id;
  final String caseId;
  final String status;
  final String message;
  final DateTime createdAt;
  final String? adminResponse;
  final String? reviewedBy;
  final DateTime? reviewedAt;

  const AppealDto({
    required this.id,
    required this.caseId,
    required this.status,
    required this.message,
    required this.createdAt,
    this.adminResponse,
    this.reviewedBy,
    this.reviewedAt,
  });

  factory AppealDto.fromJson(Map<String, dynamic> json) {
    return AppealDto(
      id: json['id'] as String,
      caseId: json['case_id'] as String,
      status: json['status'] as String,
      message: json['message'] as String,
      createdAt: DateTime.parse(json['created_at'] as String),
      adminResponse: json['admin_response'] as String?,
      reviewedBy: json['reviewed_by'] as String?,
      reviewedAt: json['reviewed_at'] != null
          ? DateTime.parse(json['reviewed_at'] as String)
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'case_id': caseId,
    'status': status,
    'message': message,
    'created_at': createdAt.toIso8601String(),
    if (adminResponse != null) 'admin_response': adminResponse,
    if (reviewedBy != null) 'reviewed_by': reviewedBy,
    if (reviewedAt != null) 'reviewed_at': reviewedAt!.toIso8601String(),
  };
}

/// Request to create an appeal — matches backend CreateAppealRequest:
///   case_id (required, uuid): the moderation case being appealed
///   message (required, 1–2000 chars): user's explanation
class CreateAppealRequestDto {
  final String caseId;
  final String message;

  const CreateAppealRequestDto({required this.caseId, required this.message});

  Map<String, dynamic> toJson() => {'case_id': caseId, 'message': message};
}

/// Request to review an appeal (admin only) — matches backend ReviewAppealRequest:
///   decision (required, one of: approve/reject/approved/rejected)
///   admin_response (optional, max 2000 chars)
class ReviewAppealRequestDto {
  final String decision;
  final String? adminResponse;

  const ReviewAppealRequestDto({required this.decision, this.adminResponse});

  Map<String, dynamic> toJson() => {
    'decision': decision,
    if (adminResponse != null) 'admin_response': adminResponse,
  };
}
