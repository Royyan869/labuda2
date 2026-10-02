import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Content Metadata Sections - Display location, hashtags
///
/// Reusable section widgets for displaying post metadata
class ContentMetadataSections {
  /// Build location section
  static Widget buildLocationSection({
    required BuildContext context,
    required String? location,
    required VoidCallback onEdit,
    required VoidCallback onRemove,
  }) {
    if (location == null || location.isEmpty) return const SizedBox.shrink();
    final scheme = Theme.of(context).colorScheme;

    return _buildSection(
      context: context,
      icon: Icons.location_on,
      iconColor: AppColors.koiOrange,
      title: 'Location',
      onEdit: onEdit,
      onRemove: onRemove,
      content: Text(
        location,
        style: TextStyle(
          fontSize: AppType.s14,
          color: scheme.onSurface,
        ),
      ),
    );
  }

  /// Build hashtags section
  static Widget buildHashtagsSection({
    required BuildContext context,
    required List<String> hashtags,
    required VoidCallback onEdit,
    required Function(String) onRemove,
  }) {
    if (hashtags.isEmpty) return const SizedBox.shrink();

    return _buildSection(
      context: context,
      icon: Icons.tag,
      iconColor: Theme.of(context).colorScheme.secondary,
      title: 'Hashtags',
      onEdit: onEdit,
      content: Wrap(
        spacing: 8,
        runSpacing: 8,
        children: hashtags
            .map(
              (tag) => Chip(
                label: Text(
                  tag,
                  style: TextStyle(
                    fontSize: AppType.s12,
                    color: Theme.of(context).colorScheme.secondary,
                    fontWeight: FontWeight.w500,
                  ),
                ),
                deleteIcon: Icon(
                  Icons.close,
                  size: AppIconSize.inlineGlyph,
                  color: Theme.of(
                    context,
                  ).colorScheme.secondary.withValues(alpha: 0.7),
                ),
                onDeleted: () => onRemove(tag),
                backgroundColor: Theme.of(
                  context,
                ).colorScheme.secondary.withValues(alpha: 0.1),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(AppShape.r16),
                ),
                side: BorderSide.none,
              ),
            )
            .toList(),
      ),
    );
  }

  // Helper method to build section wrapper
  static Widget _buildSection({
    required BuildContext context,
    required IconData icon,
    required Color iconColor,
    required String title,
    required VoidCallback onEdit,
    VoidCallback? onRemove,
    required Widget content,
  }) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      margin: const EdgeInsets.only(bottom: AppMetrics.p12),
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerLow,
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Row(
                children: [
                  Icon(icon, size: AppIconSize.action, color: iconColor),
                  const SizedBox(width: 8),
                  Text(
                    title,
                    style: TextStyle(
                      fontSize: AppType.s14,
                      fontWeight: FontWeight.w600,
                      color: scheme.onSurface,
                    ),
                  ),
                ],
              ),
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  GestureDetector(
                    onTap: onEdit,
                    child: Icon(
                      Icons.edit,
                      size: AppIconSize.inlineGlyph,
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                  if (onRemove != null) ...[
                    const SizedBox(width: 12),
                    GestureDetector(
                      onTap: onRemove,
                      child: Icon(
                        Icons.close,
                        size: AppIconSize.action,
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ],
              ),
            ],
          ),
          const SizedBox(height: 8),
          content,
        ],
      ),
    );
  }
}
