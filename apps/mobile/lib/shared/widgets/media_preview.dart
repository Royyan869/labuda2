import 'dart:io';
import 'package:flutter/material.dart';
import 'package:labuda/shared/widgets/media_image_item.dart';
import 'package:labuda/shared/widgets/media_video_item.dart';
import 'package:labuda/shared/widgets/media_reorderable_list.dart';

/// Widget untuk preview media yang dipilih user
///
/// Mendukung reorder images, remove images/videos, dan menampilkan cover image
///
/// MOVED FROM CREATION MODULE: Shared widget yang bisa digunakan oleh semua modules
class MediaPreview extends StatelessWidget {
  const MediaPreview({
    super.key,
    this.selectedImages = const [],
    this.selectedVideos = const [],
    this.onImageReorder,
    this.onImageRemove,
    this.onVideoRemove,
    this.showCoverBadge = true,
    this.imageHeight = 100,
    this.imageWidth = 100,
  });

  final List<File> selectedImages;
  final List<File> selectedVideos;
  final void Function(int oldIndex, int newIndex)? onImageReorder;
  final void Function(int index)? onImageRemove;
  final VoidCallback? onVideoRemove;
  final bool showCoverBadge;
  final double imageHeight;
  final double imageWidth;

  @override
  Widget build(BuildContext context) {
    if (selectedImages.isEmpty && selectedVideos.isEmpty) {
      return const SizedBox.shrink();
    }

    return SizedBox(height: imageHeight, child: _buildMixedMediaPreview());
  }

  Widget _buildMixedMediaPreview() {
    // For images only, use ReorderableListView to allow cover photo change
    if (selectedVideos.isEmpty && selectedImages.length > 1) {
      return MediaReorderableList(
        images: selectedImages,
        onReorder: onImageReorder,
        onRemove: onImageRemove,
        showCoverBadge: showCoverBadge,
        height: imageHeight,
        width: imageWidth,
      );
    }

    // Mixed content atau single image - tidak perlu reorder
    return ListView(
      scrollDirection: Axis.horizontal,
      clipBehavior: Clip.none, // Agar shadow/border tidak terpotong
      children: [
        // Images
        ...selectedImages.asMap().entries.map((entry) {
          final index = entry.key;
          final image = entry.value;
          return MediaImageItem(
            image: image,
            index: index,
            onRemove: onImageRemove != null
                ? () => onImageRemove!(index)
                : null,
            showCoverBadge: showCoverBadge,
            height: imageHeight,
            width: imageWidth,
          );
        }),
        // Videos
        ...selectedVideos.map(
          (video) => MediaVideoItem(
            video: video,
            onRemove: onVideoRemove,
            height: imageHeight,
            width: imageWidth,
          ),
        ),
      ],
    );
  }
}
