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
}
