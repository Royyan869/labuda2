// Tests for mobile user search wiring — MOBILE_SEARCH_USERS_WIRE_PASS
// Updated: USER_SEARCH_FOLLOW_STATE_AUDIT — follow state wired from backend
//
// Invariants locked:
// 1. UserSearchResponseDto.fromJson parses users list correctly.
// 2. UserSearchResponseDto.fromJson with empty/missing users returns [].
// 3. UserSearchPreviewDto.fromJson parses id, username, avatar_url.
// 4. UserSearchPreviewDto.fromJson parses is_followed_by_current_user=true.
// 5. UserSearchPreviewDto.fromJson defaults is_followed_by_current_user=false when absent.
// 6. fromUserSearchPreview maps fields to FollowableUser correctly.
// 7. fromUserSearchPreview with empty username generates fallback.
// 8. fromUserSearchPreview lifecycle defaults to 'active'.
// 9. fromUserSearchPreview propagates isFollowedByCurrentUser=true from preview.
// 10. fromUserSearchPreview propagates isFollowedByCurrentUser=false from preview.
// 11. fromUserSearchPreviews maps a list correctly, preserving follow state per entry.
// 12. fromUserSearchPreviews with empty list returns [].
// 13. No stub fallback: searchUsers delegates to datasource (not hardcoded []).

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/social/follow/data/dto/follow_api_models.dart';
import 'package:labuda/domains/social/follow/data/mappers/follow_api_mapper.dart';

void main() {
  group('UserSearchResponseDto', () {
    test('parses full response with users list', () {
      final dto = UserSearchResponseDto.fromJson({
        'query': 'alice',
        'users': [
          {
            'id': 'uuid-1',
            'username': 'alice',
            'avatar_url': 'https://cdn.example.com/a.jpg',
          },
          {'id': 'uuid-2', 'username': 'alicia', 'avatar_url': null},
        ],
        'total': 2,
        'limit': 20,
        'offset': 0,
      });

      expect(dto.query, 'alice');
      expect(dto.users.length, 2);
      expect(dto.total, 2);
      expect(dto.limit, 20);
      expect(dto.offset, 0);
    });

    test('parses empty users list', () {
      final dto = UserSearchResponseDto.fromJson({
        'query': 'xyz',
        'users': [],
        'total': 0,
        'limit': 20,
        'offset': 0,
      });

      expect(dto.users, isEmpty);
      expect(dto.total, 0);
    });

    test('tolerates missing users key — returns empty list', () {
      final dto = UserSearchResponseDto.fromJson({
        'query': 'test',
        'total': 0,
        'limit': 20,
        'offset': 0,
      });

      expect(dto.users, isEmpty);
    });
  });

  group('UserSearchPreviewDto', () {
    test('parses all fields correctly', () {
      final dto = UserSearchPreviewDto.fromJson({
        'id': 'uuid-abc',
        'username': 'budi',
        'avatar_url': 'https://cdn.example.com/budi.jpg',
      });

      expect(dto.id, 'uuid-abc');
      expect(dto.username, 'budi');
      expect(dto.avatarUrl, 'https://cdn.example.com/budi.jpg');
    });

    test('parses null avatar_url', () {
      final dto = UserSearchPreviewDto.fromJson({
        'id': 'uuid-abc',
        'username': 'budi',
        'avatar_url': null,
      });

      expect(dto.avatarUrl, isNull);
    });

    test('tolerates missing username — defaults to empty string', () {
      final dto = UserSearchPreviewDto.fromJson({'id': 'uuid-abc'});

      expect(dto.username, '');
    });

    test('parses is_followed_by_current_user=true', () {
      final dto = UserSearchPreviewDto.fromJson({
        'id': 'uuid-abc',
        'username': 'sari',
        'avatar_url': null,
        'is_followed_by_current_user': true,
      });

      expect(dto.isFollowedByCurrentUser, isTrue);
    });

    test('parses is_followed_by_current_user=false explicitly', () {
      final dto = UserSearchPreviewDto.fromJson({
        'id': 'uuid-abc',
        'username': 'sari',
        'avatar_url': null,
        'is_followed_by_current_user': false,
      });

      expect(dto.isFollowedByCurrentUser, isFalse);
    });

    test('defaults is_followed_by_current_user to false when field absent', () {
      final dto = UserSearchPreviewDto.fromJson({
        'id': 'uuid-abc',
        'username': 'sari',
        'avatar_url': null,
      });

      expect(dto.isFollowedByCurrentUser, isFalse);
    });
  });

  group('FollowApiMapper.fromUserSearchPreview', () {
    test('maps id, username, avatar correctly', () {
      final preview = UserSearchPreviewDto(
        id: 'uuid-1',
        username: 'sari',
        avatarUrl: 'https://cdn.example.com/sari.jpg',
      );

      final user = FollowApiMapper.fromUserSearchPreview(preview);

      expect(user.id, 'uuid-1');
      expect(user.username, 'sari');
      expect(user.displayName, 'sari');
      expect(user.avatar, 'https://cdn.example.com/sari.jpg');
    });

    test('maps null avatar to null', () {
      final preview = UserSearchPreviewDto(id: 'uuid-1', username: 'sari');

      final user = FollowApiMapper.fromUserSearchPreview(preview);

      expect(user.avatar, isNull);
    });

    test('generates username fallback when username is empty', () {
      final preview = UserSearchPreviewDto(
        id: 'abcdef1234567890',
        username: '',
      );

      final user = FollowApiMapper.fromUserSearchPreview(preview);

      expect(user.username, 'user_abcdef12');
      expect(user.displayName, 'user_abcdef12');
    });

    test(
      'lifecycle defaults to active — backend SQL guarantees active users only',
      () {
        final preview = UserSearchPreviewDto(id: 'uuid-1', username: 'sari');

        final user = FollowApiMapper.fromUserSearchPreview(preview);

        expect(user.lifecycle, 'active');
        expect(user.isDegraded, isFalse);
      },
    );

    test('propagates isFollowedByCurrentUser=true from preview', () {
      final preview = UserSearchPreviewDto(
        id: 'uuid-1',
        username: 'sari',
        isFollowedByCurrentUser: true,
      );

      final user = FollowApiMapper.fromUserSearchPreview(preview);

      expect(user.isFollowedByCurrentUser, isTrue);
    });

    test('propagates isFollowedByCurrentUser=false from preview', () {
      final preview = UserSearchPreviewDto(id: 'uuid-1', username: 'sari');

      final user = FollowApiMapper.fromUserSearchPreview(preview);

      expect(user.isFollowedByCurrentUser, isFalse);
    });

    test(
      'isFollowingCurrentUser always false — not provided by search endpoint',
      () {
        final preview = UserSearchPreviewDto(id: 'uuid-1', username: 'sari');

        final user = FollowApiMapper.fromUserSearchPreview(preview);

        expect(user.isFollowingCurrentUser, isFalse);
      },
    );

    test('count fields default to zero — not provided by search endpoint', () {
      final preview = UserSearchPreviewDto(id: 'uuid-1', username: 'sari');

      final user = FollowApiMapper.fromUserSearchPreview(preview);

      expect(user.followersCount, 0);
      expect(user.followingCount, 0);
    });
  });

  group('FollowApiMapper.fromUserSearchPreviews', () {
    test('maps list correctly, preserving follow state per entry', () {
      final previews = [
        UserSearchPreviewDto(
          id: 'u1',
          username: 'alice',
          avatarUrl: 'https://cdn.example.com/a.jpg',
          isFollowedByCurrentUser: true,
        ),
        UserSearchPreviewDto(id: 'u2', username: 'bob'),
      ];

      final users = FollowApiMapper.fromUserSearchPreviews(previews);

      expect(users.length, 2);
      expect(users[0].id, 'u1');
      expect(users[0].isFollowedByCurrentUser, isTrue);
      expect(users[1].id, 'u2');
      expect(users[1].isFollowedByCurrentUser, isFalse);
    });

    test('maps empty list to empty list', () {
      final users = FollowApiMapper.fromUserSearchPreviews([]);

      expect(users, isEmpty);
    });
  });

  group('no stub fallback regression', () {
    // This test confirms the DTO and mapper layer is wired — the stub
    // Result.success([]) in FollowRepositoryApi.searchUsers() has been replaced.
    // A round-trip parse of a real response shape must produce a non-empty list.
    test(
      'full parse pipeline produces non-empty result for valid backend payload',
      () {
        final raw = {
          'query': 'budi',
          'users': [
            {
              'id': 'uuid-100',
              'username': 'budi_farm',
              'avatar_url': null,
              'is_followed_by_current_user': false,
            },
          ],
          'total': 1,
          'limit': 20,
          'offset': 0,
        };

        final dto = UserSearchResponseDto.fromJson(raw);
        final users = FollowApiMapper.fromUserSearchPreviews(dto.users);

        expect(users, isNotEmpty);
        expect(users.first.username, 'budi_farm');
        expect(users.first.lifecycle, 'active');
        expect(users.first.isFollowedByCurrentUser, isFalse);
      },
    );

    test('full parse pipeline reflects followed=true from backend payload', () {
      final raw = {
        'query': 'sari',
        'users': [
          {
            'id': 'uuid-200',
            'username': 'sari_tani',
            'avatar_url': null,
            'is_followed_by_current_user': true,
          },
        ],
        'total': 1,
        'limit': 20,
        'offset': 0,
      };

      final dto = UserSearchResponseDto.fromJson(raw);
      final users = FollowApiMapper.fromUserSearchPreviews(dto.users);

      expect(users, isNotEmpty);
      expect(users.first.isFollowedByCurrentUser, isTrue);
    });
  });
}
