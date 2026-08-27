import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return SizedBox(
      height: height,
      width: double.infinity,
      child: Stack(
        fit: StackFit.expand,
        children: [
          // Cover photo atau gradient fallback
          _buildCoverImage(isDark),

          // Gradient overlay di bottom untuk readability
          _buildGradientOverlay(isDark),

          // Collapse overlay - fades in saat scroll collapse untuk readability
          _buildCollapseOverlay(isDark),
        ],
      ),
    );
  }

  Widget _buildCoverImage(bool isDark) {
    if (coverPhotoUrl != null && coverPhotoUrl!.isNotEmpty) {
      return Image.network(
        coverPhotoUrl!,
        fit: BoxFit.cover,
        errorBuilder: (context, error, stackTrace) =>
            _buildGradientFallback(isDark),
        loadingBuilder: (context, child, loadingProgress) {
          if (loadingProgress == null) return child;
          return Stack(
            fit: StackFit.expand,
            children: [
              _buildGradientFallback(isDark),
              Center(
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  color: AppColors.neutralWhite.withValues(alpha: 0.7),
                ),
              ),
            ],
          );
        },
      );
    }
    return _buildGradientFallback(isDark);
  }

  Widget _buildGradientFallback(bool isDark) {
    return Container(
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: isDark
              ? [
                  AppColors.darkGray700,
                  AppColors.darkGray800,
                  AppColors.darkGray900,
                ]
              : [
                  AppColors.primaryRed.withValues(alpha: 0.8),
                  AppColors.primaryRed.withValues(alpha: 0.6),
                  AppColors.primaryRed.withValues(alpha: 0.4),
                ],
        ),
      ),
    );
  }

  Widget _buildGradientOverlay(bool isDark) {
    // Background color based on theme (matches ProfileScreen container)
    final backgroundColor = isDark
        ? AppColors.darkGray800
        : AppColors.neutralWhite;

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
  Widget _buildCollapseOverlay(bool isDark) {
    // Background color matches AppBar backgroundColor
    final backgroundColor = isDark
        ? AppColors.darkGray800
        : AppColors.neutralWhite;

    return Opacity(
      opacity: collapseProgress,
      child: Container(color: backgroundColor),
    );
  }
}
