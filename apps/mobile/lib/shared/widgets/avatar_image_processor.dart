import 'dart:convert';
import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:labuda/core/services/s3_service.dart';
import 'package:labuda/shared/shared.dart';
import 'web_image_cropper.dart';
import 'flutter_crop_image.dart';

/// Avatar image processing utilities
///
/// Features:
/// - Platform-specific image picking
/// - Image cropping (web/mobile) using pure Flutter implementation
/// - AWS S3 upload
/// - Cache management
///
/// MIGRATED from Firebase Storage to AWS S3
class AvatarImageProcessor {
  /// Pick and process image from given source
  static Future<void> pickAndCropImage(
    BuildContext context,
    ImageSource source,
    String userId,
    Function(String? avatarUrl) onAvatarUpdated, {
    double aspectRatio = 1.0,
    bool circularCrop = true,
    String cropTitle = 'Crop Avatar',
  }) async {
    try {
      if (kIsWeb && source == ImageSource.camera) {
        if (context.mounted) {
          AppSnackBar.showError(
            context,
            'Camera not supported on web. Please use gallery.',
          );
        }
        return;
      }

      final ImagePicker picker = ImagePicker();
      final XFile? image = await picker.pickImage(
        source: source,
        maxWidth: 2048,
        maxHeight: 2048,
        imageQuality: 95,
      );

      if (image != null && context.mounted) {
        await _handleImagePicked(
          context,
          image,
          userId,
          onAvatarUpdated,
          aspectRatio: aspectRatio,
          circularCrop: circularCrop,
          cropTitle: cropTitle,
        );
      }
    } catch (e) {
      if (context.mounted) {
        AppSnackBar.showError(context, 'Failed to pick image: $e');
      }
    }
  }

  static Future<void> _handleImagePicked(
    BuildContext context,
    XFile image,
    String userId,
    Function(String? avatarUrl) onAvatarUpdated, {
    double aspectRatio = 1.0,
    bool circularCrop = true,
    String cropTitle = 'Crop Avatar',
  }) async {
    try {
      if (kIsWeb) {
        await _showWebCropper(
          context,
          image,
          userId,
          onAvatarUpdated,
          aspectRatio: aspectRatio,
          circularCrop: circularCrop,
          cropTitle: cropTitle,
        );
      } else {
        await _showMobileCropper(
          context,
          image,
          userId,
          onAvatarUpdated,
          aspectRatio: aspectRatio,
          circularCrop: circularCrop,
          cropTitle: cropTitle,
        );
      }
    } catch (e) {
      if (context.mounted) {
        AppSnackBar.showError(context, 'Failed to crop image: $e');
      }
    }
  }

  static Future<void> _showWebCropper(
    BuildContext context,
    XFile imageFile,
    String userId,
    Function(String? avatarUrl) onAvatarUpdated, {
    double aspectRatio = 1.0,
    bool circularCrop = true,
    String cropTitle = 'Crop Avatar',
  }) async {
    if (!context.mounted) return;

    await Navigator.of(context).push(
      MaterialPageRoute<void>(
        fullscreenDialog: true,
        builder: (context) => WebImageCropper(
          imageFile: imageFile,
          aspectRatio: aspectRatio,
          withCircleUi: circularCrop,
          title: cropTitle,
          onCropped: (croppedBytes) async {
            Navigator.of(context).pop();
            // For web, convert cropped bytes to data URL for preview
            final base64String = base64Encode(croppedBytes);
            final dataUrl = 'data:image/png;base64,$base64String';
            onAvatarUpdated(dataUrl);
          },
          onCancel: () => Navigator.of(context).pop(),
        ),
      ),
    );
  }

  static Future<void> _showMobileCropper(
    BuildContext context,
    XFile imageFile,
    String userId,
    Function(String? avatarUrl) onAvatarUpdated, {
    double aspectRatio = 1.0,
    bool circularCrop = true,
    String cropTitle = 'Crop Avatar',
  }) async {
    if (!context.mounted) return;

    await Navigator.of(context).push(
      MaterialPageRoute<void>(
        fullscreenDialog: true,
        builder: (builderContext) => FlutterImageCropper(
          imageFile: imageFile,
          aspectRatio: aspectRatio,
          withCircleUi: circularCrop,
          title: cropTitle,
          onCropped: (croppedBytes) async {
            try {
              // Save cropped bytes to temp file and return local path
              final tempPath = await _saveCroppedBytesToTempFile(
                croppedBytes,
                userId,
              );
              // DON'T pop here - FlutterImageCropper already pops itself (line 105)
              onAvatarUpdated(tempPath);
            } catch (e) {
              if (context.mounted) {
                AppSnackBar.showError(context, 'Gagal menyimpan gambar: $e');
              }
              onAvatarUpdated(null);
            }
          },
        ),
      ),
    );
  }

  /// Save cropped bytes to temporary file and return path
  static Future<String> _saveCroppedBytesToTempFile(
    Uint8List croppedBytes,
    String userId,
  ) async {
    try {
      final directory = await Directory.systemTemp.createTemp('avatar_');

      final timestamp = DateTime.now().millisecondsSinceEpoch;
      final tempFile = File('${directory.path}/cropped_$timestamp.jpg');

      await tempFile.writeAsBytes(croppedBytes);

      return tempFile.path;
    } catch (e) {
      throw Exception('Failed to save cropped image: $e');
    }
  }

  /// Upload avatar to AWS S3
  static Future<void> uploadAvatar(
    dynamic imageData, // Can be Uint8List or String (file path)
    String userId,
    Function(String? avatarUrl) onAvatarUpdated,
  ) async {
    try {
      await _cleanupOldAvatarFiles(userId);

      final extension = kIsWeb ? 'png' : 'jpg';
      final contentType = kIsWeb ? 'image/png' : 'image/jpeg';
      final key = 'images/avatars/$userId.$extension';

      final s3Service = S3Service();
      String? downloadUrl;

      if (imageData is Uint8List) {
        // Cropped bytes from FlutterImageCropper or WebImageCropper
        final result = await s3Service.uploadImageBytesWithKey(
          imageData,
          key,
          contentType: contentType,
        );
        if (result.isSuccess) {
          downloadUrl = result.data;
        }
      } else if (imageData is String) {
        // File path (for backward compatibility)
        final file = File(imageData);
        final result = await s3Service.uploadImageWithKey(file, key);
        if (result.isSuccess) {
          downloadUrl = result.data;
        }
      } else {
        throw Exception('Invalid image data type');
      }

      if (downloadUrl == null) {
        onAvatarUpdated(null);
        return;
      }

      // Use consistent URL with cache-busting for all platforms
      final timestamp = DateTime.now().millisecondsSinceEpoch;
      final finalUrl = '$downloadUrl?t=$timestamp';

      // Clear all cache variations before updating
      await clearImageCache(downloadUrl);
      await clearImageCache(finalUrl);

      await Future.delayed(const Duration(milliseconds: 500));
      onAvatarUpdated(finalUrl);
    } catch (e) {
      onAvatarUpdated(null);
    }
  }

  /// Clear image cache for various URL formats
  static Future<void> clearImageCache(String imageUrl) async {
    try {
      // Clear the exact URL
      await CachedNetworkImage.evictFromCache(imageUrl);

      // Clear base URL without parameters
      final baseUrl = imageUrl.split('?')[0];
      await CachedNetworkImage.evictFromCache(baseUrl);

      // Clear timestamped variations
      await CachedNetworkImage.evictFromCache(
        '$baseUrl?t=${DateTime.now().millisecondsSinceEpoch}',
      );

      // Clear previous timestamped URLs
      final now = DateTime.now().millisecondsSinceEpoch;
      for (int i = 0; i < 10; i++) {
        final pastTimestamp = now - (i * 1000);
        await CachedNetworkImage.evictFromCache('$baseUrl?t=$pastTimestamp');
      }

      // Clear different file extensions
      if (baseUrl.contains('.')) {
        final baseWithoutExt = baseUrl.substring(0, baseUrl.lastIndexOf('.'));
        for (final ext in ['jpg', 'jpeg', 'png', 'webp']) {
          await CachedNetworkImage.evictFromCache('$baseWithoutExt.$ext');
          await CachedNetworkImage.evictFromCache(
            '$baseWithoutExt.$ext?t=${DateTime.now().millisecondsSinceEpoch}',
          );
        }
      }
    } catch (e) {
      // Cache clearing failed, not critical
    }
  }

  static Future<void> _cleanupOldAvatarFiles(String userId) async {
    final currentExt = kIsWeb ? 'png' : 'jpg';
    final extensionsToClean = [
      'jpg',
      'jpeg',
      'png',
      'webp',
      'gif',
    ].where((ext) => ext != currentExt).toList();

    final s3Service = S3Service();

    for (final ext in extensionsToClean) {
      try {
        // Construct S3 URL for deletion
        final key = 'images/avatars/$userId.$ext';
        final s3Url =
            'https://labuda-videos.s3.ap-southeast-1.amazonaws.com/$key';
        await s3Service.deleteFile(s3Url);
      } catch (e) {
        // File doesn't exist, continue
      }
    }
  }
}
