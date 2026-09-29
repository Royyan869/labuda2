/// Canonical media policy — the ONE place that says what a surface may pick.
///
/// `maxTotal`, `maxImages`, and `maxVideos` are all ENFORCED by
/// `MediaUploadOrchestrator`. A cap that no code checks is a lie the UI tells
/// the user, so do not add a field here unless the engine reads it.
class MediaUploadConfig {
  /// Hard ceiling for one composer (foto + video share this budget).
  final int maxTotal;
  final int maxImages;
  final int maxVideos;
  final int maxImageSizeMb;
  final int maxVideoSizeMb;

  /// Upload namespace for the immediate-upload path. Identity surfaces
  /// (avatar/cover/store) upload through fixed keys owned by their services.
  final String imageFolder;
  final String videoFolder;

  const MediaUploadConfig({
    required this.maxTotal,
    required this.maxImages,
    required this.maxVideos,
    this.maxImageSizeMb = 10,
    this.maxVideoSizeMb = 100,
    required this.imageFolder,
    required this.videoFolder,
  });

  bool get videoAllowed => maxVideos > 0;

  /// How many more media of [kind] the composer may still take.
  MediaCounts remaining(MediaCounts current) => MediaCounts(
    images: (maxImages - current.images).clamp(0, maxImages),
    videos: (maxVideos - current.videos).clamp(0, maxVideos),
  );

  int remainingTotalFor(MediaCounts current) => maxTotal - current.total;

  static const forContent = MediaUploadConfig(
    maxTotal: 20,
    maxImages: 15,
    maxVideos: 5,
    imageFolder: 'images/content',
    videoFolder: 'videos/content',
  );

  static const forCommerce = MediaUploadConfig(
    maxTotal: 10,
    maxImages: 10,
    maxVideos: 10,
    imageFolder: 'images/commerce',
    videoFolder: 'videos/commerce',
  );

  static const forComment = MediaUploadConfig(
    maxTotal: 5,
    maxImages: 4,
    maxVideos: 1,
    imageFolder: 'images/content',
    videoFolder: 'videos/content',
  );

  static const forChat = MediaUploadConfig(
    maxTotal: 5,
    maxImages: 4,
    maxVideos: 1,
    imageFolder: 'images/chat',
    videoFolder: 'videos/chat',
  );

  /// Order evidence: exactly the product rule — 1 video + up to 5 photos.
  static const forEvidence = MediaUploadConfig(
    maxTotal: 6,
    maxImages: 5,
    maxVideos: 1,
    imageFolder: 'images/evidence',
    videoFolder: 'videos/evidence',
  );

  /// Identity surfaces (avatar, cover, store photo): ONE photo, no video, crop
  /// after pick. Upload is a fixed key owned by the caller's service.
  ///
  /// KYC is deliberately NOT a preset: its capture surface is a dedicated live
  /// camera with no gallery path in the code at all, so "wajib kamera" is
  /// structural rather than a flag this config would have to remember to set.
  static const forIdentity = MediaUploadConfig(
    maxTotal: 1,
    maxImages: 1,
    maxVideos: 0,
    imageFolder: 'images/identity',
    videoFolder: 'videos/identity',
  );
}

/// How many media of each kind a composer already holds. Cap math is per-type
/// because "1 video + 5 foto" is a product rule, not a total.
class MediaCounts {
  final int images;
  final int videos;

  const MediaCounts({this.images = 0, this.videos = 0});

  int get total => images + videos;
}
