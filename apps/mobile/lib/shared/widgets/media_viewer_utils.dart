import 'package:flutter/material.dart';

/// Utility class untuk media viewer operations
class MediaViewerUtils {
  /// Deteksi media type berdasarkan URL extension
  static bool isVideoUrl(String url) {
    final videoExtensions = [
      '.mp4',
      '.mov',
      '.avi',
      '.mkv',
      '.wmv',
      '.flv',
      '.webm',
      '.m4v',
    ];
    final lowerUrl = url.toLowerCase();
    return videoExtensions.any((ext) => lowerUrl.contains(ext));
  }

  /// Static method untuk show media viewer
  static void showMediaViewer(
    BuildContext context, {
    required List<String> mediaUrls,
    int initialIndex = 0,
    String? title,
    required Widget Function({
      required List<String> mediaUrls,
      int initialIndex,
      String? title,
    })
    mediaViewerBuilder,
  }) {
    if (mediaUrls.isEmpty) return;

    showDialog(
      context: context,
      barrierColor: Colors.black87,
      builder: (context) => mediaViewerBuilder(
        mediaUrls: mediaUrls,
        initialIndex: initialIndex,
        title: title,
      ),
    );
  }
}
