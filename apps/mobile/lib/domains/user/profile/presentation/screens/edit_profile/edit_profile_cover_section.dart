import 'dart:io';

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Cover Photo Section Widget for Edit Profile
class EditProfileCoverSection extends StatelessWidget {
  final String userId;
  final String? coverPhotoUrl;
  final String? selectedCoverPath;
  final bool isCoverMarkedForRemoval;
  final VoidCallback onChangeCover;
  final VoidCallback onRemoveCover;

  const EditProfileCoverSection({
    super.key,
    required this.userId,
    this.coverPhotoUrl,
    this.selectedCoverPath,
    required this.isCoverMarkedForRemoval,
    required this.onChangeCover,
    required this.onRemoveCover,
  });

  bool get _hasCover =>
      selectedCoverPath != null ||
      (coverPhotoUrl != null && !isCoverMarkedForRemoval);

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Cover Photo',
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray700,
          ),
        ),
        const SizedBox(height: 8),
        GestureDetector(
          onTap: onChangeCover,
          child: AspectRatio(
            aspectRatio: 16 / 9,
            child: Container(
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(12),
                color: isDark
                    ? AppColors.darkGray700
                    : AppColors.neutralGray200,
                image: _getCoverDecorationImage(),
              ),
              child: Stack(
                children: [
                  // Gradient overlay
                  Container(
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(12),
                      gradient: LinearGradient(
                        begin: Alignment.topCenter,
                        end: Alignment.bottomCenter,
                        colors: [
                          Colors.transparent,
                          Colors.black.withValues(alpha: 0.3),
                        ],
                      ),
                    ),
                  ),
                  // Camera icon
                  Center(
                    child: Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        color: AppColors.neutralBlack.withValues(alpha: 0.5),
                        shape: BoxShape.circle,
                      ),
                      child: const Icon(
                        Icons.camera_alt_outlined,
                        color: AppColors.neutralWhite,
                        size: 28,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
        // Remove button
        if (_hasCover)
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: TextButton.icon(
              onPressed: onRemoveCover,
              icon: const Icon(Icons.delete_outline, size: 18),
              label: const Text('Remove Cover'),
              style: TextButton.styleFrom(foregroundColor: AppColors.error),
            ),
          ),
      ],
    );
  }

  DecorationImage? _getCoverDecorationImage() {
    if (isCoverMarkedForRemoval) return null;

    if (selectedCoverPath != null) {
      return DecorationImage(
        image: FileImage(File(selectedCoverPath!)),
        fit: BoxFit.cover,
      );
    }

    if (coverPhotoUrl != null) {
      return DecorationImage(
        image: NetworkImage(coverPhotoUrl!),
        fit: BoxFit.cover,
      );
    }

    return null;
  }
}
