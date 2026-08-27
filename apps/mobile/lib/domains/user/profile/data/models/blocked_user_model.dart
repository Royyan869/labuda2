/// Blocked User Model
/// Represents a user that has been blocked
/// MIGRATED: Now using API (fromJson/toJson), Firestore methods removed
class BlockedUserModel {
  final String id;
  final String username;
  final String? avatarUrl;
  final DateTime blockedAt;

  const BlockedUserModel({
    required this.id,
    required this.username,
    this.avatarUrl,
    required this.blockedAt,
  });

  /// Create from API JSON response
  factory BlockedUserModel.fromJson(Map<String, dynamic> json) {
    return BlockedUserModel(
      id: json['id'] as String? ?? json['blocked_user_id'] as String? ?? '',
      username: json['username'] as String? ?? 'unknown',
      avatarUrl: json['avatar_url'] as String? ?? json['avatarUrl'] as String?,
      blockedAt: json['blocked_at'] != null
          ? DateTime.parse(json['blocked_at'] as String)
          : (json['blockedAt'] is DateTime
                ? json['blockedAt'] as DateTime
                : DateTime.now()),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'username': username,
      'avatar_url': avatarUrl,
      'blocked_at': blockedAt.toIso8601String(),
    };
  }
}
