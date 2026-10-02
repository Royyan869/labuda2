import 'dart:io';

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Cover Photo Section Widget for Edit Profile
class EditProfileCoverSection extends StatelessWidget {
  final String? coverPhotoUrl;
  final String? selectedCoverPath;
  final bool isCoverMarkedForRemoval;
  final VoidCallback onChangeCover;
  final VoidCallback onRemoveCover;

  const EditProfileCoverSection({
    super.key,
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
    final scheme = Theme.of(context).colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Cover Photo',
          style: TextStyle(
            fontSize: AppType.s14,
            fontWeight: FontWeight.w600,
            color: scheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 8),
        GestureDetector(
          onTap: onChangeCover,
          child: AspectRatio(
            aspectRatio: 16 / 9,
            child: Container(
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(AppShape.r12),
                color: scheme.surfaceContainerHighest,
                image: _getCoverDecorationImage(),
              ),
              child: Stack(
                children: [
                  // Gradient overlay
                  Container(
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(AppShape.r12),
                      gradient: LinearGradient(
                        begin: Alignment.topCenter,
                        end: Alignment.bottomCenter,
                        colors: [
                          Colors.transparent,
                          scheme.scrim.withValues(alpha: 0.3),
                        ],
                      ),
                    ),
                  ),
                  // Camera icon
                  Center(
                    child: Container(
                      padding: const EdgeInsets.all(AppMetrics.p12),
                      decoration: BoxDecoration(
                        color: scheme.scrim.withValues(alpha: 0.5),
                        shape: BoxShape.circle,
                      ),
                      child: Icon(
                        Icons.camera_alt_outlined,
                        color: scheme.onPrimary,
                        size: AppIconSize.emphasis,
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
            padding: const EdgeInsets.only(top: AppMetrics.p8),
            child: TextButton.icon(
              onPressed: onRemoveCover,
              icon: const Icon(Icons.delete_outline, size: AppIconSize.action),
              label: const Text('Remove Cover'),
              style: TextButton.styleFrom(foregroundColor: context.statusColors.error),
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
