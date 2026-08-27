// Dart
// Flutter
import 'package:flutter/material.dart';

// External

// Internal
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/src/providers/upload_progress_provider.dart';

/// Utility class untuk upload task operations
class UploadTaskUtils {
  /// Get task icon berdasarkan type dan status
  static Widget buildTaskIcon(UploadTaskType type, UploadTaskStatus status) {
    IconData iconData;
    Color iconColor;

    // Determine icon
    switch (type) {
      case UploadTaskType.post:
        iconData = Icons.article_outlined;
        break;
      case UploadTaskType.request:
        iconData = Icons.help_outline;
        break;
      case UploadTaskType.listing:
        iconData = Icons.inventory_2_outlined;
        break;
      case UploadTaskType.auction:
        iconData = Icons.gavel_outlined;
        break;
    }

    // Determine color
    switch (status) {
      case UploadTaskStatus.completed:
        iconColor = AppColors.statusSuccess;
        break;
      case UploadTaskStatus.failed:
        iconColor = AppColors.statusError;
        break;
      case UploadTaskStatus.uploading:
      case UploadTaskStatus.processing:
        iconColor = AppColors.primaryBlue;
        break;
      default:
        iconColor = AppColors.neutralGray500;
    }

    return Icon(iconData, size: 20, color: iconColor);
  }

  /// Get task title berdasarkan type
  static String getTaskTitle(UploadTaskType type) {
    switch (type) {
      case UploadTaskType.post:
        return 'Mengunggah Post';
      case UploadTaskType.request:
        return 'Mengunggah Request';
      case UploadTaskType.listing:
        return 'Mengunggah Listing';
      case UploadTaskType.auction:
        return 'Mengunggah Lelang';
    }
  }
}
