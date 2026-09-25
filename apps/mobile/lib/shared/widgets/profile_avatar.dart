import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

/// CANONICAL personal user avatar — the single authority for rendering a
/// user's avatar anywhere in the app.
///
/// Business truth (Owner decision 2026-09-24):
/// - A user avatar is ALWAYS the user's photo, or the `Icons.person` user
///   icon when there is no photo. There is no initials fallback anywhere.
/// - Rendering goes through [StableNetworkImage] (gapless playback) so a
///   rotating signed URL never flashes a placeholder over a visible frame.
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

  const ProfileAvatar({
    super.key,
    required this.userId,
    required this.size,
    this.imageUrl,
    this.onTap,
    this.showShadow = true,
    this.showEditIcon = false,
    this.onEditTap,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

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
                          color: AppColors.dark.withValues(
                            alpha: isDark ? 0.3 : 0.1,
                          ),
                          blurRadius: 6,
                          offset: const Offset(0, 2),
                        ),
                      ]
                    : null,
              ),
              child: ClipOval(child: _buildAvatarContent(context, isDark)),
            ),

            // Edit icon only (no verification badge)
            if (showEditIcon)
              Positioned(bottom: 2, right: 2, child: _buildEditIcon(context)),
          ],
        ),
      ),
    );
  }

  Widget _buildAvatarContent(BuildContext context, bool isDark) {
    if (imageUrl != null && imageUrl!.trim().isNotEmpty) {
      // StableNetworkImage keeps the last successful frame visible while a
      // new signed URL loads (gapless), and falls back to the user icon on
      // error. One authority for user avatar rendering, no flicker.
      return StableNetworkImage(
        imageUrl: imageUrl,
        fit: BoxFit.cover,
        fallback: _buildUserIcon(isDark),
      );
    }

    return _buildUserIcon(isDark);
  }

  Widget _buildUserIcon(bool isDark) {
    return Container(
      color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
      alignment: Alignment.center,
      child: Icon(
        Icons.person,
        size: size * 0.5,
        color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
      ),
    );
  }

  Widget _buildEditIcon(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final iconSize = (size * 0.3).clamp(16.0, 28.0);

    return GestureDetector(
      onTap: onEditTap,
      child: Container(
        width: iconSize,
        height: iconSize,
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray700,
          shape: BoxShape.circle,
          border: Border.all(color: AppColors.neutralWhite, width: 1.5),
        ),
        child: Icon(
          Icons.camera_alt,
          color: AppColors.neutralWhite,
          size: iconSize * 0.6,
        ),
      ),
    );
  }
}
