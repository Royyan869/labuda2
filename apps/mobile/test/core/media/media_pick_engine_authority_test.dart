import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// NEGATIVE CONTRACT — one media pick engine.
///
/// `MediaUploadOrchestrator` is the single pick→validate authority. The only
/// exception is the avatar single-shot OS camera (`avatar_image_processor`,
/// feeding the crop flow) — a documented variant, not a parallel engine.
/// Every other surface (content, commerce, comment, chat, dispute, refund,
/// external product) picks through the orchestrator.
void main() {
  group('single pick engine', () {
    final libRoot = Directory('lib');

    List<File> dartFiles() => libRoot
        .listSync(recursive: true)
        .whereType<File>()
        .where((f) => f.path.endsWith('.dart'))
        .toList();

    test('no direct ImagePicker use outside the engine + avatar variant', () {
      const allowed = [
        'core/media/media_upload_orchestrator.dart',
        'shared/widgets/avatar_image_processor.dart',
      ];
      final offenders = <String>[];
      for (final file in dartFiles()) {
        final source = file.readAsStringSync();
        if (!source.contains('ImagePicker(')) continue;
        final relative = file.path.replaceAll(r'\', '/');
        if (allowed.any(relative.endsWith)) continue;
        offenders.add(relative);
      }
      expect(
        offenders,
        isEmpty,
        reason:
            'pick through MediaUploadOrchestrator; avatar single-shot is the '
            'only documented variant. Offenders: $offenders',
      );
    });

    test('crop entry carries no dead feature flags', () {
      final source = File(
        'lib/shared/widgets/avatar_editor_widget.dart',
      ).readAsStringSync();
      expect(
        source.contains('showAdvancedCropper'),
        isFalse,
        reason:
            'dead flag purged — crop shape is expressed via '
            'aspectRatio/circularCrop/cropTitle',
      );
    });

    test('no parallel avatar upload/cache authority in the processor', () {
      final source = File(
        'lib/shared/widgets/avatar_image_processor.dart',
      ).readAsStringSync();
      for (final banned in ['uploadAvatar(', 'clearImageCache(', '?t=']) {
        expect(
          source.contains(banned),
          isFalse,
          reason:
              'AvatarUploadService owns fixed-key upload; AppImage cache owns '
              'caching. Found: $banned',
        );
      }
    });

    test('no redefined media limit constants outside MediaUploadConfig', () {
      final offenders = <String>[];
      for (final file in dartFiles()) {
        final relative = file.path.replaceAll(r'\', '/');
        if (relative.endsWith('core/media/media_upload_config.dart')) continue;
        final source = file.readAsStringSync();
        for (final line in source.split('\n')) {
          final trimmed = line.trim();
          if (trimmed.startsWith('//')) continue;
          if (RegExp(
            r'(static const|final) int max(Images|Videos|Total|ImageSizeMb|VideoSizeMb)\s*=',
          ).hasMatch(trimmed)) {
            offenders.add('$relative: $trimmed');
          }
        }
      }
      expect(
        offenders,
        isEmpty,
        reason:
            'limits live ONLY in MediaUploadConfig; callers read the preset. '
            'Offenders: $offenders',
      );
    });

    test('no flat upload folders — every presign names a domain namespace', () {
      final source = File(
        'lib/core/services/s3_service.dart',
      ).readAsStringSync();
      for (final banned in [
        "_requestMediaPresignURL(contentType, 'images')",
        '_requestMediaPresignURL(contentType, "images")',
        "_requestMediaPresignURL(contentType, 'videos')",
        '_requestMediaPresignURL(contentType, "videos")',
      ]) {
        expect(
          source.contains(banned),
          isFalse,
          reason:
              'flat images//videos roots are rejected by the backend; every '
              'upload names images/content|commerce|chat|evidence or '
              'videos/content|commerce|chat|evidence. Found: $banned',
        );
      }
    });

    test('no local video-extension sniffing outside the engine', () {
      final offenders = <String>[];
      for (final file in dartFiles()) {
        final relative = file.path.replaceAll(r'\', '/');
        if (relative.endsWith('core/media/media_upload_orchestrator.dart')) {
          continue;
        }
        final source = file.readAsStringSync();
        for (final line in source.split('\n')) {
          final trimmed = line.trim();
          if (trimmed.startsWith('//')) continue;
          if (trimmed.contains("'.mp4'") ||
              trimmed.contains('".mp4"') ||
              trimmed.contains("'/videos/'") ||
              trimmed.contains('"/videos/"')) {
            offenders.add('$relative: $trimmed');
          }
        }
      }
      expect(
        offenders,
        isEmpty,
        reason:
            'video detection lives ONLY in MediaUploadOrchestrator.isVideoUrl/'
            'isVideoFile. Offenders: $offenders',
      );
    });
  });
}
