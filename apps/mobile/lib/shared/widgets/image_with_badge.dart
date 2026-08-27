import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Badge position on image
enum BadgePosition { topLeft, topRight, bottomLeft, bottomRight }

/// Video badge overlay configuration
class VideoBadgeConfig {
  final bool show;
  final BadgePosition position;
  final String? label;

  const VideoBadgeConfig({
    this.show = false,
    this.position = BadgePosition.bottomRight,
    this.label = 'Video',
  });

  const VideoBadgeConfig.show({
    this.position = BadgePosition.bottomRight,
    this.label = 'Video',
  }) : show = true;
}

/// Image count badge configuration
class ImageCountBadgeConfig {
  final int count;
  final BadgePosition position;

  const ImageCountBadgeConfig({
    required this.count,
    this.position = BadgePosition.topRight,
  });

  bool get show => count > 1;
}

/// Status badge configuration for overlays (SOLD, LIVE, etc.)
class StatusOverlayConfig {
  final String label;
  final Color backgroundColor;
  final Color textColor;
  final IconData? icon;
  final BadgePosition position;

  const StatusOverlayConfig({
    required this.label,
    required this.backgroundColor,
    this.textColor = Colors.white,
    this.icon,
    this.position = BadgePosition.topLeft,
  });

  /// Factory for SOLD status
  factory StatusOverlayConfig.sold() => StatusOverlayConfig(
    label: 'TERJUAL',
    backgroundColor: AppColors.statusError,
    icon: Icons.sell,
  );

  /// Factory for RESERVED status
  factory StatusOverlayConfig.reserved() => StatusOverlayConfig(
    label: 'RESERVED',
    backgroundColor: AppColors.statusWarning,
    textColor: AppColors.neutralGray900,
    icon: Icons.bookmark,
  );

  /// Factory for FOR SALE status
  factory StatusOverlayConfig.forSale() => StatusOverlayConfig(
    label: 'DIJUAL',
    backgroundColor: AppColors.statusSuccess,
    icon: Icons.local_offer,
  );

  /// Factory for LIVE status
  factory StatusOverlayConfig.live() => StatusOverlayConfig(
    label: 'LIVE',
    backgroundColor: AppColors.primaryRed,
    icon: Icons.fiber_manual_record,
  );

  /// Factory for OUT OF STOCK status
  factory StatusOverlayConfig.outOfStock() => StatusOverlayConfig(
    label: 'HABIS',
    backgroundColor: AppColors.neutralGray600,
    icon: Icons.do_not_disturb,
  );

  /// Factory for FEATURED status
  factory StatusOverlayConfig.featured() => StatusOverlayConfig(
    label: 'FEATURED',
    backgroundColor: AppColors.primaryRed,
    position: BadgePosition.topRight,
    icon: Icons.star,
  );

  /// Factory for PROMOTED status
  factory StatusOverlayConfig.promoted() => StatusOverlayConfig(
    label: 'Dipromosikan',
    backgroundColor: AppColors.coinPrimary,
    position: BadgePosition.topRight,
    icon: Icons.star,
  );
}

/// Reusable Image with Badge overlay widget
///
/// Consolidates common patterns for:
/// - Video badge overlays
/// - Status overlays (SOLD, LIVE, etc.)
/// - Image count badges
/// - Featured/Promoted badges
///
/// Usage:
/// ```dart
/// ImageWithBadge(
///   imageUrl: 'https://example.com/image.jpg',
///   aspectRatio: 1.0,
///   videoBadge: VideoBadgeConfig.show(),
///   statusOverlay: StatusOverlayConfig.sold(),
///   imageCount: ImageCountBadgeConfig(count: 5),
/// )
/// ```
class ImageWithBadge extends StatelessWidget {
  /// Image URL
  final String? imageUrl;

  /// Aspect ratio for the image container
  final double aspectRatio;

  /// Border radius for the image
  final double borderRadius;

  /// Video badge configuration
  final VideoBadgeConfig? videoBadge;

  /// Status overlay configuration (SOLD, LIVE, etc.)
  final StatusOverlayConfig? statusOverlay;

  /// Image count badge configuration
  final ImageCountBadgeConfig? imageCount;

  /// Custom badges to add (positioned manually)
  final List<Widget>? customBadges;

  /// Placeholder widget when image is loading
  final Widget? placeholder;

  /// Error widget when image fails to load
  final Widget? errorWidget;

  /// Fit mode for the image
  final BoxFit fit;

  /// Callback when image is tapped
  final VoidCallback? onTap;

  /// Whether to show a dark overlay (for sold items, etc.)
  final bool showDarkOverlay;

  /// Dark overlay opacity (0.0 - 1.0)
  final double darkOverlayOpacity;

  const ImageWithBadge({
    super.key,
    this.imageUrl,
    this.aspectRatio = 1.0,
    this.borderRadius = 8.0,
    this.videoBadge,
    this.statusOverlay,
    this.imageCount,
    this.customBadges,
    this.placeholder,
    this.errorWidget,
    this.fit = BoxFit.cover,
    this.onTap,
    this.showDarkOverlay = false,
    this.darkOverlayOpacity = 0.4,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    Widget content = AspectRatio(
      aspectRatio: aspectRatio,
      child: ClipRRect(
        borderRadius: BorderRadius.circular(borderRadius),
        child: Stack(
          fit: StackFit.expand,
          children: [
            // Base image
            _buildImage(isDark),

            // Dark overlay (optional)
            if (showDarkOverlay)
              Container(
                color: Colors.black.withValues(alpha: darkOverlayOpacity),
              ),

            // Video badge
            if (videoBadge?.show == true) _buildVideoBadge(videoBadge!),

            // Status overlay
            if (statusOverlay != null) _buildStatusOverlay(statusOverlay!),

            // Image count badge
            if (imageCount?.show == true) _buildImageCountBadge(imageCount!),

            // Custom badges
            if (customBadges != null) ...customBadges!,
          ],
        ),
      ),
    );

    if (onTap != null) {
      content = GestureDetector(onTap: onTap, child: content);
    }

    return content;
  }

  Widget _buildImage(bool isDark) {
    if (imageUrl == null || imageUrl!.isEmpty) {
      return Container(
        color: isDark ? AppColors.neutralGray800 : AppColors.neutralGray200,
        child: Center(
          child: Icon(
            Icons.image_outlined,
            size: 32,
            color: isDark ? AppColors.neutralGray600 : AppColors.neutralGray400,
          ),
        ),
      );
    }

    return AppImage(
      imageUrl: imageUrl!,
      fit: fit,
      placeholder: placeholder,
      errorWidget: errorWidget,
    );
  }

  Widget _buildVideoBadge(VideoBadgeConfig config) {
    return Positioned(
      top:
          config.position == BadgePosition.topLeft ||
              config.position == BadgePosition.topRight
          ? 8
          : null,
      bottom:
          config.position == BadgePosition.bottomLeft ||
              config.position == BadgePosition.bottomRight
          ? 8
          : null,
      left:
          config.position == BadgePosition.topLeft ||
              config.position == BadgePosition.bottomLeft
          ? 8
          : null,
      right:
          config.position == BadgePosition.topRight ||
              config.position == BadgePosition.bottomRight
          ? 8
          : null,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        decoration: BoxDecoration(
          color: Colors.black.withValues(alpha: 0.7),
          borderRadius: BorderRadius.circular(4),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.play_circle_filled, color: Colors.white, size: 16),
            if (config.label != null) ...[
              const SizedBox(width: 4),
              Text(
                config.label!,
                style: AppTypography.labelSmall.copyWith(color: Colors.white),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildStatusOverlay(StatusOverlayConfig config) {
    return Positioned(
      top:
          config.position == BadgePosition.topLeft ||
              config.position == BadgePosition.topRight
          ? 8
          : null,
      bottom:
          config.position == BadgePosition.bottomLeft ||
              config.position == BadgePosition.bottomRight
          ? 8
          : null,
      left:
          config.position == BadgePosition.topLeft ||
              config.position == BadgePosition.bottomLeft
          ? 8
          : null,
      right:
          config.position == BadgePosition.topRight ||
              config.position == BadgePosition.bottomRight
          ? 8
          : null,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 3),
        decoration: BoxDecoration(
          color: config.backgroundColor,
          borderRadius: BorderRadius.circular(4),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (config.icon != null) ...[
              Icon(config.icon, size: 12, color: config.textColor),
              const SizedBox(width: 2),
            ],
            Text(
              config.label,
              style: AppTypography.labelSmall.copyWith(
                color: config.textColor,
                fontWeight: FontWeight.bold,
                fontSize: 10,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildImageCountBadge(ImageCountBadgeConfig config) {
    return Positioned(
      top:
          config.position == BadgePosition.topLeft ||
              config.position == BadgePosition.topRight
          ? 8
          : null,
      bottom:
          config.position == BadgePosition.bottomLeft ||
              config.position == BadgePosition.bottomRight
          ? 8
          : null,
      left:
          config.position == BadgePosition.topLeft ||
              config.position == BadgePosition.bottomLeft
          ? 8
          : null,
      right:
          config.position == BadgePosition.topRight ||
              config.position == BadgePosition.bottomRight
          ? 8
          : null,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        decoration: BoxDecoration(
          color: Colors.black.withValues(alpha: 0.7),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.photo_library, size: 14, color: Colors.white),
            const SizedBox(width: 4),
            Text(
              '${config.count}',
              style: AppTypography.labelSmall.copyWith(
                color: Colors.white,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Extension for easily creating positioned badges
extension PositionedBadgeExtension on Widget {
  Widget positionedAt(BadgePosition position, {double offset = 8}) {
    return Positioned(
      top:
          position == BadgePosition.topLeft ||
              position == BadgePosition.topRight
          ? offset
          : null,
      bottom:
          position == BadgePosition.bottomLeft ||
              position == BadgePosition.bottomRight
          ? offset
          : null,
      left:
          position == BadgePosition.topLeft ||
              position == BadgePosition.bottomLeft
          ? offset
          : null,
      right:
          position == BadgePosition.topRight ||
              position == BadgePosition.bottomRight
          ? offset
          : null,
      child: this,
    );
  }
}
