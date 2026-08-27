import 'package:equatable/equatable.dart';

/// Lightweight User entity for search and tagging purposes.
/// Contains only minimal data needed for UI display.
///
/// NOTE: This is a read-only projection for search/display.
/// For full user data, use the authentication feature's user entity.
/// MIGRATED: Firestore methods removed, now using API only
class UserSearch extends Equatable {
  final String userId;
  final String username;
  final String? avatarUrl;

  const UserSearch({
    required this.userId,
    required this.username,
    this.avatarUrl,
  });

  /// Create from JSON (API response)
  factory UserSearch.fromJson(Map<String, dynamic> json) {
    return UserSearch(
      userId: json['userId'] as String,
      username: json['username'] as String? ?? '',
      avatarUrl: json['avatarUrl'] as String?,
    );
  }

  /// Convert to JSON
  Map<String, dynamic> toJson() {
    return {'userId': userId, 'username': username, 'avatarUrl': avatarUrl};
  }

  /// Display label for UI — always the @username handle.
  ///
  /// OWNER TRUTH: public identity is username only. fullName is private
  /// and must not surface as a public identity label.
  String get displayName => '@$username';

  /// Handle for UI (@username)
  String get handle => '@$username';

  @override
  List<Object?> get props => [userId, username, avatarUrl];
}
