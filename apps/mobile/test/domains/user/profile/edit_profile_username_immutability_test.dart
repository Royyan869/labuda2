// Stage 1C — Part A: Edit Profile username immutability.
//
// Business truth: username is the canonical identity, immutable after
// registration; Edit Profile / Settings must NOT allow username changes.
//
// Proofs:
//   1. Edit Profile does not expose an editable username field.
//   2. Saving profile does not attempt a username mutation (the save payload
//      omits `username`).
//   3. The username value loaded for display stays the current canonical one
//      and is never submitted as a change.
//
// These are source-contract proofs over the canonical Edit Profile files, in
// the same style as the existing edit_profile_canonical_identity_contract_test
// — asserting the actual implementation surface rather than a parallel stub.

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _readSource(String relativePath) =>
    File(relativePath).readAsStringSync();

void main() {
  final personalSection = _readSource(
    'lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_personal_section.dart',
  );
  final saveHandler = _readSource(
    'lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_save_handler.dart',
  );

  group('Edit Profile username immutability', () {
    test('username field is not editable (read-only display)', () {
      // The username AppTextField is rendered read-only and wrapped in an
      // AbsorbPointer so it cannot receive input.
      expect(personalSection.contains('enabled: false'), isTrue);
      expect(personalSection.contains('AbsorbPointer'), isTrue);
      // It no longer registers a validator for username input (no edit path).
      expect(personalSection.contains('validator:'), isFalse);
    });

    test('username is labelled as immutable identity (not a rename field)', () {
      expect(personalSection.contains("labelText: 'Username'"), isTrue);
      expect(personalSection.contains('IMMUTABLE'), isTrue);
    });

    test('save handler does NOT send username in the profile-update payload',
        () {
      // savePersonal builds the authController.updateProfile call WITHOUT a
      // `username:` argument.
      expect(saveHandler.contains('authController.updateProfile('), isTrue);
      expect(saveHandler.contains('username:'), isFalse);
    });

    test('save handler no longer reads usernameController for submission', () {
      // The save path must not reference the username controller when building
      // the mutation. (The getter still exists for the read-only display, but
      // it is not wired into the save payload.)
      expect(
        saveHandler.contains(
          'usernameController.text.trim()',
        ),
        isFalse,
      );
    });

    test('username value stays the canonical current value', () {
      // The screen loads the username from the authenticated user and renders
      // it unchanged; immutability is enforced because nothing re-writes it.
      final screen = _readSource(
        'lib/domains/user/profile/presentation/screens/unified_edit_profile_screen.dart',
      );
      expect(
        screen.contains('_usernameController.text = user.username;'),
        isTrue,
      );
    });

    test('no username mutation via the canonical Edit Profile save path', () {
      // The combined Edit Profile source must not contain an active
      // `username:` write in the update call. (Bio still legitimately has a
      // hintText; only the username field is read-only.)
      expect(saveHandler.contains('username: usernameController'), isFalse);
      // The read-only username field specifically disables editing.
      expect(personalSection.contains('enabled: false'), isTrue);
    });
  });
}