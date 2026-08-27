import 'dart:math';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/profile_feed_tab.dart';
import 'package:labuda/shared/object/object_preview_provider.dart';
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';
import 'package:labuda/shared/services/logger_service.dart';

class _SequenceContentRepository implements ContentRepository {
  _SequenceContentRepository(this.responses);

  final List<ContentRepositoryResult<ContentAuthorPage>> responses;
  int calls = 0;

  @override
  Future<ContentRepositoryResult<ContentAuthorPage>> getContentsByAuthorPaged(
    String authorId, {
    int limit = 20,
    String? cursor,
  }) async {
    calls++;
    final index = min(calls - 1, responses.length - 1);
    return responses[index];
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
  Future<ContentRepositoryResult<Content>> getContentById(
    String contentId,
  ) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByAuthor(
    String authorId, {
    int? limit,
    int? offset,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<List<Content>>> getContents({
    int? limit,
    int? offset,
    ContentType? type,
    String? location,
    ContentStatus? status,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<Content>> updateContent(
    String contentId,
    Content content,
  ) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<void>> deleteContent(String contentId) async =>
      ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<Content>> fulfillRequest(
    String contentId,
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
  Future<ContentRepositoryResult<List<Content>>> getTrendingContents({
    int? limit,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByLocation({
    required String location,
    int? limit,
  }) async => ContentRepositoryResult.error('not used');

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Widget _buildApp(ContentRepository repository) {
  return ProviderScope(
    overrides: [
      contentRepositoryProvider.overrideWithValue(repository),
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
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
}) {
  return Content(
    id: id,
    content: content,
    authorId: 'author-1',
    authorUsername: 'author',
    authorAvatarUrl: null,
    type: ContentType.post,
    status: ContentStatus.active,
    media: const [],
    tags: const [],
    taggedUsers: const [],
    mentionedUserIds: const [],
    engagement: const ContentEngagement(),
    moderationInfo: const ContentModerationInfo(),
    createdAt: DateTime.utc(2026, 6, 2, 10, 0),
    updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
  );
}

ContentAuthorPage _page({
  required List<Content> items,
  bool hasMore = false,
  String? nextCursor,
}) {
  return ContentAuthorPage(
    items: items,
    hasMore: hasMore,
    nextCursor: nextCursor,
  );
}

void main() {
  testWidgets('initial failure does not become empty', (tester) async {
    final repository = _SequenceContentRepository([
      ContentRepositoryResult.error('Connection timed out. Please try again.'),
    ]);

    await tester.pumpWidget(_buildApp(repository));
    await tester.pumpAndSettle();

    expect(find.text('Failed to load content'), findsOneWidget);
    expect(find.text('Try Again'), findsOneWidget);
    expect(find.text('No Content Yet'), findsNothing);
    expect(find.byType(ObjectPreviewCard), findsNothing);
    expect(find.text('initial content'), findsNothing);
  });

  testWidgets('retry after failure succeeds', (tester) async {
    final repository = _SequenceContentRepository([
      ContentRepositoryResult.error('Connection timed out. Please try again.'),
      ContentRepositoryResult.success(
        _page(
          items: [_content(id: 'content-1', content: 'retry success content')],
        ),
      ),
    ]);

    await tester.pumpWidget(_buildApp(repository));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Try Again'));
    await tester.pump();
    await tester.pumpAndSettle();

    expect(repository.calls, 2);
    expect(find.text('Failed to load content'), findsNothing);
    expect(find.text('Try Again'), findsNothing);
    expect(find.text('No Content Yet'), findsNothing);
    expect(find.text('retry success content'), findsOneWidget);
  });

  testWidgets('successful empty response renders the empty state', (
    tester,
  ) async {
    final repository = _SequenceContentRepository([
      ContentRepositoryResult.success(_page(items: const [])),
    ]);

    await tester.pumpWidget(_buildApp(repository));
    await tester.pumpAndSettle();

    expect(find.text('No Content Yet'), findsOneWidget);
    expect(find.text('Failed to load content'), findsNothing);
    expect(find.text('Try Again'), findsNothing);
  });

  testWidgets('successful content still renders share-reference content', (
    tester,
  ) async {
    final repository = _SequenceContentRepository([
      ContentRepositoryResult.success(
        _page(
          items: [
            _content(
              id: 'content-1',
              content: 'share reference content',
              resourceProjection: ContentResourceProjection.fromJson(
                <String, dynamic>{
                  'state': 'LIVE',
                  'resource_type': 'fixed_price_sale',
                  'resource_id': 'listing-1',
                  'fixed_price_sale': <String, dynamic>{
                    'title': 'listing share',
                    'media': <Map<String, dynamic>>[],
                    'thumbnail_url': 'https://example.com/listing.jpg',
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
                },
              ),
            ),
          ],
        ),
      ),
    ]);

    await tester.pumpWidget(_buildApp(repository));
    await tester.pumpAndSettle();

    expect(find.byType(ObjectPreviewCard), findsOneWidget);
    expect(find.text('share reference content'), findsOneWidget);
    expect(find.text('Failed to load content'), findsNothing);
    expect(find.text('No Content Yet'), findsNothing);
  });
}
