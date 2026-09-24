import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/ui/src/helpers/media_picker_helper.dart';
import 'package:labuda/shared/ui/src/screens/custom_camera_screen.dart';
import 'media_upload_config.dart';

/// Single canonical orchestrator for foto+video pick → validate → upload → URLs.
///
/// Replaces duplicated logic in ContentMediaHandler + ForSaleMediaHandler.
/// Exposes 2 modes:
///  - pickLocalFiles() — deferred upload (content: returns File, preview lokal)
///  - pickAndUpload() / showPickerAndUpload() — immediate upload (commerce/komentar/chat: returns List<String> URLs)
///
/// Single place to change wechat_assets_picker, S3 presign, max limits.
class MediaUploadOrchestrator {
  final MediaUploadConfig config;

  const MediaUploadOrchestrator({required this.config});

  // Presets
  factory MediaUploadOrchestrator.forContent() =>
      const MediaUploadOrchestrator(config: MediaUploadConfig.forContent);
  factory MediaUploadOrchestrator.forCommerce() =>
      const MediaUploadOrchestrator(config: MediaUploadConfig.forCommerce);
  factory MediaUploadOrchestrator.forComment() =>
      const MediaUploadOrchestrator(config: MediaUploadConfig.forComment);
  factory MediaUploadOrchestrator.forChat() =>
      const MediaUploadOrchestrator(config: MediaUploadConfig.forChat);

  // ── file type helpers ──
  static const _videoExts = {'mp4', 'mov', 'webm', 'm4v', 'avi', 'mkv', '3gp', 'wmv'};

  static bool isVideoFile(File file) {
    final ext = file.path.split('.').last.toLowerCase();
    return _videoExts.contains(ext);
  }

  // ── pick without upload (content) ──
  Future<List<File>> pickLocalFiles({
    required BuildContext context,
    int currentCount = 0,
  }) async {
    final remaining = config.remaining(currentCount);
    if (remaining <= 0) {
      _showError(context, 'Maksimal ${config.maxTotal} media');
      return [];
    }
    try {
      final paths = await MediaPickerHelper.pickMedia(
        context: context,
        maxAssets: remaining,
      );
      if (paths == null || paths.isEmpty) return [];
      return await _validateFiles(paths, context);
    } catch (_) {
      if (!context.mounted) return [];
      _showError(context, 'Gagal memilih media. Coba lagi.');
      return [];
    }
  }

  Future<List<File>> openCameraLocal({
    required BuildContext context,
    int currentCount = 0,
  }) async {
    final remaining = config.remaining(currentCount);
    if (remaining <= 0) {
      _showError(context, 'Maksimal ${config.maxTotal} media');
      return [];
    }
    try {
      final paths = await CustomCameraScreen.show(context);
      if (paths == null || paths.isEmpty) return [];
      return await _validateFiles(paths, context);
    } catch (_) {
      if (!context.mounted) return [];
      _showError(context, 'Gagal membuka kamera. Coba lagi.');
      return [];
    }
  }

  // ── pick + immediate upload (commerce/comment/chat) ──
  Future<List<String>> pickAndUpload({
    required BuildContext context,
    int currentCount = 0,
  }) async {
    final files = await pickLocalFiles(context: context, currentCount: currentCount);
    if (files.isEmpty) return [];
    if (!context.mounted) return [];
    return uploadFiles(context: context, files: files);
  }

  Future<List<String>> openCameraAndUpload({
    required BuildContext context,
    int currentCount = 0,
  }) async {
    final files = await openCameraLocal(context: context, currentCount: currentCount);
    if (files.isEmpty) return [];
    if (!context.mounted) return [];
    return uploadFiles(context: context, files: files);
  }

  Future<List<String>> uploadFiles({
    required BuildContext context,
    required List<File> files,
  }) async {
    if (files.isEmpty) return [];
    final s3 = ProviderScope.containerOf(context, listen: false).read(s3ServiceProvider);
    final List<String> urls = [];
    int fail = 0;
    for (final f in files) {
      final isVideo = isVideoFile(f);
      final res = isVideo ? await s3.uploadVideo(f) : await s3.uploadImage(f);
      if (res.isSuccess && res.data != null) {
        urls.add(res.data!);
      } else {
        fail++;
      }
    }
    if (!context.mounted) return urls;
    if (fail > 0) {
      _showError(context, '${urls.length} berhasil, $fail gagal');
    } else if (urls.isNotEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('${urls.length} media berhasil diupload'),
          backgroundColor: AppColors.successGreen,
          duration: const Duration(seconds: 2),
        ),
      );
    }
    return urls;
  }

  // ── bottomSheet entry (single place, outerContext-safe) ──
  static void showPicker({
    required BuildContext context,
    required MediaUploadConfig config,
    required Future<void> Function(List<String> urls) onUploaded,
    int currentCount = 0,
  }) {
    final orchestrator = MediaUploadOrchestrator(config: config);
    final outer = context;
    showModalBottomSheet(
      context: outer,
      backgroundColor: Colors.transparent,
      builder: (sheetCtx) => Container(
        decoration: BoxDecoration(
          color: Theme.of(sheetCtx).scaffoldBackgroundColor,
          borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
        ),
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _sheetOption(
              context: sheetCtx,
              icon: Icons.photo_library,
              label: 'Galeri',
              onTap: () async {
                Navigator.pop(sheetCtx);
                final files = await orchestrator.pickLocalFiles(
                  context: outer,
                  currentCount: currentCount,
                );
                if (!outer.mounted) return;
                if (files.isEmpty) return;
                await _handleWithProgress(
                  context: outer,
                  files: files,
                  orchestrator: orchestrator,
                  onUploaded: onUploaded,
                );
              },
            ),
            _sheetOption(
              context: sheetCtx,
              icon: Icons.camera_alt,
              label: 'Kamera',
              onTap: () async {
                Navigator.pop(sheetCtx);
                final files = await orchestrator.openCameraLocal(
                  context: outer,
                  currentCount: currentCount,
                );
                if (!outer.mounted) return;
                if (files.isEmpty) return;
                await _handleWithProgress(
                  context: outer,
                  files: files,
                  orchestrator: orchestrator,
                  onUploaded: onUploaded,
                );
              },
            ),
          ],
        ),
      ),
    );
  }

  static Future<void> _handleWithProgress({
    required BuildContext context,
    required List<File> files,
    required MediaUploadOrchestrator orchestrator,
    required Future<void> Function(List<String> urls) onUploaded,
  }) async {
    if (!context.mounted) return;
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (_) => const _ProgressDialog(),
    );
    List<String> urls = [];
    try {
      urls = await orchestrator.uploadFiles(context: context, files: files);
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Upload gagal: $e'), backgroundColor: AppColors.statusError),
        );
      }
    } finally {
      if (context.mounted && Navigator.of(context).canPop()) {
        Navigator.of(context).pop();
      }
    }
    if (!context.mounted) return;
    if (urls.isNotEmpty) {
      await onUploaded(urls);
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Tidak ada media yang berhasil diupload. Coba lagi.'),
          backgroundColor: AppColors.statusError,
        ),
      );
    }
  }

  // ── validation ──
  Future<List<File>> _validateFiles(List<String> paths, BuildContext context) async {
    final List<File> valid = [];
    for (final p in paths) {
      final f = File(p);
      if (!context.mounted) continue;
      if (await _validateSingle(f, context)) valid.add(f);
    }
    return valid;
  }

  Future<bool> _validateSingle(File file, BuildContext context) async {
    final bytes = await file.length();
    final mb = bytes / (1024 * 1024);
    final isVideo = isVideoFile(file);
    final max = isVideo ? config.maxVideoSizeMb : config.maxImageSizeMb;
    if (mb > max) {
      if (!context.mounted) return false;
      _showError(context, 'Ukuran ${isVideo ? "video" : "foto"} maksimal ${max}MB');
      return false;
    }
    return true;
  }

  void _showError(BuildContext context, String msg) {
    if (!context.mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(msg), backgroundColor: AppColors.statusError, duration: const Duration(seconds: 4)),
    );
  }

  static Widget _sheetOption({
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

class _ProgressDialog extends StatelessWidget {
  const _ProgressDialog();
  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const CircularProgressIndicator(),
          const SizedBox(height: 16),
          Text('Mengupload media...', style: TextStyle(color: Theme.of(context).colorScheme.onSurface)),
        ],
      ),
    );
  }
}
