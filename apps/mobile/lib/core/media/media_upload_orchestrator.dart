import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:labuda/core/core.dart';
import 'package:wechat_assets_picker/wechat_assets_picker.dart';
import 'package:labuda/shared/ui/src/helpers/media_picker_helper.dart';
import 'package:labuda/shared/ui/src/screens/custom_camera_screen.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'media_upload_config.dart';

/// Single canonical orchestrator for foto+video pick → validate → upload → URLs.
///
/// Replaces the duplicated pick → validate → upload logic of the old
/// per-domain media handlers (for_sale now routes through MediaGridUploader).
/// Two modes, ONE engine:
///  - pickLocalFiles() — deferred upload: returns local Files; the composer
///    previews them and uploads at Send (komentar, chat).
///  - showPicker() — immediate upload: returns `List<String>` URLs (grid surfaces).
///  - showAttachSheet() — the single attachment sheet every surface shares.
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

  // ── counts (cap math input) ──
  /// What a deferred composer already holds, from its LOCAL files.
  static MediaCounts countsOfFiles(Iterable<File> files) {
    var images = 0;
    var videos = 0;
    for (final file in files) {
      if (isVideoFile(file)) {
        videos++;
      } else {
        images++;
      }
    }
    return MediaCounts(images: images, videos: videos);
  }

  /// What a grid already holds, from the UPLOADED urls it renders.
  static MediaCounts countsOfUrls(Iterable<String> urls) {
    var images = 0;
    var videos = 0;
    for (final url in urls) {
      if (isVideoUrl(url)) {
        videos++;
      } else {
        images++;
      }
    }
    return MediaCounts(images: images, videos: videos);
  }

  // ── file type helpers ──
  // Single video-detection authority for every surface (pick, preview,
  // render, upload). Extension set + owned `videos/` key prefix.
  static const _videoExts = {'mp4', 'mov', 'webm', 'm4v', 'avi', 'mkv', '3gp', 'wmv'};

  static bool isVideoFile(File file) => isVideoPath(file.path);

  static bool isVideoUrl(String url) => isVideoPath(url.trim());

  static bool isVideoPath(String path) {
    final lower = path.trim().toLowerCase();
    if (lower.isEmpty) return false;
    final pathPart = lower.split('?').first;
    final ext = pathPart.split('.').last;
    if (_videoExts.contains(ext)) return true;
    return pathPart.contains('/videos/');
  }

  // ── pick without upload (every deferred surface) ──
  /// Picks are capped by the policy — total AND per type. `current` is what the
  /// composer already holds, so "1 video + 5 foto" is enforced against the real
  /// state instead of being a number in a config file nobody reads.
  Future<List<File>> pickLocalFiles({
    required BuildContext context,
    MediaCounts current = const MediaCounts(),
  }) async {
    if (config.remainingTotalFor(current) <= 0) {
      _showError(context, _limitMessage());
      return [];
    }
    try {
      final paths = await MediaPickerHelper.pickMedia(
        context: context,
        maxAssets: config.remainingTotalFor(current),
        requestType: config.videoAllowed
            ? RequestType.common
            : RequestType.image,
      );
      if (paths == null || paths.isEmpty) return [];
      if (!context.mounted) return [];
      return await _acceptFiles(paths, context: context, current: current);
    } catch (_) {
      if (!context.mounted) return [];
      _showError(context, 'Gagal memilih media. Coba lagi.');
      return [];
    }
  }

  Future<List<File>> openCameraLocal({
    required BuildContext context,
    MediaCounts current = const MediaCounts(),
  }) async {
    if (config.remainingTotalFor(current) <= 0) {
      _showError(context, _limitMessage());
      return [];
    }
    try {
      final paths = await CustomCameraScreen.show(context);
      if (paths == null || paths.isEmpty) return [];
      if (!context.mounted) return [];
      return await _acceptFiles(paths, context: context, current: current);
    } catch (_) {
      if (!context.mounted) return [];
      _showError(context, 'Gagal membuka kamera. Coba lagi.');
      return [];
    }
  }

  // ── pick + upload in one call (grid surfaces) ──
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
      final res = isVideo
          ? await s3.uploadVideo(
              f,
              folder: config.videoFolder,
            )
          : await s3.uploadImage(
              f,
              folder: config.imageFolder,
            );
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
      AppSnackBar.showSuccess(
        context,
        '${urls.length} media berhasil diupload',
        duration: const Duration(seconds: 2),
      );
    }
    return urls;
  }

  /// Deferred-upload engine for composers (komentar + chat). Uploads only the
  /// items that have no URL yet, in order, and stops at the FIRST failure: the
  /// caller keeps the composer, so a retry resumes exactly where it stopped and
  /// never re-uploads a file that already succeeded. No modal, no dialog —
  /// upload progress belongs inside the composer, not on top of it.
  /// Uploads the batch AT SEND, one file at a time, reporting per file.
  ///
  /// [roomId] switches the SAME loop to chat's register → PUT flow, where the
  /// item ends up carrying an `assetId` instead of a read URL. Files are
  /// independent: a failure on file 3 does not strand files 4 and 5 — it is
  /// recorded on that file, the rest still upload, and the batch reports failure
  /// only if ANY file failed. That is exactly what a retry re-runs, because items
  /// that already hold a URL/asset are skipped.
  ///
  /// [onChanged] is called whenever a file changes state or advances, so the
  /// composer owns the rebuild.
  static Future<bool> uploadPending({
    required BuildContext context,
    required MediaUploadConfig config,
    required List<MediaPendingItem> items,
    String? roomId,
    void Function()? onChanged,
  }) async {
    if (items.isEmpty) return true;
    final s3 = ProviderScope.containerOf(
      context,
      listen: false,
    ).read(s3ServiceProvider);

    MediaPendingItem? current;
    s3.onPutProgress = (sent, total) {
      final item = current;
      if (item == null || total <= 0) return;
      if (item.reportProgress(sent / total)) onChanged?.call();
    };

    var failures = 0;
    try {
      for (final item in items) {
        if (item.uploaded) continue;

        item.phase = MediaUploadPhase.uploading;
        item.progress = 0;
        item.error = null;
        current = item;
        onChanged?.call();

        String? value; // read URL (chat stores its asset id on the item instead)
        String? failure;
        if (roomId != null) {
          final res = await s3.uploadChatMedia(
            roomId: roomId,
            file: item.file,
          );
          if (res.isSuccess && res.data != null) {
            item.assetId = res.data!.assetId;
            value = res.data!.readUrl;
          } else {
            failure = res.error ?? 'Upload media gagal';
          }
        } else if (isVideoFile(item.file)) {
          final res = await s3.uploadVideo(
            item.file,
            folder: config.videoFolder,
          );
          if (res.isSuccess && res.data != null) {
            value = res.data;
          } else {
            failure = res.error ?? 'Upload media gagal';
          }
        } else {
          final res = await s3.uploadImage(
            item.file,
            folder: config.imageFolder,
          );
          if (res.isSuccess && res.data != null) {
            value = res.data;
          } else {
            failure = res.error ?? 'Upload media gagal';
          }
        }

        if (value == null) {
          item.phase = MediaUploadPhase.failed;
          item.error = failure ?? 'Upload media gagal';
          item.progress = 0;
          failures++;
        } else {
          item.url = value;
          item.phase = MediaUploadPhase.uploaded;
          item.progress = 1;
        }
        onChanged?.call();
      }
    } finally {
      current = null;
      s3.onPutProgress = null;
    }

    if (failures > 0 && context.mounted) {
      _showErrorStatic(
        context,
        failures == 1
            ? '1 media gagal diunggah. Ketuk Coba lagi.'
            : '$failures media gagal diunggah. Ketuk Coba lagi.',
      );
    }
    return failures == 0;
  }

  // ── bottomSheet entry — ONE attachment sheet for every surface ──
  //
  // Entries: Galeri (foto & video in ONE system picker), Kamera, plus
  // caller-provided actions (commerce). Picking NEVER uploads: the caller gets
  // local Files and owns the timing. Komentar/chat defer to Send, so a cancel
  // costs nothing, leaves no orphan S3 object, and the blocking progress modal
  // is gone from both composers.
  /// Entries are DERIVED from the policy: `videoAllowed` decides the picker
  /// request type and the copy. Identity surfaces (one photo, no video) reuse
  /// this sheet instead of owning a second modal design.
  static void showAttachSheet({
    required BuildContext context,
    required MediaUploadConfig config,
    required Future<void> Function(List<File> files) onPicked,
    MediaCounts current = const MediaCounts(),
    List<MediaSheetAction> extraActions = const [],
  }) {
    final orchestrator = MediaUploadOrchestrator(config: config);
    final outer = context;
    showModalBottomSheet(
      context: outer,
      backgroundColor: Colors.transparent,
      builder: (sheetCtx) => Container(
        decoration: BoxDecoration(
          color: Theme.of(sheetCtx).scaffoldBackgroundColor,
          borderRadius: const BorderRadius.vertical(top: Radius.circular(AppShape.r20)),
        ),
        padding: const EdgeInsets.all(AppMetrics.p20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _sheetOption(
              context: sheetCtx,
              icon: Icons.photo_library,
              label: 'Galeri',
              subtitle: config.videoAllowed ? 'Foto & video' : 'Foto',
              onTap: () async {
                Navigator.pop(sheetCtx);
                final files = await orchestrator.pickLocalFiles(
                  context: outer,
                  current: current,
                );
                if (!outer.mounted || files.isEmpty) return;
                await onPicked(files);
              },
            ),
            _sheetOption(
              context: sheetCtx,
              icon: Icons.camera_alt,
              label: 'Kamera',
              subtitle: config.videoAllowed
                  ? 'Ambil foto atau video baru'
                  : 'Ambil foto baru',
              onTap: () async {
                Navigator.pop(sheetCtx);
                final files = await orchestrator.openCameraLocal(
                  context: outer,
                  current: current,
                );
                if (!outer.mounted || files.isEmpty) return;
                await onPicked(files);
              },
            ),
            if (extraActions.isNotEmpty) ...[
              const Divider(),
              for (final action in extraActions)
                _sheetOption(
                  context: sheetCtx,
                  icon: action.icon,
                  label: action.label,
                  subtitle: action.subtitle,
                  onTap: () {
                    Navigator.pop(sheetCtx);
                    action.onTap();
                  },
                ),
            ],
          ],
        ),
      ),
    );
  }

  /// Immediate-upload entry (content/commerce grids): the SAME sheet, uploading
  /// on pick. Kept for surfaces that own their own grid state.
  static void showPicker({
    required BuildContext context,
    required MediaUploadConfig config,
    required Future<void> Function(List<String> urls) onUploaded,
    MediaCounts current = const MediaCounts(),
  }) {
    final orchestrator = MediaUploadOrchestrator(config: config);
    showAttachSheet(
      context: context,
      config: config,
      current: current,
      onPicked: (files) => _handleWithProgress(
        context: context,
        files: files,
        orchestrator: orchestrator,
        onUploaded: onUploaded,
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
        AppSnackBar.showError(context, 'Upload gagal: $e');
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
      AppSnackBar.showError(
        context,
        'Tidak ada media yang berhasil diupload. Coba lagi.',
      );
    }
  }

  // ── headless single-type picks (evidence/external): same engine, no sheet ──
  // Business rules (counts, required-vs-optional) stay with the caller;
  // picking mechanics + MB validation live here.
  static Future<XFile?> pickGalleryVideo({
    required BuildContext context,
    Duration maxDuration = const Duration(minutes: 2),
  }) async {
    try {
      final video = await ImagePicker().pickVideo(
        source: ImageSource.gallery,
        maxDuration: maxDuration,
      );
      if (video == null || !context.mounted) return null;
      final ok = await validateFile(
        File(video.path),
        context,
        MediaUploadConfig.forEvidence,
      );
      return ok ? video : null;
    } catch (_) {
      if (!context.mounted) return null;
      _showErrorStatic(context, 'Gagal memilih video. Coba lagi.');
      return null;
    }
  }

  static Future<List<XFile>> pickGalleryImages({
    required BuildContext context,
    int maxAssets = 5,
    int imageQuality = 80,
    double maxWidth = 1920,
    double maxHeight = 1920,
  }) async {
    try {
      final photos = await ImagePicker().pickMultiImage(
        imageQuality: imageQuality,
        maxWidth: maxWidth,
        maxHeight: maxHeight,
      );
      if (photos.isEmpty || !context.mounted) return [];
      // The policy caps the per-pick budget; accumulation stays the caller's
      // guard (evidence: 1 video + up to 5 foto).
      final budget = maxAssets.clamp(
        0,
        MediaUploadConfig.forEvidence.maxImages,
      );
      final valid = <XFile>[];
      for (final p in photos.take(budget)) {
        if (!context.mounted) break;
        final ok = await validateFile(
          File(p.path),
          context,
          MediaUploadConfig.forEvidence,
        );
        if (ok) valid.add(p);
      }
      return valid;
    } catch (_) {
      if (!context.mounted) return [];
      _showErrorStatic(context, 'Gagal memilih foto. Coba lagi.');
      return [];
    }
  }

  // ── acceptance (per-type caps + size) ──
  /// Applies the entire policy and reports exactly what was dropped. A pick
  /// that would exceed the per-type caps is clipped WITH a message — never
  /// silently accepted, never silently dropped.
  Future<List<File>> _acceptFiles(
    List<String> paths, {
    required BuildContext context,
    required MediaCounts current,
  }) async {
    final room = config.remaining(current);
    var imagesLeft = room.images;
    var videosLeft = room.videos;
    final accepted = <File>[];
    var dropped = 0;

    for (final p in paths) {
      if (!context.mounted) break;
      final file = File(p);
      final isVideo = isVideoFile(file);
      if (isVideo ? videosLeft <= 0 : imagesLeft <= 0) {
        dropped++;
        continue;
      }
      if (!await validateFile(file, context, config)) {
        dropped++;
        continue;
      }
      if (isVideo) {
        videosLeft--;
      } else {
        imagesLeft--;
      }
      accepted.add(file);
    }

    if (dropped > 0 && context.mounted) {
      _showErrorStatic(context, _limitMessage());
    }
    return accepted;
  }

  String _limitMessage() => config.videoAllowed
      ? 'Maksimal ${config.maxImages} foto & ${config.maxVideos} video'
      : 'Maksimal ${config.maxImages} foto';

  static Future<bool> validateFile(
    File file,
    BuildContext context,
    MediaUploadConfig cfg,
  ) async {
    final bytes = await file.length();
    final mb = bytes / (1024 * 1024);
    final isVideo = isVideoFile(file);
    final max = isVideo ? cfg.maxVideoSizeMb : cfg.maxImageSizeMb;
    if (mb > max) {
      if (!context.mounted) return false;
      _showErrorStatic(
        context,
        'Ukuran ${isVideo ? "video" : "foto"} maksimal ${max}MB',
      );
      return false;
    }
    return true;
  }

  void _showError(BuildContext context, String msg) =>
      _showErrorStatic(context, msg);

  static void _showErrorStatic(BuildContext context, String msg) {
    if (!context.mounted) return;
    AppSnackBar.showError(context, msg, duration: const Duration(seconds: 4));
  }

  static Widget _sheetOption({
    required BuildContext context,
    required IconData icon,
    required String label,
    String? subtitle,
    required VoidCallback onTap,
  }) {
    return ListTile(
      leading: Icon(icon, color: Theme.of(context).colorScheme.primary),
      title: Text(label),
      subtitle: subtitle == null ? null : Text(subtitle),
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

/// One extra entry a caller adds to the canonical attach sheet (the commerce
/// reference on komentar/chat). The sheet owns the presentation; the caller
/// owns the capability gate.
class MediaSheetAction {
  final IconData icon;
  final String label;
  final String? subtitle;
  final VoidCallback onTap;

  const MediaSheetAction({
    required this.icon,
    required this.label,
    this.subtitle,
    required this.onTap,
  });
}

/// Local, not-yet-uploaded media held by a composer (deferred upload). `url`
/// becomes non-null the moment S3 accepts the file, so a retry after a partial
/// failure never uploads the same file twice.
/// One attachment's own lifecycle.
///
/// The batch is judged FILE BY FILE: a five-file send can succeed on four and
/// fail on the fifth, and a retry then re-runs exactly that one file. A single
/// "uploading?" flag for the whole batch cannot say which file is stuck.
enum MediaUploadPhase { waiting, uploading, uploaded, failed }

class MediaPendingItem {
  final File file;

  /// Read URL (comment) once the upload went through.
  String? url;

  /// Room-scoped asset id (chat) — same lifecycle, different payload.
  String? assetId;

  MediaUploadPhase phase = MediaUploadPhase.waiting;

  /// Byte progress of the current upload, 0..1.
  double progress = 0;

  /// Why this file failed. Shown on the strip for THIS file, not for the batch.
  String? error;

  MediaPendingItem(this.file);

  bool get uploaded => url != null || assetId != null;
  bool get isVideo => MediaUploadOrchestrator.isVideoFile(file);

  /// Reports progress without waking the UI for every byte.
  ///
  /// A video PUT fires this thousands of times; 1% steps are visually
  /// indistinguishable, so anything smaller is dropped.
  bool reportProgress(double value) {
    if (phase != MediaUploadPhase.uploading) return false;
    final next = value.clamp(0.0, 1.0).toDouble();
    if (next < 1.0 && next - progress < 0.01) return false;
    progress = next;
    return true;
  }
}
