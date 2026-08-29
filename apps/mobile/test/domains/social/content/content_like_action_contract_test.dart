import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_like_action.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/auth_user.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_controller.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_state.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _ControlledLikeRepository implements LikeRepository {
  _ControlledLikeRepository(this.stats);

  final LikeStats stats;
  final Completer<Result<bool>> toggleCompleter = Completer<Result<bool>>();
  int toggleCalls = 0;

  @override
  Future<Result<bool>> toggleLike({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async {
    toggleCalls += 1;
    return toggleCompleter.future;
  }

  @override
  Future<Result<LikeStats>> getLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async {
    return Result.success(stats);
  }

  @override
  Future<Result<bool>> hasUserLiked({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async {
    return Result.success(stats.isLikedByCurrentUser);
  }

  @override
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) {
    return Stream<LikeStats>.value(stats);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

AuthStateAuthenticated _authenticatedState() {
  return AuthStateAuthenticated(
    AuthUser(
      id: 'viewer-1',
      createdAt: DateTime.utc(2026, 7, 23),
      updatedAt: DateTime.utc(2026, 7, 23),
      email: 'viewer@example.com',
      username: 'viewer',
      avatarUrl: null,
      isEmailVerified: true,
      roles: const [],
      provider: ShonaAuthProvider.email,
      lifecycle: ContentLifecycle.active,
    ),
    emailVerified: true,
  );
}

Widget _wrap(
  Widget child, {
  LikeRepository? likeRepository,
  AuthState? authState,
}) {
  final repo =
      likeRepository ??
      _ControlledLikeRepository(
        const LikeStats(
          targetId: 'content-1',
          targetType: LikeTargetType.content,
          totalLikes: 4,
          isLikedByCurrentUser: true,
        ),
      );
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(authState ?? _authenticatedState()),
      ),
      likeRepositoryProvider.overrideWithValue(repo),
    ],
    child: MaterialApp(home: Scaffold(body: child)),
  );
}

void main() {
  testWidgets('renders canonical liked state and count', (tester) async {
    await tester.pumpWidget(
      _wrap(
        const ContentLikeAction(
          contentId: 'content-1',
          contentOwnerId: 'author-1',
          fallbackLikeCount: 4,
          showLabel: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byIcon(Icons.favorite), findsOneWidget);
    expect(find.text('4'), findsOneWidget);
    expect(find.text('Like'), findsOneWidget);
  });

  testWidgets('single widget deduplicates rapid taps while mutation in flight', (
    tester,
  ) async {
    final repo = _ControlledLikeRepository(
      const LikeStats(
        targetId: 'content-1',
        targetType: LikeTargetType.content,
        totalLikes: 4,
        isLikedByCurrentUser: false,
      ),
    );

    await tester.pumpWidget(
      _wrap(
        const ContentLikeAction(
          contentId: 'content-1',
          contentOwnerId: 'author-1',
          fallbackLikeCount: 4,
        ),
        likeRepository: repo,
      ),
    );
    await tester.pumpAndSettle();

    // Tap the same widget twice rapidly — second tap should be deduped
    await tester.tap(find.byType(ContentLikeAction));
    await tester.pump();
    await tester.tap(find.byType(ContentLikeAction));
    await tester.pump();

    expect(repo.toggleCalls, 1);

    repo.toggleCompleter.complete(Result.success(true));
    await tester.pumpAndSettle();
  });
}
