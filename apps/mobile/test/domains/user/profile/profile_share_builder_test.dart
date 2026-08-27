import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/profile/presentation/screens/profile_screen/profile_share_builder.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

void main() {
  test('active canonical profile builds a canonical profile share target', () {
    final target = buildProfileShareTarget(
      userId: 'user-42',
      username: 'user_deadbeef',
      bio: '',
      avatarUrl: 'https://cdn.example.com/avatar.jpg',
      lifecycle: ContentLifecycle.active,
    );

    expect(target, isNotNull);
    expect(target!.id, 'user-42');
    expect(target.title, '@user_deadbeef');
    expect(target.description, 'Lihat profil @user_deadbeef di LABUDA');
    expect(target.imageUrl, 'https://cdn.example.com/avatar.jpg');
    expect(target.publicShareUrl, 'https://labuda.app/profile/user-42');
  });

  test('genuine stored user_deadbeef remains valid and not synthetic', () {
    final target = buildProfileShareTarget(
      userId: 'user-42',
      username: 'user_deadbeef',
      bio: 'A short bio',
      avatarUrl: null,
      lifecycle: ContentLifecycle.active,
    );

    expect(target, isNotNull);
    expect(target!.title, '@user_deadbeef');
    expect(target.description, 'A short bio');
    expect(target.imageUrl, isNull);
  });

  test('blank or unavailable usernames do not produce share payloads', () {
    expect(
      buildProfileShareTarget(
        userId: 'user-42',
        username: '',
        bio: null,
        avatarUrl: 'https://cdn.example.com/avatar.jpg',
        lifecycle: ContentLifecycle.active,
      ),
      isNull,
    );
    expect(
      buildProfileShareTarget(
        userId: 'user-42',
        username: '   ',
        bio: null,
        avatarUrl: 'https://cdn.example.com/avatar.jpg',
        lifecycle: ContentLifecycle.active,
      ),
      isNull,
    );
    expect(
      buildProfileShareTarget(
        userId: 'user-42',
        username: 'alice',
        bio: null,
        avatarUrl: 'https://cdn.example.com/avatar.jpg',
        lifecycle: ContentLifecycle.unavailable,
      ),
      isNull,
    );
    expect(
      buildProfileShareTarget(
        userId: 'user-42',
        username: 'alice',
        bio: null,
        avatarUrl: 'https://cdn.example.com/avatar.jpg',
        lifecycle: ContentLifecycle.removed,
      ),
      isNull,
    );
  });
}
