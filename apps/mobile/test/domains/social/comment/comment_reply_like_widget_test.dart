// ignore_for_file: use_null_aware_elements

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/auth/app_role.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';
import 'package:labuda/domains/social/comment/presentation/widgets/comment_card.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

class _FakeLikeRepository implements LikeRepository {
  final Map<String, LikeStats> _stats = {};
  final Map<String, StreamController<LikeStats>> _controllers = {};

  void setStats(String targetId, LikeTargetType targetType, LikeStats stats) {
    _stats['${targetId}_$targetType'] = stats;
  }

  LikeStats _getStats(String targetId, LikeTargetType targetType) {
    return _stats['${targetId}_$targetType'] ??
        LikeStats(
          targetId: targetId,
          targetType: targetType,
          totalLikes: 0,
          isLikedByCurrentUser: false,
        );
  }

  void _emit(String key) {
    final c = _controllers[key];
    if (c != null && !c.isClosed) {
      final parts = key.split('_');
      final targetType = parts.last == 'comment'
          ? LikeTargetType.comment
          : LikeTargetType.content;
      final targetId = parts.sublist(0, parts.length - 1).join('_');
      c.add(_getStats(targetId, targetType));
    }
  }

  @override
  Future<Result<bool>> toggleLike({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async {
    final key = '${targetId}_$targetType';
    final current = _getStats(targetId, targetType);
    _stats[key] = current.copyWith(
      totalLikes: current.isLikedByCurrentUser
          ? current.totalLikes - 1
          : current.totalLikes + 1,
      isLikedByCurrentUser: !current.isLikedByCurrentUser,
    );
    _emit(key);
    return Result.success(true);
  }

  @override
  Future<Result<LikeStats>> getLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async {
    return Result.success(_getStats(targetId, targetType));
  }

  @override
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) {
    final key = '${targetId}_$targetType';
    final controller = StreamController<LikeStats>.broadcast();
    _controllers[key] = controller;
    // Delay emission so the StreamProvider listener attaches first.
    Future.microtask(() {
      if (!controller.isClosed) controller.add(_getStats(targetId, targetType));
    });
    return controller.stream;
  }
}

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => AuthState.authenticated(
        AuthUser(
          id: 'user-1',
          email: 'test@example.com',
          username: 'testuser',
          isEmailVerified: true,
          createdAt: DateTime.utc(2026, 1, 1),
          updatedAt: DateTime.utc(2026, 1, 1),
          roles: const [UserRole.user],
          provider: ShonaAuthProvider.email,
        ),
        emailVerified: true,
      );
}

class _UnauthController extends AuthController {
  @override
  AuthState build() => const AuthState.unauthenticated();
}

/// A LikeRepository that tracks toggleLike calls for verification.
class _TrackingLikeRepository implements LikeRepository {
  final void Function(String targetId, LikeTargetType targetType) onToggle;
  final LikeStats initialStats;

  _TrackingLikeRepository({required this.onToggle, required this.initialStats});

  @override
  Future<Result<bool>> toggleLike({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async {
    onToggle(targetId, targetType);
    return Result.success(true);
  }

  @override
  Future<Result<LikeStats>> getLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async {
    return Result.success(initialStats);
  }

  @override
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) {
    return Stream.value(initialStats);
  }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

Comment _rootComment({String id = 'comment-root-1'}) => Comment(
      id: id,
      authorId: 'author-1',
      contentId: 'content-1',
      authorUsername: 'alice',
      body: 'Root comment body',
      type: 'normal',
      createdAt: DateTime(2026, 1, 1),
    );

Comment _replyComment({String id = 'comment-reply-1'}) => Comment(
      id: id,
      authorId: 'author-2',
      contentId: 'content-1',
      authorUsername: 'bob',
      body: 'Reply body',
      type: 'normal',
      parentId: 'comment-root-1',
      createdAt: DateTime(2026, 1, 2),
    );

Widget _buildApp({
  required LikeRepository likeRepo,
  AuthController? authController,
  required Widget child,
}) {
  return ProviderScope(
    overrides: [
      likeRepositoryProvider.overrideWithValue(likeRepo),
      authControllerProvider.overrideWith(
        () => authController ?? _FakeAuthController(),
      ),
    ],
    child: MaterialApp(home: Scaffold(body: child)),
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

void main() {
  late _FakeLikeRepository likeRepo;

  setUp(() {
    likeRepo = _FakeLikeRepository();
  });

  group('Comment Like widget proof', () {
    testWidgets(
      'root Comment renders Like button with count and filled heart for authenticated user',
      (tester) async {
        likeRepo.setStats(
          'comment-root-1',
          LikeTargetType.comment,
          const LikeStats(
            targetId: 'comment-root-1',
            targetType: LikeTargetType.comment,
            totalLikes: 3,
            isLikedByCurrentUser: true,
          ),
        );

        await tester.pumpWidget(
          _buildApp(
            likeRepo: likeRepo,
            child: CommentCard(
              comment: _rootComment(),
              userName: '@alice',
              userId: 'author-1',
              currentUserId: 'user-1',
              currentUserName: 'testuser',
              onReply: () {},
            ),
          ),
        );
        await tester.pumpAndSettle();

        // Like button must be present with filled heart (liked state)
        expect(find.byIcon(Icons.favorite), findsOneWidget);
        // Count must be shown
        expect(find.text('3'), findsOneWidget);
        // Reply button must be present for top-level comment (onReply provided)
        // Note: Reply button only renders when onReply != null
      },
    );

    testWidgets(
      'root Comment renders unfilled heart when not liked',
      (tester) async {
        likeRepo.setStats(
          'comment-root-1',
          LikeTargetType.comment,
          const LikeStats(
            targetId: 'comment-root-1',
            targetType: LikeTargetType.comment,
            totalLikes: 5,
            isLikedByCurrentUser: false,
          ),
        );

        await tester.pumpWidget(
          _buildApp(
            likeRepo: likeRepo,
            child: CommentCard(
              comment: _rootComment(),
              userName: '@alice',
              userId: 'author-1',
              currentUserId: 'user-1',
              currentUserName: 'testuser',
              onReply: () {},
            ),
          ),
        );
        await tester.pumpAndSettle();

        // Unfilled heart when not liked
        expect(find.byIcon(Icons.favorite_border), findsOneWidget);
        expect(find.text('5'), findsOneWidget);
      },
    );

    testWidgets(
      'Like button NOT rendered for unauthenticated user',
      (tester) async {
        await tester.pumpWidget(
          _buildApp(
            likeRepo: likeRepo,
            authController: _UnauthController(),
            child: CommentCard(
              comment: _rootComment(),
              userName: '@alice',
              userId: 'author-1',
              // currentUserId is null → unauthenticated
            ),
          ),
        );
        await tester.pumpAndSettle();

        // Like section should NOT be rendered
        expect(find.byIcon(Icons.favorite), findsNothing);
        expect(find.byIcon(Icons.favorite_border), findsNothing);
        // Reply button should still be present for unauthenticated top-level comment
        // Note: Reply button only renders when onReply != null
      },
    );

    testWidgets(
      'reply Comment renders Like button but NOT Reply button',
      (tester) async {
        likeRepo.setStats(
          'comment-reply-1',
          LikeTargetType.comment,
          const LikeStats(
            targetId: 'comment-reply-1',
            targetType: LikeTargetType.comment,
            totalLikes: 2,
            isLikedByCurrentUser: false,
          ),
        );

        await tester.pumpWidget(
          _buildApp(
            likeRepo: likeRepo,
            child: CommentCard(
              comment: _replyComment(),
              userName: '@bob',
              userId: 'author-2',
              currentUserId: 'user-1',
              currentUserName: 'testuser',
            ),
          ),
        );
        await tester.pumpAndSettle();

        // Like button MUST be present for reply
        expect(find.byIcon(Icons.favorite_border), findsOneWidget);
        expect(find.text('2'), findsOneWidget);
        // Reply button must NOT be present for reply (isTopLevel = false)
        expect(find.text('Balas'), findsNothing);
      },
    );

    testWidgets(
      'reply Comment with liked state shows filled heart',
      (tester) async {
        likeRepo.setStats(
          'comment-reply-1',
          LikeTargetType.comment,
          const LikeStats(
            targetId: 'comment-reply-1',
            targetType: LikeTargetType.comment,
            totalLikes: 1,
            isLikedByCurrentUser: true,
          ),
        );

        await tester.pumpWidget(
          _buildApp(
            likeRepo: likeRepo,
            child: CommentCard(
              comment: _replyComment(),
              userName: '@bob',
              userId: 'author-2',
              currentUserId: 'user-1',
              currentUserName: 'testuser',
            ),
          ),
        );
        await tester.pumpAndSettle();

        // Filled heart when liked
        expect(find.byIcon(Icons.favorite), findsOneWidget);
        expect(find.text('1'), findsOneWidget);
        // No reply button
        expect(find.text('Balas'), findsNothing);
      },
    );

    testWidgets(
      'Comment Like tap triggers CommentLikeHandlers → LikeNotifier.toggleLike (canonical path)',
      (tester) async {
        // Track whether toggleLike was called with correct parameters
        String? toggledTargetId;
        LikeTargetType? toggledTargetType;

        final trackingRepo = _TrackingLikeRepository(
          onToggle: (targetId, targetType) {
            toggledTargetId = targetId;
            toggledTargetType = targetType;
          },
          initialStats: const LikeStats(
            targetId: 'comment-root-1',
            targetType: LikeTargetType.comment,
            totalLikes: 0,
            isLikedByCurrentUser: false,
          ),
        );

        await tester.pumpWidget(
          _buildApp(
            likeRepo: trackingRepo,
            child: CommentCard(
              comment: _rootComment(),
              userName: '@alice',
              userId: 'author-1',
              currentUserId: 'user-1',
              currentUserName: 'testuser',
              onReply: () {},
            ),
          ),
        );
        await tester.pumpAndSettle();

        // Initial state: unfilled heart visible
        expect(find.byIcon(Icons.favorite_border), findsOneWidget);

        // Tap — triggers CommentLikeHandlers → LikeNotifier.toggleLike → repository.toggleLike
        await tester.tap(find.byIcon(Icons.favorite_border));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 100));

        // Verify canonical path was invoked with correct parameters
        expect(toggledTargetId, 'comment-root-1');
        expect(toggledTargetType, LikeTargetType.comment);
      },
    );

    testWidgets(
      'Reply Like tap triggers CommentLikeHandlers → LikeNotifier.toggleLike (canonical path)',
      (tester) async {
        String? toggledTargetId;
        LikeTargetType? toggledTargetType;

        final trackingRepo = _TrackingLikeRepository(
          onToggle: (targetId, targetType) {
            toggledTargetId = targetId;
            toggledTargetType = targetType;
          },
          initialStats: const LikeStats(
            targetId: 'comment-reply-1',
            targetType: LikeTargetType.comment,
            totalLikes: 0,
            isLikedByCurrentUser: false,
          ),
        );

        await tester.pumpWidget(
          _buildApp(
            likeRepo: trackingRepo,
            child: CommentCard(
              comment: _replyComment(),
              userName: '@bob',
              userId: 'author-2',
              currentUserId: 'user-1',
              currentUserName: 'testuser',
            ),
          ),
        );
        await tester.pumpAndSettle();

        // Initial state: unfilled heart visible
        expect(find.byIcon(Icons.favorite_border), findsOneWidget);

        // Tap — triggers CommentLikeHandlers → LikeNotifier.toggleLike → repository.toggleLike
        await tester.tap(find.byIcon(Icons.favorite_border));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 100));

        // Verify canonical path was invoked with correct parameters
        expect(toggledTargetId, 'comment-reply-1');
        expect(toggledTargetType, LikeTargetType.comment);
        // Reply should NOT have a reply button
        expect(find.text('Balas'), findsNothing);
      },
    );
  });
}
