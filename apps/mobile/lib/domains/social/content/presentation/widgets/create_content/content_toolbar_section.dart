import 'dart:io';
import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_toolbar_widget.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_media_handler.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_event_handlers.dart';

/// Widget for post creation toolbar with keyboard-aware padding
class ContentToolbarSection extends StatelessWidget {
  final ContentMediaHandler mediaHandler;
  final List<File> selectedImages;
  final List<File> selectedVideos;
  final List<String> taggedPeople;
  final bool hasLocation;
  final Function(List<File> images, List<File> videos) onMediaAdded;
  final VoidCallback onTagPeople;
  final VoidCallback onAddLocation;
  final bool isDark;

  const ContentToolbarSection({
    super.key,
    required this.mediaHandler,
    required this.selectedImages,
    required this.selectedVideos,
    required this.taggedPeople,
    required this.hasLocation,
    required this.onMediaAdded,
    required this.onTagPeople,
    required this.onAddLocation,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        border: Border(
          top: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
            width: 1,
          ),
        ),
      ),
      padding: EdgeInsets.only(
        bottom: MediaQuery.of(context).viewInsets.bottom > 0
            ? 0
            : MediaQuery.of(context).padding.bottom,
      ),
      child: ContentToolbarWidget(
        onGalleryTap: () => _handleGalleryTap(context),
        onCameraTap: () => _handleCameraTap(context),
        onTagPeopleTap: onTagPeople,
        onLocationTap: onAddLocation,
        taggedPeopleCount: taggedPeople.length,
        hasLocation: hasLocation,
      ),
    );
  }

  Future<void> _handleGalleryTap(BuildContext context) async {
    final media = await ContentEventHandlers.handleGalleryPick(
      context: context,
      mediaHandler: mediaHandler,
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
      mediaHandler: mediaHandler,
      currentMediaCount: selectedImages.length + selectedVideos.length,
    );
    if (media.isNotEmpty) {
      final categorized = ContentEventHandlers.processMediaFiles(media);
      onMediaAdded(categorized['images']!, categorized['videos']!);
    }
  }
}
