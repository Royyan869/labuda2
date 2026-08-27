import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_submission_handler.dart';
import 'package:labuda/shared/src/providers/upload_progress_provider.dart';

class _User {
  final String id;
  final String username;
  String? get avatarUrl => null;
  const _User(this.id, this.username);
}

class _FakeS3Service extends S3Service {
  final Set<String> failedImages;
  final bool throwOnImage;
  int imageCalls = 0;

  _FakeS3Service({this.failedImages = const {}, this.throwOnImage = false});

  @override
  Future<Result<MediaEntity>> uploadImageWithBlurhash(File file) async {
    imageCalls++;
    if (throwOnImage) throw StateError('image upload exploded');
    if (failedImages.contains(file.path)) {
      return Result.error('image upload failed');
    }
    return Result.success(
      MediaEntity(
        id: 'media-$imageCalls',
        originalUrl: 'https://cdn.example.com/$imageCalls.jpg',
        type: MediaType.image,
        createdAt: DateTime.utc(2026, 1, 1),
      ),
    );
  }
}

class _RecordingRepository implements ContentRepository {
  int createCalls = 0;
  List<MediaEntity> receivedMedia = const [];

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
    createCalls++;
    receivedMedia = media;
    return ContentRepositoryResult.success(
      Content(
        id: 'content-1',
        content: content,
        authorId: authorId,
        authorUsername: authorUsername,
        status: ContentStatus.active,
        media: media,
        tags: tags,
        taggedUsers: taggedUsers,
        mentionedUserIds: const [],
        settings: settings,
        engagement: const ContentEngagement(),
        moderationInfo: const ContentModerationInfo(),
        createdAt: DateTime.utc(2026, 1, 1),
        updatedAt: DateTime.utc(2026, 1, 1),
      ),
    );
  }

  @override
  Future<ContentRepositoryResult<void>> deleteContent(String contentId) async =>
      ContentRepositoryResult.error('unused');
  @override
  Future<ContentRepositoryResult<Content>> getContentById(
    String contentId,
  ) async => ContentRepositoryResult.error('unused');
  @override
  Future<ContentRepositoryResult<List<Content>>> getContents({
    int? limit,
    int? offset,
    String? location,
    ContentStatus? status,
  }) async => ContentRepositoryResult.error('unused');
  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByAuthor(
    String authorId, {
    int? limit,
    int? offset,
  }) async => ContentRepositoryResult.error('unused');
  @override
  Future<ContentRepositoryResult<ContentAuthorPage>> getContentsByAuthorPaged(
    String authorId, {
    int limit = 20,
    String? cursor,
  }) async => ContentRepositoryResult.error('unused');
  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByLocation({
    required String location,
    int? limit,
  }) async => ContentRepositoryResult.error('unused');
  @override
  Future<ContentRepositoryResult<List<Content>>> getTrendingContents({
    int? limit,
  }) async => ContentRepositoryResult.error('unused');
  @override
  Future<ContentRepositoryResult<Content>> updateContent(
    String contentId,
    Content content,
  ) async => ContentRepositoryResult.error('unused');
  @override
  Future<ContentRepositoryResult<ContentSearchResult>> searchContents({
    required String query,
    int? limit,
    int? offset,
    String? location,
  }) async => ContentRepositoryResult.error('unused');
}

Future<File> _file(String name) async {
  final dir = await Directory.systemTemp.createTemp('content_media_failure_');
  return File('${dir.path}${Platform.pathSeparator}$name')
    ..writeAsBytesSync(const [1, 2, 3]);
}

Future<
  ({
    ProviderContainer container,
    _RecordingRepository repository,
    UploadProgressNotifier notifier,
  })
>
_setup(_FakeS3Service s3) async {
  final repository = _RecordingRepository();
  final container = ProviderContainer(
    overrides: [
      s3ServiceProvider.overrideWithValue(s3),
      contentRepositoryProvider.overrideWithValue(repository),
    ],
  );
  return (
    container: container,
    repository: repository,
    notifier: container.read(uploadProgressProvider.notifier),
  );
}

Future<void> _submit({
  required ProviderContainer container,
  required UploadProgressNotifier notifier,
  required List<File> images,
}) {
  return ContentSubmissionHandler.performBackgroundUpload(
    user: const _User('user-1', 'buyer'),
    taskId: 'task-1',
    notifier: notifier,
    selectedImages: images,
    selectedVideos: const [],
    content: 'content',
    hashtags: const [],
    taggedPeople: const [],
    postVisibility: 'Public',
    selectedLocation: null,
    container: container,
  );
}

void main() {
  test('empty media preserves content creation behavior', () async {
    final setup = await _setup(_FakeS3Service());
    addTearDown(setup.container.dispose);
    setup.notifier.startUpload(
      taskId: 'task-1',
      type: UploadTaskType.post,
      description: 'test',
      totalSteps: 2,
    );
    await _submit(
      container: setup.container,
      notifier: setup.notifier,
      images: const [],
    );
    expect(setup.repository.createCalls, 1);
    expect(setup.repository.receivedMedia, isEmpty);
  });

  test('current boundary succeeds with all selected media', () async {
    final a = await _file('a.jpg');
    final b = await _file('b.jpg');
    final setup = await _setup(_FakeS3Service());
    addTearDown(setup.container.dispose);
    setup.notifier.startUpload(
      taskId: 'task-1',
      type: UploadTaskType.post,
      description: 'test',
      totalSteps: 3,
    );
    await _submit(
      container: setup.container,
      notifier: setup.notifier,
      images: [a, b],
    );
    expect(
      setup.repository.createCalls,
      1,
      reason:
          'task=${setup.container.read(uploadProgressProvider).activeUploads["task-1"]?.errorMessage}',
    );
    expect(setup.repository.receivedMedia, hasLength(2));
    expect(
      setup.container
          .read(uploadProgressProvider)
          .activeUploads['task-1']
          ?.status,
      UploadTaskStatus.completed,
    );
  });

  test('failed Result stops submission and marks progress failed', () async {
    final a = await _file('a.jpg');
    final b = await _file('b.jpg');
    final setup = await _setup(_FakeS3Service(failedImages: {b.path}));
    addTearDown(setup.container.dispose);
    setup.notifier.startUpload(
      taskId: 'task-1',
      type: UploadTaskType.post,
      description: 'test',
      totalSteps: 3,
    );
    await _submit(
      container: setup.container,
      notifier: setup.notifier,
      images: [a, b],
    );
    expect(setup.repository.createCalls, 0);
    expect(
      setup.container
          .read(uploadProgressProvider)
          .activeUploads['task-1']
          ?.status,
      UploadTaskStatus.failed,
    );
  });

  test('current boundary fails without create when upload throws', () async {
    final a = await _file('a.jpg');
    final setup = await _setup(_FakeS3Service(throwOnImage: true));
    addTearDown(setup.container.dispose);
    setup.notifier.startUpload(
      taskId: 'task-1',
      type: UploadTaskType.post,
      description: 'test',
      totalSteps: 3,
    );
    await _submit(
      container: setup.container,
      notifier: setup.notifier,
      images: [a],
    );
    expect(setup.repository.createCalls, 0);
    expect(
      setup.container
          .read(uploadProgressProvider)
          .activeUploads['task-1']
          ?.status,
      UploadTaskStatus.failed,
    );
  });
}
