import 'dart:io';
import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/ui/src/helpers/media_picker_helper.dart';
import 'package:labuda/shared/ui/src/screens/custom_camera_screen.dart';

/// Content Media Handler - Handles photo and video selection for posts
///
/// Features:
/// - Photo selection from gallery (supports mixed photo+video)
/// - Camera with photo/video toggle
/// - Multiple selection support
/// - File size validation
/// - Media preview
class ContentMediaHandler {
  static const int maxImages = 15;
  static const int maxVideos = 5;
  static const int maxImageSizeMb = 10;
  static const int maxVideoSizeMb = 100;

  /// Pick media from gallery (photos + videos)
  Future<List<File>> pickMediaFromGallery({
    required BuildContext context,
    int currentMediaCount = 0,
  }) async {
    final maxAssets = (maxImages + maxVideos) - currentMediaCount;
    if (maxAssets <= 0) {
      _showError(context, 'Maximum media limit reached');
      return [];
    }

    try {
      final mediaUrls = await MediaPickerHelper.pickMedia(
        context: context,
        maxAssets: maxAssets,
      );

      if (mediaUrls == null || mediaUrls.isEmpty) return [];

      final List<File> validFiles = [];
      for (final path in mediaUrls) {
        final file = File(path);
        final isVideo = _isVideoFile(file);

        if (isVideo) {
          if (!context.mounted) continue;
          if (await _validateVideoFile(file, context)) {
            validFiles.add(file);
          }
        } else {
          if (!context.mounted) continue;
          if (await _validateImageFile(file, context)) {
            validFiles.add(file);
          }
        }
      }

      return validFiles;
    } catch (e) {
      if (!context.mounted) return [];
      _showError(context, 'Gagal memilih media. Coba lagi.');
      return [];
    }
  }

  /// Open camera (supports both photo and video)
  Future<List<File>> openCamera({
    required BuildContext context,
    int currentMediaCount = 0,
  }) async {
    final maxAssets = (maxImages + maxVideos) - currentMediaCount;
    if (maxAssets <= 0) {
      _showError(context, 'Maximum media limit reached');
      return [];
    }

    try {
      final mediaUrls = await CustomCameraScreen.show(context);

      if (mediaUrls == null || mediaUrls.isEmpty) return [];

      final List<File> validFiles = [];
      for (final path in mediaUrls) {
        final file = File(path);
        final isVideo = _isVideoFile(file);

        if (isVideo) {
          if (!context.mounted) continue;
          if (await _validateVideoFile(file, context)) {
            validFiles.add(file);
          }
        } else {
          if (!context.mounted) continue;
          if (await _validateImageFile(file, context)) {
            validFiles.add(file);
          }
        }
      }

      return validFiles;
    } catch (e) {
      if (!context.mounted) return [];
      _showError(context, 'Gagal membuka kamera. Coba lagi.');
      return [];
    }
  }

  /// Check if file is video
  bool _isVideoFile(File file) {
    final extension = file.path.split('.').last.toLowerCase();
    return ['mp4', 'mov', 'avi', 'mkv'].contains(extension);
  }

  /// Validate image file
  Future<bool> _validateImageFile(File file, BuildContext context) async {
    final bytes = await file.length();
    final sizeMB = bytes / (1024 * 1024);

    if (sizeMB > maxImageSizeMb) {
      if (!context.mounted) return false;
      _showError(context, 'Image size must be less than ${maxImageSizeMb}MB');
      return false;
    }
    return true;
  }

  /// Validate video file
  Future<bool> _validateVideoFile(File file, BuildContext context) async {
    final bytes = await file.length();
    final sizeMB = bytes / (1024 * 1024);

    if (sizeMB > maxVideoSizeMb) {
      if (!context.mounted) return false;
      _showError(context, 'Video size must be less than ${maxVideoSizeMb}MB');
      return false;
    }
    return true;
  }

  /// Show error messages
  void _showError(BuildContext context, String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        backgroundColor: AppColors.statusError,
        duration: const Duration(seconds: 4),
      ),
    );
  }
}
