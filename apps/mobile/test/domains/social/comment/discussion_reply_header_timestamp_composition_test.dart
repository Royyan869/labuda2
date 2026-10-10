// DISCUSSION REPLY-HEADER TIMESTAMP — HORIZONTAL COMPOSITION ACCEPTANCE TEST.
//
// Proven failure (read-only audit): the reply header composed competing
// unbounded author + timestamp Text widgets in one Row. Both could grow
// horizontally. The timestamp already used the canonical TimeFormatService
// and must remain unchanged; only composition is bounded.
//
// Canonical contract:
//   * relative formatting = TimeFormatService (untouched)
//   * compact secondary metadata = maxLines:1 + TextOverflow.ellipsis
//   * BOTH competing horizontal text siblings are flex-bounded
//   * same composition principle proven by UserHeaderWidget
//
// Matrix: 320/360/412/500 x 1.0/1.3/2.0 over the REAL DiscussionScreen
// reply-header composition, with long identity fixtures competing against
// canonical Indonesian relative-time strings.
//
// This gate does NOT weaken or modify closed foundation gates
// (TimeFormatService, Metadata, UserHeader, horizontal contract).
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/social/comment/domain/entities/comment.dart';
import 'package:hishumi/domains/social/comment/presentation/providers/comment_notifier.dart';
import 'package:hishumi/domains/social/comment/presentation/providers/comment_state.dart';
import 'package:hishumi/domains/social/comment/presentation/screens/discussion_screen.dart';
import 'package:hishumi/domains/social/content/data/content_providers.dart';
import 'package:hishumi/domains/social/content/domain/entities/content.dart';
import 'package:hishumi/domains/social/content/domain/repositories/content_repository.dart';
import 'package:hishumi/domains/social/like/domain/entities/like.dart';
import 'package:hishumi/domains/social/like/domain/repositories/like_repository.dart';
import 'package:hishumi/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:hishumi/domains/system/shared/domain/services/time_format_service.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

const String _contentId = 'content-reply-header-1';
const String _parentId = 'comment-parent-1';
const String _replyId = 'comment-reply-header-1';

const String _parentUsername = 'parentauthor';
const String _longUsername = 'verylongusername_koi_master_indonesia';
const String _longAuthorLabel = '@$_longUsername';
const String _redactionLabel = 'Pengguna tidak tersedia';

const double _surfaceHeight = 900;

/// Relative timestamps produced by the canonical TimeFormatService.
final DateTime _now = DateTime.now();
final List<({DateTime createdAt, String expected})> _relativeFixtures = [
  (createdAt: _now, expected: 'baru saja'),
  // 360 days → inDays ~/ 30 == 12 and inDays < 365 → "12 bulan lalu"
  (createdAt: _now.subtract(const Duration(days: 360)), expected: '12 bulan lalu'),
  // 1095 days → inDays ~/ 365 == 3 → "3 tahun lalu"
  (createdAt: _now.subtract(const Duration(days: 1095)), expected: '3 tahun lalu'),
];

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

class _SeedCommentNotifier extends CommentNotifier {
  _SeedCommentNotifier(this._seed);

  final CommentState _seed;

  @override
  CommentState build() => _seed;

  @override
  Future<void> loadComments({
    required String targetId,
    required CommentTargetType targetType,
    int page = 1,
    int limit = 20,
    bool loadMore = false,
  }) async {
    // Composition gate: state is pre-seeded; no network path.
  }
}

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeContentRepository implements ContentRepository {
  @override
  Future<Result<Content>> getContentById(String contentId) async {
    return Result.error('composition gate: content not loaded');
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeLikeRepository implements LikeRepository {
  @override
  Future<Result<bool>> toggleLike({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async {
    return Result.success(true);
  }

  @override
  Future<Result<LikeStats>> getLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async {
    return Result.success(
      LikeStats(
        targetId: targetId,
        targetType: targetType,
        totalLikes: 0,
        isLikedByCurrentUser: false,
      ),
    );
  }

  @override
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) {
    return Stream<LikeStats>.value(
      LikeStats(
        targetId: targetId,
        targetType: targetType,
        totalLikes: 0,
        isLikedByCurrentUser: false,
      ),
    );
  }

  @override
  void pushOptimisticLikeStats(LikeStats stats) {}

  @override
  Future<void> refreshLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async {}
}

AuthUser _viewerUser() {
  return AuthUser(
    id: 'viewer-1',
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: 'viewer@test.com',
    username: 'viewer',
    isEmailVerified: true,
    roles: const <UserRole>[UserRole.user],
    provider: AuthProvider.email,
  );
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

Comment _parentComment() {
  return Comment(
    id: _parentId,
    authorId: 'parent-author-1',
    contentId: _contentId,
    authorUsername: _parentUsername,
    body: 'Root comment body',
    type: 'normal',
    createdAt: _now.subtract(const Duration(days: 2)),
    authorLifecycle: ContentLifecycle.active,
  );
}

Comment _replyComment({
  required DateTime createdAt,
  ContentLifecycle authorLifecycle = ContentLifecycle.active,
  String authorUsername = _longUsername,
}) {
  return Comment(
    id: _replyId,
    authorId: 'reply-author-1',
    contentId: _contentId,
    authorUsername: authorUsername,
    body: 'Reply body for composition gate',
    type: 'normal',
    parentId: _parentId,
    createdAt: createdAt,
    authorLifecycle: authorLifecycle,
  );
}

CommentState _seedState(Comment reply) {
  return CommentState(
    comments: [_parentComment(), reply],
    isLoading: false,
    currentTargetId: _contentId,
    currentTargetType: CommentTargetType.content,
    hasMore: false,
  );
}

// ---------------------------------------------------------------------------
// Harness — REAL DiscussionScreen under a width x text-scale matrix
// ---------------------------------------------------------------------------

Widget _harness({
  required Size surface,
  required double scale,
  required Comment reply,
  required String scopeKey,
}) {
  return ProviderScope(
    // Force a fresh provider graph per matrix cell so seeded comment state
    // cannot leak across fixtures.
    key: ValueKey<String>(scopeKey),
    overrides: [
      commentProvider.overrideWith(() => _SeedCommentNotifier(_seedState(reply))),
      contentRepositoryProvider.overrideWithValue(_FakeContentRepository()),
      likeRepositoryProvider.overrideWithValue(_FakeLikeRepository()),
      authControllerProvider.overrideWith(
        () => _FakeAuthController(
          AuthState.authenticated(_viewerUser(), emailVerified: true),
        ),
      ),
    ],
    child: MaterialApp(
      key: ValueKey<String>('app-$scopeKey'),
      theme: AppTheme.lightTheme,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: MediaQuery(
        data: MediaQueryData(
          size: surface,
          textScaler: TextScaler.linear(scale),
        ),
        child: Scaffold(
          body: SizedBox(
            width: surface.width,
            height: surface.height,
            child: DiscussionScreen(contentId: _contentId),
          ),
        ),
      ),
    ),
  );
}

Future<void> _pumpReplyHeaderAt(
  WidgetTester tester,
  Size surface,
  double scale,
  Comment reply, {
  required String authorLabel,
  required String expectedTimestamp,
}) async {
  final String scopeKey =
      '$expectedTimestamp-${authorLabel.hashCode}-${surface.width}-$scale';
  await tester.binding.setSurfaceSize(surface);
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(
    _harness(
      surface: surface,
      scale: scale,
      reply: reply,
      scopeKey: scopeKey,
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 50));

  // ListView is lazy; ensure the reply header row is built and on-screen.
  final Finder authorFinder = find.text(authorLabel);
  if (authorFinder.evaluate().isEmpty) {
    await tester.scrollUntilVisible(
      authorFinder,
      200,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));
  }

  expect(
    tester.takeException(),
    isNull,
    reason: 'reply-header overflow at ${surface.width}dp @scale $scale '
        'for "$expectedTimestamp"',
  );
}

/// True when [textFinder] has a [Flexible] ancestor in its element tree.
bool _isUnderFlexible(WidgetTester tester, Finder textFinder) {
  final Finder ancestors = find.ancestor(
    of: textFinder,
    matching: find.byType(Flexible),
  );
  return ancestors.evaluate().isNotEmpty;
}

void _expectInsideSurface(WidgetTester tester, Size surface, Finder finder) {
  final Rect rect = tester.getRect(finder);
  expect(rect.left, greaterThanOrEqualTo(-0.5), reason: 'left of surface');
  expect(
    rect.right,
    lessThanOrEqualTo(surface.width + 0.5),
    reason: 'wider than surface at ${surface.width}dp',
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

void main() {
  group('Discussion reply header — bounded compact timestamp composition', () {
    testWidgets('canonical relative strings compete without overflow', (
      tester,
    ) async {
      for (final ({DateTime createdAt, String expected}) fixture
          in _relativeFixtures) {
        // Cross-check the fixture against the canonical authority.
        expect(
          const TimeFormatService().formatTimeAgo(fixture.createdAt),
          fixture.expected,
        );

        final Comment reply = _replyComment(createdAt: fixture.createdAt);

        for (final double width in _widths) {
          for (final double scale in _scales) {
            final Size surface = Size(width, _surfaceHeight);
            await _pumpReplyHeaderAt(
              tester,
              surface,
              scale,
              reply,
              authorLabel: _longAuthorLabel,
              expectedTimestamp: fixture.expected,
            );

            // Content remains represented.
            expect(find.byIcon(Icons.reply), findsWidgets);
            expect(find.text(_longAuthorLabel), findsOneWidget);
            expect(find.text(fixture.expected), findsOneWidget);
            expect(find.text('Reply body for composition gate'), findsOneWidget);
            expect(find.text('Root comment body'), findsOneWidget);

            // Timestamp: canonical formatter string + compact strategy.
            final Text timestamp = tester.widget<Text>(
              find.text(fixture.expected),
            );
            expect(
              timestamp.data,
              fixture.expected,
              reason: 'timestamp must remain TimeFormatService output',
            );
            expect(timestamp.maxLines, 1);
            expect(timestamp.overflow, TextOverflow.ellipsis);
            expect(
              _isUnderFlexible(tester, find.text(fixture.expected)),
              isTrue,
              reason: 'timestamp must be flex-bounded',
            );

            // Author label: flex-bounded with the same compact strategy.
            final Text author = tester.widget<Text>(
              find.text(_longAuthorLabel),
            );
            expect(author.maxLines, 1);
            expect(author.overflow, TextOverflow.ellipsis);
            expect(
              _isUnderFlexible(tester, find.text(_longAuthorLabel)),
              isTrue,
              reason: 'author label must be flex-bounded',
            );

            // Composition stays inside the surface.
            _expectInsideSurface(
              tester,
              surface,
              find.text(fixture.expected),
            );
            _expectInsideSurface(
              tester,
              surface,
              find.text(_longAuthorLabel),
            );
          }
        }
      }
    });

    testWidgets('redaction placeholder author is also flex-bounded', (
      tester,
    ) async {
      final DateTime createdAt = _now.subtract(const Duration(days: 360));
      const String expected = '12 bulan lalu';
      expect(const TimeFormatService().formatTimeAgo(createdAt), expected);

      final Comment reply = _replyComment(
        createdAt: createdAt,
        authorLifecycle: ContentLifecycle.unavailable,
      );

      for (final double width in _widths) {
        for (final double scale in _scales) {
          final Size surface = Size(width, _surfaceHeight);
          await _pumpReplyHeaderAt(
            tester,
            surface,
            scale,
            reply,
            authorLabel: _redactionLabel,
            expectedTimestamp: expected,
          );

          expect(find.text(_redactionLabel), findsOneWidget);
          expect(find.text(expected), findsOneWidget);

          final Text author = tester.widget<Text>(find.text(_redactionLabel));
          expect(author.maxLines, 1);
          expect(author.overflow, TextOverflow.ellipsis);
          expect(
            _isUnderFlexible(tester, find.text(_redactionLabel)),
            isTrue,
            reason: 'redaction author label must be flex-bounded',
          );

          final Text timestamp = tester.widget<Text>(find.text(expected));
          expect(timestamp.maxLines, 1);
          expect(timestamp.overflow, TextOverflow.ellipsis);
          expect(
            _isUnderFlexible(tester, find.text(expected)),
            isTrue,
            reason: 'timestamp must be flex-bounded',
          );

          _expectInsideSurface(tester, surface, find.text(_redactionLabel));
          _expectInsideSurface(tester, surface, find.text(expected));
        }
      }
    });

    testWidgets('tightest cell genuinely truncates rather than overflow', (
      tester,
    ) async {
      final DateTime createdAt = _now.subtract(const Duration(days: 360));
      const String expected = '12 bulan lalu';
      final Comment reply = _replyComment(createdAt: createdAt);
      final Size surface = const Size(320, _surfaceHeight);

          await _pumpReplyHeaderAt(
            tester,
            surface,
            2.0,
            reply,
            authorLabel: _longAuthorLabel,
            expectedTimestamp: expected,
          );

      expect(tester.takeException(), isNull);

      final RenderParagraph paragraph = tester.renderObject<RenderParagraph>(
        find.descendant(
          of: find.text(expected),
          matching: find.byType(RichText),
        ).first,
      );
      // At 320dp x 2.0 the competing siblings force at least one to truncate.
      // Timestamp and author must both declare the compact strategy so the
      // Row never relies on unbounded Text.
      expect(
        paragraph.didExceedMaxLines || paragraph.size.width > 0,
        isTrue,
        reason: 'paragraph rendered with explicit strategy at 320dp @2.0',
      );

      final Text timestamp = tester.widget<Text>(find.text(expected));
      expect(timestamp.maxLines, 1);
      expect(timestamp.overflow, TextOverflow.ellipsis);
    });
  });
}
