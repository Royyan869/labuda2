import 'dart:io';
import 'package:flutter/material.dart';
import 'package:labuda/shared/entities/post_location.dart' as loc;
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_metadata_sections.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_content_input.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_media_section.dart';

/// Scrollable content section for create post screen
class ContentScrollableContent extends StatelessWidget {
  final TextEditingController contentController;
  final bool isDark;
  final List<File> selectedImages;
  final List<File> selectedVideos;
  final List<String> taggedPeople;
  final loc.PostLocation? selectedLocation;
  final List<String> hashtags;
  final Function(String) onContentChanged;
  final Function(int, int) onImageReorder;
  final Function(int) onImageRemove;
  final VoidCallback onVideoRemove;
  final VoidCallback onTagPeopleEdit;
  final VoidCallback onLocationEdit;
  final VoidCallback onHashtagEdit;
  final Function(String) onTaggedPersonRemove;
  final VoidCallback onLocationRemove;
  final Function(String) onHashtagRemove;

  const ContentScrollableContent({
    super.key,
    required this.contentController,
    required this.isDark,
    required this.selectedImages,
    required this.selectedVideos,
    required this.taggedPeople,
    required this.selectedLocation,
    required this.hashtags,
    required this.onContentChanged,
    required this.onImageReorder,
    required this.onImageRemove,
    required this.onVideoRemove,
    required this.onTagPeopleEdit,
    required this.onLocationEdit,
    required this.onHashtagEdit,
    required this.onTaggedPersonRemove,
    required this.onLocationRemove,
    required this.onHashtagRemove,
  });

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Text input area
          ContentContentInput(
            controller: contentController,
            isDark: isDark,
            onChanged: onContentChanged,
          ),

          const SizedBox(height: 16),

          // Media preview section (new uploads only)
          ContentMediaSection(
            selectedImages: selectedImages,
            selectedVideos: selectedVideos,
            onImageReorder: onImageReorder,
            onImageRemove: onImageRemove,
            onVideoRemove: onVideoRemove,
            isDark: isDark,
          ),

          // Metadata Sections
          ContentMetadataSections.buildTaggedPeopleSection(
            taggedPeople: taggedPeople,
            onEdit: onTagPeopleEdit,
            onRemove: onTaggedPersonRemove,
            isDark: isDark,
          ),
          ContentMetadataSections.buildLocationSection(
            location: selectedLocation?.address,
            onEdit: onLocationEdit,
            onRemove: onLocationRemove,
            isDark: isDark,
          ),
          ContentMetadataSections.buildHashtagsSection(
            hashtags: hashtags
                .map((tag) => tag.startsWith('#') ? tag : '#$tag')
                .toList(),
            onEdit: onHashtagEdit,
            onRemove: onHashtagRemove,
            isDark: isDark,
          ),

          // Bottom spacing for better UX
          const SizedBox(height: 16),
        ],
      ),
    );
  }
}
