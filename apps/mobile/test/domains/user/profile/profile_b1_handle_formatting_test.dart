import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// SLICE B1: Profile handle formatting contract tests.
///
/// Source-contract tests verifying that the profile_screen.dart
/// preserves raw username for identity data, routes visible seller
/// identity through the shared renderer, and keeps lifecycle redaction
/// labels intact.

String _profileScreenSource() => File(
  'lib/domains/user/profile/presentation/screens/profile_screen.dart',
).readAsStringSync();

void main() {
  group('Profile B1 handle formatting', () {
    // ------------------------------------------------------------------
    // Raw username vs formatted handle separation
    // ------------------------------------------------------------------
    test("profileData['username'] is raw username (no @ prefix)", () {
      final source = _profileScreenSource();

      // profileData['username'] must be raw — check both branches
      final ownProfileUsername = RegExp(r"'username':\s*user\.username");
      final viewedProfileUsername = RegExp(r"'username':\s*renderedUsername");

      expect(
        ownProfileUsername.hasMatch(source),
        isTrue,
        reason:
            "Own-profile branch must store raw user.username in profileData['username'].",
      );
      expect(
        viewedProfileUsername.hasMatch(source),
        isTrue,
        reason:
            "Viewed-profile branch must store raw renderedUsername in profileData['username'].",
      );
    });

    test('profile screen no longer stores a manual display-name field', () {
      final source = _profileScreenSource();

      expect(source.contains("profileData['name']"), isFalse);
      expect(
        source.contains("Text(profileData['storeName']"),
        isFalse,
        reason:
            'ProfileScreen must not manually render store name text outside SellerIdentityView.',
      );
      expect(
        source.contains("Text(profileData['username']"),
        isFalse,
        reason:
            'ProfileScreen must not manually render username text outside SellerIdentityView.',
      );
      expect(
        source.contains('SellerIdentityViewVariant.profile'),
        isTrue,
        reason:
            'ProfileScreen must route seller identity through the shared profile renderer.',
      );
    });

    // ------------------------------------------------------------------
    // No bare @ formatting in handle presentation
    // ------------------------------------------------------------------
    test('no bare @-interpolation remains for username handles', () {
      final source = _profileScreenSource();

      // The only remaining @ should be inside formatHandle calls
      // or inside lifecycle redaction labels.
      // Check for the exact old pattern — '@${user.username}' must be gone.
      expect(
        source.contains(r"'@${user.username}'"),
        isFalse,
        reason: 'Bare @-interpolation for username must be removed.',
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

    // ------------------------------------------------------------------
    // Store name still comes from the canonical seller identity payload
    // ------------------------------------------------------------------
    test('store name rendering still reads from profileData["storeName"]', () {
      final source = _profileScreenSource();

      expect(
        source.contains("profileData['storeName']"),
        isTrue,
        reason:
            'Store name must remain sourced from profileData["storeName"].',
      );
    });
  });
}
