import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:hishumi/shared/widgets/app_image.dart';

/// Canonical store-photo preview — the single authority for rendering a
/// seller store photo in onboarding (step 2 + preview step).
///
/// State machine: empty → selected(local) → uploading → uploaded.
/// Priority is server truth: once [displayUrl] is present the uploaded result
/// renders, never the stale local file. While [isUploading] the local file
/// renders with a progress veil. Local bytes use the correct local provider
/// per platform (`Image.file` / blob `Image.network` on web); backend URLs
/// always render through [AppImage] as-is.
class StorePhotoPreview extends StatelessWidget {
  final String? localPath;
  final String? displayUrl;
  final bool isUploading;
  final double size;

  const StorePhotoPreview({
    super.key,
    this.localPath,
    this.displayUrl,
    this.isUploading = false,
    this.size = 120,
  });

  bool get hasLocalSelection =>
      localPath != null && localPath!.isNotEmpty;
  bool get hasUploadedResult =>
      displayUrl != null && displayUrl!.isNotEmpty;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        color: scheme.surfaceContainer,
        border: Border.all(color: scheme.outlineVariant, width: 2),
      ),
      child: ClipOval(child: _buildContent(context)),
    );
  }

  Widget _buildContent(BuildContext context) {
    if (hasUploadedResult && !isUploading) {
      return AppImage.avatar(imageUrl: displayUrl, size: size);
    }
    if (hasLocalSelection) {
      return Stack(
        fit: StackFit.expand,
        children: [
          kIsWeb
              ? Image.network(
                  localPath!,
                  fit: BoxFit.cover,
                  errorBuilder: (context, error, stackTrace) =>
                      _fallback(context),
                )
              : Image.file(
                  File(localPath!),
                  fit: BoxFit.cover,
                  errorBuilder: (context, error, stackTrace) =>
                      _fallback(context),
                ),
          if (isUploading)
            Container(
              color: Theme.of(
                context,
              ).colorScheme.scrim.withValues(alpha: 0.45),
              child: const Center(
                child: SizedBox(
                  width: 28,
                  height: 28,
                  child: CircularProgressIndicator(strokeWidth: 3),
                ),
              ),
            ),
        ],
      );
    }
    return _fallback(context);
  }

  Widget _fallback(BuildContext context) {
    return Icon(
      Icons.store_outlined,
      size: size * 0.4,
      color: Theme.of(context).colorScheme.onSurfaceVariant,
    );
  }
}
