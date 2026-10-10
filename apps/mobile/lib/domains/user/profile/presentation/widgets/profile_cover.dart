import 'package:flutter/material.dart';
import 'package:hishumi/shared/widgets/app_image.dart';

/// Cover photo section untuk Profile V2
///
/// Features:
/// - Display cover photo atau gradient fallback
/// - Gradient fade to background di bottom (transparent effect)
/// - Edit button telah dipindahkan ke AppBar
class ProfileCover extends StatelessWidget {
  final String? coverPhotoUrl;
  final bool isOwnProfile;
  final double height;
  final double collapseProgress; // 0.0 (expanded) to 1.0 (collapsed)

  const ProfileCover({
    super.key,
    this.coverPhotoUrl,
    this.isOwnProfile = false,
    this.height = 180,
    this.collapseProgress = 0.0, // Default: fully expanded
  });

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: height,
      width: double.infinity,
      child: Stack(
        fit: StackFit.expand,
        children: [
          // Cover photo atau gradient fallback
          _buildCoverImage(context),

          // Gradient overlay di bottom untuk readability
          _buildGradientOverlay(context),

          // Collapse overlay - fades in saat scroll collapse untuk readability
          _buildCollapseOverlay(context),
        ],
      ),
    );
  }

  Widget _buildCoverImage(BuildContext context) {
    if (coverPhotoUrl != null && coverPhotoUrl!.isNotEmpty) {
      return AppImage(
        imageUrl: coverPhotoUrl,
        fit: BoxFit.cover,
        errorWidget: _buildGradientFallback(context),
        placeholder: _buildGradientFallback(context),
      );
    }
    return _buildGradientFallback(context);
  }

  Widget _buildGradientFallback(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            scheme.primary.withValues(alpha: 0.8),
            scheme.primary.withValues(alpha: 0.6),
            scheme.primary.withValues(alpha: 0.4),
          ],
        ),
      ),
    );
  }

  Widget _buildGradientOverlay(BuildContext context) {
    // Background color based on theme (matches ProfileScreen container)
    final backgroundColor = Theme.of(context).colorScheme.surface;

    return Positioned(
      left: 0,
      right: 0,
      bottom: 0,
      height: height * 0.6, // Extended gradient area
      child: Container(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [
              Colors.transparent,
              backgroundColor.withValues(alpha: 0.3),
              backgroundColor.withValues(alpha: 0.7),
              backgroundColor,
            ],
            stops: const [0.0, 0.3, 0.7, 1.0],
          ),
        ),
      ),
    );
  }

  /// Overlay yang fade in saat AppBar collapse untuk readability
  Widget _buildCollapseOverlay(BuildContext context) {
    // Background color matches AppBar backgroundColor
    final backgroundColor = Theme.of(context).colorScheme.surface;

    return Opacity(
      opacity: collapseProgress,
      child: Container(color: backgroundColor),
    );
  }
}
