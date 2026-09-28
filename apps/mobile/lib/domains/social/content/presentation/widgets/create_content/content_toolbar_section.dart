import 'dart:io';
import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_toolbar_widget.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_event_handlers.dart';

/// Widget for post creation toolbar with keyboard-aware padding
class ContentToolbarSection extends StatelessWidget {
  final List<File> selectedImages;
  final List<File> selectedVideos;
  final int taggedPeopleCount;
  final bool hasLocation;
  final Function(List<File> images, List<File> videos) onMediaAdded;
  final VoidCallback onTagPeople;
  final VoidCallback onAddLocation;

  const ContentToolbarSection({
    super.key,
    required this.selectedImages,
    required this.selectedVideos,
    required this.taggedPeopleCount,
    required this.hasLocation,
    required this.onMediaAdded,
    required this.onTagPeople,
    required this.onAddLocation,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      decoration: BoxDecoration(
        color: scheme.surface,
        border: Border(
          top: BorderSide(
            color: scheme.outlineVariant,
            width: 1,
          ),
        ),
      ),
      padding: EdgeInsets.only(
        bottom: MediaQuery.of(context).viewInsets.bottom > AppMetrics.p0
            ? AppMetrics.p0
            : MediaQuery.of(context).padding.bottom,
      ),
      child: ContentToolbarWidget(
        onGalleryTap: () => _handleGalleryTap(context),
        onCameraTap: () => _handleCameraTap(context),
        onTagPeopleTap: onTagPeople,
        onLocationTap: onAddLocation,
        taggedPeopleCount: taggedPeopleCount,
        hasLocation: hasLocation,
      ),
    );
  }

  Future<void> _handleGalleryTap(BuildContext context) async {
    final media = await ContentEventHandlers.handleGalleryPick(
      context: context,
      currentMediaCount: selectedImages.length + selectedVideos.length,
    );
    if (media.isNotEmpty) {
      final categorized = ContentEventHandlers.processMediaFiles(media);
      onMediaAdded(categorized['images']!, categorized['videos']!);
    }
  }

  Future<void> _handleCameraTap(BuildContext context) async {
    final media = await ContentEventHandlers.handleCamera(
      context: context,
      currentMediaCount: selectedImages.length + selectedVideos.length,
    );
    if (media.isNotEmpty) {
      final categorized = ContentEventHandlers.processMediaFiles(media);
      onMediaAdded(categorized['images']!, categorized['videos']!);
    }
  }
}
