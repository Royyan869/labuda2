import 'dart:io';
import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:labuda/core/core.dart';

/// Text Input Media Preview Widget - Generic media preview for text inputs
class TextInputMediaPreview extends StatelessWidget {
  final List<String> selectedMediaUrls;
  final Function(int index)? onRemoveMedia;
  final Function(int index)? onMediaTap;
  final bool isDark;

  const TextInputMediaPreview({
    super.key,
    required this.selectedMediaUrls,
    this.onRemoveMedia,
    this.onMediaTap,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
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
                    color: isDark
                        ? AppColors.darkGray700
                        : AppColors.neutralGray100,
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
                                    color: isDark
                                        ? AppColors.darkGray600
                                        : AppColors.neutralGray200,
                                    child: Icon(
                                      Icons.play_circle_filled,
                                      color: AppColors.neutralWhite,
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
                                      color: AppColors.neutralGray100,
                                      child: const Center(
                                        child:
                                            CircularProgressIndicator.adaptive(),
                                      ),
                                    ),
                                    errorWidget: (context, url, error) =>
                                        Container(
                                          color: AppColors.neutralGray100,
                                          child: Icon(
                                            Icons.broken_image_outlined,
                                            size: 24,
                                            color: AppColors.neutralGray600,
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
                              decoration: const BoxDecoration(
                                color: AppColors.primaryRed,
                                shape: BoxShape.circle,
                              ),
                              child: const Icon(
                                Icons.close,
                                size: 12,
                                color: AppColors.neutralWhite,
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
