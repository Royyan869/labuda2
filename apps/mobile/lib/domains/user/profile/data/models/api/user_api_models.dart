// User API Models for Go Backend Integration
//
// These models match the Go backend DTOs in:
// backend/internal/domain/user/dto/user_dto.go

import 'package:equatable/equatable.dart';
import 'package:labuda/shared/shared.dart';

/// Response from POST /api/v1/auth/firebase/exchange when the profile is incomplete.
class FirebaseExchangeIncompleteResponse {
  final String userId;
  final String accessToken;
  final String expiresAt;
  final bool requiresProfileCompletion;
  final String? email;

  const FirebaseExchangeIncompleteResponse({
    required this.userId,
    required this.accessToken,
    required this.expiresAt,
    required this.requiresProfileCompletion,
    this.email,
  });

  factory FirebaseExchangeIncompleteResponse.fromJson(
    Map<String, dynamic> json,
  ) {
    return FirebaseExchangeIncompleteResponse(
      userId: json['user_id'] as String? ?? '',
      accessToken: json['access_token'] as String? ?? '',
      expiresAt: json['expires_at'] as String? ?? '',
      requiresProfileCompletion:
          json['requires_profile_completion'] as bool? ?? false,
      email: json['email'] as String?,
    );
  }
}

/// Response from POST /api/v1/auth/firebase/exchange and
/// POST /api/v1/auth/complete-profile when the session is complete.
class FirebaseExchangeCompleteResponse {
  final String userId;
  final String accessToken;
  final String refreshToken;
  final String expiresAt;
  final String refreshExpiresAt;
  final bool requiresProfileCompletion;
  final bool created;

  const FirebaseExchangeCompleteResponse({
    required this.userId,
    required this.accessToken,
    required this.refreshToken,
    required this.expiresAt,
    required this.refreshExpiresAt,
    required this.requiresProfileCompletion,
    required this.created,
  });

  factory FirebaseExchangeCompleteResponse.fromJson(Map<String, dynamic> json) {
    return FirebaseExchangeCompleteResponse(
      userId: json['user_id'] as String? ?? '',
      accessToken: json['access_token'] as String? ?? '',
      refreshToken: json['refresh_token'] as String? ?? '',
      expiresAt: json['expires_at'] as String? ?? '',
      refreshExpiresAt: json['refresh_expires_at'] as String? ?? '',
      requiresProfileCompletion:
          json['requires_profile_completion'] as bool? ?? false,
      created: json['created'] as bool? ?? false,
    );
  }
}

/// Unified response from POST /api/v1/auth/firebase/exchange.
///
/// The backend returns either an incomplete-session payload (restricted
/// completion token only) or a complete-session payload (access + refresh).
class FirebaseExchangeResponse {
  final String userId;
  final String accessToken;
  final String? refreshToken;
  final String expiresAt;
  final String? refreshExpiresAt;
  final bool requiresProfileCompletion;
  final bool created;
  final String? email;

  const FirebaseExchangeResponse({
    required this.userId,
    required this.accessToken,
    required this.expiresAt,
    required this.requiresProfileCompletion,
    required this.created,
    this.refreshToken,
    this.refreshExpiresAt,
    this.email,
  });

  bool get isComplete =>
      refreshToken != null &&
      refreshToken!.isNotEmpty &&
      refreshExpiresAt != null;

  factory FirebaseExchangeResponse.fromJson(Map<String, dynamic> json) {
    final refreshToken = json['refresh_token'] as String?;
    final refreshExpiresAt = json['refresh_expires_at'] as String?;
    final requiresProfileCompletion =
        json['requires_profile_completion'] as bool? ?? false;
    final created = json['created'] as bool? ?? false;
    return FirebaseExchangeResponse(
      userId: json['user_id'] as String? ?? '',
      accessToken: json['access_token'] as String? ?? '',
      refreshToken: refreshToken,
      expiresAt: json['expires_at'] as String? ?? '',
      refreshExpiresAt: refreshExpiresAt,
      requiresProfileCompletion: requiresProfileCompletion,
      created: created,
      email: json['email'] as String?,
    );
  }

  FirebaseExchangeCompleteResponse? get asCompleteResponse {
    if (!isComplete || refreshExpiresAt == null) {
      return null;
    }
    return FirebaseExchangeCompleteResponse(
      userId: userId,
      accessToken: accessToken,
      refreshToken: refreshToken!,
      expiresAt: expiresAt,
      refreshExpiresAt: refreshExpiresAt!,
      requiresProfileCompletion: requiresProfileCompletion,
      created: created,
    );
  }
}

/// Response from POST /api/v1/auth/refresh
///
/// Returned by the backend when a platform refresh token is rotated.
/// Both access_token and refresh_token are new single-use tokens.
/// Mobile must persist both and discard the old refresh token.
class BackendRefreshResponse {
  final String accessToken;
  final String refreshToken; // new single-use rotation token
  final String expiresAt;
  final String refreshExpiresAt;

  const BackendRefreshResponse({
    required this.accessToken,
    required this.refreshToken,
    required this.expiresAt,
    required this.refreshExpiresAt,
  });

  factory BackendRefreshResponse.fromJson(Map<String, dynamic> json) {
    return BackendRefreshResponse(
      accessToken: json['access_token'] as String? ?? '',
      refreshToken: json['refresh_token'] as String? ?? '',
      expiresAt: json['expires_at'] as String? ?? '',
      refreshExpiresAt: json['refresh_expires_at'] as String? ?? '',
    );
  }
}

/// Single active session entry from GET /api/v1/auth/sessions.
///
/// Maps directly to the safe fields emitted by mapSessionDeviceSummaries.
/// Sensitive fields (token_hash, jti, ip_hash, raw token) are never present.
class AuthSessionDto {
  final String familyId;
  final String? deviceId;
  final String? deviceName;
  final String? platform;
  final String? appVersion;
  final DateTime issuedAt;
  final DateTime expiresAt;
  final DateTime? lastUsedAt;
  final bool? fcmTokenActive;

  const AuthSessionDto({
    required this.familyId,
    this.deviceId,
    this.deviceName,
    this.platform,
    this.appVersion,
    required this.issuedAt,
    required this.expiresAt,
    this.lastUsedAt,
    this.fcmTokenActive,
  });

  factory AuthSessionDto.fromJson(Map<String, dynamic> json) {
    return AuthSessionDto(
      familyId: json['family_id'] as String? ?? '',
      deviceId: json['device_id'] as String?,
      deviceName: json['device_name'] as String?,
      platform: json['platform'] as String?,
      appVersion: json['app_version'] as String?,
      issuedAt: _parseDateTime(json['issued_at']) ?? DateTime.now(),
      expiresAt: _parseDateTime(json['expires_at']) ?? DateTime.now(),
      lastUsedAt: _parseDateTime(json['last_used_at']),
      fcmTokenActive: json['fcm_token_active'] as bool?,
    );
  }

  static DateTime? _parseDateTime(dynamic v) {
    if (v == null) return null;
    try {
      return DateTime.parse(v.toString());
    } catch (_) {
      return null;
    }
  }
}

/// Request to update user profile
class UpdateProfileApiRequest {
  final String? bio;
  final String? avatarUrl;
  final DateTime? dateOfBirth;
  final String? gender;
  final String? location;
  final String? preferredLang;

  // Social media handles
  final String? instagramHandle;
  final String? facebookHandle;
  final String? twitterHandle;
  final String? tiktokHandle;
  final String? youtubeHandle;
  final String? websiteUrl;

  /// Canonical cover photo reference. Persisted as a STORAGE KEY
  /// (images/profile-covers/{userId}.jpg); an empty string clears the cover.
  final String? coverPhotoUrl;

  // Privacy settings
  final String? visibility;
  final bool? showPhoneNumber;
  final bool? showEmail;
  final bool? showLocation;
  final String? allowMessagesFrom;
  final bool? allowTagging;
  final bool? showActivityStatus;
  final bool? showTransactionCount;

  const UpdateProfileApiRequest({
    this.bio,
    this.avatarUrl,
    this.coverPhotoUrl,
    this.dateOfBirth,
    this.gender,
    this.location,
    this.preferredLang,
    this.instagramHandle,
    this.facebookHandle,
    this.twitterHandle,
    this.tiktokHandle,
    this.youtubeHandle,
    this.websiteUrl,
    this.visibility,
    this.showPhoneNumber,
    this.showEmail,
    this.showLocation,
    this.allowMessagesFrom,
    this.allowTagging,
    this.showActivityStatus,
    this.showTransactionCount,
  });

  Map<String, dynamic> toJson() {
    final map = <String, dynamic>{};

    if (bio != null) {
      map['bio'] = bio;
    }
    if (avatarUrl != null) {
      map['avatar_url'] = avatarUrl;
    }
    if (coverPhotoUrl != null) {
      // Empty string is the canonical clear signal (backend → NULL).
      map['cover_photo_url'] = coverPhotoUrl;
    }
    if (dateOfBirth != null) {
      map['date_of_birth'] = dateOfBirth!.toIso8601String();
    }
    if (gender != null) {
      map['gender'] = gender;
    }
    if (location != null) {
      map['location'] = location;
    }
    if (preferredLang != null) {
      map['preferred_lang'] = preferredLang;
    }

    if (instagramHandle != null) {
      map['instagram_handle'] = instagramHandle;
    }
    if (facebookHandle != null) {
      map['facebook_handle'] = facebookHandle;
    }
    if (twitterHandle != null) {
      map['twitter_handle'] = twitterHandle;
    }
    if (tiktokHandle != null) {
      map['tiktok_handle'] = tiktokHandle;
    }
    if (youtubeHandle != null) {
      map['youtube_handle'] = youtubeHandle;
    }
    if (websiteUrl != null) {
      map['website_url'] = websiteUrl;
    }

    if (visibility != null) {
      map['visibility'] = visibility;
    }
    if (showPhoneNumber != null) {
      map['show_phone_number'] = showPhoneNumber;
    }
    if (showEmail != null) {
      map['show_email'] = showEmail;
    }
    if (showLocation != null) {
      map['show_location'] = showLocation;
    }
    if (allowMessagesFrom != null) {
      map['allow_messages_from'] = allowMessagesFrom;
    }
    if (allowTagging != null) {
      map['allow_tagging'] = allowTagging;
    }
    if (showActivityStatus != null) {
      map['show_activity_status'] = showActivityStatus;
    }
    if (showTransactionCount != null) {
      map['show_transaction_count'] = showTransactionCount;
    }

    return map;
  }
}

/// User response from Go backend
class UserApiResponse extends Equatable {
  final String id;
  final String email;
  final String? phoneNumber;
  final String?
  username; // Username comes from the backend user profile, not Firebase display data.
  final String accountStatus;
  final List<String> roles; // Roles from PostgreSQL (Single Source of Truth)
  // SellerBadge removed in PHASE 1A remediation - use sellerTier instead
  final String?
  sellerTier; // Canonical seller performance tier (basic, pro, elite)

  // === Seller State (BACKEND AUTHORITY) ===
  // These fields provide honest seller state from backend - no guessing from role
  final bool?
  hasSellerProfile; // Has created a seller profile (workspace identity)
  final String? sellerSubscriptionStatus; // active, expired, none
  final bool?
  hasMarketAuthority; // MARKET authority: has profile + active subscription

  // === Penalty Points (BACKEND AUTHORITY) ===
  final int? totalPenaltyPoints;
  final int? activePenaltyPoints;

  // === Verification Flags (BACKEND AUTHORITY) ===
  final bool? isPhoneVerified;
  final bool? isEmailVerified;
  final bool? isIdVerified;
  final bool? isFarmVerified;
  final DateTime? phoneVerifiedAt;
  final DateTime? emailVerifiedAt;
  final DateTime? idVerifiedAt;
  final DateTime? farmVerifiedAt;

  final DateTime createdAt;
  final DateTime updatedAt;
  final String? location;
  final UserProfileApiResponse? profile;

  const UserApiResponse({
    required this.id,
    required this.email,
    this.phoneNumber,
    this.username, // Username comes from the backend user profile, not Firebase display data.
    required this.accountStatus,
    required this.roles,
    this.sellerTier,
    this.hasSellerProfile,
    this.sellerSubscriptionStatus,
    this.hasMarketAuthority,
    this.totalPenaltyPoints,
    this.activePenaltyPoints,
    this.isPhoneVerified,
    this.isEmailVerified,
    this.isIdVerified,
    this.isFarmVerified,
    this.phoneVerifiedAt,
    this.emailVerifiedAt,
    this.idVerifiedAt,
    this.farmVerifiedAt,
    required this.createdAt,
    required this.updatedAt,
    this.location,
    this.profile,
  });

  factory UserApiResponse.fromJson(Map<String, dynamic> json) {
    LoggerService.instance.debug('UserApiResponse.fromJson - parsing');

    // Helper function for safe string parsing
    String safeString(String key, {String defaultValue = ''}) {
      final value = json[key];
      if (value == null) {
        LoggerService.instance.warning(
          'Field "$key" is null, using default: "$defaultValue"',
        );
        return defaultValue;
      }
      return value.toString();
    }

    // Helper function for safe DateTime parsing
    DateTime? safeDateTime(String key) {
      final value = json[key];
      if (value == null) return null;
      try {
        return DateTime.parse(value.toString());
      } catch (e) {
        LoggerService.instance.warning(
          'Failed to parse DateTime for "$key": $value',
        );
        return null;
      }
    }

    // Parse required fields with defaults.
    // Only `id` is hard-required; email remains best-effort because the
    // backend can omit it for phone-only identities.
    final id = safeString('id');
    final email = safeString('email');

    if (id.isEmpty) {
      LoggerService.instance.error(
        'UserApiResponse: "id" is empty — raw keys: ${json.keys.toList()}',
      );
      throw Exception('Required field "id" is missing in UserApiResponse');
    }

    if (email.isEmpty) {
      LoggerService.instance.warning(
        'UserApiResponse: "email" is empty or null (id=$id)',
      );
    }

    // Parse roles from backend (array of strings)
    List<String> parsedRoles = ['buyer']; // Default role
    if (json['roles'] != null) {
      if (json['roles'] is List) {
        parsedRoles = (json['roles'] as List).map((e) => e.toString()).toList();
      } else if (json['roles'] is String) {
        parsedRoles = [json['roles'] as String];
      }
    }

    final parsed = UserApiResponse(
      id: id,
      email: email,
      phoneNumber: json['phone_number']?.toString(),
      username: safeString(
        'username',
      ), // Phase 1: Preserve backend username when present
      accountStatus: safeString('account_status', defaultValue: 'active'),
      roles: parsedRoles,
      sellerTier: json['seller_tier']?.toString(),
      // S2: Seller state fields from backend
      hasSellerProfile: json['has_seller_profile'] as bool?,
      sellerSubscriptionStatus: json['seller_subscription_status']?.toString(),
      hasMarketAuthority: json['has_market_authority'] as bool?,
      // Penalty points
      totalPenaltyPoints: json['total_penalty_points'] as int?,
      activePenaltyPoints: json['active_penalty_points'] as int?,
      // Verification flags
      isPhoneVerified:
          (json['is_phone_verified'] ?? json['phone_verified']) as bool?,
      isEmailVerified:
          (json['is_email_verified'] ?? json['email_verified']) as bool?,
      isIdVerified: json['is_id_verified'] as bool?,
      isFarmVerified: json['is_farm_verified'] as bool?,
      phoneVerifiedAt: safeDateTime('phone_verified_at'),
      emailVerifiedAt: safeDateTime('email_verified_at'),
      idVerifiedAt: safeDateTime('id_verified_at'),
      farmVerifiedAt: safeDateTime('farm_verified_at'),
      createdAt: safeDateTime('created_at') ?? DateTime.now(),
      updatedAt: safeDateTime('updated_at') ?? DateTime.now(),
      location: (() {
        final value = json['location'];
        if (value == null) return null;
        final parsed = value.toString().trim();
        return parsed.isEmpty ? null : parsed;
      })(),
      profile: json['profile'] != null
          ? UserProfileApiResponse.fromJson(json['profile'])
          : null,
    );

    // 🔍 DEBUG: Log parsed values
    LoggerService.instance.debug('UserApiResponse.fromJson - parsed values');
    LoggerService.instance.debug('UserApiResponse: id=${parsed.id}');
    LoggerService.instance.debug('UserApiResponse: email=(redacted)');
    LoggerService.instance.debug(
      'UserApiResponse: isEmailVerified=${parsed.isEmailVerified}',
    );
    LoggerService.instance.debug(
      'UserApiResponse: isPhoneVerified=${parsed.isPhoneVerified}',
    );
    LoggerService.instance.debug(
      'UserApiResponse: emailVerifiedAt=${parsed.emailVerifiedAt}',
    );

    return parsed;
  }

  @override
  List<Object?> get props => [
    id,
    email,
    username,
    accountStatus,
    roles,
    sellerTier,
    totalPenaltyPoints,
    activePenaltyPoints,
    location,
  ];
}

/// User profile response from Go backend
class UserProfileApiResponse extends Equatable {
  final String id;
  final String username;
  final String? bio;
  final String? avatarUrl;

  /// Resolved cover photo read URL (backend resolves the persisted storage
  /// key through mediaresolve). Null when the user has no cover.
  final String? coverPhotoUrl;
  final int followersCount;
  final int followingCount;
  final DateTime? dateOfBirth;
  final String? gender;
  final String? location;
  final String preferredLang;
  final DateTime? lastActiveAt;
  final SocialMediaApiResponse? socialMedia;
  final PrivacySettingsApiResponse? privacy;

  const UserProfileApiResponse({
    required this.id,
    required this.username,
    this.bio,
    this.avatarUrl,
    this.coverPhotoUrl,
    this.followersCount = 0,
    this.followingCount = 0,
    this.dateOfBirth,
    this.gender,
    this.location,
    required this.preferredLang,
    this.lastActiveAt,
    this.socialMedia,
    this.privacy,
  });

  factory UserProfileApiResponse.fromJson(Map<String, dynamic> json) {
    LoggerService.instance.debug('UserProfileApiResponse.fromJson - parsing');

    // Helper function for safe string parsing
    String? safeString(String key) {
      final value = json[key];
      return value?.toString();
    }

    return UserProfileApiResponse(
      id: safeString('id') ?? '',
      username: safeString('username') ?? '',
      bio: safeString('bio'),
      avatarUrl: safeString('avatar_url'),
      coverPhotoUrl: safeString('cover_photo_url'),
      followersCount: (json['followers_count'] as num?)?.toInt() ?? 0,
      followingCount: (json['following_count'] as num?)?.toInt() ?? 0,
      dateOfBirth: json['date_of_birth'] != null
          ? DateTime.parse(json['date_of_birth'].toString())
          : null,
      gender: safeString('gender'),
      location: safeString('location'),
      preferredLang: safeString('preferred_lang') ?? 'id',
      lastActiveAt: json['last_active_at'] != null
          ? DateTime.parse(json['last_active_at'].toString())
          : null,
      socialMedia: json['social_media'] != null
          ? SocialMediaApiResponse.fromJson(json['social_media'])
          : null,
      privacy: json['privacy'] != null
          ? PrivacySettingsApiResponse.fromJson(json['privacy'])
          : null,
    );
  }

  @override
  List<Object?> get props => [id, username, followersCount, followingCount];
}

/// Social media handles response
class SocialMediaApiResponse extends Equatable {
  final String? instagramHandle;
  final String? facebookHandle;
  final String? twitterHandle;
  final String? tiktokHandle;
  final String? youtubeHandle;
  final String? websiteUrl;

  const SocialMediaApiResponse({
    this.instagramHandle,
    this.facebookHandle,
    this.twitterHandle,
    this.tiktokHandle,
    this.youtubeHandle,
    this.websiteUrl,
  });

  factory SocialMediaApiResponse.fromJson(Map<String, dynamic> json) {
    return SocialMediaApiResponse(
      instagramHandle: json['instagram_handle'] as String?,
      facebookHandle: json['facebook_handle'] as String?,
      twitterHandle: json['twitter_handle'] as String?,
      tiktokHandle: json['tiktok_handle'] as String?,
      youtubeHandle: json['youtube_handle'] as String?,
      websiteUrl: json['website_url'] as String?,
    );
  }

  @override
  List<Object?> get props => [instagramHandle, facebookHandle, twitterHandle];
}

/// Privacy settings response
class PrivacySettingsApiResponse extends Equatable {
  final String visibility;
  final bool showPhoneNumber;
  final bool showEmail;
  final bool showLocation;
  final String allowMessagesFrom;
  final bool allowTagging;
  final bool showActivityStatus;
  final bool showTransactionCount;

  const PrivacySettingsApiResponse({
    required this.visibility,
    required this.showPhoneNumber,
    required this.showEmail,
    required this.showLocation,
    required this.allowMessagesFrom,
    required this.allowTagging,
    required this.showActivityStatus,
    required this.showTransactionCount,
  });

  factory PrivacySettingsApiResponse.fromJson(Map<String, dynamic> json) {
    return PrivacySettingsApiResponse(
      visibility: json['visibility'] as String? ?? 'public',
      showPhoneNumber: json['show_phone_number'] as bool? ?? false,
      showEmail: json['show_email'] as bool? ?? false,
      showLocation: json['show_location'] as bool? ?? true,
      allowMessagesFrom: json['allow_messages_from'] as String? ?? 'everyone',
      allowTagging: json['allow_tagging'] as bool? ?? true,
      showActivityStatus: json['show_activity_status'] as bool? ?? true,
      showTransactionCount: json['show_transaction_count'] as bool? ?? true,
    );
  }

  @override
  List<Object?> get props => [visibility, showPhoneNumber, showEmail];
}
