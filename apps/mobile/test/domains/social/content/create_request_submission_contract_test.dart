import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_submission_handler.dart';
import 'package:labuda/shared/src/providers/upload_progress_provider.dart';

bool _capturedCreateCalled = false;

class _FakeUser {
  final String id;
  final String username;
  final String? avatarUrl;

  const _FakeUser({required this.id, required this.username, this.avatarUrl});
}

class _FakeContentRepository implements ContentRepository {
  @override
  Future<ContentRepositoryResult<Content>> createContent({
    required String authorId,
    String? authorUsername,
    String? authorAvatarUrl,
    required String content,
    List<MediaEntity> media = const [],
    List<String> tags = const [],
    List<String> taggedUsers = const [],
    ContentSettings settings = const ContentSettings(),
    ContentLocation? location,
  }) async {
    _capturedCreateCalled = true;
    return ContentRepositoryResult.success(
      Content(
        id: 'content-1',
        content: content,
        authorId: authorId,
        authorUsername: authorUsername,
        authorAvatarUrl: authorAvatarUrl,
        status: ContentStatus.active,
        media: media,
        tags: tags,
        taggedUsers: taggedUsers,
        mentionedUserIds: const [],
        settings: settings,
        engagement: const ContentEngagement(),
        location: location,
        moderationInfo: const ContentModerationInfo(),
        createdAt: DateTime.utc(2026, 6, 2),
        updatedAt: DateTime.utc(2026, 6, 2),
      ),
    );
  }

  @override
  Future<ContentRepositoryResult<void>> deleteContent(String contentId) async {
    return ContentRepositoryResult.error('not used');
  }

  @override
  Future<ContentRepositoryResult<Content>> getContentById(
    String contentId,
  ) async {
    return ContentRepositoryResult.error('not used');
  }

  @override
  Future<ContentRepositoryResult<List<Content>>> getContents({
    int? limit,
    int? offset,
    String? location,
    ContentStatus? status,
  }) async {
    return ContentRepositoryResult.error('not used');
  }

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByAuthor(
    String authorId, {
    int? limit,
    int? offset,
  }) async {
    return ContentRepositoryResult.error('not used');
  }

  @override
  Future<ContentRepositoryResult<ContentAuthorPage>> getContentsByAuthorPaged(
    String authorId, {
    int limit = 20,
    String? cursor,
  }) async {
    return ContentRepositoryResult.error('not used');
  }

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByLocation({
    required String location,
    int? limit,
  }) async {
    return ContentRepositoryResult.error('not used');
  }

  @override
  Future<ContentRepositoryResult<List<Content>>> getTrendingContents({
    int? limit,
  }) async {
    return ContentRepositoryResult.error('not used');
  }

  @override
  Future<ContentRepositoryResult<Content>> updateContent(
    String contentId,
    Content content,
  ) async {
    return ContentRepositoryResult.error('not used');
  }

  @override
  Future<ContentRepositoryResult<ContentSearchResult>> searchContents({
    required String query,
    int? limit,
    int? offset,
    String? location,
  }) async {
    return ContentRepositoryResult.error('not used');
  }
}

void main() {
  test(
    'performBackgroundUpload forwards generic content payload to createContent',
    () async {
      _capturedCreateCalled = false;
      final container = ProviderContainer(
        overrides: [
          contentRepositoryProvider.overrideWithValue(_FakeContentRepository()),
          s3ServiceProvider.overrideWithValue(S3Service()),
        ],
      );
      addTearDown(container.dispose);

      final notifier = container.read(uploadProgressProvider.notifier);
      const user = _FakeUser(
        id: 'user-1',
        username: 'yayan',
        avatarUrl: 'https://example.com/avatar.png',
      );

      await ContentSubmissionHandler.performBackgroundUpload(
        user: user,
        taskId: 'task-1',
        notifier: notifier,
        selectedImages: const [],
        selectedVideos: const [],
        content: 'request content',
        hashtags: const [],
        taggedPeople: const [],
        postVisibility: 'Public',
        selectedLocation: null,
        container: container,
      );

      expect(_capturedCreateCalled, isTrue);

      final source = File(
        'lib/domains/social/content/presentation/widgets/create_content/content_submission_handler.dart',
      ).readAsStringSync();
      expect(source, isNot(contains('fulfillRequest')));
      expect(source, isNot(contains('/fulfill')));
    },
  );
}
