import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/presentation/screens/create_content_screen.dart';
import 'package:labuda/domains/social/content/data/dto/content_dto.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_type_visibility_header.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

AuthUser _authUser() {
  return AuthUser(
    id: 'user-1',
    createdAt: DateTime.utc(2026, 7, 23),
    updatedAt: DateTime.utc(2026, 7, 23),
    email: 'user-1@example.com',
    username: 'creator',
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
  );
}

/// C1B negative contracts for create-content UI and DTO serialization.
void main() {
  group('ContentVisibilityHeader contract', () {
    Widget wrap() {
      return MaterialApp(
        home: Scaffold(
          body: ContentVisibilityHeader(
            authenticatedUser: null,
            postVisibility: 'Public',
            onVisibilityChanged: (_) {},
          ),
        ),
      );
    }

    testWidgets('has no Post/Request type selector and no comments toggle', (tester) async {
      await tester.pumpWidget(wrap());
      await tester.pump();

      expect(find.text('Post'), findsNothing);
      expect(find.text('Request'), findsNothing);
      expect(find.text('Comments'), findsNothing);
      expect(find.byType(Switch), findsNothing);
      expect(find.text('Public'), findsOneWidget);
    });
  });

  group('CreateContentScreen constructor contract', () {
    test('constructor accepts no content-type parameter', () {
      const screen = CreateContentScreen();
      expect(screen, isA<CreateContentScreen>());
    });

    test('source exposes no type/initialType parameter', () {
      final source = File(
        'lib/domains/social/content/presentation/screens/create_content_screen.dart',
      ).readAsStringSync();

      expect(source, isNot(contains('initialType')));
      expect(source, isNot(contains('ContentType.post')));
      expect(source, isNot(contains('ContentType.request')));
    });
  });

  group('CreateContentScreen universal composer contract', () {
    Widget wrap() {
      return ProviderScope(
        overrides: [
          authControllerProvider.overrideWith(
            () => _FakeAuthController(
              AuthState.authenticated(_authUser(), emailVerified: true),
            ),
          ),
        ],
        child: const MaterialApp(home: CreateContentScreen()),
      );
    }

    testWidgets('renders the generic create-content shell', (tester) async {
      await tester.pumpWidget(wrap());
      await tester.pumpAndSettle();

      expect(find.text('Create Content'), findsOneWidget);
      expect(find.text('Post'), findsNothing);
      expect(find.text('Request'), findsNothing);
      expect(find.text('Comments'), findsNothing);
    });

    testWidgets('does not expose legacy link picker text', (tester) async {
      await tester.pumpWidget(wrap());
      await tester.pumpAndSettle();

      expect(find.text('Produk Dijual'), findsNothing);
      expect(find.text('Link'), findsNothing);
      expect(find.text('Lelang'), findsNothing);
    });
  });

  group('CreateContentDto serialization', () {
    test('has no type field and omits allow_comments', () {
      const dto = CreateContentDto(content: 'hello');
      final json = dto.toJson();

      expect(json.containsKey('type'), isFalse);
      expect(json.containsKey('allow_comments'), isFalse);
    });
  });

  group('UpdateContentDto serialization', () {
    test('omits allow_comments and legacy write-only identity fields', () {
      const dto = UpdateContentDto(content: 'updated');
      final json = dto.toJson();

      expect(json.containsKey('allow_comments'), isFalse);
      expect(json.containsKey('share_reference'), isFalse);
      expect(json.containsKey('resource_occurrence'), isFalse);
      expect(json.containsKey('targetType'), isFalse);
      expect(json.containsKey('targetId'), isFalse);
      expect(json.containsKey('preview'), isFalse);
    });
  });
}
