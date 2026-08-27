// Follow API Models for Go Backend Integration
//
// These models match the Go backend DTOs in:
// backend/internal/domain/social/dto/*.go
//
// Coverage:
// - Follow (handler_follow.go)
// - Block (handler_block.go)
// - Mute (handler_mute.go)

import 'package:equatable/equatable.dart';
import 'package:labuda/core/api/models/common_api_models.dart';

// =============================================================================
// Follow Request/Response DTOs
// =============================================================================

/// Follow relationship response
class FollowApiResponse extends Equatable {
  final String id;
  final String followerId;
  final String followingId;
  final String status;
  final DateTime createdAt;
  final UserBriefApiResponse? follower;
  final UserBriefApiResponse? following;

  const FollowApiResponse({
    required this.id,
    required this.followerId,
    required this.followingId,
    required this.status,
    required this.createdAt,
    this.follower,
    this.following,
  });

  factory FollowApiResponse.fromJson(Map<String, dynamic> json) {
    return FollowApiResponse(
      id: json['id'] as String,
      followerId: json['follower_id'] as String,
      followingId: json['following_id'] as String,
      status: json['status'] as String? ?? 'active',
      createdAt: DateTime.parse(json['created_at'] as String),
      follower: json['follower'] != null
          ? UserBriefApiResponse.fromJson(
              json['follower'] as Map<String, dynamic>,
            )
          : null,
      following: json['following'] != null
          ? UserBriefApiResponse.fromJson(
              json['following'] as Map<String, dynamic>,
            )
          : null,
    );
  }

  @override
  List<Object?> get props => [id, followerId, followingId, status];
}

/// Follow statistics response
class FollowStatsApiResponse {
  final String userId;
  final int followersCount;
  final int followingCount;

  const FollowStatsApiResponse({
    required this.userId,
    required this.followersCount,
    required this.followingCount,
  });

  factory FollowStatsApiResponse.fromJson(Map<String, dynamic> json) {
    return FollowStatsApiResponse(
      userId: json['user_id'] as String? ?? json['id'] as String? ?? '',
      followersCount: (json['followers_count'] as num?)?.toInt() ?? 0,
      followingCount: (json['following_count'] as num?)?.toInt() ?? 0,
    );
  }

  @override
  String toString() =>
      'FollowStatsApiResponse(userId: $userId, followers: $followersCount, following: $followingCount)';
}

/// Follow status between two users
class FollowStatusApiResponse {
  final bool isFollowing;
  final bool isFollowedBy;
  final bool isMutual;

  const FollowStatusApiResponse({
    required this.isFollowing,
    required this.isFollowedBy,
    required this.isMutual,
  });

  factory FollowStatusApiResponse.fromJson(Map<String, dynamic> json) {
    return FollowStatusApiResponse(
      isFollowing: json['is_following'] as bool? ?? false,
      isFollowedBy: json['is_followed_by'] as bool? ?? false,
      isMutual: json['is_mutual'] as bool? ?? false,
    );
  }

  @override
  String toString() =>
      'FollowStatusApiResponse(isFollowing: $isFollowing, isFollowedBy: $isFollowedBy, isMutual: $isMutual)';
}

// =============================================================================
// Block Request/Response DTOs
// =============================================================================

/// Block relationship response
class BlockApiResponse {
  final String id;
  final String userId;
  final String blockedUserId;
  final String? reason;
  final DateTime createdAt;
  final UserBriefApiResponse? blockedUser;

  const BlockApiResponse({
    required this.id,
    required this.userId,
    required this.blockedUserId,
    this.reason,
    required this.createdAt,
    this.blockedUser,
  });

  factory BlockApiResponse.fromJson(Map<String, dynamic> json) {
    return BlockApiResponse(
      id: json['id'] as String,
      userId: json['user_id'] as String,
      blockedUserId: json['blocked_user_id'] as String,
      reason: json['reason'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
      blockedUser: json['blocked_user'] != null
          ? UserBriefApiResponse.fromJson(
              json['blocked_user'] as Map<String, dynamic>,
            )
          : null,
    );
  }

  @override
  String toString() =>
      'BlockApiResponse(id: $id, userId: $userId, blockedUserId: $blockedUserId)';
}

// =============================================================================
// Mute Request/Response DTOs
// =============================================================================

/// Mute relationship response
class MuteApiResponse {
  final String id;
  final String userId;
  final String mutedUserId;
  final DateTime? expiresAt;
  final bool isPermanent;
  final DateTime createdAt;
  final UserBriefApiResponse? mutedUser;

  const MuteApiResponse({
    required this.id,
    required this.userId,
    required this.mutedUserId,
    this.expiresAt,
    required this.isPermanent,
    required this.createdAt,
    this.mutedUser,
  });

  factory MuteApiResponse.fromJson(Map<String, dynamic> json) {
    return MuteApiResponse(
      id: json['id'] as String,
      userId: json['user_id'] as String,
      mutedUserId: json['muted_user_id'] as String,
      expiresAt: json['expires_at'] != null
          ? DateTime.parse(json['expires_at'] as String)
          : null,
      isPermanent: json['is_permanent'] as bool? ?? true,
      createdAt: DateTime.parse(json['created_at'] as String),
      mutedUser: json['muted_user'] != null
          ? UserBriefApiResponse.fromJson(
              json['muted_user'] as Map<String, dynamic>,
            )
          : null,
    );
  }

  @override
  String toString() =>
      'MuteApiResponse(id: $id, userId: $userId, mutedUserId: $mutedUserId, isPermanent: $isPermanent)';
}

// =============================================================================
// S5 — Follow List UserCard DTOs
// Matches backend publiccard.UserCard shape emitted by GET /users/:id/followers
// and GET /users/:id/following after S5 hydration.
// =============================================================================

/// Single lifecycle-aware user card in a follow list response.
/// Maps backend publiccard.UserCard: {id, username, avatar_url, lifecycle}.
class FollowListUserCardDto extends Equatable {
  final String id;
  final String username;
  final String? avatarUrl;
  final int followersCount;
  final int followingCount;

  /// Coarsened public lifecycle: "active" | "unavailable" | "removed".
  /// Null treated as "active" by callers (pre-S5 backend compat).
  final String? lifecycle;

  const FollowListUserCardDto({
    required this.id,
    required this.username,
    this.avatarUrl,
    this.followersCount = 0,
    this.followingCount = 0,
    this.lifecycle,
  });

  factory FollowListUserCardDto.fromJson(Map<String, dynamic> json) {
    return FollowListUserCardDto(
      id: json['id'] as String,
      username: json['username'] as String? ?? '',
      avatarUrl: json['avatar_url'] as String?,
      followersCount: (json['followers_count'] as num?)?.toInt() ?? 0,
      followingCount: (json['following_count'] as num?)?.toInt() ?? 0,
      lifecycle: json['lifecycle'] as String?,
    );
  }

  @override
  List<Object?> get props => [
    id,
    username,
    avatarUrl,
    followersCount,
    followingCount,
    lifecycle,
  ];
}

/// Response body for GET /users/:id/followers and GET /users/:id/following.
/// Backend emits: {"followers": [...UserCard], "limit": 20} or
///               {"following": [...UserCard], "limit": 20}.
class FollowListResponseDto extends Equatable {
  final List<FollowListUserCardDto> items;
  final int limit;

  const FollowListResponseDto({required this.items, required this.limit});

  factory FollowListResponseDto.fromFollowersJson(Map<String, dynamic> json) {
    return FollowListResponseDto(
      items: (json['followers'] as List<dynamic>? ?? [])
          .map((e) => FollowListUserCardDto.fromJson(e as Map<String, dynamic>))
          .toList(),
      limit: json['limit'] as int? ?? 20,
    );
  }

  factory FollowListResponseDto.fromFollowingJson(Map<String, dynamic> json) {
    return FollowListResponseDto(
      items: (json['following'] as List<dynamic>? ?? [])
          .map((e) => FollowListUserCardDto.fromJson(e as Map<String, dynamic>))
          .toList(),
      limit: json['limit'] as int? ?? 20,
    );
  }

  @override
  List<Object?> get props => [items, limit];
}

// =============================================================================
// User Search DTOs
// Matches GET /api/v1/search/users response shape.
// Backend: search_handler.go SearchUsers → userPreviewsToResponse
// Per-user fields: {id, username, avatar_url} only.
// Lifecycle defaults to 'active' (SQL: account_status='active' AND deleted_at IS NULL).
// =============================================================================

/// Single user preview from GET /api/v1/search/users.
class UserSearchPreviewDto extends Equatable {
  final String id;
  final String username;
  final String? avatarUrl;

  /// Whether the authenticated viewer already follows this user.
  /// Backend populates this from user_follows; defaults false when absent.
  final bool isFollowedByCurrentUser;

  const UserSearchPreviewDto({
    required this.id,
    required this.username,
    this.avatarUrl,
    this.isFollowedByCurrentUser = false,
  });

  factory UserSearchPreviewDto.fromJson(Map<String, dynamic> json) {
    return UserSearchPreviewDto(
      id: json['id'] as String,
      username: json['username'] as String? ?? '',
      avatarUrl: json['avatar_url'] as String?,
      isFollowedByCurrentUser:
          json['is_followed_by_current_user'] as bool? ?? false,
    );
  }

  @override
  List<Object?> get props => [id, username, avatarUrl, isFollowedByCurrentUser];
}

/// Response envelope for GET /api/v1/search/users.
/// Backend emits: {"query": "...", "users": [...], "total": 5, "limit": 20, "offset": 0}
class UserSearchResponseDto {
  final String query;
  final List<UserSearchPreviewDto> users;
  final int total;
  final int limit;
  final int offset;

  const UserSearchResponseDto({
    required this.query,
    required this.users,
    required this.total,
    required this.limit,
    required this.offset,
  });

  factory UserSearchResponseDto.fromJson(Map<String, dynamic> json) {
    return UserSearchResponseDto(
      query: json['query'] as String? ?? '',
      users: (json['users'] as List<dynamic>? ?? [])
          .map((e) => UserSearchPreviewDto.fromJson(e as Map<String, dynamic>))
          .toList(),
      total: (json['total'] as num?)?.toInt() ?? 0,
      limit: (json['limit'] as num?)?.toInt() ?? 20,
      offset: (json['offset'] as num?)?.toInt() ?? 0,
    );
  }
}
