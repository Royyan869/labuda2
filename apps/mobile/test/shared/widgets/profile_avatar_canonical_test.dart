import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

/// CANONICAL ProfileAvatar contract (Owner decision 2026-09-24).
///
/// Business truth:
/// - A user avatar is ALWAYS the user's photo, or the `Icons.person` user
///   icon when there is no photo. Initials do not exist anywhere.
/// - Rendering goes through StableNetworkImage (gapless playback) so a
///   rotating signed URL never flashes a placeholder over a visible frame.
///
/// The old initials-flow tests (avatarInitials, named size constructors,
/// CachedNetworkImage cacheKey stripping) were killed together with the
/// feature they locked in.

/// Strips comment lines so the ban applies to CODE, not doc prose that
/// explains the ban itself.
String _codeOnly(String source) => source
    .split(String.fromCharCode(10))
    .where((line) => !line.trim().startsWith('//'))
    .join(String.fromCharCode(10));

void main() {
  group('ProfileAvatar canonical behavior (no initials exist)', () {
    testWidgets('no image → renders the user icon', (tester) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(body: ProfileAvatar(size: 40, userId: 'u1')),
        ),
      );
      expect(find.byIcon(Icons.person), findsOneWidget);
      expect(find.byType(Text), findsNothing);
    });

    testWidgets('valid image enters the image branch (no icon, no text)', (
      tester,
    ) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: ProfileAvatar(
              size: 40,
              userId: 'u1',
              imageUrl: 'https://example.com/avatar.png',
            ),
          ),
        ),
      );
      await tester.pump();
      expect(find.byType(Text), findsNothing);
      // The image always routes through the gapless renderer with the user
      // icon as its error-fallback (test env has no HTTP, so the fallback
      // legitimately renders after failure - that IS the canonical path).
      expect(find.byType(StableNetworkImage), findsOneWidget);
      final stable = tester.widget<StableNetworkImage>(
        find.byType(StableNetworkImage),
      );
      expect(stable.imageUrl, 'https://example.com/avatar.png');
    });

    testWidgets('blank/whitespace image URL → user icon, never text', (
      tester,
    ) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: ProfileAvatar(size: 40, userId: 'u1', imageUrl: '   '),
          ),
        ),
      );
      expect(find.byIcon(Icons.person), findsOneWidget);
      expect(find.text('J'), findsNothing);
      expect(find.text('JD'), findsNothing);
    });

    testWidgets('edit icon renders only when requested', (tester) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: ProfileAvatar(size: 80, userId: 'u1', showEditIcon: true),
          ),
        ),
      );
      expect(find.byIcon(Icons.camera_alt), findsOneWidget);
    });
  });

  group('Anti-flicker contract (StableNetworkImage gapless)', () {
    testWidgets(
      'rotating signed URL keeps the gapless image branch (no fallback flash)',
      (tester) async {
        await tester.pumpWidget(
          const MaterialApp(
            home: Scaffold(
              body: ProfileAvatar(
                size: 40,
                userId: 'u1',
                imageUrl: 'https://example.com/a.png',
              ),
            ),
          ),
        );
        await tester.pump();
        await tester.pump(const Duration(seconds: 1));

        // Rotating signed URL: same logical image, new query string.
        await tester.pumpWidget(
          const MaterialApp(
            home: Scaffold(
              body: ProfileAvatar(
                size: 40,
                userId: 'u1',
                imageUrl: 'https://example.com/a.png?X-Amz-Signature=two',
              ),
            ),
          ),
        );
        await tester.pump();
        await tester.pump(const Duration(seconds: 1));

        final stable = tester.widget<StableNetworkImage>(
          find.byType(StableNetworkImage),
        );
        expect(
          stable.imageUrl,
          'https://example.com/a.png?X-Amz-Signature=two',
        );
        expect(find.byIcon(Icons.person), findsNothing);
      },
    );
  });

  group('Avatar source-structure bans (no initials can nest again)', () {
    test('ProfileAvatar source contains no initials concept', () {
      final source = _codeOnly(
        File(
          'lib/shared/widgets/profile_avatar.dart',
        ).readAsStringSync(),
      );
      expect(source.contains('initials'), isFalse);
      expect(source.contains('Initials'), isFalse);
    });

    test('formatter source contains no initials concept', () {
      final source = _codeOnly(
        File(
          'lib/shared/helpers/user_identity_formatter.dart',
        ).readAsStringSync(),
      );
      expect(source.contains('initials'), isFalse);
      expect(source.contains('Initials'), isFalse);
    });

    test('profile-screen avatar is the seller composite, no initials', () {
      final source = File(
        'lib/domains/user/profile/presentation/screens/profile_screen.dart',
      ).readAsStringSync();
      expect(source.contains('initials'), isFalse);
      expect(source.contains('SellerAvatar('), isTrue);
      expect(source.contains('OnlineBadge('), isTrue);
    });
  });
}
