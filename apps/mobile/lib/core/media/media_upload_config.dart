/// Canonical media upload config — single source for foto+video limits
/// across content, for_sale, auction, komentar, chat.

class MediaUploadConfig {
  final int maxImages;
  final int maxVideos;
  final int maxTotal;
  final int maxImageSizeMb;
  final int maxVideoSizeMb;

  const MediaUploadConfig({
    required this.maxImages,
    required this.maxVideos,
    required this.maxTotal,
    this.maxImageSizeMb = 10,
    this.maxVideoSizeMb = 100,
  });

  int get maxMedia => maxTotal;

  static const forContent = MediaUploadConfig(
    maxImages: 15,
    maxVideos: 5,
    maxTotal: 20,
    maxImageSizeMb: 10,
    maxVideoSizeMb: 100,
  );

  static const forCommerce = MediaUploadConfig(
    maxImages: 10,
    maxVideos: 10,
    maxTotal: 10,
    maxImageSizeMb: 10,
    maxVideoSizeMb: 100,
  );

  static const forComment = MediaUploadConfig(
    maxImages: 4,
    maxVideos: 1,
    maxTotal: 5,
    maxImageSizeMb: 10,
    maxVideoSizeMb: 100,
  );

  static const forChat = MediaUploadConfig(
    maxImages: 4,
    maxVideos: 1,
    maxTotal: 5,
    maxImageSizeMb: 10,
    maxVideoSizeMb: 100,
  );

  int remaining(int currentCount) {
    final r = maxTotal - currentCount;
    return r < 0 ? 0 : r;
  }
}
