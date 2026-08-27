import 'dart:io';
import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      width: width,
      height: height,
      margin: const EdgeInsets.only(right: 8),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        color: isDark ? AppColors.darkGray600 : AppColors.neutralGray100,
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(12),
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
                    _buildErrorImage(isDark),
              ),
            ),

            // Cover badge
            if (showCoverBadge && index == 0) _buildCoverBadge(),

            // Remove button
            if (onRemove != null) _buildRemoveButton(),
          ],
        ),
      ),
    );
  }

  Widget _buildCoverBadge() {
    return Positioned(
      top: 8,
      left: 8,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        decoration: BoxDecoration(
          color: AppColors.primaryRed,
          borderRadius: BorderRadius.circular(4),
        ),
        child: Text(
          'Cover',
          style: TextStyle(
            color: AppColors.neutralWhite,
            fontSize: 10,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }

  Widget _buildRemoveButton() {
    return Positioned(
      top: 4,
      right: 4,
      child: GestureDetector(
        onTap: onRemove,
        child: Container(
          width: 24,
          height: 24,
          decoration: BoxDecoration(
            color: AppColors.error.withValues(alpha: 0.9),
            shape: BoxShape.circle,
          ),
          child: Icon(Icons.close, color: AppColors.neutralWhite, size: 16),
        ),
      ),
    );
  }

  Widget _buildErrorImage(bool isDark) {
    return Container(
      width: double.infinity,
      height: double.infinity,
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray500 : AppColors.neutralGray200,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Icon(
        Icons.broken_image_outlined,
        color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
        size: 32,
      ),
    );
  }
}
