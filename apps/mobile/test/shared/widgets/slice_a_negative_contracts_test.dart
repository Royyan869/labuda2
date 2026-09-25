import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/helpers/user_identity_formatter.dart';
import 'package:labuda/shared/shared.dart';

/// Negative-contract tests for the canonical shared-avatar chain.
///
/// Business truth (Owner decision 2026-09-24):
/// - A user avatar is ALWAYS the user's photo or the user icon.
/// - Initials do not exist ANYWHERE in the avatar chain - the parameter,
///   the fallback, the formatter method, and every named constructor were
///   killed at the source so the old design cannot nest again.
///
/// These verify (through source-structure inspection, because
/// [HybridAvatar] pulls deep Riverpod dependencies):
/// - shared [ProfileAvatar] source contains no initials concept at all
/// - [ProfileAvatar] exposes no named constructors (vestigial sizes gone)
/// - [UserIdentityFormatter] no longer carries an initials method
/// - [UserHeaderWidget] renders through the canonical [HybridAvatar]
/// - profile-screen avatar boundary stays on the seller composite

/// Reads the shared [ProfileAvatar] source file once.
String _profileAvatarSource() =>
    File('lib/shared/widgets/profile_avatar.dart').readAsStringSync();


/// Strips comment lines so the ban applies to CODE, not doc prose that
/// explains the ban itself.
String _codeOnly(String source) => source
    .split(String.fromCharCode(10))
    .where((line) => !line.trim().startsWith('//'))
    .join(String.fromCharCode(10));

void main() {
  group('Shared ProfileAvatar API contracts', () {
    test('source contains no initials concept at all', () {
      final source = _codeOnly(_profileAvatarSource());

      expect(
        source.toLowerCase().contains('initials'),
        isFalse,
        reason:
            'Initials are banned business-wide (Owner decision 2026-09-24). '
            'A user avatar is ALWAYS the user photo or the user icon.',
      );

      expect(source.contains('fromUserId'), isFalse);
      expect(source.contains('UserInitialsHelper'), isFalse);
    });

    test('no vestigial named constructors remain', () {
      final source = _codeOnly(_profileAvatarSource());

      expect(
        source.contains('static ProfileAvatar'),
        isFalse,
        reason:
            'Named size constructors (.small/.medium/...) were removed with '
            'the initials flow - call ProfileAvatar(size: ...) directly.',
      );
    });

    test('renders through StableNetworkImage (anti-flicker authority)', () {
      final source = _codeOnly(_profileAvatarSource());

      expect(
        source.contains('StableNetworkImage'),
        isTrue,
        reason:
            'Rotating signed URLs must never flash a placeholder - all '
            'user-avatar rendering goes through gapless StableNetworkImage.',
      );
      expect(
        source.contains('CachedNetworkImage'),
        isFalse,
        reason:
            'CachedNetworkImage is not the canonical renderer for avatars; '
            'use StableNetworkImage.',
      );
    });

    test('fallback is the user icon, never text', () {
      final source = _codeOnly(_profileAvatarSource());

      expect(source.contains('Icons.person'), isTrue);
      expect(
        source.contains('Text('),
        isFalse,
        reason: 'A user avatar never renders text - no initials exist.',
      );
    });
  });

  group('UserIdentityFormatter contract', () {
    test('formatter source contains no initials concept', () {
      final source = _codeOnly(
        File(
          'lib/shared/helpers/user_identity_formatter.dart',
        ).readAsStringSync(),
      );

      expect(
        source.toLowerCase().contains('initials'),
        isFalse,
        reason: 'avatarInitials was killed at the source; the formatter '
            'handles username presentation only.',
      );
    });

    test('formatHandle returns null for bare @', () {
      expect(UserIdentityFormatter.formatHandle('@'), isNull);
      expect(UserIdentityFormatter.formatHandle(''), isNull);
      expect(UserIdentityFormatter.formatHandle('john_doe'), '@john_doe');
    });
  });

  group('UserHeaderWidget canonical rendering', () {
    test('renders through the canonical HybridAvatar', () {
      final source = File(
        'lib/shared/widgets/user_header_widget.dart',
      ).readAsStringSync();

      expect(source.contains('HybridAvatar('), isTrue);
      expect(
        source.toLowerCase().contains('initials'),
        isFalse,
        reason: 'No initials anywhere in the header chain.',
      );
    });
  });

  group('Profile-screen avatar boundary', () {
    test('uses the seller composite and never @-prefixes username literals',
        () {
      final source = File(
        'lib/domains/user/profile/presentation/screens/profile_screen.dart',
      ).readAsStringSync();

      expect(
        source.contains('ProfileAvatar('),
        isFalse,
        reason:
            'The profile header avatar must be the seller-aware composite '
            '(SellerAvatar) - the dual/single gate lives there.',
      );
      expect(source.contains('SellerAvatar('), isTrue);
      expect(
        source.toLowerCase().contains('initials'),
        isFalse,
        reason: 'No initials anywhere in the profile avatar boundary.',
      );
      // NOTE: @-prefixed literals in profileData ('name'/'username') belong
      // to the TEXT-display contract, not the avatar boundary - avatars no
      // longer read profileData['username'] at all.
    });
  });
}
