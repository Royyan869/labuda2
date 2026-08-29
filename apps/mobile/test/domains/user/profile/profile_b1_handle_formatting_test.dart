import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// SLICE B1: Profile handle formatting contract tests.
///
/// Source-contract tests verifying that the profile_screen.dart
/// preserves username identity data, routes visible seller
/// identity through the shared renderer, and keeps lifecycle redaction
/// labels intact.

String _profileScreenSource() => File(
  'lib/domains/user/profile/presentation/screens/profile_screen.dart',
).readAsStringSync();

void main() {
  group('Profile B1 handle formatting', () {
    // ------------------------------------------------------------------
    // Username handle formatting
    // ------------------------------------------------------------------
    test('profileData["username"] stores @-prefixed handle', () {
      final source = _profileScreenSource();

      // Both branches store @-prefixed username:
      // Own-profile: 'username': '@${user.username}'
      // Viewed-profile: 'username': renderedUsername (which is '@${user.username}')
      expect(
        source.contains(r"'@${user.username}'"),
        isTrue,
        reason: 'Profile must store @-prefixed username handle.',
      );
    });

    test('profile screen uses profileData["name"] for display name', () {
      final source = _profileScreenSource();

      // ProfileScreen renders profileData['name'] for the flying identity text
      expect(
        source.contains("profileData['name']"),
        isTrue,
        reason: 'ProfileScreen uses profileData["name"] for display identity.',
      );
    });

    // ------------------------------------------------------------------
    // Lifecycle preservation
    // ------------------------------------------------------------------
    test('degraded lifecycle preserves redaction label', () {
      final source = _profileScreenSource();

      // The degraded branch must still use lifecycle.publicRedactionLabel
      expect(
        source.contains('lifecycle.publicRedactionLabel'),
        isTrue,
        reason:
            'Degraded profile must preserve lifecycle-based redaction label.',
      );
    });
  });
}
