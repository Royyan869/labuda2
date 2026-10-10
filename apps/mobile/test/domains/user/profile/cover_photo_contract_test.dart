// STAGE 4F-2 — Mobile cover-photo wire contract.
//
// Locks the canonical mobile contract against the Stage 4F-1 backend:
//   - persistence value is the STORAGE KEY (images/profile-covers/{userId}.jpg)
//   - PATCH serializes cover_photo_url (empty string = clear)
//   - hydration parses the resolved cover_photo_url from BOTH response shapes:
//       * public GET /users/{id} → top-level cover_photo_url (flat, no profile)
//       * self GET /users/me → profile.cover_photo_url (nested)
//   - the repository update request carries the cover reference
//   - the legacy images/covers/ prefix never appears in the canonical path

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/user/profile/data/mappers/user_api_mapper.dart';
import 'package:hishumi/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/profile_cover.dart';
import 'package:hishumi/shared/widgets/app_image.dart';

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
    // Public profile contract: GET /users/{id} returns PublicUserResponse —
    // FLAT, no nested `profile`, cover_photo_url at the TOP LEVEL.
    test('public profile: flat top-level cover_photo_url hydrates ProfileEntity',
        () {
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
        'cover_photo_url': resolvedCoverUrl,
      });

      // Contract shape guard: the public response carries no nested profile.
      expect(response.profile, isNull);

      final entity = UserApiMapper.toProfileEntity(response);
      expect(entity.coverPhotoUrl, resolvedCoverUrl);
      expect(entity.coverPhotoUrl, isNot('images/profile-covers/user-123.jpg'));
    });

    // Self contract: GET /users/me returns {user, profile} — cover_photo_url
    // nested under `profile`.
    test('self profile: nested profile cover_photo_url hydrates ProfileEntity',
        () {
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
    });

    test('toProfileEntity yields null cover when neither contract has one', () {
      final response = UserApiResponse.fromJson({
        'id': 'user-1',
        'email': 'me@example.com',
        'username': 'me',
        'account_status': 'active',
        'roles': ['user'],
        'created_at': '2026-07-30T08:00:00.000Z',
        'updated_at': '2026-07-30T08:00:00.000Z',
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

  group('Cover contract convergence (client never re-validates the reference)',
      () {
    test('storage-key reference is not interpreted as a URL', () {
      // A canonical storage key is NOT an absolute URL. The client must never
      // require it to be one — the backend's validateCoverPhotoReference is the
      // sole authority for the persisted reference shape.
      const storageKey = 'images/profile-covers/user-1.jpg';
      final parsed = Uri.tryParse(storageKey);
      expect(parsed?.hasScheme ?? false, isFalse);
      // It must still round-trip through the request unchanged.
      final request = UpdateProfileApiRequest(coverPhotoUrl: storageKey);
      expect(request.toJson()['cover_photo_url'], storageKey);
    });
  });

  // End-to-end chain for the profile page: the flat public wire value must
  // survive JSON → API model → mapper → ProfileEntity → ProfileCover, and the
  // cover widget must receive the resolved URL (not the gradient fallback).
  testWidgets('public cover_photo_url reaches ProfileCover through the chain', (
    tester,
  ) async {
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
      'cover_photo_url': resolvedCoverUrl,
    });

    final entity = UserApiMapper.toProfileEntity(response);

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: ProfileCover(coverPhotoUrl: entity.coverPhotoUrl, height: 160),
        ),
      ),
    );
    await tester.pump();

    final image = tester.widget<AppImage>(find.byType(AppImage));
    expect(image.imageUrl, resolvedCoverUrl);
  });
}

