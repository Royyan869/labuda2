import 'dart:convert';
import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:labuda/shared/shared.dart';
import 'web_image_cropper.dart';
import 'flutter_crop_image.dart';

/// Single crop authority: pick (OS single-shot camera/gallery) → crop
/// (crop_your_image on mobile, canvas on web) → cropped local path.
/// Upload is the caller's job (S3 fixed key via the domain service).
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

}
