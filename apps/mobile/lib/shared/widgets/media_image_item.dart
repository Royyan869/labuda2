import 'dart:io';
import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

class MediaImageItem extends StatelessWidget {
  final File image;
  final int index;
  final VoidCallback? onRemove;
  final bool showCoverBadge;
  final double height;
  final double width;

  const MediaImageItem({
    super.key,
    required this.image,
    required this.index,
    this.onRemove,
    required this.showCoverBadge,
    required this.height,
    required this.width,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Container(
      width: width,
      height: height,
      margin: const EdgeInsets.only(right: AppMetrics.p8),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppShape.r12),
        color: scheme.surfaceContainerHighest,
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(AppShape.r12),
        child: Stack(
          children: [
            // Image
            SizedBox(
              width: double.infinity,
              height: double.infinity,
              child: Image.file(
                image,
                fit: BoxFit.cover,
                errorBuilder: (context, error, stackTrace) =>
                    _buildErrorImage(scheme),
              ),
            ),

            // Cover badge
            if (showCoverBadge && index == 0) _buildCoverBadge(scheme),

            // Remove button
            if (onRemove != null) _buildRemoveButton(scheme),
          ],
        ),
      ),
    );
  }

  Widget _buildCoverBadge(ColorScheme scheme) {
    return Positioned(
      top: 8,
      left: 8,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4),
        decoration: BoxDecoration(
          color: scheme.primary,
          borderRadius: BorderRadius.circular(AppShape.r4),
        ),
        child: Text(
          'Cover',
          style: TextStyle(
            color: scheme.onPrimary,
            fontSize: AppType.s10,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }

  Widget _buildRemoveButton(ColorScheme scheme) {
    return Positioned(
      top: 4,
      right: 4,
      child: GestureDetector(
        onTap: onRemove,
        child: Container(
          width: 24,
          height: 24,
          decoration: BoxDecoration(
            color: scheme.error.withValues(alpha: 0.9),
            shape: BoxShape.circle,
          ),
          child: Icon(Icons.close, color: scheme.onPrimary, size: 16),
        ),
      ),
    );
  }

  Widget _buildErrorImage(ColorScheme scheme) {
    return Container(
      width: double.infinity,
      height: double.infinity,
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      child: Icon(
        Icons.broken_image_outlined,
        color: scheme.onSurfaceVariant,
        size: 32,
      ),
    );
  }
}
