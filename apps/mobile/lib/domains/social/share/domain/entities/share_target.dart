/// Base URL for public share links other than profiles.
const String kPublicShareBaseUrl = 'https://labuda-79de2.web.app';

/// Public web base for profile sharing and QR codes.
const String kPublicProfileBaseUrl = 'https://labuda.app';

/// Entity representing content that can be shared externally
/// Domain entity - pure, no Flutter dependencies
///
/// This is for generating share links, rich previews, and external sharing
/// (e.g., to WhatsApp, Telegram). NOT for internal entity references.
class ShareTarget {
  final String id;
  final ExternalShareType type;
  final String title;
  final String description;
  final String? imageUrl;
  final String? _publicShareUrl;
  final Map<String, dynamic> metadata;

  const ShareTarget({
    required this.id,
    required this.type,
    required this.title,
    required this.description,
    this.imageUrl,
    String? publicShareUrl,
    this.metadata = const {},
  }) : _publicShareUrl = publicShareUrl;

  /// Generate share text for native sharing.
  String get shareText {
    final buffer = StringBuffer();
    buffer.writeln(title);
    if (description.isNotEmpty) {
      buffer.writeln();
      buffer.writeln(description);
    }
    buffer.writeln();
    buffer.writeln('Lihat selengkapnya di LABUDA App:');
    buffer.writeln(publicShareUrl);
    return buffer.toString();
  }

  /// Generate public share URL for external sharing.
  String generatePublicShareUrl([String? baseUrl]) {
    final base = baseUrl ?? kPublicShareBaseUrl;
    switch (type) {
      case ExternalShareType.post:
        return '$base/content/$id';
      case ExternalShareType.listing:
        return '$base/for-sale/$id';
      case ExternalShareType.request:
        return '$base/content/$id';
      case ExternalShareType.auction:
        return '$base/auction/$id';
      case ExternalShareType.profile:
        return '${baseUrl ?? kPublicProfileBaseUrl}/profile/$id';
    }
  }

  /// Public URL used in native share, copy-link, and QR flows.
  String get publicShareUrl => _publicShareUrl ?? generatePublicShareUrl();

  /// Copy with modified fields
  ShareTarget copyWith({
    String? id,
    ExternalShareType? type,
    String? title,
    String? description,
    String? imageUrl,
    String? publicShareUrl,
    Map<String, dynamic>? metadata,
  }) {
    return ShareTarget(
      id: id ?? this.id,
      type: type ?? this.type,
      title: title ?? this.title,
      description: description ?? this.description,
      imageUrl: imageUrl ?? this.imageUrl,
      publicShareUrl: publicShareUrl ?? _publicShareUrl,
      metadata: metadata ?? this.metadata,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ShareTarget &&
          runtimeType == other.runtimeType &&
          id == other.id &&
          type == other.type;

  @override
  int get hashCode => id.hashCode ^ type.hashCode;
}

/// Types of content that can be shared externally
/// Used for generating share links and rich previews
enum ExternalShareType { post, listing, request, auction, profile }

/// Extension for ExternalShareType helpers
extension ExternalShareTypeExtension on ExternalShareType {
  /// Canonical chat wire-type for [ChatResourceOccurrenceResourceType.fromWire].
  /// Maps every [ExternalShareType] to the chat resource wire vocabulary
  /// (profile/content/for_sale/auction).
  String get wireTargetType {
    switch (this) {
      case ExternalShareType.post:
        return 'content';
      case ExternalShareType.listing:
        return 'for_sale';
      case ExternalShareType.request:
        return 'content';
      case ExternalShareType.auction:
        return 'auction';
      case ExternalShareType.profile:
        return 'profile';
    }
  }

  String get displayName {
    switch (this) {
      case ExternalShareType.post:
        return 'Post';
      case ExternalShareType.listing:
        return 'Produk';
      case ExternalShareType.request:
        return 'Request';
      case ExternalShareType.auction:
        return 'Lelang';
      case ExternalShareType.profile:
        return 'Profil';
    }
  }

  String get collectionName {
    switch (this) {
      case ExternalShareType.post:
        return 'contents'; // Unified content collection
      case ExternalShareType.listing:
        return 'listings';
      case ExternalShareType.request:
        return 'contents'; // Requests are now stored in contents collection
      case ExternalShareType.auction:
        return 'auctions';
      case ExternalShareType.profile:
        return 'users';
    }
  }
}
