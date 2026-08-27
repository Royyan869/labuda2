import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/social/follow/data/dto/follow_api_models.dart';
import 'package:labuda/domains/social/follow/data/mappers/follow_api_mapper.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';

void main() {
  group('follow count convergence', () {
    test('follow-list DTO parses backend count fields', () {
      final dto = FollowListUserCardDto.fromJson({
        'id': 'user-1',
        'username': 'alice',
        'avatar_url': 'https://cdn.example.com/a.jpg',
        'followers_count': 11,
        'following_count': 7,
        'lifecycle': 'active',
      });

      expect(dto.followersCount, 11);
      expect(dto.followingCount, 7);
      expect(dto.lifecycle, 'active');
    });

    test('follow-list mapper preserves backend counts in FollowableUser', () {
      final dto = FollowListUserCardDto(
        id: 'user-1',
        username: 'alice',
        avatarUrl: 'https://cdn.example.com/a.jpg',
        followersCount: 11,
        followingCount: 7,
        lifecycle: 'active',
      );

      final user = FollowApiMapper.fromFollowListCard(dto);

      expect(user.followersCount, 11);
      expect(user.followingCount, 7);
    });

    test('follow stats DTO accepts public profile response shape', () {
      final stats = FollowStatsApiResponse.fromJson({
        'id': 'user-1',
        'followers_count': 11,
        'following_count': 7,
      });

      expect(stats.userId, 'user-1');
      expect(stats.followersCount, 11);
      expect(stats.followingCount, 7);
    });

    test('user profile DTO parses live counts from /users/me payload', () {
      final dto = UserProfileApiResponse.fromJson({
        'id': 'user-1',
        'username': 'alice',
        'followers_count': 11,
        'following_count': 7,
        'preferred_lang': 'id',
      });

      expect(dto.followersCount, 11);
      expect(dto.followingCount, 7);
    });
  });
}
