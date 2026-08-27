import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/shared.dart';

/// Negative-contract tests for Slice A canonical shared-avatar chain.
///
/// These verify:
/// - shared [ProfileAvatar] primary constructor excludes `userId` and
///   `initials` (narrow parameter-block inspection)
/// - every named constructor excludes `userId` and `initials`
/// - shared [ProfileAvatar] source never calls
///   [UserInitialsHelper] / `fromUserId`
/// - [UserHeaderWidget] delegates handle formatting to
///   [UserIdentityFormatter.formatHandle]
/// - profile-screen avatar boundary supplies raw username (no `@` prefix)
///
/// UserHeaderWidget is a [ConsumerWidget] that pulls deep Riverpod
/// dependencies (HybridAvatar → authControllerProvider → apiClientProvider).
/// We verify its contract through source-structure checks rather than widget
/// pumping, to avoid wiring a test-only provider graph.

/// Reads the shared [ProfileAvatar] source file once.
String _profileAvatarSource() =>
    File('lib/shared/widgets/profile_avatar.dart').readAsStringSync();

/// Extracts the primary-constructor parameter block:
///   const ProfileAvatar({ … });
String _primaryConstructorBlock(String source) {
  final start = source.indexOf('const ProfileAvatar({');
  if (start == -1) throw StateError('Primary constructor not found');
  final end = source.indexOf('});', start);
  if (end == -1) throw StateError('Primary constructor end not found');
  return source.substring(start, end + 3); // include '});'
}

/// Extracts the parameter block for a named constructor:
///   `static ProfileAvatar <name>({ … }) { … }`
String _namedConstructorBlock(String source, String name) {
  final start = source.indexOf('static ProfileAvatar $name({');
  if (start == -1) throw StateError('Named constructor $name not found');
  final end = source.indexOf('}) {', start);
  if (end == -1) throw StateError('Named constructor $name end not found');
  return source.substring(start, end + 2); // include '})'
}

void main() {
  group('Shared ProfileAvatar API contracts', () {
    // ------------------------------------------------------------------
    // Constructor parameter-block locks
    // ------------------------------------------------------------------
    test('primary constructor excludes userId and initials', () {
      final block = _primaryConstructorBlock(_profileAvatarSource());

      expect(
        block.contains('userId'),
        isFalse,
        reason:
            'Primary constructor must not expose userId parameter.\n'
            'Block:\n$block',
      );

      expect(
        block.contains('initials'),
        isFalse,
        reason:
            'Primary constructor must not expose initials parameter.\n'
            'Block:\n$block',
      );
    });

    test('every named constructor excludes userId and initials', () {
      final source = _profileAvatarSource();
      const names = [
        'small',
        'medium',
        'large',
        'extraLarge',
        'comment',
        'postHeader',
      ];

      for (final name in names) {
        final block = _namedConstructorBlock(source, name);

        expect(
          block.contains('userId'),
          isFalse,
          reason:
              'Named constructor $name must not expose userId parameter.\n'
              'Block:\n$block',
        );

        expect(
          block.contains('initials'),
          isFalse,
          reason:
              'Named constructor $name must not expose initials parameter.\n'
              'Block:\n$block',
        );
      }
    });

    // ------------------------------------------------------------------
    // Legacy-pattern bans (whole-file)
    // ------------------------------------------------------------------
    test('source never calls UserInitialsHelper or fromUserId', () {
      final source = _profileAvatarSource();

      expect(
        source.contains('UserInitialsHelper'),
        isFalse,
        reason:
            'Shared ProfileAvatar must not call UserInitialsHelper '
            '— use UserIdentityFormatter.avatarInitials instead.',
      );

      expect(
        source.contains('fromUserId'),
        isFalse,
        reason:
            'Shared ProfileAvatar must not call fromUserId — fallback '
            'initials derive from canonical username only.',
      );
    });
  });

  group('UserHeaderWidget handle formatting', () {
    test('delegates to formatHandle — source imports formatter', () {
      final source = File(
        'lib/shared/widgets/user_header_widget.dart',
      ).readAsStringSync();

      // Must use UserIdentityFormatter.formatHandle, never manual @-concat.
      expect(
        source.contains('UserIdentityFormatter.formatHandle'),
        isTrue,
        reason:
            'UserHeaderWidget must delegate handle formatting to '
            'UserIdentityFormatter.formatHandle.',
      );

      // Must NOT construct a handle manually.
      expect(
        source.contains("'@\$") ||
            source.contains("'@\${") ||
            source.contains('"@\$') ||
            source.contains('"@\${'),
        isFalse,
        reason:
            'UserHeaderWidget must not construct @-prefixed handles '
            'manually — use formatHandle().',
      );
    });

    test('formatHandle returns null for bare @', () {
      // Unit test on the formatter — formatHandle('@') → null.
      // When UserHeaderWidget gets null, it displays '' (line 73: ?? '').
      // This prevents bare '@' or '@@' in the rendered widget.
      expect(UserIdentityFormatter.formatHandle('@'), isNull);
      expect(UserIdentityFormatter.formatHandle(''), isNull);
      expect(UserIdentityFormatter.formatHandle('john_doe'), '@john_doe');
    });
  });

  group('Profile-screen avatar boundary supplies raw username', () {
    test('profileData[username] is never @-prefixed in profile_screen.dart', () {
      final source = File(
        'lib/domains/user/profile/presentation/screens/profile_screen.dart',
      ).readAsStringSync();

      expect(
        source.contains("'username': '@"),
        isFalse,
        reason:
            "Found @-prefixed literal in profileData['username'] assignment — "
            'avatar must receive raw username, not presentation handle.',
      );

      expect(
        source.contains('"username": "@'),
        isFalse,
        reason:
            'Found @-prefixed literal (double-quoted) in username assignment.',
      );
    });

    test(
      'profileData[username] is never @-prefixed in profile_screen.dart',
      () {
        final source = File(
          'lib/domains/user/profile/presentation/screens/profile_screen.dart',
        ).readAsStringSync();

        expect(
          source.contains("'username': '@"),
          isFalse,
          reason:
              'profile_screen must not inject @-prefixed username literal.',
        );
      },
    );
  });
}
