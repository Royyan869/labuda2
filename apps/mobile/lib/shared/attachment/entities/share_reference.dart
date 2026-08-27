library;

/// SHARE CONTRACT ALIGNMENT V1
///
/// Share = Reference Distribution Layer
/// - Share does NOT copy objects
/// - Share does NOT mutate source
/// - Share creates a reference with cached preview data
///
/// Reference Types:
/// - content: Universal social content
/// - for_sale: Fixed-price sale surfaces
/// - auction: Active auctions
/// - profile: User profiles
///
/// SEMANTIC RULES:
/// - targetType + targetId is the CANONICAL reference
/// - Preview data (title, imageUrl) is CACHE for UI only
/// - Business logic MUST resolve through backend using canonical IDs
/// - Preview data can be stale - never use for business decisions

import 'package:equatable/equatable.dart';
import 'package:labuda/shared/object/object_preview.dart' as obj;

// ============================================================================
// Enums
// ============================================================================

/// Types of entities that can be shared
enum ShareTargetType {
  content('content', 'content', '/content', 'contents', 'Content'),
  forSale('for_sale', 'for_sale', '/for-sale', 'for-sales', 'Produk Dijual'),
  auction('auction', 'auction', '/auction', 'auctions', 'Lelang'),
  profile('profile', 'profile', '/user', 'users', 'Profil');

  const ShareTargetType(
    this.wireValue,
    this.objectType,
    this.navigationPath,
    this.apiPath,
    this.displayName,
  );

  final String wireValue;
  final String objectType;
  final String navigationPath;
  final String apiPath;
  final String displayName;

  static ShareTargetType? fromString(String value) {
    for (final type in ShareTargetType.values) {
      if (type.wireValue == value) {
        return type;
      }
    }
    return null;
  }
}

// ============================================================================
// Value Objects
// ============================================================================

// ============================================================================
// Main Entity
// ============================================================================

/// ShareReference represents a reference to shared content.
/// It contains the canonical reference and cached preview data.
///
/// CONTRACT:
/// - targetType + targetId is the CANONICAL reference to the entity
/// - Use targetId for all backend queries and business logic
/// - Preview data is cached for UI display only and may be stale
///
/// **SEPARATION CLEANUP:** ShareReference is now independent from Attachment system.
/// It's a pure transport object for cross-domain references, not part of the legacy
/// Attachment wrapper hierarchy.
class ShareReference extends Equatable {
  /// The type of entity being referenced
  final ShareTargetType targetType;

  /// The canonical ID of the entity being referenced
  /// USE THIS for all backend queries
  final String targetId;

  /// Wire-level target type used when serializing for API/chat transport.
  /// Defaults to [targetType.wireValue], but chat content shares can override this
  /// to an older content wire value without changing the logical domain type.
  final String wireTargetType;

  /// Cached preview data for UI display
  /// DO NOT use for business decisions
  final obj.ObjectPreview preview;

  ShareReference({
    required this.targetType,
    required this.targetId,
    String? wireTargetType,
    required this.preview,
  }) : wireTargetType = wireTargetType ?? targetType.wireValue;

  // ============================================================================
  // Factory Constructors
  // ============================================================================

  /// Factory to create a share reference for content
  factory ShareReference.content({
    required String contentId,
    required String title,
    String? imageUrl,
    String? wireTargetType,
    bool isAvailable = true,
    bool isSold = false,
    bool isClosed = false,
    bool isDeleted = false,
  }) {
    return ShareReference(
      targetType: ShareTargetType.content,
      targetId: contentId,
      wireTargetType: wireTargetType,
      preview: obj.ObjectPreview(
        id: contentId,
        type: 'content',
        title: title,
        imageUrl: imageUrl,
        status: isAvailable ? 'available' : 'unavailable',
        isAvailable: isAvailable,
        isSold: isSold,
        isClosed: isClosed,
        isDeleted: isDeleted,
      ),
    );
  }

  /// Factory to create a share reference for a fixed-price sale
  factory ShareReference.forSale({
    required String forSaleId,
    required String title,
    String? imageUrl,
    String? wireTargetType,
    bool isAvailable = true,
    bool isSold = false,
    bool isClosed = false,
    bool isDeleted = false,
  }) {
    return ShareReference(
      targetType: ShareTargetType.forSale,
      targetId: forSaleId,
      wireTargetType: wireTargetType,
      preview: obj.ObjectPreview(
        id: forSaleId,
        type: ShareTargetType.forSale.objectType,
        title: title,
        imageUrl: imageUrl,
        status: isSold ? 'sold' : (isAvailable ? 'available' : 'unavailable'),
        isAvailable: isAvailable,
        isSold: isSold,
        isClosed: isClosed,
        isDeleted: isDeleted,
      ),
    );
  }

  /// Factory to create a share reference for auction
  factory ShareReference.auction({
    required String auctionId,
    required String title,
    String? imageUrl,
    String? wireTargetType,
    bool isAvailable = true,
    bool isSold = false,
    bool isClosed = false,
    bool isDeleted = false,
  }) {
    return ShareReference(
      targetType: ShareTargetType.auction,
      targetId: auctionId,
      wireTargetType: wireTargetType,
      preview: obj.ObjectPreview(
        id: auctionId,
        type: 'auction',
        title: title,
        imageUrl: imageUrl,
        status: isClosed ? 'ended' : (isAvailable ? 'active' : 'unavailable'),
        isAvailable: isAvailable,
        isSold: isSold,
        isClosed: isClosed,
        isDeleted: isDeleted,
      ),
    );
  }

  /// Factory to create a share reference for profile
  factory ShareReference.profile({
    required String profileId,
    required String name,
    String? avatarUrl,
    String? wireTargetType,
    bool isAvailable = true,
    bool isSold = false,
    bool isClosed = false,
    bool isDeleted = false,
  }) {
    return ShareReference(
      targetType: ShareTargetType.profile,
      targetId: profileId,
      wireTargetType: wireTargetType,
      preview: obj.ObjectPreview(
        id: profileId,
        type: 'profile',
        title: name,
        imageUrl: avatarUrl,
        status: isAvailable ? 'available' : 'unavailable',
        isAvailable: isAvailable,
        isSold: isSold,
        isClosed: isClosed,
        isDeleted: isDeleted,
      ),
    );
  }

  // ============================================================================
  // Business Logic
  // ============================================================================

  /// Check if this share reference has a valid target ID
  bool get isValid => targetId.isNotEmpty;

  /// Check if this share reference has an image in preview
  bool get hasImage => preview.imageUrl != null && preview.imageUrl!.isNotEmpty;

  /// Get the internal navigation path for this share reference.
  String get navigationPath => '${targetType.navigationPath}/$targetId';

  /// Get the object preview type used by the live preview resolvers.
  String get objectType => targetType.objectType;

  /// Get the API path for this share reference
  String get apiPath => '${targetType.apiPath}/$targetId';

  /// Wire target type that is safe to emit into chat payloads.
  ///
  /// Canonical content references emit `content` directly.
  String? get chatWireTargetType {
    if (targetType != ShareTargetType.content) {
      return wireTargetType;
    }

    return wireTargetType == 'content' ? wireTargetType : null;
  }

  /// Returns a ShareReference normalized for chat transport, or null when
  /// the reference is not valid for chat payloads.
  ShareReference? asChatReference() {
    final chatWireType = chatWireTargetType;
    if (chatWireType == null) {
      return null;
    }
    if (chatWireType == wireTargetType) {
      return this;
    }
    return copyWith(wireTargetType: chatWireType);
  }

  /// Get the display name for this share reference
  String get displayName => targetType.displayName;

  @override
  List<Object?> get props => [targetType, targetId, wireTargetType, preview];

  ShareReference copyWith({
    ShareTargetType? targetType,
    String? targetId,
    String? wireTargetType,
    obj.ObjectPreview? preview,
  }) {
    return ShareReference(
      targetType: targetType ?? this.targetType,
      targetId: targetId ?? this.targetId,
      wireTargetType: wireTargetType ?? this.wireTargetType,
      preview: preview ?? this.preview,
    );
  }

  /// Convert to JSON for API requests
  Map<String, dynamic> toJson() {
    return {
      'targetType': wireTargetType,
      'targetId': targetId,
      'preview': {
        'title': preview.title,
        'imageUrl': preview.imageUrl,
        'isAvailable': preview.isAvailable,
        'isSold': preview.isSold,
        'isClosed': preview.isClosed,
        'isDeleted': preview.isDeleted,
      },
    };
  }

  /// Create from JSON
  ///
  /// ZERO LEGACY MODE: Invalid targetType will fail - no fallback
  factory ShareReference.fromJson(Map<String, dynamic> json) {
    final wireTargetType = json['targetType'] as String;
    final targetType = _logicalTargetTypeFromWire(wireTargetType);
    final targetId = json['targetId'] as String;
    final previewJson = json['preview'] as Map<String, dynamic>?;
    final isAvailable = previewJson?['isAvailable'] as bool? ?? true;
    final isSold = previewJson?['isSold'] as bool? ?? false;
    final isClosed = previewJson?['isClosed'] as bool? ?? false;
    final isDeleted = previewJson?['isDeleted'] as bool? ?? false;

    // Determine status from flags
    String status;
    switch (targetType) {
      case ShareTargetType.forSale:
        status = isSold ? 'sold' : (isAvailable ? 'available' : 'unavailable');
        break;
      case ShareTargetType.auction:
        status = isClosed ? 'ended' : (isAvailable ? 'active' : 'unavailable');
        break;
      case ShareTargetType.content:
      case ShareTargetType.profile:
        status = isAvailable ? 'available' : 'unavailable';
        break;
    }

    return ShareReference(
      targetType: targetType,
      targetId: targetId,
      wireTargetType: wireTargetType,
      preview: obj.ObjectPreview(
        id: targetId,
        type: targetType.objectType,
        title: previewJson?['title'] ?? '',
        imageUrl: previewJson?['imageUrl'] as String?,
        status: status,
        isAvailable: isAvailable,
        isSold: isSold,
        isClosed: isClosed,
        isDeleted: isDeleted,
      ),
    );
  }
}

ShareTargetType _logicalTargetTypeFromWire(String wireTargetType) {
  final parsed = ShareTargetType.fromString(wireTargetType);
  if (parsed != null) {
    return parsed;
  }
  throw FormatException('Invalid share target type: $wireTargetType');
}
