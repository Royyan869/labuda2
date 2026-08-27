import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Content Toolbar Widget - Action toolbar for post creation
///
/// Features:
/// - Quick media actions (Gallery, Camera)
/// - Tag people
/// - Add location (Content only - Request uses auto location)
/// Note: Settings moved to header (visibility dropdown & allow comments switch)
class ContentToolbarWidget extends StatelessWidget {
  final VoidCallback onGalleryTap;
  final VoidCallback onCameraTap;
  final VoidCallback onTagPeopleTap;
  final VoidCallback onLocationTap;
  final int taggedPeopleCount;
  final bool hasLocation;

  const ContentToolbarWidget({
    super.key,
    required this.onGalleryTap,
    required this.onCameraTap,
    required this.onTagPeopleTap,
    required this.onLocationTap,
    this.taggedPeopleCount = 0,
    this.hasLocation = false,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 60,
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 8),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceEvenly,
        children: [
          _ToolbarIcon(
            icon: Icons.photo_library,
            color: AppColors.primaryGreen,
            label: 'Gallery',
            onTap: onGalleryTap,
          ),
          _ToolbarIcon(
            icon: Icons.camera_alt,
            color: AppColors.primaryBlue,
            label: 'Camera',
            onTap: onCameraTap,
          ),
          _ToolbarIcon(
            icon: Icons.person_add,
            color: AppColors.primaryRed,
            label: 'Tag',
            badge: taggedPeopleCount > 0 ? taggedPeopleCount.toString() : null,
            onTap: onTagPeopleTap,
          ),
          _ToolbarIcon(
            icon: Icons.location_on,
            color: AppColors.koiOrange,
            label: 'Location',
            badge: hasLocation ? '✓' : null,
            onTap: onLocationTap,
          ),
        ],
      ),
    );
  }
}

/// Individual toolbar icon widget
class _ToolbarIcon extends StatelessWidget {
  final IconData icon;
  final Color color;
  final String label;
  final VoidCallback onTap;
  final String? badge;

  const _ToolbarIcon({
    required this.icon,
    required this.color,
    required this.label,
    required this.onTap,
    this.badge,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(8),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 4),
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(icon, size: 20, color: color),
                const SizedBox(height: 2),
                Text(
                  label,
                  style: TextStyle(
                    fontSize: 10,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ],
            ),
            if (badge != null)
              Positioned(
                right: -4,
                top: -2,
                child: Container(
                  padding: const EdgeInsets.all(3),
                  decoration: BoxDecoration(
                    color: AppColors.primaryRed,
                    shape: BoxShape.circle,
                  ),
                  constraints: const BoxConstraints(
                    minWidth: 14,
                    minHeight: 14,
                  ),
                  child: Text(
                    badge!,
                    style: const TextStyle(
                      color: AppColors.neutralWhite,
                      fontSize: 8,
                      fontWeight: FontWeight.bold,
                    ),
                    textAlign: TextAlign.center,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
