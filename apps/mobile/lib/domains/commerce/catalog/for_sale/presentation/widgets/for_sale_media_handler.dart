import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/ui/src/helpers/media_picker_helper.dart';
import 'package:labuda/shared/ui/src/screens/custom_camera_screen.dart';

/// @deprecated Gunakan [MediaUploadOrchestrator.forCommerce()] — 1 mesin foto+video.
/// Dipertahankan untuk kompatibilitas, delegasi ke orchestrator.
@Deprecated('Gunakan MediaUploadOrchestrator.forCommerce()')
class ForSaleMediaHandler {
  static const int maxMedia = 10;
  static const int maxImageSizeMb = 10;
  static const int maxVideoSizeMb = 100;
  static const int maxVideoDurationMs = 180000; // 3 minutes



  /// Whether a file is a video based on extension.
  static bool _isVideoFile(File file) {
    final ext = file.path.split('.').last.toLowerCase();
    return const {'mp4', 'mov', 'webm', 'm4v', 'avi', 'mkv', '3gp', 'wmv'}.contains(ext);
  }

  /// Pick media (photos + videos) from gallery.
  Future<List<File>> pickMediaFromGallery({
    required BuildContext context,
    int currentMediaCount = 0,
  }) async {
    final maxAssets = maxMedia - currentMediaCount;
    if (maxAssets <= 0) {
      _showError(context, 'Maksimal $maxMedia media untuk forSale');
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
        if (!context.mounted) continue;
        if (await _validateFile(file, context)) {
          validFiles.add(file);
        }
      }

      return validFiles;
    } catch (e) {
      if (!context.mounted) return [];
      _showError(context, 'Gagal memilih media. Coba lagi.');
      return [];
    }
  }

  /// Open camera for photo capture.
  Future<List<File>> openCamera({
    required BuildContext context,
    int currentMediaCount = 0,
  }) async {
    final maxAssets = maxMedia - currentMediaCount;
    if (maxAssets <= 0) {
      _showError(context, 'Maksimal $maxMedia media untuk forSale');
      return [];
    }

    try {
      final mediaUrls = await CustomCameraScreen.show(context);

      if (mediaUrls == null || mediaUrls.isEmpty) return [];

      final List<File> validFiles = [];
      for (final path in mediaUrls) {
        final file = File(path);
        if (!context.mounted) continue;
        if (await _validateFile(file, context)) {
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

  /// Upload media to S3 and return URLs.
  ///
  /// Uses canonical primitives:
  /// - S3Service.uploadImage() for image files
  /// - S3Service.uploadVideo() for video files
  ///
  /// Preserves selection order (sequential upload, no concurrency).
  ///
  /// The S3 service is read from the canonical [s3ServiceProvider] via the
  /// nearest [ProviderScope] downstream of [context], so tests can override it
  /// with a fake without changing the handler API.
  Future<List<String>> uploadMedia({
    required BuildContext context,
    required List<File> files,
  }) async {
    if (files.isEmpty) return [];

    final s3Service = ProviderScope.containerOf(context, listen: false).read(s3ServiceProvider);
    final List<String> uploadedUrls = [];
    int successCount = 0;
    int failCount = 0;

    for (final file in files) {
      final isVideo = _isVideoFile(file);
      final result = isVideo
          ? await s3Service.uploadVideo(file)
          : await s3Service.uploadImage(file);

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
        '$successCount media berhasil diupload, $failCount gagal',
      );
    } else if (successCount > 0) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('$successCount media berhasil diupload'),
          backgroundColor: AppColors.successGreen,
          duration: const Duration(seconds: 2),
        ),
      );
    }

    return uploadedUrls;
  }

  /// Validate file (size check).
  Future<bool> _validateFile(File file, BuildContext context) async {
    final bytes = await file.length();
    final sizeMB = bytes / (1024 * 1024);
    final isVideo = _isVideoFile(file);
    final maxSize = isVideo ? maxVideoSizeMb : maxImageSizeMb;

    if (sizeMB > maxSize) {
      if (!context.mounted) return false;
      _showError(context, 'Ukuran ${isVideo ? "video" : "foto"} maksimal ${maxSize}MB');
      return false;
    }
    return true;
  }

  /// Show error messages.
  void _showError(BuildContext context, String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        backgroundColor: AppColors.statusError,
        duration: const Duration(seconds: 4),
      ),
    );
  }

  /// Show media picker bottom sheet — Gallery & Camera.
  ///
  /// Fixed P1: uses [outerContext] (form scaffold) for pick/upload, not the
  /// ephemeral bottomSheet [sheetContext] which is unmounted after pop.
  static void showMediaPicker({
    required BuildContext context,
    required Future<void> Function(List<String> urls) onMediaUploaded,
    int currentMediaCount = 0,
  }) {
    final handler = ForSaleMediaHandler();
    final outerContext = context;

    showModalBottomSheet(
      context: outerContext,
      backgroundColor: Colors.transparent,
      builder: (sheetContext) => Container(
        decoration: BoxDecoration(
          color: Theme.of(sheetContext).scaffoldBackgroundColor,
          borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
        ),
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _buildOption(
              context: sheetContext,
              icon: Icons.photo_library,
              label: 'Galeri',
              onTap: () async {
                Navigator.pop(sheetContext);
                // Use outerContext — sheetContext is dead after pop.
                final files = await handler.pickMediaFromGallery(
                  context: outerContext,
                  currentMediaCount: currentMediaCount,
                );
                if (!outerContext.mounted) return;
                await _handleMediaSelection(
                  context: outerContext,
                  handler: handler,
                  files: files,
                  onMediaUploaded: onMediaUploaded,
                );
              },
            ),
            _buildOption(
              context: sheetContext,
              icon: Icons.camera_alt,
              label: 'Kamera',
              onTap: () async {
                Navigator.pop(sheetContext);
                final files = await handler.openCamera(
                  context: outerContext,
                  currentMediaCount: currentMediaCount,
                );
                if (!outerContext.mounted) return;
                await _handleMediaSelection(
                  context: outerContext,
                  handler: handler,
                  files: files,
                  onMediaUploaded: onMediaUploaded,
                );
              },
            ),
          ],
        ),
      ),
    );
  }

  static Future<void> _handleMediaSelection({
    required BuildContext context,
    required ForSaleMediaHandler handler,
    required List<File> files,
    required Future<void> Function(List<String> urls) onMediaUploaded,
  }) async {
    if (files.isEmpty) return;

    if (!context.mounted) return;

    // Show progress dialog
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => const _UploadProgressDialog(),
    );

    List<String> urls = [];
    try {
      urls = await handler.uploadMedia(context: context, files: files);
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Upload gagal: $e'),
            backgroundColor: AppColors.statusError,
          ),
        );
      }
    } finally {
      if (context.mounted) {
        // Close progress dialog — pop only if dialog is still on stack.
        if (Navigator.of(context).canPop()) {
          Navigator.of(context).pop();
        }
      }
    }

    if (!context.mounted) return;

    // Always notify if urls exist; surface empty case explicitly.
    if (urls.isNotEmpty) {
      await onMediaUploaded(urls);
    } else {
      // No leaked silent failure — user already saw per-file error/snackbar
      // from uploadMedia, but ensure form knows attempt finished.
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Tidak ada media yang berhasil diupload. Coba lagi.'),
          backgroundColor: AppColors.statusError,
        ),
      );
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

/// Upload progress dialog.
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
            'Mengupload media...',
            style: TextStyle(color: Theme.of(context).colorScheme.onSurface),
          ),
        ],
      ),
    );
  }
}
