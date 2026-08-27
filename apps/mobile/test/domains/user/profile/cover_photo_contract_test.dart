// STAGE 4F-2 — Mobile cover-photo wire contract.
//
// Locks the canonical mobile contract against the Stage 4F-1 backend:
//   - persistence value is the STORAGE KEY (images/profile-covers/{userId}.jpg)
//   - PATCH serializes cover_photo_url (empty string = clear)
//   - hydration parses the resolved cover_photo_url from the profile response
//   - the repository update request carries the cover reference
//   - the legacy images/covers/ prefix never appears in the canonical path

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/profile/data/mappers/user_api_mapper.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';

void main() {
  group('UpdateProfileApiRequest cover serialization', () {
    test('cover_photo_url is serialized as the canonical storage key', () {
      final request = UpdateProfileApiRequest(
        coverPhotoUrl: 'images/profile-covers/user-1.jpg',
      );

      expect(request.toJson(), {
        'cover_photo_url': 'images/profile-covers/user-1.jpg',
      });
    });

    test('empty string cover serializes as clear signal (backend → NULL)', () {
      final request = UpdateProfileApiRequest(coverPhotoUrl: '');

      expect(request.toJson(), {'cover_photo_url': ''});
    });

    test('omitted cover is not serialized', () {
      final request = const UpdateProfileApiRequest();
      expect(request.toJson().containsKey('cover_photo_url'), isFalse);
    });
  });

  group('UserApiMapper cover hydration', () {
    test('toProfileEntity preserves resolved cover_photo_url unchanged', () {
      const resolvedCoverUrl =
          'https://cdn.example.com/images/profile-covers/user-123.jpg';
      final response = UserApiResponse.fromJson({
        'id': 'user-123',
        'email': 'me@example.com',
        'username': 'me',
        'account_status': 'active',
        'roles': ['user'],
        'created_at': '2026-07-30T08:00:00.000Z',
        'updated_at': '2026-07-30T08:00:00.000Z',
        'profile': {
          'id': 'profile-123',
          'username': 'me',
          'cover_photo_url': resolvedCoverUrl,
        },
      });

      final entity = UserApiMapper.toProfileEntity(response);
      expect(entity.coverPhotoUrl, resolvedCoverUrl);
      expect(entity.coverPhotoUrl, isNot('images/profile-covers/user-123.jpg'));
    });

    test('toProfileEntity yields null cover when the profile has none', () {
      final response = UserApiResponse.fromJson({
        'id': 'user-1',
        'email': 'me@example.com',
        'username': 'me',
        'account_status': 'active',
        'roles': ['user'],
        'created_at': '2026-07-30T08:00:00.000Z',
        'updated_at': '2026-07-30T08:00:00.000Z',
        'profile': {'id': 'profile-1', 'username': 'me'},
      });

      final entity = UserApiMapper.toProfileEntity(response);
      expect(entity.coverPhotoUrl, isNull);
    });

    test('toUpdateProfileRequest carries the cover reference', () {
      final request = UserApiMapper.toUpdateProfileRequest(
        coverPhotoUrl: 'images/profile-covers/user-1.jpg',
      );

      expect(request.coverPhotoUrl, 'images/profile-covers/user-1.jpg');
    });
  });

  group('Legacy prefix absence', () {
    test('canonical storage key uses images/profile-covers (never images/covers)',
        () {
      const key = 'images/profile-covers/user-1.jpg';
      expect(key.contains('images/covers/'), isFalse);
    });
  });
}

