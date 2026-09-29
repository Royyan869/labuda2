/// Canonical media upload config — single source for foto+video limits
/// across content, for_sale, auction, komentar, chat.
class MediaUploadConfig {
  final int maxImages;
  final int maxVideos;
  final int maxTotal;
  final int maxImageSizeMb;
  final int maxVideoSizeMb;
  final String imageFolder;
  final String videoFolder;

  const MediaUploadConfig({
    required this.maxImages,
    required this.maxVideos,
    required this.maxTotal,
    this.maxImageSizeMb = 10,
    this.maxVideoSizeMb = 100,
    required this.imageFolder,
    required this.videoFolder,
  });

  int get maxMedia => maxTotal;

  static const forContent = MediaUploadConfig(
    maxImages: 15,
    maxVideos: 5,
    maxTotal: 20,
    maxImageSizeMb: 10,
    maxVideoSizeMb: 100,
    imageFolder: 'images/content',
    videoFolder: 'videos/content',
  );

  static const forCommerce = MediaUploadConfig(
    maxImages: 10,
    maxVideos: 10,
    maxTotal: 10,
    maxImageSizeMb: 10,
    maxVideoSizeMb: 100,
    imageFolder: 'images/commerce',
    videoFolder: 'videos/commerce',
  );

  static const forComment = MediaUploadConfig(
    maxImages: 4,
    maxVideos: 1,
    maxTotal: 5,
    maxImageSizeMb: 10,
    maxVideoSizeMb: 100,
    imageFolder: 'images/content',
    videoFolder: 'videos/content',
  );

  static const forChat = MediaUploadConfig(
    maxImages: 4,
    maxVideos: 1,
    maxTotal: 5,
    maxImageSizeMb: 10,
    maxVideoSizeMb: 100,
    imageFolder: 'images/chat',
    videoFolder: 'videos/chat',
  );

  /// Evidence (dispute/refund): 1 required video + up to 5 photos.
  /// Counts stay with the dialog; MB caps are canonical here.
  static const forEvidence = MediaUploadConfig(
    maxImages: 5,
    maxVideos: 1,
    maxTotal: 6,
    maxImageSizeMb: 10,
    maxVideoSizeMb: 100,
    imageFolder: 'images/evidence',
    videoFolder: 'videos/evidence',
  );

  int remaining(int currentCount) {
    final r = maxTotal - currentCount;
    return r < 0 ? 0 : r;
  }
}
