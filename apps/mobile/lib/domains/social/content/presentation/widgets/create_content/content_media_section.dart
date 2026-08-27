import 'dart:io';
import 'package:flutter/material.dart';
import 'package:labuda/shared/widgets/media_preview.dart';

/// Widget that shows the live create-flow media preview
///
/// Displays newly selected media (images/videos) with reorder/delete.
class ContentMediaSection extends StatelessWidget {
  final List<File> selectedImages;
  final List<File> selectedVideos;
  final Function(int, int) onImageReorder;
  final Function(int) onImageRemove;
  final VoidCallback onVideoRemove;
  final bool isDark;

  const ContentMediaSection({
    super.key,
    required this.selectedImages,
    required this.selectedVideos,
    required this.onImageReorder,
    required this.onImageRemove,
    required this.onVideoRemove,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // New media preview (newly uploaded)
        if (selectedImages.isNotEmpty || selectedVideos.isNotEmpty) ...[
          MediaPreview(
            selectedImages: selectedImages,
            selectedVideos: selectedVideos,
            onImageReorder: onImageReorder,
            onImageRemove: onImageRemove,
            onVideoRemove: onVideoRemove,
            imageHeight: 120,
            imageWidth: 120,
          ),
          const SizedBox(height: 16),
        ],
      ],
    );
  }
}
