// COMMENT CARD TIMESTAMP — COMPOSITION ACCEPTANCE TEST (KEEP).
//
// Scope pin: ONLY the timestamp on CommentCard. Discussion reply header,
// Feed Renderer, Content Detail, Rating, Chat, Support, Seller Earnings
// are out of scope and must not be touched by this gate.
//
// Audit evidence (real-consumer matrix probe, pre-test):
//   * Authority: TimeFormatService (canonical relative Indonesian) — KEEP.
//   * Placement: timestamp is a Column child under Expanded author body,
//     NOT a horizontal sibling of author name/username/actions.
//   * Owner path (no seller badge): no RenderFlex overflow across
//     320/360/412/500 × 1.0/1.3/2.0 for all canonical relative strings.
//   * Guest seller path: RenderFlex overflow is proven, but it is the
//     name Row (Flexible displayName + fixed "Respons Penjual" badge)
//     competing inside Expanded — NOT the timestamp. Timestamp rect stays
//     inside the surface on every cell even when that badge Row overflows.
//
// Contract for THIS target:
//   * relative formatting = TimeFormatService (untouched)
//   * timestamp remains a Column child (vertical position) — do NOT force
//     Flexible merely for uniformity with Row consumers
//   * no production change is justified for the timestamp itself
//
// Matrix: 320/360/412/500 x 1.0/1.3/2.0 over the REAL CommentCard.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';
import 'package:labuda/domains/social/comment/presentation/widgets/comment_card.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart'
    show likeRepositoryProvider;
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

const String _longUsername = 'verylongusername_koi_master_indonesia';
const String _longDisplayName = 'Koi Farm Nusantara Jaya Sentosa Premium';
const String _contentId = 'comment-card-ts-1';
const double _surfaceHeight = 900;

final DateTime _now = DateTime.now();
final List<({DateTime createdAt, String expected})> _relativeFixtures = [
  (createdAt: _now, expected: 'baru saja'),
  (
    createdAt: _now.subtract(const Duration(minutes: 59)),
    expected: '59 menit lalu',
  ),
  (
    createdAt: _now.subtract(const Duration(days: 360)),
    expected: '12 bulan lalu',
  ),
  (
    createdAt: _now.subtract(const Duration(days: 1095)),
    expected: '3 tahun lalu',
  ),
];

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);
  final AuthState _state;
  @override
  AuthState build() => _state;
}

class _FakeLikeRepository implements LikeRepository {
  @override
  Future<Result<bool>> toggleLike({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async {
    return Result.success(false);
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
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
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

Comment _comment({required DateTime createdAt, bool seller = false}) {
  return Comment(
    id: 'comment-1',
    authorId: 'author-1',
    contentId: _contentId,
    authorUsername: _longUsername,
    body: 'Composition gate body for CommentCard',
    type: seller ? 'commerce_reference' : 'normal',
    createdAt: createdAt,
  );
}

Widget _harness({
  required Size surface,
  required double scale,
  required Widget subject,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(
          AuthState.authenticated(_viewerUser(), emailVerified: true),
        ),
      ),
      likeRepositoryProvider.overrideWithValue(_FakeLikeRepository()),
    ],
    child: MaterialApp(
      theme: AppTheme.lightTheme,
      home: MediaQuery(
        data: MediaQueryData(
          size: surface,
          textScaler: TextScaler.linear(scale),
        ),
        child: Scaffold(
          body: Center(
            child: SizedBox(
              width: surface.width - 24,
              height: _surfaceHeight,
              child: SingleChildScrollView(child: subject),
            ),
          ),
        ),
      ),
    ),
  );
}

bool _underFlexible(WidgetTester tester, Finder textFinder) {
  final Finder ancestors = find.ancestor(
    of: textFinder,
    matching: find.byType(Flexible),
  );
  return ancestors.evaluate().isNotEmpty;
}

bool _underColumn(WidgetTester tester, Finder textFinder) {
  final Finder ancestors = find.ancestor(
    of: textFinder,
    matching: find.byType(Column),
  );
  return ancestors.evaluate().isNotEmpty;
}

bool _underRow(WidgetTester tester, Finder textFinder) {
  final Finder ancestors = find.ancestor(
    of: textFinder,
    matching: find.byType(Row),
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

void main() {
  group('CommentCard timestamp — Column placement KEEP contract', () {
    testWidgets(
      'owner path: canonical relative strings stay inside with no overflow',
      (tester) async {
        for (final ({DateTime createdAt, String expected}) fixture
            in _relativeFixtures) {
          expect(
            const TimeFormatService().formatTimeAgo(fixture.createdAt),
            fixture.expected,
          );

          for (final double width in _widths) {
            for (final double scale in _scales) {
              final Size surface = Size(width, _surfaceHeight);
              await tester.binding.setSurfaceSize(surface);
              addTearDown(() => tester.binding.setSurfaceSize(null));
              await tester.pumpWidget(
                _harness(
                  surface: surface,
                  scale: scale,
                  subject: CommentCard(
                    comment: _comment(createdAt: fixture.createdAt),
                    userName: _longDisplayName,
                    userUsername: _longUsername,
                    userId: 'author-1',
                    currentUserId: 'author-1',
                    currentUserName: 'author',
                  ),
                ),
              );
              await tester.pump();
              await tester.pump(const Duration(milliseconds: 50));

              expect(
                tester.takeException(),
                isNull,
                reason:
                    'owner CommentCard overflow at ${fixture.expected} '
                    '${width}dp x $scale',
              );

              expect(find.text(fixture.expected), findsOneWidget);
              expect(find.text(_longDisplayName), findsOneWidget);
              expect(find.text('@$_longUsername'), findsWidgets);

              final Text timestamp = tester.widget<Text>(
                find.text(fixture.expected),
              );
              expect(
                timestamp.data,
                fixture.expected,
                reason: 'timestamp must remain TimeFormatService output',
              );

              // KEEP: Column child, not a flex-bounded Row sibling.
              expect(
                _underColumn(tester, find.text(fixture.expected)),
                isTrue,
                reason: 'timestamp must remain under the author Column',
              );
              expect(
                _underFlexible(tester, find.text(fixture.expected)),
                isFalse,
                reason:
                    'timestamp must NOT be forced into Flexible for uniformity',
              );

              _expectInsideSurface(
                tester,
                surface,
                find.text(fixture.expected),
              );
              _expectInsideSurface(
                tester,
                surface,
                find.text(_longDisplayName),
              );
            }
          }
        }
      },
    );

    testWidgets(
      'guest seller path: timestamp itself stays inside even when badge Row overflows',
      (tester) async {
        // Audit isolation: seller-badge overflow is a separate composition
        // defect (Flexible displayName + fixed "Respons Penjual" badge).
        // This gate proves the TIMESTAMP target is still safe on that path.
        final DateTime createdAt = _now.subtract(
          const Duration(minutes: 59),
        );
        const expected = '59 menit lalu';
        expect(const TimeFormatService().formatTimeAgo(createdAt), expected);

        for (final double width in _widths) {
          for (final double scale in _scales) {
            final Size surface = Size(width, _surfaceHeight);
            await tester.binding.setSurfaceSize(surface);
            addTearDown(() => tester.binding.setSurfaceSize(null));
            await tester.pumpWidget(
              _harness(
                surface: surface,
                scale: scale,
                subject: CommentCard(
                  comment: _comment(createdAt: createdAt, seller: true),
                  userName: _longDisplayName,
                  userUsername: _longUsername,
                  userId: 'author-1',
                  currentUserId: 'viewer-1',
                  currentUserName: 'viewer',
                ),
              ),
            );
            await tester.pump();
            await tester.pump(const Duration(milliseconds: 50));

            // Consume any exception (including the known badge-Row overflow)
            // so the test can assert timestamp safety independently.
            final Object? exception = tester.takeException();

            final Finder tsFinder = find.text(expected);
            expect(tsFinder, findsOneWidget);

            final Text timestamp = tester.widget<Text>(tsFinder);
            expect(timestamp.data, expected);

            final Rect rect = tester.getRect(tsFinder);
            expect(
              rect.left,
              greaterThanOrEqualTo(-0.5),
              reason: 'timestamp left of surface at ${width}dp x $scale',
            );
            expect(
              rect.right,
              lessThanOrEqualTo(surface.width + 0.5),
              reason:
                  'timestamp wider than surface at ${width}dp x $scale '
                  '(exception=$exception)',
            );

            expect(
              _underColumn(tester, tsFinder),
              isTrue,
              reason: 'timestamp remains a Column child on guest seller path',
            );
            expect(
              _underFlexible(tester, tsFinder),
              isFalse,
              reason: 'timestamp remains unflexed Column child',
            );

            // If an exception occurred, it must not be attributed to the
            // timestamp Text being wider than the surface.
            if (exception != null) {
              expect(
                rect.right,
                lessThanOrEqualTo(surface.width + 0.5),
                reason:
                    'timestamp must stay inside even when sibling composition '
                    'overflows: $exception',
              );
            }
          }
        }
      },
    );

    testWidgets('formatter authority remains TimeFormatService', (
      tester,
    ) async {
      final DateTime createdAt = _now.subtract(
        const Duration(days: 360),
      );
      const expected = '12 bulan lalu';
      expect(const TimeFormatService().formatTimeAgo(createdAt), expected);

      await tester.binding.setSurfaceSize(const Size(412, _surfaceHeight));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      await tester.pumpWidget(
        _harness(
          surface: const Size(412, _surfaceHeight),
          scale: 1.0,
          subject: CommentCard(
            comment: _comment(createdAt: createdAt),
            userName: _longDisplayName,
            userUsername: _longUsername,
            userId: 'author-1',
          ),
        ),
      );
      await tester.pump();

      final Text timestamp = tester.widget<Text>(find.text(expected));
      expect(timestamp.data, expected);
    });

    testWidgets(
      'author name Row composition is independent of timestamp (KEEP evidence)',
      (tester) async {
        // Owner path: name Row is Flexible displayName with no badge.
        final DateTime createdAt = _now;
        await tester.binding.setSurfaceSize(const Size(320, _surfaceHeight));
        addTearDown(() => tester.binding.setSurfaceSize(null));
        await tester.pumpWidget(
          _harness(
            surface: const Size(320, _surfaceHeight),
            scale: 1.0,
            subject: CommentCard(
              comment: _comment(createdAt: createdAt),
              userName: _longDisplayName,
              userUsername: _longUsername,
              userId: 'author-1',
              currentUserId: 'author-1',
              currentUserName: 'author',
            ),
          ),
        );
        await tester.pump();

        expect(tester.takeException(), isNull);

        final Text author = tester.widget<Text>(
          find.text(_longDisplayName).first,
        );
        expect(author.maxLines, 1);
        expect(author.overflow, TextOverflow.ellipsis);
        expect(
          _underFlexible(tester, find.text(_longDisplayName).first),
          isTrue,
          reason: 'author name remains flex-bounded in its own Row',
        );

        final Finder tsFinder = find.text(
          const TimeFormatService().formatTimeAgo(createdAt),
        );
        expect(
          _underColumn(tester, tsFinder),
          isTrue,
          reason: 'timestamp remains under the author Column',
        );
        expect(
          _underFlexible(tester, tsFinder),
          isFalse,
          reason: 'timestamp is not a flex sibling of the author name',
        );
        expect(
          _underRow(tester, tsFinder),
          isTrue,
          reason:
              'timestamp is still inside the outer authorSection Row tree '
              '(as a Column descendant, not a Row child)',
        );
      },
    );
  });
}
