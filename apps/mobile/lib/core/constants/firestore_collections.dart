/// Firestore Collections Constants
///
/// Domain-based naming convention untuk scalability dan clarity
/// Format: {domain}_{entity}_{sub-entity}
///
/// Last updated: 2025-01-16
/// Migration: Phase 1 - Domain-based restructure
library;

/// ============================================================================
/// AUTH DOMAIN - Authentication & User Management
/// ============================================================================
class AuthCollections {
  /// Core authentication users
  /// Stores: uid, email, emailVerified, provider, createdAt
  static const String users = 'auth_users';

  /// Reserved usernames for uniqueness
  /// Stores: userId, displayUsername, createdAt
  static const String reservedUsernames = 'auth_reserved_usernames';

  /// DEPRECATED - Old collection names (for migration)
  @Deprecated('Use AuthCollections.users instead')
  static const String usersOld = 'users';

  @Deprecated('Use AuthCollections.reservedUsernames instead')
  static const String usernamesOld = 'usernames';
}

/// ============================================================================
/// PROFILE DOMAIN - User Profiles & Personal Data
/// ============================================================================
class ProfileCollections {
  /// Public user profiles
  /// Stores: displayName, username, photoUrl, bio, userRole, stats
  static const String profiles = 'profile_public';

  /// Private user addresses
  /// Stores: street, district, city, province, postalCode, coordinates
  static const String addresses = 'profile_addresses';

  /// KYC verifications (Single Source of Truth for all identity verification)
  /// Stores: KTP + Selfie + Address + verification status
  /// Used by: Setting Profile, Upgrade Seller, Transaction verification
  static const String kycVerifications = 'profile_verifications_kyc';

  /// DEPRECATED - KTP verifications (merged into kycVerifications)
  /// Old separate KTP collection - now part of KYC entity
  @Deprecated(
    'Use ProfileCollections.kycVerifications instead - KTP is now part of KYC entity',
  )
  static const String ktpVerifications = 'profile_verifications_ktp';

  /// Business information (for sellers)
  /// Stores: businessName, businessType, businessAddress, taxId
  static const String businessInfo = 'profile_business_info';

  /// DEPRECATED - Old collection names (for migration)
  @Deprecated('Use ProfileCollections.profiles instead')
  static const String profilesOld = 'profiles';

  @Deprecated('Use ProfileCollections.addresses instead')
  static const String addressesOld = 'user_addresses';

  @Deprecated('Use ProfileCollections.ktpVerifications instead')
  static const String ktpVerificationsOld = 'ktp_verifications';

  @Deprecated('Use ProfileCollections.kycVerifications instead')
  static const String kycVerificationsOld = 'kyc_verifications';
}

/// ============================================================================
/// LISTING DOMAIN - Marketplace Listings (Canonical)
/// ============================================================================
/// ⚠️ CANONICAL LISTING CONTRACT V1 (2026-03-31) ⚠️
///
/// The "collection" legacy domain has been completely replaced by "listing".
/// All listing data is now managed through the backend Listing API.
/// Flutter app no longer directly accesses Firestore for listings.
///
/// Backend API endpoint: /api/v1/listings
/// Access via: ListingRemoteDatasource
class ListingCollections {
  /// NOTE: Direct Firestore access is DEPRECATED.
  /// Use backend Listing API via ListingRemoteDatasource instead.
  /// The "listings" collection in Firestore is being phased out.
  @Deprecated('Use backend Listing API via ListingRemoteDatasource')
  static const String listings = 'listings';
}

/// ============================================================================
/// AUCTION DOMAIN - Auction Listings & Bidding
/// ============================================================================
class AuctionCollections {
  /// Auction listings
  /// Stores: title, description, koiDetails, openingBid, currentBid, status
  static const String auctions = 'auctions';

  /// Bids on auctions
  /// Stores: auctionId, bidderId, amount, bidTime, status
  static const String bids = 'auction_bids';

  /// Auction views analytics
  /// Stores: auctionId, viewerId, viewedAt
  static const String views = 'auction_views';

  /// Auction watchlist (per user)
  /// Stores: auctionIds array, lastUpdated
  static const String watchlist = 'auction_watchlist';
}

/// ============================================================================
/// CHAT DOMAIN - Messaging & Communication
/// ============================================================================
class ChatCollections {
  /// Chat conversations
  /// Stores: participants, lastMessage, lastMessageTime, unreadCount
  static const String conversations = 'chat_conversations';

  /// Chat messages (subcollection under conversations)
  /// Stores: senderId, text, mediaUrl, sentAt, readAt
  static const String messages = 'messages';

  /// Blocked users (per user)
  /// Stores: blockedUserIds array, lastUpdated
  static const String blockedUsers = 'chat_blocked_users';

  /// User presence/online status
  /// Stores: userId, status, lastSeen
  static const String presence = 'chat_presence';

  /// DEPRECATED - Old collection names (for migration)
  @Deprecated('Use ChatCollections.blockedUsers instead')
  static const String blockedUsersOld = 'blocked_users';
}

/// ============================================================================
/// FOLLOW DOMAIN - Social Connections
/// ============================================================================
class FollowCollections {
  /// Follow relationships
  /// Stores: followerId, followingId, createdAt
  static const String follows = 'follows';

  /// Follower counts cache
  /// Stores: userId, followerCount, followingCount, lastUpdated
  static const String stats = 'follow_stats';
}

/// ============================================================================
/// NOTIFICATION DOMAIN - User Notifications
/// ============================================================================
class NotificationCollections {
  /// User notifications
  /// Stores: userId, type, title, message, data, readAt, createdAt
  static const String notifications = 'notifications';

  /// FCM tokens for push notifications
  /// Stores: token, updatedAt, badgeCount
  static const String fcmTokens = 'fcm_tokens';

  /// Push notification tokens (deprecated, use fcmTokens)
  /// Stores: userId, token, platform, deviceId, lastUpdated
  @Deprecated('Use NotificationCollections.fcmTokens instead')
  static const String tokens = 'notification_tokens';

  /// User notification preferences
  /// Stores: pushEnabled, orderNotifications, chatNotifications, securityAlerts
  static const String preferences = 'notification_preferences';
}

/// ============================================================================
/// REQUEST DOMAIN - User Requests & Support
/// ============================================================================
class RequestCollections {
  /// User requests (general)
  /// Stores: userId, type, status, description, createdAt
  static const String requests = 'requests';

  /// Support tickets
  /// Stores: userId, subject, status, priority, createdAt
  static const String supportTickets = 'support_tickets';
}

/// ============================================================================
/// ANALYTICS DOMAIN - App Analytics & Metrics
/// ============================================================================
class AnalyticsCollections {
  /// Profile view analytics
  /// Stores: profileId, viewerId, viewedAt, source
  static const String profileViews = 'analytics_profile_views';

  /// Search queries
  /// Stores: query, userId, resultCount, timestamp
  static const String searchQueries = 'analytics_search_queries';

  /// User activity logs
  /// Stores: userId, action, metadata, timestamp
  static const String activityLogs = 'analytics_activity_logs';

  /// DEPRECATED - Old collection names (for migration)
  @Deprecated('Use AnalyticsCollections.profileViews instead')
  static const String profileViewsOld = 'profile_views';
}

/// ============================================================================
/// ADMIN DOMAIN - Admin Operations & Audit
/// ============================================================================
class AdminCollections {
  /// Admin audit logs
  /// Stores: adminId, action, targetType, targetId, metadata, timestamp
  static const String auditLogs = 'admin_audit_logs';

  /// Platform metrics cache
  /// Stores: date, userCount, orderCount, revenue, activeUsers
  static const String platformMetrics = 'admin_platform_metrics';

  /// System configuration
  /// Stores: key, value, description, updatedBy, updatedAt
  static const String systemConfig = 'admin_system_config';
}

/// ============================================================================
/// HELPER CLASS - Migration Support
/// ============================================================================
class FirestoreCollectionMigration {
  /// Get mapping of old collection names to new ones
  static Map<String, String> get migrationMap => {
    // Auth
    'users': AuthCollections.users,
    'usernames': AuthCollections.reservedUsernames,

    // Profile
    'profiles': ProfileCollections.profiles,
    'user_addresses': ProfileCollections.addresses,
    'ktp_verifications': ProfileCollections.kycVerifications,
    'kyc_verifications': ProfileCollections.kycVerifications,

    'collection_views': 'listing_views', // Backend now manages
    'collection_reviews': 'listing_reviews', // Backend now manages
    // Chat
    'blocked_users': ChatCollections.blockedUsers,

    // Analytics
    'profile_views': AnalyticsCollections.profileViews,
  };

  /// Check if collection name is deprecated
  static bool isDeprecated(String collectionName) {
    return migrationMap.containsKey(collectionName);
  }

  /// Get new collection name from old one
  static String? getNewName(String oldName) {
    return migrationMap[oldName];
  }
}
