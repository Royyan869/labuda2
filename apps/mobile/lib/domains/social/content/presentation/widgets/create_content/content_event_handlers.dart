import 'dart:io';
import 'package:flutter/material.dart';
import 'package:labuda/core/src/config/google_config.dart';
import 'package:labuda/shared/widgets/user_search_bottom_sheet.dart';
import 'package:labuda/shared/widgets/interactive_map_picker_bottom_sheet.dart';
import 'package:labuda/shared/entities/post_location.dart' as loc;
import 'package:labuda/domains/social/content/presentation/widgets/content_media_handler.dart';

/// Handles events for create post screen
class ContentEventHandlers {
  /// Handle tag people action
  static Future<List<String>?> handleTagPeople({
    required BuildContext context,
    required List<String> alreadyTaggedPeople,
  }) async {
    return await UserSearchBottomSheet.show(
      context: context,
      alreadyTaggedUserIds: alreadyTaggedPeople,
      maxSelections: 50,
    );
  }

  /// Handle add location action
  static Future<loc.PostLocation?> handleAddLocation({
    required BuildContext context,
    required loc.PostLocation? currentLocation,
  }) async {
    return await InteractiveMapPickerBottomSheet.show(
      context: context,
      initialLocation: currentLocation,
      googleApiKey: GoogleConfig.isConfigured ? GoogleConfig.apiKey : null,
    );
  }

  /// Handle gallery media selection
  static Future<List<File>> handleGalleryPick({
    required BuildContext context,
    required ContentMediaHandler mediaHandler,
    required int currentMediaCount,
  }) async {
    return await mediaHandler.pickMediaFromGallery(
      context: context,
      currentMediaCount: currentMediaCount,
    );
  }

  /// Handle camera capture
  static Future<List<File>> handleCamera({
    required BuildContext context,
    required ContentMediaHandler mediaHandler,
    required int currentMediaCount,
  }) async {
    return await mediaHandler.openCamera(
      context: context,
      currentMediaCount: currentMediaCount,
    );
  }

  /// Process media files and categorize them
  static Map<String, List<File>> processMediaFiles(List<File> files) {
    final images = <File>[];
    final videos = <File>[];

    for (final file in files) {
      final extension = file.path.split('.').last.toLowerCase();
      final isVideo = ['mp4', 'mov', 'avi', 'mkv'].contains(extension);
      if (isVideo) {
        videos.add(file);
      } else {
        images.add(file);
      }
    }

    return {'images': images, 'videos': videos};
  }
}
