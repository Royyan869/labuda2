import 'dart:io';
import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/ui/src/helpers/media_picker_helper.dart';
import 'package:labuda/shared/ui/src/screens/custom_camera_screen.dart';

/// ForSale Media Handler - Handles photo selection for forSales
///
/// Features:
/// - Photo selection from gallery
/// - Camera with photo capture
/// - Multiple selection support
/// - File size validation
/// - S3 upload with proper error handling
class ForSaleMediaHandler {
  static const int maxImages = 10;
  static const int maxImageSizeMb = 10;

  /// Pick photos from gallery
  Future<List<File>> pickPhotosFromGallery({
    required BuildContext context,
    int currentMediaCount = 0,
  }) async {
    final maxAssets = maxImages - currentMediaCount;
    if (maxAssets <= 0) {
      _showError(context, 'Maksimal $maxImages foto untuk forSale');
      return [];
    }

    try {
      final mediaUrls = await MediaPickerHelper.pickPhotos(
        context: context,
        maxAssets: maxAssets,
      );

      if (mediaUrls == null || mediaUrls.isEmpty) return [];

      final List<File> validFiles = [];
      for (final path in mediaUrls) {
        final file = File(path);
        if (!context.mounted) continue;
        if (await _validateImageFile(file, context)) {
          validFiles.add(file);
        }
      }

      return validFiles;
    } catch (e) {
      if (!context.mounted) return [];
      _showError(context, 'Gagal memilih foto. Coba lagi.');
      return [];
    }
  }

  /// Open camera for photo capture
  Future<List<File>> openCamera({
    required BuildContext context,
    int currentMediaCount = 0,
  }) async {
    final maxAssets = maxImages - currentMediaCount;
    if (maxAssets <= 0) {
      _showError(context, 'Maksimal $maxImages foto untuk forSale');
      return [];
    }

    try {
      final mediaUrls = await CustomCameraScreen.show(context);

      if (mediaUrls == null || mediaUrls.isEmpty) return [];

      final List<File> validFiles = [];
      for (final path in mediaUrls) {
        final file = File(path);
        if (!context.mounted) continue;
        if (await _validateImageFile(file, context)) {
          validFiles.add(file);
        }
      }

      return validFiles;
    } catch (e) {
      if (!context.mounted) return [];
      _showError(context, 'Gagal membuka kamera. Coba lagi.');
      return [];
    }
  }

  /// Upload photos to S3 and return URLs
  ///
  /// Returns a list of successfully uploaded URLs.
  /// Failed uploads are reported via SnackBar but don't block the entire operation.
  Future<List<String>> uploadPhotos({
    required BuildContext context,
    required List<File> photos,
  }) async {
    if (photos.isEmpty) return [];

    final s3Service = S3Service();
    final List<String> uploadedUrls = [];
    int successCount = 0;
    int failCount = 0;

    for (final photo in photos) {
      final result = await s3Service.uploadImage(photo);
      if (result.isSuccess && result.data != null) {
        uploadedUrls.add(result.data!);
        successCount++;
      } else {
        failCount++;
      }
    }

    if (!context.mounted) return uploadedUrls;

    // Report results
    if (failCount > 0) {
      _showError(
        context,
        '$successCount foto berhasil diupload, $failCount gagal',
      );
    } else if (successCount > 0) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('$successCount foto berhasil diupload'),
          backgroundColor: AppColors.successGreen,
          duration: const Duration(seconds: 2),
        ),
      );
    }

    return uploadedUrls;
  }

  /// Validate image file
  Future<bool> _validateImageFile(File file, BuildContext context) async {
    final bytes = await file.length();
    final sizeMB = bytes / (1024 * 1024);

    if (sizeMB > maxImageSizeMb) {
      if (!context.mounted) return false;
      _showError(context, 'Ukuran foto maksimal ${maxImageSizeMb}MB');
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

  /// Show media picker bottom sheet - 2 options: Gallery & Camera
  static void showMediaPicker({
    required BuildContext context,
    required Future<void> Function(List<String> urls) onMediaUploaded,
    int currentMediaCount = 0,
  }) {
    final handler = ForSaleMediaHandler();

    showModalBottomSheet(
      context: context,
      backgroundColor: Colors.transparent,
      builder: (context) => Container(
        decoration: BoxDecoration(
          color: Theme.of(context).scaffoldBackgroundColor,
          borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
        ),
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _buildOption(
              context: context,
              icon: Icons.photo_library,
              label: 'Galeri',
              onTap: () async {
                final ctx = context;
                Navigator.pop(ctx);
                final photos = await handler.pickPhotosFromGallery(
                  context: ctx,
                  currentMediaCount: currentMediaCount,
                );
                if (ctx.mounted) {
                  await _handlePhotoSelection(
                    context: ctx,
                    handler: handler,
                    photos: photos,
                    onMediaUploaded: onMediaUploaded,
                  );
                }
              },
            ),
            _buildOption(
              context: context,
              icon: Icons.camera_alt,
              label: 'Kamera',
              onTap: () async {
                final ctx = context;
                Navigator.pop(ctx);
                final photos = await handler.openCamera(
                  context: ctx,
                  currentMediaCount: currentMediaCount,
                );
                if (ctx.mounted) {
                  await _handlePhotoSelection(
                    context: ctx,
                    handler: handler,
                    photos: photos,
                    onMediaUploaded: onMediaUploaded,
                  );
                }
              },
            ),
          ],
        ),
      ),
    );
  }

  static Future<void> _handlePhotoSelection({
    required BuildContext context,
    required ForSaleMediaHandler handler,
    required List<File> photos,
    required Future<void> Function(List<String> urls) onMediaUploaded,
  }) async {
    if (photos.isEmpty) return;

    // Show uploading indicator
    if (!context.mounted) return;

    // Show progress dialog
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (context) => const _UploadProgressDialog(),
    );

    final urls = await handler.uploadPhotos(context: context, photos: photos);

    // Close progress dialog
    if (!context.mounted) return;
    Navigator.of(context).pop();

    // Notify callback with uploaded URLs
    if (urls.isNotEmpty) {
      await onMediaUploaded(urls);
    }
  }

  static Widget _buildOption({
    required BuildContext context,
    required IconData icon,
    required String label,
    required VoidCallback onTap,
  }) {
    return ListTile(
      leading: Icon(icon, color: AppColors.primaryRed),
      title: Text(label),
      onTap: onTap,
    );
  }
}

/// Upload progress dialog
class _UploadProgressDialog extends StatelessWidget {
  const _UploadProgressDialog();

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const CircularProgressIndicator(),
          const SizedBox(height: 16),
          Text(
            'Mengupload foto...',
            style: TextStyle(color: Theme.of(context).colorScheme.onSurface),
          ),
        ],
      ),
    );
  }
}
