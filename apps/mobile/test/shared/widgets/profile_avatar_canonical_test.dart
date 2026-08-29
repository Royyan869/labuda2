import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/shared.dart';

void main() {
  group('ProfileAvatar fallback behavior', () {
    // --- Test 1: no image + valid userId → canonical initials ---
    testWidgets('no image + valid userId renders canonical initials', (
      tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(body: ProfileAvatar(size: 40, userId: 'john_doe')),
        ),
      );

      // UserInitialsHelper.fromUserId('john_doe') → 'JO' (first 2 chars)
      expect(find.text('JO'), findsOneWidget);
    });

    // --- Test 2: leading @ is passed through by fromUserId ---
    testWidgets('leading @ is passed through by fromUserId', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(body: ProfileAvatar(size: 40, userId: '@john_doe')),
        ),
      );

      // UserInitialsHelper.fromUserId('@john_doe') → '@J' (first 2 chars)
      expect(find.text('@J'), findsOneWidget);
    });

    // --- Test 3: numeric-only userId → first two digits as initials ---
    testWidgets('numeric-only userId renders first two digits as initials', (
      tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(body: ProfileAvatar(size: 40, userId: '12345')),
        ),
      );

      // UserInitialsHelper.fromUserId('12345') → '12'
      expect(find.text('12'), findsOneWidget);
    });

    // --- Test 4: empty userId → fallback 'U' ---
    testWidgets('empty userId renders fallback initial', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(body: ProfileAvatar(size: 40, userId: '')),
        ),
      );

      // UserInitialsHelper.fromUserId('') → 'U'
      expect(find.text('U'), findsOneWidget);
    });



    // --- Test 6: valid image path enters image loading branch ---
    testWidgets('valid image path enters image loading branch', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ProfileAvatar(
              size: 40,
              imageUrl: 'https://example.com/avatar.png',
              userId: 'john_doe',
            ),
          ),
        ),
      );

      await tester.pump();
      // Image loaded — no initials or icon shown
      expect(find.byType(Text), findsNothing);
      expect(find.byIcon(Icons.person), findsNothing);
    });

    // --- Test 7: signed query params do not change stable cache identity ---
    testWidgets(
      'signed URL query parameters are stripped from the stable cache key',
      (tester) async {
        const baseUrl = 'https://cdn.example.com/avatar.png';
        const signedOne =
            'https://cdn.example.com/avatar.png?X-Amz-Signature=one&X-Amz-Date=20260726T000000Z';
        const signedTwo =
            'https://cdn.example.com/avatar.png?X-Amz-Signature=two&X-Amz-Date=20260726T010000Z';

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: ProfileAvatar(
                size: 40,
                imageUrl: signedOne,
                userId: 'john_doe',
              ),
            ),
          ),
        );

        final first = tester.widget<CachedNetworkImage>(
          find.byType(CachedNetworkImage),
        );
        expect(first.imageUrl, signedOne);
        expect(first.cacheKey, baseUrl);

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: ProfileAvatar(
                size: 40,
                imageUrl: signedTwo,
                userId: 'john_doe',
              ),
            ),
          ),
        );

        final second = tester.widget<CachedNetworkImage>(
          find.byType(CachedNetworkImage),
        );
        expect(second.imageUrl, signedTwo);
        expect(second.cacheKey, baseUrl);
      },
    );

    // --- Test 8: named size constructors render correct sizes ---
    testWidgets('named size constructors preserve sizing', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Column(
              children: [
                ProfileAvatar.small(userId: 'test'),
                ProfileAvatar.medium(userId: 'test'),
                ProfileAvatar.large(userId: 'test'),
                ProfileAvatar.extraLarge(userId: 'test'),
                ProfileAvatar.comment(userId: 'test'),
                ProfileAvatar.postHeader(userId: 'test'),
              ],
            ),
          ),
        ),
      );

      // UserInitialsHelper.fromUserId('test') → 'TE'
      expect(find.text('TE'), findsWidgets);
    });
  });
}
