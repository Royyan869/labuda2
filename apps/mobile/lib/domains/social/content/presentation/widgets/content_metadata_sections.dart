import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Content Metadata Sections - Display location, hashtags
///
/// Reusable section widgets for displaying post metadata
class ContentMetadataSections {
  /// Build location section
  static Widget buildLocationSection({
    required String? location,
    required VoidCallback onEdit,
    required VoidCallback onRemove,
    required bool isDark,
  }) {
    if (location == null || location.isEmpty) return const SizedBox.shrink();

    return _buildSection(
      icon: Icons.location_on,
      iconColor: AppColors.koiOrange,
      title: 'Location',
      isDark: isDark,
      onEdit: onEdit,
      onRemove: onRemove,
      content: Text(
        location,
        style: TextStyle(
          fontSize: 14,
          color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray700,
        ),
      ),
    );
  }

  /// Build hashtags section
  static Widget buildHashtagsSection({
    required List<String> hashtags,
    required VoidCallback onEdit,
    required Function(String) onRemove,
    required bool isDark,
  }) {
    if (hashtags.isEmpty) return const SizedBox.shrink();

    return _buildSection(
      icon: Icons.tag,
      iconColor: AppColors.primaryBlue,
      title: 'Hashtags',
      isDark: isDark,
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
                    fontSize: 12,
                    color: AppColors.primaryBlue,
                    fontWeight: FontWeight.w500,
                  ),
                ),
                deleteIcon: Icon(
                  Icons.close,
                  size: 16,
                  color: AppColors.primaryBlue.withValues(alpha: 0.7),
                ),
                onDeleted: () => onRemove(tag),
                backgroundColor: AppColors.primaryBlue.withValues(alpha: 0.1),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(16),
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
    required IconData icon,
    required Color iconColor,
    required String title,
    required bool isDark,
    required VoidCallback onEdit,
    VoidCallback? onRemove,
    required Widget content,
  }) {
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isDark
            ? AppColors.darkGray800.withValues(alpha: 0.5)
            : AppColors.neutralGray50,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Row(
                children: [
                  Icon(icon, size: 18, color: iconColor),
                  const SizedBox(width: 8),
                  Text(
                    title,
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: isDark
                          ? AppColors.neutralGray300
                          : AppColors.neutralGray700,
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
                      size: 16,
                      color: isDark
                          ? AppColors.neutralGray500
                          : AppColors.neutralGray600,
                    ),
                  ),
                  if (onRemove != null) ...[
                    const SizedBox(width: 12),
                    GestureDetector(
                      onTap: onRemove,
                      child: Icon(
                        Icons.close,
                        size: 18,
                        color: isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray600,
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
