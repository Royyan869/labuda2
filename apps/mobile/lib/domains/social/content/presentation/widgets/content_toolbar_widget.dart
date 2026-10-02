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
    // Height is CONTENT-DRIVEN on purpose. A hand-summed literal height
    // (this used to be `height: 60`) has no slack budget in the foundation
    // ladder, so any step move in AppType overflows the icon+label column.
    // Let the row size itself from the roles instead.
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p8),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceEvenly,
        children: [
          _ToolbarIcon(
            icon: Icons.photo_library,
            color: context.statusColors.success,
            label: 'Gallery',
            onTap: onGalleryTap,
          ),
          _ToolbarIcon(
            icon: Icons.camera_alt,
            color: Theme.of(context).colorScheme.secondary,
            label: 'Camera',
            onTap: onCameraTap,
          ),
          _ToolbarIcon(
            icon: Icons.person_add,
            color: Theme.of(context).colorScheme.primary,
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
    final scheme = Theme.of(context).colorScheme;

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r8),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p4, vertical: AppMetrics.p4),
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(icon, size: AppIconSize.action, color: color),
                const SizedBox(height: 2),
                Text(
                  label,
                  style: TextStyle(
                    fontSize: AppType.s12,
                    color: scheme.onSurfaceVariant,
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
                  padding: const EdgeInsets.all(AppMetrics.p4),
                  decoration: BoxDecoration(
                    color: scheme.primary,
                    shape: BoxShape.circle,
                  ),
                  constraints: const BoxConstraints(
                    minWidth: 14,
                    minHeight: 14,
                  ),
                  child: Text(
                    badge!,
                    style: TextStyle(
                      color: scheme.onPrimary,
                      fontSize: AppType.s12,
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
