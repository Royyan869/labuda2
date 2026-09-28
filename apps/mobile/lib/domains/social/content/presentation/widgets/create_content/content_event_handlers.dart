import 'dart:io';
import 'package:flutter/material.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';
import 'package:labuda/core/src/config/google_config.dart';
import 'package:labuda/shared/widgets/interactive_map_picker_bottom_sheet.dart';
import 'package:labuda/shared/entities/post_location.dart' as loc;

/// Handles events for create post screen.
///
/// Media picking runs through the canonical [MediaUploadOrchestrator]
/// (`forContent`, deferred): same limits, same validation, same camera as
/// every other surface.
class ContentEventHandlers {
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
    required int currentMediaCount,
  }) async {
    return await MediaUploadOrchestrator.forContent().pickLocalFiles(
      context: context,
      currentCount: currentMediaCount,
    );
  }

  /// Handle camera capture
  static Future<List<File>> handleCamera({
    required BuildContext context,
    required int currentMediaCount,
  }) async {
    return await MediaUploadOrchestrator.forContent().openCameraLocal(
      context: context,
      currentCount: currentMediaCount,
    );
  }

  /// Process media files and categorize them
  static Map<String, List<File>> processMediaFiles(List<File> files) {
    final images = <File>[];
    final videos = <File>[];

    for (final file in files) {
      if (MediaUploadOrchestrator.isVideoFile(file)) {
        videos.add(file);
      } else {
        images.add(file);
      }
    }

    return {'images': images, 'videos': videos};
  }
}
