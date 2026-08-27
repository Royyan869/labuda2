import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/domains/social/content/data/content_providers.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';
import 'package:labuda/domains/social/content/domain/repositories/content_repository.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/profile_feed_tab.dart';
import 'package:labuda/shared/object/object_preview_provider.dart';
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';
import 'package:labuda/shared/widgets/repost_attribution_bar.dart';

class _FakeLogger implements ILoggerService {
  Result<void> _ok() => Result.success(null);

  @override
  Future<Result<void>> clearLogs() async => _ok();

  @override
  Future<Result<void>> debug(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _ok();

  @override
  Future<void> debugCallingGetCurrentUser() async {}

  @override
  Future<void> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async {}

  @override
  Future<void> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async {}

  @override
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  ) async {}

  @override
  Future<void> debugSync(String userId) async {}

  @override
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  ) async {}

  @override
  Future<void> debugSyncFailed(String userId, String? errorMessage) async {}

  @override
  Future<void> debugSyncSuccess(String userId) async {}

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => _ok();

  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) async => Result.success(const []);

  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => _ok();

  @override
  Future<Result<void>> info(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _ok();

  @override
  Future<Result<void>> log(
    String message, {
    LogLevel level = LogLevel.debug,
  }) async => _ok();

  @override
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
  }) async => _ok();

  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) async => _ok();

  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) async => _ok();

  @override
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) async => _ok();

  @override
  Future<Result<void>> setLogLevel(LogLevel level) async => _ok();

  @override
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _ok();
}

class _FakeContentRepository implements ContentRepository {
  final List<Content> items;

  _FakeContentRepository(this.items);

  ContentRepositoryResult<ContentAuthorPage> _page() {
    return ContentRepositoryResult.success(
      ContentAuthorPage(items: items, nextCursor: null, hasMore: false),
    );
  }

  @override
  Future<ContentRepositoryResult<Content>> createContent({
    required String authorId,
    String? authorUsername,
    String? authorAvatarUrl,
    required String content,
    ContentType type = ContentType.post,
    List<MediaEntity> media = const [],
    List<String> tags = const [],
    List<String> taggedUsers = const [],
    ContentSettings settings = const ContentSettings(),
    ContentLocation? location,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<void>> deleteContent(String contentId) async =>
      ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<List<Content>>> getContents({
    int? limit,
    int? offset,
    ContentType? type,
    String? location,
    ContentStatus? status,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByAuthor(
    String authorId, {
    int? limit,
    int? offset,
  }) async => ContentRepositoryResult.success(items);

  @override
  Future<ContentRepositoryResult<ContentAuthorPage>> getContentsByAuthorPaged(
    String authorId, {
    int limit = 20,
    String? cursor,
  }) async => _page();

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByLocation({
    required String location,
    int? limit,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<Content>> getContentById(
    String contentId,
  ) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<List<Content>>> getTrendingContents({
    int? limit,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<Content>> updateContent(
    String contentId,
    Content content,
  ) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<ContentSearchResult>> searchContents({
    required String query,
    int? limit,
    int? offset,
    ContentType? type,
    String? location,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<Content>> fulfillRequest(
    String contentId,
  ) async => ContentRepositoryResult.error('not used');
}

Widget _app(List<Content> items) {
  return ProviderScope(
    overrides: [
      contentRepositoryProvider.overrideWithValue(
        _FakeContentRepository(items),
      ),
      loggerServiceProvider.overrideWithValue(_FakeLogger()),
      objectPreviewProvider.overrideWith((ref, reference) async => null),
    ],
    child: const MaterialApp(
      home: Scaffold(body: ProfileFeedTab(userId: 'user-1')),
    ),
  );
}

Content _content({
  required String id,
  required String content,
  required ContentType type,
  ContentStatus status = ContentStatus.active,
  String? originalAuthorId,
  ContentResourceProjection? resourceProjection,
}) {
  return Content(
    id: id,
    content: content,
    authorId: 'author-1',
    authorUsername: 'author',
    authorAvatarUrl: null,
    type: type,
    status: status,
    media: const [],
    tags: const [],
    taggedUsers: const [],
    mentionedUserIds: const [],
    engagement: const ContentEngagement(),
    moderationInfo: const ContentModerationInfo(),
    createdAt: DateTime.utc(2026, 6, 2, 10, 0),
    updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
    originalAuthorId: originalAuthorId,
    resourceProjection: resourceProjection,
  );
}

Map<String, dynamic> _fixedPriceSaleProjection({
  required String resourceId,
  required String title,
  required String thumbnailUrl,
}) {
  return <String, dynamic>{
    'state': 'LIVE',
    'resource_type': 'fixed_price_sale',
    'resource_id': resourceId,
    'fixed_price_sale': <String, dynamic>{
      'title': title,
      'media': <Map<String, dynamic>>[],
      'thumbnail_url': thumbnailUrl,
      'price': 1500000,
      'status': 'active',
      'quantity_available': 3,
      'can_interact': true,
      'seller': <String, dynamic>{
        'user': <String, dynamic>{
          'id': 'seller-1',
          'username': 'seller',
        },
      },
    },
  };
}

Map<String, dynamic> _auctionProjection({
  required String resourceId,
  required String title,
  required String thumbnailUrl,
}) {
  return <String, dynamic>{
    'state': 'LIVE',
    'resource_type': 'auction',
    'resource_id': resourceId,
    'auction': <String, dynamic>{
      'title': title,
      'media': <Map<String, dynamic>>[],
      'thumbnail_url': thumbnailUrl,
      'lifecycle': 'active',
      'current_bid': 1750000,
      'buy_now_price': 2500000,
      'end_at': '2026-08-10T10:00:00.000Z',
      'can_interact': true,
      'seller': <String, dynamic>{
        'user': <String, dynamic>{
          'id': 'seller-1',
          'username': 'seller',
        },
      },
    },
  };
}

Map<String, dynamic> _profileProjection({
  required String resourceId,
  required String username,
  required String avatarUrl,
}) {
  return <String, dynamic>{
    'state': 'LIVE',
    'resource_type': 'profile',
    'resource_id': resourceId,
    'profile': <String, dynamic>{
      'username': username,
      'avatar_url': avatarUrl,
      'lifecycle': 'active',
    },
  };
}

void main() {
  testWidgets('normal profile content stays unchanged', (tester) async {
    await tester.pumpWidget(
      _app([
        _content(
          id: 'post-1',
          content: 'normal profile post',
          type: ContentType.post,
        ),
      ]),
    );
    await tester.pumpAndSettle();

    expect(find.byType(RepostAttributionBar), findsNothing);
    expect(find.byType(ObjectPreviewCard), findsNothing);
    expect(find.text('normal profile post'), findsOneWidget);
  });

  testWidgets('profile repost stays unchanged', (tester) async {
    await tester.pumpWidget(
      _app([
        _content(
          id: 'repost-1',
          content: 'repost profile content',
          type: ContentType.post,
          originalAuthorId: 'original-author',
        ),
      ]),
    );
    await tester.pumpAndSettle();

    expect(find.byType(RepostAttributionBar), findsOneWidget);
    expect(find.byType(ObjectPreviewCard), findsNothing);
    expect(find.text('Repost'), findsOneWidget);
  });

  testWidgets('listing shareReference is visible in profile tab', (
    tester,
  ) async {
    await tester.pumpWidget(
      _app([
        _content(
          id: 'listing-1',
          content: 'listing share content',
          type: ContentType.post,
          resourceProjection: ContentResourceProjection.fromJson(
            _fixedPriceSaleProjection(
              resourceId: 'listing-1',
              title: 'listing share',
              thumbnailUrl: 'https://example.com/listing.jpg',
            ),
          ),
        ),
      ]),
    );
    await tester.pumpAndSettle();

    expect(find.byType(RepostAttributionBar), findsNothing);
    expect(find.byType(ObjectPreviewCard), findsOneWidget);
    expect(find.text('listing share'), findsOneWidget);
  });

  testWidgets('auction shareReference is visible in profile tab', (
    tester,
  ) async {
    await tester.pumpWidget(
      _app([
        _content(
          id: 'auction-1',
          content: 'auction share content',
          type: ContentType.post,
          resourceProjection: ContentResourceProjection.fromJson(
            _auctionProjection(
              resourceId: 'auction-1',
              title: 'auction share',
              thumbnailUrl: 'https://example.com/auction.jpg',
            ),
          ),
        ),
      ]),
    );
    await tester.pumpAndSettle();

    expect(find.byType(RepostAttributionBar), findsNothing);
    expect(find.byType(ObjectPreviewCard), findsOneWidget);
    expect(find.text('auction share'), findsOneWidget);
  });

  testWidgets('profile shareReference is visible in profile tab', (
    tester,
  ) async {
    await tester.pumpWidget(
      _app([
        _content(
          id: 'profile-1',
          content: 'profile share content',
          type: ContentType.post,
          resourceProjection: ContentResourceProjection.fromJson(
            _profileProjection(
              resourceId: 'profile-1',
              username: 'profile share',
              avatarUrl: 'https://example.com/profile.jpg',
            ),
          ),
        ),
      ]),
    );
    await tester.pumpAndSettle();

    expect(find.byType(RepostAttributionBar), findsNothing);
    expect(find.byType(ObjectPreviewCard), findsOneWidget);
    expect(find.text('profile share'), findsOneWidget);
  });
}
