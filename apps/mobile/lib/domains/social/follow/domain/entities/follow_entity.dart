import 'package:equatable/equatable.dart';

/// Follow Entity - relationship antar users
class Follow extends Equatable {
  final String id;
  final String followerId; // User yang nge-follow
  final String followingId; // User yang di-follow
  final DateTime createdAt;
  final FollowStatus status;

  const Follow({
    required this.id,
    required this.followerId,
    required this.followingId,
    required this.createdAt,
    this.status = FollowStatus.active,
  });

  @override
  List<Object?> get props => [id, followerId, followingId, createdAt, status];

  Follow copyWith({
    String? id,
    String? followerId,
    String? followingId,
    DateTime? createdAt,
    FollowStatus? status,
  }) {
    return Follow(
      id: id ?? this.id,
      followerId: followerId ?? this.followerId,
      followingId: followingId ?? this.followingId,
      createdAt: createdAt ?? this.createdAt,
      status: status ?? this.status,
    );
  }
}

/// User Profile untuk follow system
class FollowableUser extends Equatable {
  final String id;
  final String username;
  final String displayName;
  final String? avatar;
  final String? bio;
  final UserType userType;
  final int followersCount;
  final int followingCount;
  // REMOVED: postsCount (not provided by API, deleted in PROFILE PURGE)
  final bool isFollowedByCurrentUser;
  final bool isFollowingCurrentUser;
  final DateTime? lastActiveAt;

  /// S5 — Coarsened public lifecycle from backend: "active" | "unavailable" | "removed".
  /// Defaults to "active" so existing construction sites remain valid.
  final String lifecycle;

  const FollowableUser({
    required this.id,
    required this.username,
    required this.displayName,
    this.avatar,
    this.bio,
    required this.userType,
    this.followersCount = 0,
    this.followingCount = 0,
    this.isFollowedByCurrentUser = false,
    this.isFollowingCurrentUser = false,
    this.lastActiveAt,
    this.lifecycle = 'active',
  });

  bool get isDegraded => lifecycle != 'active';

  @override
  List<Object?> get props => [
    id,
    username,
    displayName,
    avatar,
    bio,
    userType,
    followersCount,
    followingCount,
    // REMOVED: postsCount (not provided by API, deleted in PROFILE PURGE)
    isFollowedByCurrentUser,
    isFollowingCurrentUser,
    lastActiveAt,
    lifecycle,
  ];

  FollowableUser copyWith({
    String? id,
    String? username,
    String? displayName,
    String? avatar,
    String? bio,
    UserType? userType,
    int? followersCount,
    int? followingCount,
    bool? isFollowedByCurrentUser,
    bool? isFollowingCurrentUser,
    DateTime? lastActiveAt,
    String? lifecycle,
  }) {
    return FollowableUser(
      id: id ?? this.id,
      username: username ?? this.username,
      displayName: displayName ?? this.displayName,
      avatar: avatar ?? this.avatar,
      bio: bio ?? this.bio,
      userType: userType ?? this.userType,
      followersCount: followersCount ?? this.followersCount,
      followingCount: followingCount ?? this.followingCount,
      isFollowedByCurrentUser:
          isFollowedByCurrentUser ?? this.isFollowedByCurrentUser,
      isFollowingCurrentUser:
          isFollowingCurrentUser ?? this.isFollowingCurrentUser,
      lastActiveAt: lastActiveAt ?? this.lastActiveAt,
      lifecycle: lifecycle ?? this.lifecycle,
    );
  }
}

/// Follow Statistics
class FollowStats extends Equatable {
  final String userId;
  final int followersCount;
  final int followingCount;
  final int mutualFollowsCount; // Mutual follows dengan current user
  final List<FollowableUser> topFollowers; // Top followers
  final List<FollowableUser> suggestedUsers; // Users to follow
  final DateTime lastUpdated;

  const FollowStats({
    required this.userId,
    required this.followersCount,
    required this.followingCount,
    this.mutualFollowsCount = 0,
    this.topFollowers = const [],
    this.suggestedUsers = const [],
    required this.lastUpdated,
  });

  @override
  List<Object?> get props => [
    userId,
    followersCount,
    followingCount,
    mutualFollowsCount,
    topFollowers,
    suggestedUsers,
    lastUpdated,
  ];
}

/// Follow Activity - untuk notification dan timeline
class FollowActivity extends Equatable {
  final String id;
  final String followerId;
  final String followerName;
  final String? followerAvatar;
  final String followingId;
  final String followingName;
  final String? followingAvatar;
  final FollowActivityType type;
  final DateTime createdAt;
  final bool isRead;

  const FollowActivity({
    required this.id,
    required this.followerId,
    required this.followerName,
    this.followerAvatar,
    required this.followingId,
    required this.followingName,
    this.followingAvatar,
    required this.type,
    required this.createdAt,
    this.isRead = false,
  });

  @override
  List<Object?> get props => [
    id,
    followerId,
    followerName,
    followerAvatar,
    followingId,
    followingName,
    followingAvatar,
    type,
    createdAt,
    isRead,
  ];
}

/// Follow Status
enum FollowStatus { active, blocked, muted }

/// User Type untuk kategorisasi
enum UserType { buyer, seller, breeder, enthusiast, judge }

/// Follow Activity Type
enum FollowActivityType {
  userStartedFollowing, // A started following B
  userUnfollowed, // A unfollowed B
  mutualFollow, // A and B now follow each other
}
