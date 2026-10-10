import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/media/media_upload_config.dart';
import 'package:hishumi/core/media/media_upload_orchestrator.dart';

/// NEGATIVE CONTRACT — one media pick engine.
///
/// `MediaUploadOrchestrator` is the single pick→validate authority, and
/// `showAttachSheet` is the single attachment-sheet DESIGN (Galeri/Kamera +
/// capability actions derived from the config). Identity surfaces (avatar,
/// cover, store photo) pick through that same sheet with
/// `MediaUploadConfig.forIdentity` and only add a CROP step; the old
/// `AvatarPickerOptions` modal and its own OS camera are purged, not variants.
/// Every other surface (content, commerce, comment, chat, dispute, refund,
/// external product) picks through the orchestrator too.
void main() {
  group('single pick engine', () {
    final libRoot = Directory('lib');

    List<File> dartFiles() => libRoot
        .listSync(recursive: true)
        .whereType<File>()
        .where((f) => f.path.endsWith('.dart'))
        .toList();

    test('no direct ImagePicker use outside the engine', () {
      const allowed = ['core/media/media_upload_orchestrator.dart'];
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
            'pick through MediaUploadOrchestrator. Offenders: $offenders',
      );
    });

    test('identity uses the canonical sheet and only adds a crop step', () {
      final editor = File(
        'lib/shared/widgets/avatar_editor_widget.dart',
      ).readAsStringSync();
      expect(
        editor.contains('showAttachSheet'),
        isTrue,
        reason:
            'avatar/cover/store must open the SAME sheet as comment and chat',
      );
      expect(
        editor.contains('MediaUploadConfig.forIdentity'),
        isTrue,
        reason:
            'identity picks one photo, never video — the policy says so, not '
            'a second modal',
      );
      expect(
        editor.contains('cropImage'),
        isTrue,
        reason: 'the crop step is the only thing identity adds',
      );
    });

    test('the second picker modal stays dead', () {
      expect(
        File('lib/shared/widgets/avatar_picker_options.dart').existsSync(),
        isFalse,
        reason:
            'the avatar picker modal is purged: ONE modal design for every '
            'surface',
      );
      final offenders = <String>[];
      for (final file in dartFiles()) {
        if (file.readAsStringSync().contains('AvatarPickerOptions')) {
          offenders.add(file.path.replaceAll(r'\', '/'));
        }
      }
      expect(
        offenders,
        isEmpty,
        reason: 'AvatarPickerOptions is dead. Offenders: $offenders',
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

    test('shimmer is banned on every media surface', () {
      final offenders = <String>[];
      for (final file in dartFiles()) {
        final relative = file.path.replaceAll(r'\', '/');
        if (!relative.endsWith('.dart')) continue;
        final source = file.readAsStringSync();
        for (final line in source.split('\n')) {
          final trimmed = line.trim();
          if (trimmed.startsWith('//')) continue;
          if (trimmed.contains('Shimmer.fromColors') ||
              trimmed.contains('package:shimmer/shimmer.dart')) {
            offenders.add('$relative: $trimmed');
          }
        }
      }
      expect(
        offenders,
        isEmpty,
        reason:
            'shimmer sweeps were reported as visually disturbing on media '
            'tiles; loading states are static mats. Offenders: $offenders',
      );
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

  /// POSITIVE PROOF that the policy is read, not decorative: the caps are the
  /// product rules (bukti pesanan = 1 video + 5 foto), and the ONE sheet is a
  /// function of the config instead of a second modal design.
  group('media policy is enforced, not decorative', () {
    test('order evidence is exactly 1 video + 5 foto', () {
      const full = MediaCounts(images: 5, videos: 1);
      final room = MediaUploadConfig.forEvidence.remaining(full);
      expect(room.images, 0);
      expect(room.videos, 0);
      expect(MediaUploadConfig.forEvidence.remainingTotalFor(full), 0);

      final videoAlreadyTaken = MediaUploadConfig.forEvidence.remaining(
        const MediaCounts(images: 0, videos: 1),
      );
      expect(videoAlreadyTaken.images, 5);
      expect(videoAlreadyTaken.videos, 0, reason: 'a second video is never allowed');
      expect(MediaUploadConfig.forEvidence.maxTotal, 6);
    });

    test('identity takes one photo and never a video', () {
      const identity = MediaUploadConfig.forIdentity;
      expect(identity.videoAllowed, isFalse);
      final room = identity.remaining(const MediaCounts());
      expect(room.images, 1);
      expect(room.videos, 0);
    });

    test('commerce caps each type AND the shared total', () {
      const commerce = MediaUploadConfig.forCommerce;
      expect(commerce.remaining(const MediaCounts(images: 10)).images, 0);
      expect(commerce.remaining(const MediaCounts(videos: 10)).videos, 0);
      expect(
        commerce.remainingTotalFor(const MediaCounts(images: 10)),
        0,
        reason: 'the shared total still closes the picker at maxTotal',
      );
    });

    testWidgets('the one attachment sheet derives its entries from the policy', (
      tester,
    ) async {
      late BuildContext entryContext;
      var config = MediaUploadConfig.forIdentity;

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) {
                entryContext = context;
                return TextButton(
                  onPressed: () => MediaUploadOrchestrator.showAttachSheet(
                    context: context,
                    config: config,
                    onPicked: (_) async {},
                  ),
                  child: const Text('open'),
                );
              },
            ),
          ),
        ),
      );

      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.text('Galeri'), findsOneWidget);
      expect(find.text('Kamera'), findsOneWidget);
      expect(
        find.text('Foto'),
        findsOneWidget,
        reason: 'identity takes photos only — the copy follows the policy',
      );
      expect(find.text('Foto & video'), findsNothing);

      Navigator.of(entryContext).pop();
      await tester.pumpAndSettle();

      config = MediaUploadConfig.forChat;
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(
        find.text('Foto & video'),
        findsOneWidget,
        reason: 'the same sheet widens when the policy allows video',
      );
    });
  });
}
