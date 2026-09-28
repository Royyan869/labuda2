import 'package:flutter/material.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:labuda/shared/widgets/media_viewer_video_player.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

class EvidenceMediaGallery extends StatelessWidget {
  final List<String> urls;

  const EvidenceMediaGallery({super.key, required this.urls});

  @override
  Widget build(BuildContext context) {
    if (urls.isEmpty) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Bukti media', style: Theme.of(context).textTheme.titleSmall),
        const SizedBox(height: 8),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: urls.asMap().entries.map((entry) {
            final url = entry.value;
            final isVideo = MediaUploadOrchestrator.isVideoUrl(url);
            return ClipRRect(
              borderRadius: BorderRadius.circular(AppShape.r8),
              child: SizedBox(
                width: 112,
                height: 112,
                child: isVideo
                    ? MediaViewerVideoPlayer(videoUrl: url)
                    : AppImage(
                        imageUrl: url,
                        cacheWidth: 224,
                        fit: BoxFit.cover,
                        errorWidget: const Icon(Icons.broken_image_outlined),
                      ),
              ),
            );
          }).toList(),
        ),
      ],
    );
  }
}
