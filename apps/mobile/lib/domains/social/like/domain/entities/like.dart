/// Target types that can be liked
///
/// BACKEND ALIGNMENT V1:
/// - content: All content items
/// - comment: Comments on content
///
/// Aligned with backend: backend/internal/social/like/entity/like.go
/// Backend validates: oneof=content comment
enum LikeTargetType { content, comment }

/// Like statistics for a specific target
class LikeStats {
  final String targetId;
  final LikeTargetType targetType;
  final int totalLikes;
  final bool isLikedByCurrentUser;

  const LikeStats({
    required this.targetId,
    required this.targetType,
    required this.totalLikes,
    required this.isLikedByCurrentUser,
  });

  LikeStats copyWith({
    String? targetId,
    LikeTargetType? targetType,
    int? totalLikes,
    bool? isLikedByCurrentUser,
  }) {
    return LikeStats(
      targetId: targetId ?? this.targetId,
      targetType: targetType ?? this.targetType,
      totalLikes: totalLikes ?? this.totalLikes,
      isLikedByCurrentUser: isLikedByCurrentUser ?? this.isLikedByCurrentUser,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;

    return other is LikeStats &&
        other.targetId == targetId &&
        other.targetType == targetType &&
        other.totalLikes == totalLikes &&
        other.isLikedByCurrentUser == isLikedByCurrentUser;
  }

  @override
  int get hashCode {
    return targetId.hashCode ^
        targetType.hashCode ^
        totalLikes.hashCode ^
        isLikedByCurrentUser.hashCode;
  }

  @override
  String toString() {
    return 'LikeStats(targetId: $targetId, targetType: $targetType, totalLikes: $totalLikes, isLikedByCurrentUser: $isLikedByCurrentUser)';
  }
}
