/// Represents where content can be shared to
/// Domain entity - pure, minimal Flutter dependencies (only IconData for UI)
/// This could be moved to presentation layer if needed
class ShareDestination {
  final ShareDestinationType type;
  final String label;
  final String iconName; // Icon name instead of IconData to keep domain pure
  final String? colorHex; // Color as hex string instead of Color
  final bool isInternal; // Internal (LABUDA) vs External (WhatsApp, etc)

  const ShareDestination({
    required this.type,
    required this.label,
    required this.iconName,
    this.colorHex,
    this.isInternal = false,
  });

  // Predefined destinations
  static const whatsapp = ShareDestination(
    type: ShareDestinationType.whatsapp,
    label: 'WhatsApp',
    iconName: 'chat',
    colorHex: '#25D366',
  );

  static const instagram = ShareDestination(
    type: ShareDestinationType.instagram,
    label: 'Instagram Story',
    iconName: 'camera_alt',
    colorHex: '#E4405F',
  );

  static const telegram = ShareDestination(
    type: ShareDestinationType.telegram,
    label: 'Telegram',
    iconName: 'send',
    colorHex: '#0088CC',
  );

  static const copyLink = ShareDestination(
    type: ShareDestinationType.copyLink,
    label: 'Copy Link',
    iconName: 'link',
  );

  static const email = ShareDestination(
    type: ShareDestinationType.email,
    label: 'Email',
    iconName: 'email',
  );

  static const shareToFeed = ShareDestination(
    type: ShareDestinationType.shareToFeed,
    label: 'Share to Feed',
    iconName: 'home',
    isInternal: true,
  );

  /// Send to Chat - NOT IMPLEMENTED
  ///
  /// PHASE 2 HARDENING: This feature is not implemented.
  /// Repository returns "Coming soon" error.
  /// Kept for potential future implementation.
  static const sendToChat = ShareDestination(
    type: ShareDestinationType.sendToChat,
    label: 'Send to Chat',
    iconName: 'message',
    isInternal: true,
  );

  static const more = ShareDestination(
    type: ShareDestinationType.more,
    label: 'More',
    iconName: 'more_horiz',
  );

  /// Get all external destinations
  static List<ShareDestination> get externalDestinations => [
    whatsapp,
    instagram,
    telegram,
    copyLink,
    email,
    more,
  ];

  /// Get all internal destinations
  ///
  /// PHASE 2 HARDENING: Only includes destinations that actually work.
  /// - sendToChat: Not implemented (returns "Coming soon")
  static List<ShareDestination> get internalDestinations => [shareToFeed];

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ShareDestination &&
          runtimeType == other.runtimeType &&
          type == other.type;

  @override
  int get hashCode => type.hashCode;
}

/// Types of share destinations
enum ShareDestinationType {
  whatsapp,
  instagram,
  telegram,
  copyLink,
  email,
  shareToFeed,
  sendToChat,
  more, // System share dialog
}
