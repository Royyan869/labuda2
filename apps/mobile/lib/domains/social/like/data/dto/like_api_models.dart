// Like API Models for Go Backend Integration
//
// These models match the Go backend DTOs in:
// backend/internal/domain/social/dto/*.go
//
// Coverage:
// - Like (handler_like.go)

import 'package:equatable/equatable.dart';

// =============================================================================
// Like Request/Response DTOs
// =============================================================================

/// Request to toggle a like
class LikeToggleApiRequest {
  final String targetId;
  final String targetType;

  const LikeToggleApiRequest({
    required this.targetId,
    required this.targetType,
  });

  Map<String, dynamic> toJson() => {
    'target_id': targetId,
    'target_type': targetType,
  };
}

/// Like statistics response
class LikeStatsApiResponse extends Equatable {
  final String targetId;
  final String targetType;
  final int count;
  final bool isLiked;

  const LikeStatsApiResponse({
    required this.targetId,
    required this.targetType,
    required this.count,
    required this.isLiked,
  });

  factory LikeStatsApiResponse.fromJson(Map<String, dynamic> json) {
    return LikeStatsApiResponse(
      targetId: json['target_id'] as String,
      targetType: json['target_type'] as String,
      count: json['count'] as int? ?? 0,
      isLiked: json['is_liked'] as bool? ?? false,
    );
  }

  @override
  List<Object?> get props => [targetId, targetType, count, isLiked];
}
