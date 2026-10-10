import 'package:flutter/material.dart';
import 'package:hishumi/shared/widgets/app_image.dart';

/// CANONICAL personal user avatar — the single authority for rendering a
/// user's avatar anywhere in the app.
///
/// Business truth (Owner decision 2026-09-24):
/// - A user avatar is ALWAYS the user's photo, or the `Icons.person` user
///   icon when there is no photo. There is no initials fallback anywhere.
/// - Rendering goes through [AppImage] (CloudFront URL as-is, cached).
///
/// This widget resolves no data: callers own avatar URL resolution
/// ([HybridAvatar] is the canonical resolver wrapper).
class ProfileAvatar extends StatelessWidget {
  final String userId;
  final double size;
  final String? imageUrl;
  final VoidCallback? onTap;
  final bool showShadow;
  final bool showEditIcon;
  final VoidCallback? onEditTap;

  /// DECODE / CACHE IDENTITY — deliberately separate from [size].
  ///
  /// When null (the normal case) it is derived from [size], which is only
  /// correct for surfaces whose display size is static. Callers that animate
  /// [size] frame by frame (Profile header collapse: 96 → 40) MUST pass a
  /// fixed value here instead: a changing decode target produces a new
  /// `ResizeImage` key every frame, the `Image` state is re-instituted, and
  /// the placeholder flashes while the photo is actually already cached.
  /// The visual size stays free to animate; only the image identity is pinned.
  final int? cacheWidth;

  const ProfileAvatar({
    super.key,
    required this.userId,
    required this.size,
    this.imageUrl,
    this.onTap,
    this.showShadow = true,
    this.showEditIcon = false,
    this.onEditTap,
    this.cacheWidth,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return GestureDetector(
      onTap: onTap,
      child: SizedBox(
        width: size,
        height: size,
        child: Stack(
          alignment: Alignment.center,
          children: [
            // Avatar container
            Container(
              width: size,
              height: size,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                boxShadow: showShadow
                    ? [
                        BoxShadow(
                          color: scheme.shadow.withValues(alpha: 0.15),
                          blurRadius: 6,
                          offset: const Offset(0, 2),
                        ),
                      ]
                    : null,
              ),
              child: ClipOval(child: _buildAvatarContent(context)),
            ),

            // Edit icon only (no verification badge)
            if (showEditIcon)
              Positioned(bottom: 2, right: 2, child: _buildEditIcon(context)),
          ],
        ),
      ),
    );
  }

  Widget _buildAvatarContent(BuildContext context) {
    if (imageUrl != null && imageUrl!.trim().isNotEmpty) {
      return AppImage(
        imageUrl: imageUrl,
        fit: BoxFit.cover,
        isCircle: true,
        width: size,
        height: size,
        cacheWidth: cacheWidth ?? (size * 2).round(),
        backgroundColor: Theme.of(
          context,
        ).colorScheme.surfaceContainerHighest,
        errorWidget: _buildUserIcon(context),
      );
    }

    return _buildUserIcon(context);
  }

  Widget _buildUserIcon(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      color: scheme.surfaceContainerHighest,
      alignment: Alignment.center,
      child: Icon(
        Icons.person,
        size: size * 0.5,
        color: scheme.onSurfaceVariant,
      ),
    );
  }

  Widget _buildEditIcon(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final iconSize = (size * 0.3).clamp(16.0, 28.0);

    // Inverted pair so the badge contrasts with any avatar photo in both
    // modes: M3's designated `inverseSurface`/`onInverseSurface` roles (the
    // ink role used to stand in as a fill here). No brightness branch.
    return GestureDetector(
      onTap: onEditTap,
      child: Container(
        width: iconSize,
        height: iconSize,
        decoration: BoxDecoration(
          color: scheme.inverseSurface,
          shape: BoxShape.circle,
          border: Border.all(color: scheme.surface, width: 1.5),
        ),
        child: Icon(
          Icons.camera_alt,
          color: scheme.onInverseSurface,
          size: iconSize * 0.6,
          semanticLabel: 'Ubah foto profil',
        ),
      ),
    );
  }
}
