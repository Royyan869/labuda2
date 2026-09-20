import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/helpers/user_identity_formatter.dart';
import 'package:labuda/shared/shared.dart';

void main() {
  group('ProfileAvatar canonical fallback behavior (UserIdentityFormatter)', () {
    testWidgets('no image + john_doe → JD via avatarInitials', (tester) async {
      final initials = UserIdentityFormatter.avatarInitials('john_doe');
      expect(initials, 'JD');
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(body: ProfileAvatar(size: 40, userId: 'x', initials: initials)),
        ),
      );
      expect(find.text('JD'), findsOneWidget);
    });

    testWidgets('alice-smith → AS via avatarInitials', (tester) async {
      final initials = UserIdentityFormatter.avatarInitials('alice-smith');
      expect(initials, 'AS');
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(body: ProfileAvatar(size: 40, userId: 'x', initials: initials)),
        ),
      );
      expect(find.text('AS'), findsOneWidget);
    });

    test('numeric-only 12345 → null (generic icon)', () {
      expect(UserIdentityFormatter.avatarInitials('12345'), isNull);
    });

    testWidgets('numeric-only renders generic icon, not initials', (tester) async {
      final initials = UserIdentityFormatter.avatarInitials('12345');
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(body: ProfileAvatar(size: 40, userId: 'x', initials: initials)),
        ),
      );
      expect(find.byIcon(Icons.person), findsOneWidget);
      expect(find.text('12'), findsNothing);
    });

    testWidgets('empty username renders generic icon', (tester) async {
      await tester.pumpWidget(
        MaterialApp(home: Scaffold(body: ProfileAvatar(size: 40, userId: 'x', initials: UserIdentityFormatter.avatarInitials('')))),
      );
      expect(find.byIcon(Icons.person), findsOneWidget);
      expect(find.text('U'), findsNothing);
    });

    testWidgets('valid image path enters image loading branch', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ProfileAvatar(size: 40, imageUrl: 'https://example.com/avatar.png', userId: 'x', initials: 'JD'),
          ),
        ),
      );
      await tester.pump();
      expect(find.byType(Text), findsNothing);
      expect(find.byIcon(Icons.person), findsNothing);
    });

    testWidgets('signed URL query params stripped from cache key', (tester) async {
      const signedOne = 'https://cdn.example.com/avatar.png?X-Amz-Signature=one';
      const baseUrl = 'https://cdn.example.com/avatar.png';
      await tester.pumpWidget(
        MaterialApp(home: Scaffold(body: ProfileAvatar(size: 40, imageUrl: signedOne, userId: 'x', initials: 'JD'))),
      );
      final first = tester.widget<CachedNetworkImage>(find.byType(CachedNetworkImage));
      expect(first.cacheKey, baseUrl);
    });

    testWidgets('named size constructors preserve sizing via initials', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Column(children: [
              ProfileAvatar.small(userId: 'x', initials: 'TE'),
              ProfileAvatar.medium(userId: 'x', initials: 'TE'),
              ProfileAvatar.large(userId: 'x', initials: 'TE'),
              ProfileAvatar.extraLarge(userId: 'x', initials: 'TE'),
              ProfileAvatar.comment(userId: 'x', initials: 'TE'),
              ProfileAvatar.postHeader(userId: 'x', initials: 'TE'),
            ]),
          ),
        ),
      );
      expect(find.text('TE'), findsWidgets);
    });
  });
}
