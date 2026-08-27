/// Feed item entity for home feed display
///
/// PROJECTION LAYER ONLY: This is a UI projection entity, NOT domain truth.
/// Source of truth is the canonical Content entity from Content domain.
///
/// R3.1: Created to replace missing shared FeedItem entity
/// This entity represents a single item in the user's home feed
///
/// CONTRACT:
/// - Only universal social content appears in home feed
/// - Commerce types (auction) are filtered by backend
/// - Engagement counts are provided (may be 0 if not supported by backend)
///
/// MEDIA INTEGRATION: Uses canonical MediaEntity from Content domain
/// Feed projection may have simplified media - resolve full Content for complete media data
library;

import 'package:labuda/domains/social/content/domain/entities/content.dart'; // For MediaEntity
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Feed item types
enum FeedItemType {
  /// Universal social content item
  content,

  /// P3A — Promoted listing injected by backend feed promotion injector.
  promotedListing,

  /// P3A — Promoted auction injected by backend feed promotion injector.
  promotedAuction,

  /// P3A — Promoted external product injected by backend feed promotion injector.
  promotedExternal,
}

/// R3.1: Extension to add displayName to FeedItemType
extension FeedItemTypeExtension on FeedItemType {
  /// Get display name for UI
  String get displayName {
    switch (this) {
      case FeedItemType.content:
        return 'Content';
      case FeedItemType.promotedListing:
        return 'Dipromosikan';
      case FeedItemType.promotedAuction:
        return 'Dipromosikan';
      case FeedItemType.promotedExternal:
        return 'Dipromosikan';
    }
  }
}

/// Feed item entity
///
/// PROJECTION LAYER: UI view of Content entity for feed display.
/// This is NOT the source of truth - always resolve canonical Content for business logic.
///
/// Represents a single content item in the user's home feed.
/// Used across home and profile features for consistent feed display.
///
/// MEDIA INTEGRATION: Uses `List<MediaEntity>` instead of `List<String>` mediaUrls
/// Source of truth: canonical Content.media from content_media table
class FeedItem {
  final String id;
  final String content;
  final String authorId;
  final String? authorUsername;
  final String? authorAvatarUrl;
  final FeedItemType type;
  final DateTime createdAt;

  /// MEDIA INTEGRATION: Media from canonical Content.media
  /// Contract: Sourced from content_media table via backend Feed API
  final List<MediaEntity> media;

  final int likes;
  final int comments;
  final List<String> likedByUsers;
  final Map<String, dynamic> additionalData;

  /// Canonical governance lifecycle. Defaults to active for null/unknown
  /// wire values so legacy payloads keep rendering today's behavior.
  final ContentLifecycle lifecycle;

  /// E2.1 — Canonical governance lifecycle for the AUTHOR identity
  /// (PublicCard.UserCard.Lifecycle). Independent of [lifecycle]; the
  /// author may be unavailable/removed while the content itself is still
  /// active. Defaults to active for null / unknown / missing wire values
  /// so legacy payloads keep rendering today's behavior.
  ///
  /// Active feed surface only — non-feed surfaces (chat, comments,
  /// notifications, profile, search) still receive null and degrade to
  /// active per the same fallback.
  final ContentLifecycle authorLifecycle;

  /// FIX-3 — Canonical governance lifecycle for the ORIGINAL AUTHOR of a
  /// repost. Sourced from `original_author_lifecycle` on the feed wire.
  /// Only meaningful when [isRepost] is true. Defaults to active for null /
  /// unknown / missing wire values (legacy reposts degrade safely to current
  /// behavior). When degraded, attribution display shows a placeholder.
  final ContentLifecycle originalAuthorLifecycle;

  const FeedItem({
    required this.id,
    required this.content,
    required this.authorId,
    this.authorUsername,
    this.authorAvatarUrl,
    required this.type,
    required this.createdAt,
    this.media = const [],
    this.likes = 0,
    this.comments = 0,
    this.likedByUsers = const [],
    this.additionalData = const {},
    this.lifecycle = ContentLifecycle.active,
    this.authorLifecycle = ContentLifecycle.active,
    this.originalAuthorLifecycle = ContentLifecycle.active,
  });

  /// Get title from additional data
  String? get title => additionalData['title'] as String?;

  /// Get caption from additional data
  String? get caption => additionalData['caption'] as String?;

  /// Check if item is hidden
  bool get isHidden => additionalData['isHidden'] as bool? ?? false;

  /// Get status from additional data
  String? get status => additionalData['status'] as String?;

  /// Check if this is a repost
  bool get isRepost => additionalData['isRepost'] as bool? ?? false;

  /// Get original author ID for reposts
  String? get originalAuthorId => additionalData['originalAuthorId'] as String?;

  /// Check if item has media
  bool get hasMedia => media.isNotEmpty;

  /// Create a copy with modified fields
  FeedItem copyWith({
    String? id,
    String? content,
    String? authorId,
    String? authorUsername,
    String? authorAvatarUrl,
    FeedItemType? type,
    DateTime? createdAt,
    List<MediaEntity>? media,
    int? likes,
    int? comments,
    List<String>? likedByUsers,
    Map<String, dynamic>? additionalData,
    ContentLifecycle? lifecycle,
    ContentLifecycle? authorLifecycle,
    ContentLifecycle? originalAuthorLifecycle,
  }) {
    return FeedItem(
      id: id ?? this.id,
      content: content ?? this.content,
      authorId: authorId ?? this.authorId,
      authorUsername: authorUsername ?? this.authorUsername,
      authorAvatarUrl: authorAvatarUrl ?? this.authorAvatarUrl,
      type: type ?? this.type,
      createdAt: createdAt ?? this.createdAt,
      media: media ?? this.media,
      likes: likes ?? this.likes,
      comments: comments ?? this.comments,
      likedByUsers: likedByUsers ?? this.likedByUsers,
      additionalData: additionalData ?? this.additionalData,
      lifecycle: lifecycle ?? this.lifecycle,
      authorLifecycle: authorLifecycle ?? this.authorLifecycle,
      originalAuthorLifecycle:
          originalAuthorLifecycle ?? this.originalAuthorLifecycle,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is FeedItem && other.id == id;
  }

  @override
  int get hashCode => id.hashCode;

  @override
  String toString() {
    return 'FeedItem(id: $id, type: $type, authorId: $authorId, createdAt: $createdAt, mediaCount: ${media.length})';
  }
}
