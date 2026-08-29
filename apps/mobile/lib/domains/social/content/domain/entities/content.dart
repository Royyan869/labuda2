// Content Domain Entities
// Pure Dart entities - bebas dari Firebase, Flutter, HTTP
//
// ============================================================================
// CONTENT DOMAIN CONTRACT
// ============================================================================
//
// CONTENT = SOCIAL PUBLISHING OBJECT, NOT COMMERCE OBJECT
// - Social content sharing (stories, showcase, tips)
// - Content does NOT authorize order/payment/finance/commerce operations
//
// RESOURCE PROJECTION TRUTH:
// - Repost creates NEW Content with canonical resourceProjection (not a copy)
// - resourceProjection contains the canonical reference (resourceType + resourceId)
// - Projection payload is cached for UI only and may be stale
//
// MEDIA INTEGRATION:
// - MediaEntity is canonical media representation
// - Feed projection uses simplified media for performance
// - Always resolve canonical Content for complete media data
// ============================================================================

import 'package:equatable/equatable.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';

/// Status content - Canonical status aligned with backend
///
/// State Transition Rules:
///   - active     -> deleted
///   - deleted    -> (terminal, no transitions allowed)
///
/// Key Invariants:
///   - Once deleted, content cannot transition back
enum ContentStatus {
  /// Content is published and visible (initial state for all content)
  active,

  /// Content is soft-deleted (terminal state)
  deleted;

  String get displayName {
    switch (this) {
      case ContentStatus.active:
        return 'Aktif';
      case ContentStatus.deleted:
        return 'Dihapus';
    }
  }

  String get description {
    switch (this) {
      case ContentStatus.active:
        return 'Content aktif dan visible';
      case ContentStatus.deleted:
        return 'Content dihapus';
    }
  }

  /// Check if content is in active state (published and visible)
  bool get isActive => this == ContentStatus.active;

  /// Check if content is deleted (terminal state)
  bool get isDeleted => this == ContentStatus.deleted;

  /// Check if this is a terminal state (no further transitions allowed)
  bool get isTerminal => this == ContentStatus.deleted;

  /// Parse a backend API string to ContentStatus.
  ///
  /// Returns null for unknown/null input (fail-closed).
  ///
  /// FIX-3 — eliminates raw magic-string comparisons at call sites.
  static ContentStatus? fromApiString(String? value) {
    switch (value) {
      case 'active':
        return ContentStatus.active;
      case 'deleted':
        return ContentStatus.deleted;
      default:
        return null; // unknown / null → fail-closed
    }
  }
}

// ContentLinkedItemType REMOVED (HARD CLEANUP BATCH):
// Content domain now uses canonical ContentResourceProjection for all cross-domain references.
// ShareTargetType.listing and ShareTargetType.auction should be used instead.

/// Enum untuk visibility post
enum ContentVisibility {
  public,
  followersOnly,
  private;

  String get displayName {
    switch (this) {
      case ContentVisibility.public:
        return 'Public';
      case ContentVisibility.followersOnly:
        return 'Followers';
      case ContentVisibility.private:
        return 'Private';
    }
  }
}

/// Enum untuk tipe media
enum MediaType { image, video }

// ============================================================================
// Value Objects
// ============================================================================

/// Value object untuk engagement metrics
class ContentEngagement extends Equatable {
  final int viewCount;
  final int likeCount;
  final int commentCount;
  final int shareCount;
  final int saveCount;
  final int reportCount;
  final List<String> likedByUsers; // For like toggle logic

  const ContentEngagement({
    this.viewCount = 0,
    this.likeCount = 0,
    this.commentCount = 0,
    this.shareCount = 0,
    this.saveCount = 0,
    this.reportCount = 0,
    this.likedByUsers = const [],
  });

  /// Business logic: Get total engagement
  int get totalEngagement => likeCount + commentCount + shareCount + saveCount;

  @override
  List<Object> get props => [
    viewCount,
    likeCount,
    commentCount,
    shareCount,
    saveCount,
    reportCount,
    likedByUsers,
  ];

  ContentEngagement copyWith({
    int? viewCount,
    int? likeCount,
    int? commentCount,
    int? shareCount,
    int? saveCount,
    int? reportCount,
    List<String>? likedByUsers,
  }) {
    return ContentEngagement(
      viewCount: viewCount ?? this.viewCount,
      likeCount: likeCount ?? this.likeCount,
      commentCount: commentCount ?? this.commentCount,
      shareCount: shareCount ?? this.shareCount,
      saveCount: saveCount ?? this.saveCount,
      reportCount: reportCount ?? this.reportCount,
      likedByUsers: likedByUsers ?? this.likedByUsers,
    );
  }
}

/// Value object untuk settings content
///
/// Comments are always supported by Content — there is no allowComments
/// business switch.
class ContentSettings extends Equatable {
  final ContentVisibility visibility;

  const ContentSettings({
    this.visibility = ContentVisibility.public,
  });

  @override
  List<Object> get props => [visibility];

  ContentSettings copyWith({
    ContentVisibility? visibility,
  }) {
    return ContentSettings(
      visibility: visibility ?? this.visibility,
    );
  }
}

/// Value object untuk lokasi content
class ContentLocation extends Equatable {
  final String? city;
  final String? province;
  final String? country;
  final double? latitude;
  final double? longitude;
  final String? placeName;

  const ContentLocation({
    this.city,
    this.province,
    this.country = 'Indonesia',
    this.latitude,
    this.longitude,
    this.placeName,
  });

  /// Business logic: Get display location
  String get displayLocation {
    if (placeName != null) return placeName!;
    if (city != null && province != null) return '$city, $province';
    return province ?? city ?? 'Lokasi tidak diketahui';
  }

  /// Business logic: Check if location has coordinates
  bool get hasCoordinates => latitude != null && longitude != null;

  @override
  List<Object?> get props => [
    city,
    province,
    country,
    latitude,
    longitude,
    placeName,
  ];

  ContentLocation copyWith({
    String? city,
    String? province,
    String? country,
    double? latitude,
    double? longitude,
    String? placeName,
  }) {
    return ContentLocation(
      city: city ?? this.city,
      province: province ?? this.province,
      country: country ?? this.country,
      latitude: latitude ?? this.latitude,
      longitude: longitude ?? this.longitude,
      placeName: placeName ?? this.placeName,
    );
  }
}

// ContentLinkedItem REMOVED (HARD CLEANUP BATCH):
// Content domain now uses canonical ContentResourceProjection for all cross-domain references.
//
// MIGRATION:
// - resourceProjection.fixedPriceSale / resourceProjection.auction
// - originalAuthorId preserves repost attribution
// - ContentLinkedItem.description -> REMOVED (was UI-only price display, not business logic)

/// Value object untuk informasi moderasi
class ContentModerationInfo extends Equatable {
  final bool isApproved;
  final bool hasBeenModerated;
  final int flagCount;
  final int violationCount;
  final List<String> detectedViolations;
  final String? moderatorId;
  final DateTime? moderatedAt;
  final String? moderationNote;
  final String? lastAction;

  const ContentModerationInfo({
    this.isApproved = true,
    this.hasBeenModerated = false,
    this.flagCount = 0,
    this.violationCount = 0,
    this.detectedViolations = const [],
    this.moderatorId,
    this.moderatedAt,
    this.moderationNote,
    this.lastAction,
  });

  /// Business logic: Check if post needs attention
  bool get needsAttention => flagCount > 0 || violationCount > 0;

  /// Business logic: Check if post is clean
  bool get isClean => isApproved && violationCount == 0 && flagCount == 0;

  @override
  List<Object?> get props => [
    isApproved,
    hasBeenModerated,
    flagCount,
    violationCount,
    detectedViolations,
    moderatorId,
    moderatedAt,
    moderationNote,
    lastAction,
  ];

  ContentModerationInfo copyWith({
    bool? isApproved,
    bool? hasBeenModerated,
    int? flagCount,
    int? violationCount,
    List<String>? detectedViolations,
    String? moderatorId,
    DateTime? moderatedAt,
    String? moderationNote,
    String? lastAction,
  }) {
    return ContentModerationInfo(
      isApproved: isApproved ?? this.isApproved,
      hasBeenModerated: hasBeenModerated ?? this.hasBeenModerated,
      flagCount: flagCount ?? this.flagCount,
      violationCount: violationCount ?? this.violationCount,
      detectedViolations: detectedViolations ?? this.detectedViolations,
      moderatorId: moderatorId ?? this.moderatorId,
      moderatedAt: moderatedAt ?? this.moderatedAt,
      moderationNote: moderationNote ?? this.moderationNote,
      lastAction: lastAction ?? this.lastAction,
    );
  }
}

/// Value object untuk media
class MediaEntity extends Equatable {
  final String id;
  final String originalUrl;
  final MediaType type;
  final String? blurhash;
  final MediaDimensions? dimensions;
  final int? position;
  final int? duration;
  final DateTime createdAt;
  final Map<String, String> variants; // key: size/type, value: url

  const MediaEntity({
    required this.id,
    required this.originalUrl,
    required this.type,
    this.blurhash,
    this.dimensions,
    this.position,
    this.duration,
    required this.createdAt,
    this.variants = const {},
  });

  /// Get thumbnail URL if available
  String? get thumbnailUrl => variants['thumbnail'] ?? originalUrl;

  /// Poster frame URL (video) — falls back to thumbnail.
  String? get posterUrl => variants['poster'] ?? thumbnailUrl;

  @override
  List<Object?> get props => [
    id,
    originalUrl,
    type,
    blurhash,
    dimensions,
    position,
    duration,
    createdAt,
    variants,
  ];

  MediaEntity copyWith({
    String? id,
    String? originalUrl,
    MediaType? type,
    String? blurhash,
    MediaDimensions? dimensions,
    int? position,
    int? duration,
    DateTime? createdAt,
    Map<String, String>? variants,
  }) {
    return MediaEntity(
      id: id ?? this.id,
      originalUrl: originalUrl ?? this.originalUrl,
      type: type ?? this.type,
      blurhash: blurhash ?? this.blurhash,
      dimensions: dimensions ?? this.dimensions,
      position: position ?? this.position,
      duration: duration ?? this.duration,
      createdAt: createdAt ?? this.createdAt,
      variants: variants ?? this.variants,
    );
  }
}

/// Value object untuk media dimensions
class MediaDimensions extends Equatable {
  final int width;
  final int height;

  const MediaDimensions({required this.width, required this.height});

  /// Get aspect ratio
  double get aspectRatio => width / height;

  /// Check if portrait
  bool get isPortrait => height > width;

  /// Check if landscape
  bool get isLandscape => width > height;

  /// Check if square
  bool get isSquare => width == height;

  @override
  List<Object> get props => [width, height];

  MediaDimensions copyWith({int? width, int? height}) {
    return MediaDimensions(
      width: width ?? this.width,
      height: height ?? this.height,
    );
  }
}

// ============================================================================
// Main Entity
// ============================================================================

/// Entitas content untuk feed dalam platform koi.
///
/// Content adalah konten individual yang dapat berupa cerita,
/// sharing informasi, showcase koleksi.
/// Unified content system.
///
/// CONTRACT: Content adalah social publishing object, BUKAN commerce object.
/// - Content tidak menjadi authority untuk order/payment/finance/commerce
/// - Listing reference comment adalah seller response, bukan status change
///
/// SHARE CONTRACT V1:
/// - Repost creates new Content with canonical resourceProjection (not a copy)
/// - Original author attribution is preserved via originalAuthorId
/// - resourceProjection contains the canonical reference to original content
class Content extends Equatable {
  final String id;
  final String content;
  final String authorId;
  final String? authorUsername;
  final String? authorAvatarUrl;
  final String? authorCity;
  final String? authorProvince;
  final ContentStatus status;

  /// D1 — canonical governance lifecycle ({active, unavailable, removed}).
  /// Separate from raw `status` — never coerce one into the other. Defaults
  /// to active when wire is null/missing/unknown via
  /// ContentLifecycleParse.fromWire so legacy payloads keep rendering
  /// today's behavior. Detail screen reads this to render the unavailable
  /// banner / removed tombstone state when the backend evaluator emits a
  /// non-active lifecycle decision.
  final ContentLifecycle lifecycle;

  /// E6 — canonical author identity lifecycle ({active, unavailable, removed}).
  /// Sourced from the wire's nested `card.author.lifecycle` slot populated
  /// by content_handler.go via publiccard.NewWithLifecycle through the
  /// lifecycle-aware author hydrator (E6).
  ///
  /// Independent from [lifecycle]: an active post by a suspended author
  /// renders with the content body visible but the author identity
  /// redacted. Defaults to active when wire is null/missing/unknown so
  /// legacy payloads keep rendering today's behavior.
  final ContentLifecycle authorLifecycle;

  /// Rich media entities with metadata.
  /// Provides blurhash for placeholders, dimensions for aspect ratio,
  /// and variants for different sizes.
  final List<MediaEntity> media;

  final List<String> tags;
  final List<String> taggedUsers; // Tagged people (@username)
  final List<String> mentionedUserIds; // Mentioned users in content (@mention)

  final ContentSettings settings; // Visibility, comments, shares settings
  final ContentEngagement engagement;
  final ContentLocation? location;
  final ContentModerationInfo moderationInfo;
  final DateTime? publishedAt;
  final DateTime? scheduledAt;
  final DateTime createdAt;
  final DateTime updatedAt;

  // SHARE CONTRACT V1: Repost attribution fields
  /// Original author ID - set when this content is a repost of another content
  final String? originalAuthorId;

  /// Canonical resource projection for linked content/card rendering.
  final ContentResourceProjection? resourceProjection;

  const Content({
    required this.id,
    required this.content,
    required this.authorId,
    this.authorUsername,
    this.authorAvatarUrl,
    this.authorCity,
    this.authorProvince,
    required this.status,
    this.lifecycle = ContentLifecycle.active,
    this.authorLifecycle = ContentLifecycle.active,
    this.media = const [],
    this.tags = const [],
    this.taggedUsers = const [],
    this.mentionedUserIds = const [],
    this.settings = const ContentSettings(),
    required this.engagement,
    this.location,
    required this.moderationInfo,
    this.publishedAt,
    this.scheduledAt,
    required this.createdAt,
    required this.updatedAt,
    this.originalAuthorId,
    this.resourceProjection,
  });

  // ============================================================================
  // Business Logic - Canonical Contract
  // ============================================================================

  /// Business logic: Check if content is visible (active and not hidden)
  /// This is the source of truth for content visibility
  bool get isVisible => status.isActive;

  /// Business logic: Check if content has linked products
  /// Note: Content are for social content, NOT selling
  /// Products can be mentioned/linked for reference only
  bool get hasLinkedProducts => resourceProjection != null;

  bool get hasResourceProjection => resourceProjection != null;

  /// Business logic: Check if content needs moderation
  bool get needsModeration =>
      moderationInfo.flagCount > 0 || !moderationInfo.isApproved;

  /// Business logic: Check if content can be edited by user
  bool canBeEditedBy(String userId) => authorId == userId && status.isActive;

  /// Business logic: Get engagement rate
  double get engagementRate {
    final totalEngagementVal =
        engagement.likeCount + engagement.commentCount + engagement.shareCount;

    if (engagement.viewCount == 0) return 0.0;
    return totalEngagementVal / engagement.viewCount;
  }

  /// Business logic: Check if post has high engagement
  bool get hasHighEngagement => engagementRate > 0.1;

  /// Business logic: Check if post content is safe
  bool get isContentSafe =>
      moderationInfo.isApproved &&
      moderationInfo.violationCount == 0 &&
      engagement.reportCount < 3;

  /// Business logic: Check if content is active and can receive
  /// commerce resource attachments in comments.
  bool get canReceiveListingResponses => status.isActive;

  // ============================================================================
  // SHARE CONTRACT V1: Repost Business Logic
  // ============================================================================

  /// Business logic: Check if this content is a repost of another content
  bool get isRepost => originalAuthorId != null;

  /// Business logic: Get the canonical ID of the original content if this is a repost
  String? get originalContentId {
    return resourceProjection?.resourceId;
  }

  @override
  List<Object?> get props => [
    id,
    content,
    authorId,
    authorUsername,
    authorAvatarUrl,
    authorCity,
    authorProvince,
    status,
    lifecycle,
    authorLifecycle,
    media,
    tags,
    taggedUsers,
    mentionedUserIds,
    settings,
    engagement,
    location,
    moderationInfo,
    publishedAt,
    scheduledAt,
    originalAuthorId,
    createdAt,
    updatedAt,
  ];

  Content copyWith({
    String? id,
    String? content,
    String? authorId,
    String? authorUsername,
    String? authorAvatarUrl,
    String? authorCity,
    String? authorProvince,
    ContentStatus? status,
    ContentLifecycle? lifecycle,
    ContentLifecycle? authorLifecycle,
    List<MediaEntity>? media,
    List<String>? tags,
    List<String>? taggedUsers,
    List<String>? mentionedUserIds,
    ContentSettings? settings,
    ContentEngagement? engagement,
    ContentLocation? location,
    ContentModerationInfo? moderationInfo,
    DateTime? publishedAt,
    DateTime? scheduledAt,
    DateTime? createdAt,
    DateTime? updatedAt,
    String? originalAuthorId,
    ContentResourceProjection? resourceProjection,
  }) {
    return Content(
      id: id ?? this.id,
      content: content ?? this.content,
      authorId: authorId ?? this.authorId,
      authorUsername: authorUsername ?? this.authorUsername,
      authorAvatarUrl: authorAvatarUrl ?? this.authorAvatarUrl,
      authorCity: authorCity ?? this.authorCity,
      authorProvince: authorProvince ?? this.authorProvince,
      status: status ?? this.status,
      lifecycle: lifecycle ?? this.lifecycle,
      authorLifecycle: authorLifecycle ?? this.authorLifecycle,
      media: media ?? this.media,
      tags: tags ?? this.tags,
      taggedUsers: taggedUsers ?? this.taggedUsers,
      mentionedUserIds: mentionedUserIds ?? this.mentionedUserIds,
      settings: settings ?? this.settings,
      engagement: engagement ?? this.engagement,
      location: location ?? this.location,
      moderationInfo: moderationInfo ?? this.moderationInfo,
      publishedAt: publishedAt ?? this.publishedAt,
      scheduledAt: scheduledAt ?? this.scheduledAt,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      originalAuthorId: originalAuthorId ?? this.originalAuthorId,
      resourceProjection: resourceProjection ?? this.resourceProjection,
    );
  }
}
