import 'dart:async';
import 'dart:io';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/shared/entities/post_location.dart' as loc;
import 'package:labuda/shared/src/providers/upload_progress_provider.dart';

/// Handles post submission and upload logic
class ContentSubmissionHandler {
  /// Start upload task and return task ID
  static String startUploadTask({
    required dynamic notifier,
    required int imageCount,
    required int videoCount,
  }) {
    final taskId = 'content_${DateTime.now().millisecondsSinceEpoch}';

    notifier.startUpload(
      taskId: taskId,
      type: UploadTaskType.post,
      description: 'Preparing content upload...',
      totalSteps: (imageCount + videoCount > 0) ? 3 : 2,
    );
    return taskId;
  }

  /// Upload media files to S3 and convert to MediaEntity
  ///
  /// CANONICAL MEDIA FLOW: Returns `List<MediaEntity>` for proper content creation.
  /// Images use uploadImageWithBlurhash which returns MediaEntity directly.
  /// videos create MediaEntity from uploaded URL.
  static Future<List<MediaEntity>> uploadMedia(
    List<File> selectedImages,
    List<File> selectedVideos,
    ProviderContainer container,
  ) async {
    final mediaEntities = <MediaEntity>[];
    final s3Service = container.read(s3ServiceProvider);

    // Upload images using canonical MediaEntity upload method.
    // Abort on first failure — partial media is not a valid content.
    for (final image in selectedImages) {
      final result = await s3Service.uploadImageWithBlurhash(image);
      if (!result.isSuccess) {
        throw Exception('Image upload failed: ${result.error}');
      }
      mediaEntities.add(result.data!);
    }

    // Upload videos and create MediaEntity from URL.
    // Abort on first failure — partial media is not a valid content.
    for (final video in selectedVideos) {
      final result = await s3Service.uploadVideo(video);
      if (!result.isSuccess) {
        throw Exception('Video upload failed: ${result.error}');
      }
      final timestamp = DateTime.now().millisecondsSinceEpoch;
      mediaEntities.add(
        MediaEntity(
          id: timestamp.toString(),
          originalUrl: result.data!,
          type: MediaType.video,
          createdAt: DateTime.now(),
        ),
      );
    }

    return mediaEntities;
  }

  /// Perform background upload and create post
  static Future<void> performBackgroundUpload({
    required dynamic user,
    required String taskId,
    required UploadProgressNotifier notifier,
    required List<File> selectedImages,
    required List<File> selectedVideos,
    required String content,
    required List<String> hashtags,
    required List<String> mentionedUserIds,
    required String postVisibility,
    required loc.PostLocation? selectedLocation,
    required ProviderContainer container,
  }) async {
    try {
      // Upload new media files
      notifier.updateProgress(
        taskId: taskId,
        progress: 0.1,
        currentStep: 1,
        status: UploadTaskStatus.uploading,
        stepDescription: 'Uploading media...',
      );
      // Upload media and get canonical MediaEntity list
      final uploadedMedia = await uploadMedia(
        selectedImages,
        selectedVideos,
        container,
      );

      notifier.updateProgress(
        taskId: taskId,
        progress: 0.8,
        currentStep: 2,
        status: UploadTaskStatus.processing,
        stepDescription: 'Creating content...',
      );

      // Convert visibility string to enum
      final visibility = postVisibility == 'Public'
          ? ContentVisibility.public
          : postVisibility == 'Followers'
          ? ContentVisibility.followersOnly
          : ContentVisibility.private;

      // Read the canonical repository directly — ContentActions notifier
      // is auto-dispose and unsafe for fire-and-forget background uploads.
      final repo = container.read(contentRepositoryProvider);
      final result = await repo.createContent(
        authorId: user.id,
        authorUsername: user.username,
        authorAvatarUrl: user.avatarUrl,
        content: content.trim(),
        media: uploadedMedia,
        tags: hashtags,
        mentionedUserIds: mentionedUserIds,
        settings: ContentSettings(
          visibility: visibility,
        ),
        location: selectedLocation != null
            ? ContentLocation(
                city: selectedLocation.address,
                province: null,
              )
            : null,
      );

      if (result.isSuccess) {
        notifier.updateProgress(
          taskId: taskId,
          progress: 1.0,
          currentStep: 3,
          status: UploadTaskStatus.completed,
          stepDescription: 'Content created successfully!',
        );
        notifier.completeUpload(taskId);
        // Feed will be refreshed automatically on return
      } else {
        throw Exception(result.error);
      }
    } catch (e) {
      notifier.failUpload(taskId, 'Upload failed: ${e.toString()}');
    }
  }
}
