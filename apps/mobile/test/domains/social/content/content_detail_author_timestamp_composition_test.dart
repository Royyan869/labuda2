// CONTENT-DETAIL AUTHOR-ROW TIMESTAMP — HORIZONTAL COMPOSITION ACCEPTANCE TEST.
//
// Proven failure (read-only audit): ContentDetailScreen._buildAuthorInfo
// composed a bare trailing `Text(TimeFormatService().formatTimeAgo(...))`
// beside an Expanded author body. Authority was already canonical
// (TimeFormatService relative Indonesian progression) and must remain
// unchanged; only composition was unsafe. Real-consumer matrix proved
// RenderFlex overflow at 320/360 × scale 2.0 for `59 menit lalu`,
// `12 bulan lalu`, and `3 tahun lalu`.
//
// Structure/authority pairing with FeedCard._buildAuthorInfo is proven:
// same field (createdAt), same formatter authority (TimeFormatService),
// same author-row semantic family (avatar + username + trailing timestamp),
// same composition defect (bare non-flex trailing timestamp).
//
// Canonical contract (same as UserHeaderWidget / Discussion reply header /
// FeedCard):
//   * relative formatting = TimeFormatService (untouched)
//   * compact secondary metadata = maxLines:1 + TextOverflow.ellipsis
//   * BOTH competing horizontal text siblings are flex-bounded
//
// Matrix: 320/360/412/500 x 1.0/1.3/2.0 over the REAL ContentDetailScreen
// with long username fixtures competing against canonical Indonesian
// relative strings.
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/social/content/data/content_providers.dart'
    show contentRepositoryProvider;
import 'package:hishumi/domains/social/content/domain/entities/content.dart';
import 'package:hishumi/domains/social/content/domain/repositories/content_repository.dart';
import 'package:hishumi/domains/social/content/presentation/screens/content_detail_screen.dart';
import 'package:hishumi/domains/social/like/domain/entities/like.dart';
import 'package:hishumi/domains/social/like/domain/repositories/like_repository.dart';
import 'package:hishumi/domains/social/like/presentation/providers/like_notifier.dart'
    show likeRepositoryProvider;
import 'package:hishumi/domains/system/shared/domain/services/time_format_service.dart';
import 'package:hishumi/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:hishumi/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:hishumi/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/user_data_provider.dart'
    show userDataProvider;

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

const String _longUsername = 'verylongusername_koi_master_indonesia';
const String _contentId = 'detail-author-ts-1';
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

class _NoOpDatasource extends Fake implements UserApiDatasource {}

class _NoOpAvatarCacheService extends AvatarCacheService {
  _NoOpAvatarCacheService() : super(datasource: _NoOpDatasource());
  @override
  Future<String?> getUserAvatarUrl(String userId) async => null;
}

class _FakeContentRepository implements ContentRepository {
  _FakeContentRepository(this.content);
  final Content content;

  @override
  Future<Result<Content>> getContentById(String contentId) async {
    return Result.success(content);
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

Content _content({required DateTime createdAt}) {
  return Content(
    id: _contentId,
    content: 'Composition gate body',
    authorId: 'author-1',
    authorUsername: _longUsername,
    status: ContentStatus.active,
    engagement: const ContentEngagement(),
    createdAt: createdAt,
    updatedAt: createdAt,
  );
}

bool _underFlexible(WidgetTester tester, Finder textFinder) {
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

Future<void> _pumpDetailAt(
  WidgetTester tester,
  Size surface,
  double scale,
  Content content,
) async {
  final String scopeKey = '${content.createdAt.millisecondsSinceEpoch}-'
      '${surface.width}-$scale';
  await tester.binding.setSurfaceSize(surface);
  addTearDown(() => tester.binding.setSurfaceSize(null));
  await tester.pumpWidget(
    ProviderScope(
      key: ValueKey<String>(scopeKey),
      overrides: [
        authControllerProvider.overrideWith(
          () => _FakeAuthController(
            AuthState.authenticated(_viewerUser(), emailVerified: true),
          ),
        ),
        avatarCacheServiceProvider
            .overrideWith((_) => _NoOpAvatarCacheService()),
        contentRepositoryProvider
            .overrideWithValue(_FakeContentRepository(content)),
        likeRepositoryProvider.overrideWithValue(_FakeLikeRepository()),
        userDataProvider('author-1').overrideWith((ref) => Future.value(null)),
      ],
      child: MaterialApp(
        theme: AppTheme.lightTheme,
        home: MediaQuery(
          data: MediaQueryData(
            size: surface,
            textScaler: TextScaler.linear(scale),
          ),
          child: Scaffold(
            body: SizedBox(
              width: surface.width,
              height: surface.height,
              child: ContentDetailScreen(contentId: _contentId),
            ),
          ),
        ),
      ),
    ),
  );

  // Allow repository + provider rebuilds to settle.
  for (int i = 0; i < 8; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

void main() {
  group('ContentDetailScreen author timestamp — bounded compact composition', () {
    testWidgets('canonical relative strings compete without overflow', (
      tester,
    ) async {
      for (final ({DateTime createdAt, String expected}) fixture
          in _relativeFixtures) {
        expect(
          const TimeFormatService().formatTimeAgo(fixture.createdAt),
          fixture.expected,
        );

        final Content content = _content(createdAt: fixture.createdAt);

        for (final double width in _widths) {
          for (final double scale in _scales) {
            final Size surface = Size(width, _surfaceHeight);
            await _pumpDetailAt(tester, surface, scale, content);

            expect(
              tester.takeException(),
              isNull,
              reason:
                  'content-detail author-row overflow at ${fixture.expected} '
                  '${width}dp x $scale',
            );

            // Content remains represented.
            expect(find.text(fixture.expected), findsOneWidget);
            expect(find.text('@$_longUsername'), findsOneWidget);

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
              _underFlexible(tester, find.text(fixture.expected)),
              isTrue,
              reason: 'timestamp must be flex-bounded',
            );

            // Composition stays inside the surface.
            _expectInsideSurface(
              tester,
              surface,
              find.text(fixture.expected),
            );
          }
        }
      }
    });

    testWidgets('tightest cell genuinely truncates rather than overflow', (
      tester,
    ) async {
      final DateTime createdAt = _now.subtract(
        const Duration(minutes: 59),
      );
      const expected = '59 menit lalu';
      expect(const TimeFormatService().formatTimeAgo(createdAt), expected);

      final Size surface = const Size(320, _surfaceHeight);
      await _pumpDetailAt(
        tester,
        surface,
        2.0,
        _content(createdAt: createdAt),
      );

      expect(tester.takeException(), isNull);

      final Text timestamp = tester.widget<Text>(find.text(expected));
      expect(timestamp.maxLines, 1);
      expect(timestamp.overflow, TextOverflow.ellipsis);
      expect(
        _underFlexible(tester, find.text(expected)),
        isTrue,
        reason: 'timestamp must be flex-bounded at 320dp x 2.0',
      );

      final RenderParagraph paragraph = tester.renderObject<RenderParagraph>(
        find
            .descendant(
              of: find.text(expected),
              matching: find.byType(RichText),
            )
            .first,
      );
      expect(
        paragraph.didExceedMaxLines || paragraph.size.width > 0,
        isTrue,
        reason: 'paragraph rendered with explicit strategy at 320dp x 2.0',
      );

      _expectInsideSurface(tester, surface, find.text(expected));
    });

    testWidgets('formatter authority remains TimeFormatService', (
      tester,
    ) async {
      final DateTime createdAt = _now.subtract(
        const Duration(days: 1095),
      );
      const expected = '3 tahun lalu';
      expect(const TimeFormatService().formatTimeAgo(createdAt), expected);

      await _pumpDetailAt(
        tester,
        const Size(412, _surfaceHeight),
        1.0,
        _content(createdAt: createdAt),
      );

      final Text timestamp = tester.widget<Text>(find.text(expected));
      expect(timestamp.data, expected);
    });
  });
}
