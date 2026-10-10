import 'package:flutter/material.dart';
import 'package:hishumi/core/media/media_upload_config.dart';
import 'package:hishumi/core/media/media_upload_orchestrator.dart';
import 'avatar_image_processor.dart';

/// ONE crop entry for avatar, cover photo, and store photo.
///
/// Picking runs through the canonical attachment sheet
/// (`MediaUploadOrchestrator.showAttachSheet`) with the identity policy: ONE
/// photo, no video, Galeri + Kamera — the same modal comment/chat use. The only
/// difference here is the post-pick CROP step, expressed honestly via
/// [aspectRatio]/[circularCrop]/[cropTitle] (avatar and store are 1:1 circular,
/// cover is 16:9 rectangular).
///
/// The cropped local path is returned to the caller, which owns the upload
/// (S3 fixed key). Removing a photo is NOT in this sheet: removal is a
/// first-class screen action (`onAvatarUpdated(null)` / the screen's remove
/// button), and putting a destructive action inside a source picker would
/// duplicate that authority and invite mis-taps.
class AvatarEditorWidget {
  static void showEditModal({
    required BuildContext context,
    required Function(String? avatarUrl) onAvatarUpdated,
    double aspectRatio = 1.0,
    bool circularCrop = true,
    String cropTitle = 'Potong Foto',
  }) {
    final outer = context;
    MediaUploadOrchestrator.showAttachSheet(
      context: outer,
      config: MediaUploadConfig.forIdentity,
      onPicked: (files) async {
        if (files.isEmpty) return;
        await AvatarImageProcessor.cropImage(
          outer,
          files.first,
          onAvatarUpdated,
          aspectRatio: aspectRatio,
          circularCrop: circularCrop,
          cropTitle: cropTitle,
        );
      },
    );
  }
}
