import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/social/share/domain/entities/share_target.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';

void main() {
  test('profile share reference resolves to internal viewed-profile route', () {
    final reference = ShareReference.profile(
      profileId: 'user-42',
      name: 'Alice',
      isAvailable: true,
    );

    expect(reference.navigationPath, equals('/user/user-42'));
  });

  test('profile share target resolves to external public profile URL', () {
    final target = ShareTarget(
      id: 'user-42',
      type: ExternalShareType.profile,
      title: 'Alice',
      description: 'Profile description',
    );

    expect(
      target.publicShareUrl,
      equals('$kPublicProfileBaseUrl/profile/user-42'),
    );
    expect(
      target.shareText,
      contains('$kPublicProfileBaseUrl/profile/user-42'),
    );
  });
}
