import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/data/dto/like_api_models.dart';

/// Mapper for Like API models to domain entities
///
/// ZERO LEGACY MODE: Only canonical target types accepted
class LikeMapper {
  // ===========================================
  // API TO DOMAIN
  // ===========================================

  /// Convert LikeStatsApiResponse to LikeStats entity
  static LikeStats toLikeStats(LikeStatsApiResponse response) {
    return LikeStats(
      targetId: response.targetId,
      targetType: _mapTargetType(response.targetType),
      totalLikes: response.count,
      isLikedByCurrentUser: response.isLiked,
    );
  }

  // ===========================================
  // DOMAIN TO API
  // ===========================================

  /// Build LikeToggleApiRequest from domain data
  static LikeToggleApiRequest buildToggleRequest({
    required String targetId,
    required LikeTargetType targetType,
  }) {
    return LikeToggleApiRequest(
      targetId: targetId,
      targetType: _mapTargetTypeToString(targetType),
    );
  }

  // ===========================================
  // TYPE MAPPINGS
  // ===========================================

  /// Map domain LikeTargetType enum to API string
  ///
  /// BACKEND ALIGNMENT V1: Only canonical types sent to backend
  /// Backend validates: oneof=content comment
  static String _mapTargetTypeToString(LikeTargetType targetType) {
    switch (targetType) {
      case LikeTargetType.content:
        return 'content';
      case LikeTargetType.comment:
        return 'comment';
    }
  }

  /// Map backend target type string to domain LikeTargetType enum.
  static LikeTargetType _mapTargetType(String targetType) {
    switch (targetType) {
      case 'content':
        return LikeTargetType.content;
      case 'comment':
        return LikeTargetType.comment;
      default:
        throw ArgumentError('Unsupported like target type: $targetType');
    }
  }
}
