import 'dart:convert';
import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:labuda/shared/shared.dart';
import 'web_image_cropper.dart';
import 'flutter_crop_image.dart';

/// Single CROP authority: local file → crop (crop_your_image on mobile, canvas
/// on web) → cropped local path.
///
/// Picking is NOT here. Every surface (avatar, cover, store photo, media
/// attachments) picks through `MediaUploadOrchestrator`; this class only runs
/// the crop step, and upload stays the caller's job.
class AvatarImageProcessor {
  static Future<void> cropImage(
    BuildContext context,
    File file,
    Function(String?) onCropped, {
    double aspectRatio = 1.0,
    bool circularCrop = true,
    String cropTitle = 'Crop Foto',
  }) async {
    try {
      if (kIsWeb) {
        await _showWebCropper(
          context,
          XFile(file.path),
          onCropped,
          aspectRatio: aspectRatio,
          circularCrop: circularCrop,
          cropTitle: cropTitle,
        );
      } else {
        await _showMobileCropper(
          context,
          XFile(file.path),
          onCropped,
          aspectRatio: aspectRatio,
          circularCrop: circularCrop,
          cropTitle: cropTitle,
        );
      }
    } catch (e) {
      debugPrint('AvatarImageProcessor: crop failed - $e');
      if (context.mounted) {
        AppSnackBar.showError(context, 'Gagal memotong gambar. Coba lagi.');
      }
      onCropped(null);
    }
  }

  static Future<void> _showWebCropper(
    BuildContext context,
    XFile imageFile,
    Function(String?) onCropped, {
    double aspectRatio = 1.0,
    bool circularCrop = true,
    String cropTitle = 'Crop Foto',
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
            onCropped(dataUrl);
          },
          onCancel: () => Navigator.of(context).pop(),
        ),
      ),
    );
  }

  static Future<void> _showMobileCropper(
    BuildContext context,
    XFile imageFile,
    Function(String?) onCropped, {
    double aspectRatio = 1.0,
    bool circularCrop = true,
    String cropTitle = 'Crop Foto',
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
              final tempPath = await _saveCroppedBytesToTempFile(croppedBytes);
              // DON'T pop here - FlutterImageCropper already pops itself.
              onCropped(tempPath);
            } catch (e) {
              debugPrint('AvatarImageProcessor: save failed - $e');
              if (context.mounted) {
                AppSnackBar.showError(context, 'Gagal menyimpan gambar. Coba lagi.');
              }
              onCropped(null);
            }
          },
        ),
      ),
    );
  }

  /// Save cropped bytes to a temporary file and return its path.
  static Future<String> _saveCroppedBytesToTempFile(
    Uint8List croppedBytes,
  ) async {
    try {
      final directory = await Directory.systemTemp.createTemp('labuda_crop_');
      final timestamp = DateTime.now().millisecondsSinceEpoch;
      final tempFile = File('${directory.path}/cropped_$timestamp.jpg');
      await tempFile.writeAsBytes(croppedBytes);
      return tempFile.path;
    } catch (e) {
      throw Exception('Failed to save cropped image: $e');
    }
  }
}
