import 'package:labuda/core/api/models/common_api_models.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/data/dto/follow_api_models.dart';

/// Mapper for Follow, Block, and Mute API models to domain entities
class FollowApiMapper {
  // ===========================================
  // FOLLOW MAPPINGS
  // ===========================================

  /// Convert FollowApiResponse to FollowableUser.
  ///
  /// Returns null when the API response is missing the embedded user block.
  /// Callers must filter nulls — we do NOT fabricate placeholder identities.
  static FollowableUser? toFollowableUserFromFollow(
    FollowApiResponse response, {
    bool useFollower = true,
  }) {
    final user = useFollower ? response.follower : response.following;
    if (user == null) return null;
    return _toFollowableUserFromBrief(user);
  }

  /// Convert list of FollowApiResponse to list of FollowableUser.
  /// Responses with missing user blocks are dropped, not surfaced as synthetic users.
  static List<FollowableUser> toFollowableUserList(
    List<FollowApiResponse> responses, {
    bool useFollower = true,
  }) {
    return responses
        .map(
          (response) =>
              toFollowableUserFromFollow(response, useFollower: useFollower),
        )
        .whereType<FollowableUser>()
        .toList();
  }

  /// Convert FollowStatsApiResponse to FollowStats
  static FollowStats toFollowStats(FollowStatsApiResponse response) {
    return FollowStats(
      userId: response.userId,
      followersCount: response.followersCount,
      followingCount: response.followingCount,
      mutualFollowsCount: 0, // Not provided by API, calculated separately
      topFollowers: const [],
      suggestedUsers: const [],
      lastUpdated: DateTime.now(),
    );
  }

  // ===========================================
  // BLOCK MAPPINGS
  // ===========================================

  /// Convert BlockApiResponse to FollowableUser.
  /// Returns null when the response is missing the embedded blocked user block.
  static FollowableUser? toFollowableUserFromBlock(BlockApiResponse response) {
    final user = response.blockedUser;
    if (user == null) return null;
    return _toFollowableUserFromBrief(user);
  }

  /// Convert list of BlockApiResponse to list of FollowableUser.
  /// Responses with missing user blocks are dropped, not surfaced as synthetic users.
  static List<FollowableUser> toFollowableUserListFromBlocks(
    List<BlockApiResponse> responses,
  ) {
    return responses
        .map(toFollowableUserFromBlock)
        .whereType<FollowableUser>()
        .toList();
  }

  // ===========================================
  // MUTE MAPPINGS
  // ===========================================

  /// Convert MuteApiResponse to FollowableUser.
  /// Returns null when the response is missing the embedded muted user block.
  static FollowableUser? toFollowableUserFromMute(MuteApiResponse response) {
    final user = response.mutedUser;
    if (user == null) return null;
    return _toFollowableUserFromBrief(user);
  }

  /// Convert list of MuteApiResponse to list of FollowableUser.
  /// Responses with missing user blocks are dropped, not surfaced as synthetic users.
  static List<FollowableUser> toFollowableUserListFromMutes(
    List<MuteApiResponse> responses,
  ) {
    return responses
        .map(toFollowableUserFromMute)
        .whereType<FollowableUser>()
        .toList();
  }

  // ===========================================
  // S5 — FOLLOW LIST CARD MAPPINGS
  // ===========================================

  /// Convert a lifecycle-aware FollowListUserCardDto (from backend publiccard)
  /// to a FollowableUser domain entity.
  static FollowableUser fromFollowListCard(FollowListUserCardDto card) {
    final resolvedUsername = card.username.isNotEmpty
        ? card.username
        : 'user_${card.id.length >= 8 ? card.id.substring(0, 8) : card.id}';
    return FollowableUser(
      id: card.id,
      username: resolvedUsername,
      displayName: resolvedUsername,
      avatar: card.avatarUrl,
      userType: UserType.buyer,
      lifecycle: card.lifecycle ?? 'active',
      followersCount: card.followersCount,
      followingCount: card.followingCount,
      isFollowedByCurrentUser: false,
      isFollowingCurrentUser: false,
    );
  }

  /// Convert list of FollowListUserCardDto to list of FollowableUser.
  static List<FollowableUser> fromFollowListCards(
    List<FollowListUserCardDto> cards,
  ) {
    return cards.map(fromFollowListCard).toList();
  }

  // ===========================================
  // USER SEARCH MAPPINGS
  // ===========================================

  /// Convert a UserSearchPreviewDto (from GET /api/v1/search/users) to FollowableUser.
  ///
  /// lifecycle defaults to 'active': the backend SQL already filters
  /// account_status='active' AND deleted_at IS NULL so all results are active.
  /// isFollowedByCurrentUser defaults to false (not provided; caller fetches
  /// follow state via a separate request if needed).
  static FollowableUser fromUserSearchPreview(UserSearchPreviewDto preview) {
    final resolvedUsername = preview.username.isNotEmpty
        ? preview.username
        : 'user_${preview.id.length >= 8 ? preview.id.substring(0, 8) : preview.id}';
    return FollowableUser(
      id: preview.id,
      username: resolvedUsername,
      displayName: resolvedUsername,
      avatar: preview.avatarUrl,
      userType: UserType.buyer,
      lifecycle: 'active',
      followersCount: 0,
      followingCount: 0,
      isFollowedByCurrentUser: preview.isFollowedByCurrentUser,
      isFollowingCurrentUser: false,
    );
  }

  /// Convert list of UserSearchPreviewDto to list of FollowableUser.
  static List<FollowableUser> fromUserSearchPreviews(
    List<UserSearchPreviewDto> previews,
  ) {
    return previews.map(fromUserSearchPreview).toList();
  }

  // ===========================================
  // INTERNAL HELPERS
  // ===========================================

  /// Convert UserBriefApiResponse to FollowableUser
  static FollowableUser _toFollowableUserFromBrief(UserBriefApiResponse user) {
    final resolvedUsername = user.username ?? 'user_${user.id.substring(0, 8)}';
    return FollowableUser(
      id: user.id,
      username: resolvedUsername,
      displayName: resolvedUsername,
      avatar: user.avatar,
      userType: UserType.buyer, // Default, not provided by brief API
      // REMOVED: postsCount (not provided by brief API, deleted in PROFILE PURGE)
      isFollowedByCurrentUser: false, // Need to check separately
      isFollowingCurrentUser: false, // Need to check separately
    );
  }
}
