import 'dart:io';
import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';

/// Text Input Media Preview Widget - Generic media preview for text inputs
class TextInputMediaPreview extends StatelessWidget {
  final List<String> selectedMediaUrls;
  final Function(int index)? onRemoveMedia;
  final Function(int index)? onMediaTap;

  const TextInputMediaPreview({
    super.key,
    required this.selectedMediaUrls,
    this.onRemoveMedia,
    this.onMediaTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      height: 80,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Row(
        children: [
          Expanded(
            child: ListView.builder(
              scrollDirection: Axis.horizontal,
              clipBehavior: Clip.none, // Agar shadow/border tidak terpotong
              itemCount: selectedMediaUrls.length,
              itemBuilder: (context, index) {
                final mediaUrl = selectedMediaUrls[index];
                final isVideo = _isVideoFile(mediaUrl);

                return Container(
                  width: 64,
                  height: 64,
                  margin: const EdgeInsets.only(right: 8),
                  decoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(8),
                    color: scheme.surfaceContainerHighest,
                  ),
                  child: Stack(
                    children: [
                      // Media preview
                      GestureDetector(
                        onTap: () => onMediaTap?.call(index),
                        child: ClipRRect(
                          borderRadius: BorderRadius.circular(8),
                          child: SizedBox(
                            width: 64,
                            height: 64,
                            child: isVideo
                                ? Container(
                                    color: scheme.surfaceContainerHighest,
                                    child: Icon(
                                      Icons.play_circle_filled,
                                      color: scheme.onSurfaceVariant,
                                      size: 32,
                                    ),
                                  )
                                : mediaUrl.startsWith('http')
                                ? CachedNetworkImage(
                                    imageUrl: mediaUrl,
                                    width: 64,
                                    height: 64,
                                    fit: BoxFit.cover,
                                    placeholder: (context, url) => Container(
                                      color: scheme.surfaceContainerHighest,
                                      child: const Center(
                                        child:
                                            CircularProgressIndicator.adaptive(),
                                      ),
                                    ),
                                    errorWidget: (context, url, error) =>
                                        Container(
                                          color: scheme.surfaceContainerHighest,
                                          child: Icon(
                                            Icons.broken_image_outlined,
                                            size: 24,
                                            color: scheme.onSurfaceVariant,
                                          ),
                                        ),
                                  )
                                : Image.file(
                                    File(mediaUrl),
                                    width: 64,
                                    height: 64,
                                    fit: BoxFit.cover,
                                  ),
                          ),
                        ),
                      ),

                      // Remove button
                      if (onRemoveMedia != null)
                        Positioned(
                          top: 4,
                          right: 4,
                          child: GestureDetector(
                            onTap: () => onRemoveMedia!(index),
                            child: Container(
                              width: 20,
                              height: 20,
                              decoration: BoxDecoration(
                                color: scheme.primary,
                                shape: BoxShape.circle,
                              ),
                              child: Icon(
                                Icons.close,
                                size: 12,
                                color: scheme.onPrimary,
                              ),
                            ),
                          ),
                        ),
                    ],
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  bool _isVideoFile(String url) {
    final extension = url.split('.').last.toLowerCase();
    return ['mp4', 'mov', 'avi', 'mkv', 'webm'].contains(extension);
  }
}
