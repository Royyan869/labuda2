import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_type_visibility_header.dart';

AuthUser _user(String username) {
  return AuthUser(
    id: 'user-1',
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: '$username@test.com',
    username: username,
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
  );
}

Widget _wrap(AuthUser? user) {
  return MaterialApp(
    home: Scaffold(
      body: ContentVisibilityHeader(
        authenticatedUser: user,
        postVisibility: 'Public',
        onVisibilityChanged: (_) {},
      ),
    ),
  );
}

void main() {
  testWidgets('create-content header shows hydrated identity then falls back', (
    tester,
  ) async {
    final user = _user('authorone');

    await tester.pumpWidget(_wrap(user));
    await tester.pump();

    expect(find.text('@authorone'), findsOneWidget);
    expect(find.byIcon(Icons.person_outline), findsNothing);

    await tester.pumpWidget(_wrap(null));
    await tester.pump();

    expect(find.text('@authorone'), findsNothing);
    expect(find.byIcon(Icons.person_outline), findsOneWidget);
  });
}
