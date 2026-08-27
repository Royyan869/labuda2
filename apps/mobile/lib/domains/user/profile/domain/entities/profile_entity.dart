import 'package:equatable/equatable.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/auth_user.dart';

/// User verification info and badges
///
/// **IMPORTANT: This is a DERIVED display model, NOT a source of truth.**
///
/// Verification flags are derived from AuthUser, which is the single source of truth.
/// This class combines verification flags with computed badges for UI display convenience.
///
/// DO NOT use this class as an independent owner of verification data.
/// Always read/write verification flags through AuthUser.
class UserVerificationInfo extends Equatable {
  final bool isPhoneVerified;
  final bool isEmailVerified;
  final bool isIdVerified;
  final bool isFarmVerified;
  final List<ProfileBadge> badges;

  const UserVerificationInfo({
    required this.isPhoneVerified,
    required this.isEmailVerified,
    required this.isIdVerified,
    required this.isFarmVerified,
    required this.badges,
  });

  /// Create UserVerificationInfo from AuthUser (canonical source of truth)
  ///
  /// This factory derives verification flags from AuthUser, ensuring
  /// that AuthUser remains the single source of truth for verification data.
  factory UserVerificationInfo.fromAuthUser(AuthUser user) {
    final badges = <ProfileBadge>[];

    // Add verification badges based on AuthUser verification flags
    if (user.isPhoneVerified == true) {
      badges.add(ProfileBadge.phoneVerified);
    }
    if (user.isEmailVerified) {
      badges.add(ProfileBadge.emailVerified);
    }
    if (user.isIdVerified == true) {
      badges.add(ProfileBadge.idVerified);
    }
    if (user.isFarmVerified == true) {
      badges.add(ProfileBadge.farmVerified);
    }

    return UserVerificationInfo(
      isPhoneVerified: user.isPhoneVerified ?? false,
      isEmailVerified: user.isEmailVerified,
      isIdVerified: user.isIdVerified ?? false,
      isFarmVerified: user.isFarmVerified ?? false,
      badges: badges,
    );
  }

  @override
  List<Object> get props => [
    isPhoneVerified,
    isEmailVerified,
    isIdVerified,
    isFarmVerified,
    badges,
  ];
}

/// Profile-level badges (verification only)
///
/// ⚠️ HONEST UI POLICY: Only verification badges that are backed by backend data
/// are included here. All fake/placeholder badges have been removed.
enum ProfileBadge { phoneVerified, emailVerified, idVerified, farmVerified }

/// Profile Entity - Extended social features only (no duplicate fields)
///
/// Core user data now comes from AuthUser (Single Source of Truth)
/// This entity only contains profile-specific social features and extended data.
class ProfileEntity extends Equatable {
  final String id;
  final String userId; // ← Reference to AuthUser.id

  // Profile-specific fields only
  final String? location;
  final String? coverPhotoUrl;
  final DateTime joinedAt;
  final DateTime? lastActiveAt;

  // Social Stats
  final ProfileStats stats;

  // Verification
  final UserVerificationInfo verification;

  // REMOVED: userType (now in AuthUser.role as single source)
  // REMOVED: roles (replaced by AuthUser.role enum)
  // REMOVED: achievements (NO backend support - deleted in PROFILE PURGE)

  // Contact Info (masked for privacy)
  final ContactInfo? contactInfo;

  // Farm Info (untuk sellers)
  final FarmInfo? farmInfo;

  const ProfileEntity({
    required this.id,
    required this.userId,
    this.location,
    this.coverPhotoUrl,
    required this.joinedAt,
    this.lastActiveAt,
    required this.stats,
    required this.verification,
    // REMOVED: userType, roles (now AuthUser.role)
    // REMOVED: achievements (NO backend support - deleted in PROFILE PURGE)
    this.contactInfo,
    this.farmInfo,
  });

  @override
  List<Object?> get props => [
    id,
    userId,
    location,
    coverPhotoUrl,
    joinedAt,
    lastActiveAt,
    stats,
    verification,
    // REMOVED: userType, roles (now AuthUser.role)
    // REMOVED: achievements (NO backend support - deleted in PROFILE PURGE)
    contactInfo,
    farmInfo,
  ];

  ProfileEntity copyWith({
    String? id,
    String? userId,
    String? location,
    String? coverPhotoUrl,
    DateTime? joinedAt,
    DateTime? lastActiveAt,
    ProfileStats? stats,
    UserVerificationInfo? verification,
    // REMOVED: userType, roles (now AuthUser.role)
    // REMOVED: achievements (NO backend support - deleted in PROFILE PURGE)
    ContactInfo? contactInfo,
    FarmInfo? farmInfo,
  }) {
    return ProfileEntity(
      id: id ?? this.id,
      userId: userId ?? this.userId,
      location: location ?? this.location,
      coverPhotoUrl: coverPhotoUrl ?? this.coverPhotoUrl,
      joinedAt: joinedAt ?? this.joinedAt,
      lastActiveAt: lastActiveAt ?? this.lastActiveAt,
      stats: stats ?? this.stats,
      verification: verification ?? this.verification,
      // REMOVED: userType, roles (now AuthUser.role)
      // REMOVED: achievements (NO backend support - deleted in PROFILE PURGE)
      contactInfo: contactInfo ?? this.contactInfo,
      farmInfo: farmInfo ?? this.farmInfo,
    );
  }
}

/// Social Statistics for profile
///
/// ⚠️ HONEST UI POLICY: Only fields backed by real backend data are included.
/// All fake/placeholder stats have been removed in PROFILE PURGE.
///
/// NOTE: Rating data (averageRating, totalReviews) is provided by the separate
/// Rating module via getUserRatingSummaryProvider. Do NOT add rating fields here.
class ProfileStats extends Equatable {
  final int followersCount;
  final int followingCount;

  // REMOVED: postsCount (no backend support, deleted in PROFILE PURGE)
  // REMOVED: averageRating (use rating module instead, deleted in PROFILE PURGE)
  // REMOVED: totalReviews (use rating module instead, deleted in PROFILE PURGE)
  // REMOVED: collectionsCount (no backend support, deleted in PROFILE PURGE)
  // REMOVED: transactionsCount (no backend support, deleted in PROFILE PURGE)

  const ProfileStats({
    required this.followersCount,
    required this.followingCount,
  });

  @override
  List<Object> get props => [followersCount, followingCount];
}

// REMOVED: UserType enum (now using AuthUser.UserRole as single source)

// REMOVED: Achievement class - NO backend support, deleted in PROFILE PURGE
// REMOVED: AchievementType enum - NO backend support, deleted in PROFILE PURGE

/// Contact information (masked for privacy)
class ContactInfo extends Equatable {
  final String? maskedPhone; // "081234***789"
  final String? maskedEmail; // "user***@email.com"
  final bool isPhonePublic;
  final bool isEmailPublic;

  // Social Media Links (store handles/usernames only, not full URLs)
  final String? instagramHandle; // "@username" or "username"
  final String? facebookHandle; // "Page Name" or "username"
  final String? tiktokHandle; // "@username" or "username"
  final String? twitterHandle; // "@username" or "username"
  final bool isSocialMediaPublic; // Single toggle for all social media

  const ContactInfo({
    this.maskedPhone,
    this.maskedEmail,
    required this.isPhonePublic,
    required this.isEmailPublic,
    this.instagramHandle,
    this.facebookHandle,
    this.tiktokHandle,
    this.twitterHandle,
    this.isSocialMediaPublic = true, // Default to public
  });

  @override
  List<Object?> get props => [
    maskedPhone,
    maskedEmail,
    isPhonePublic,
    isEmailPublic,
    instagramHandle,
    facebookHandle,
    tiktokHandle,
    twitterHandle,
    isSocialMediaPublic,
  ];

  ContactInfo copyWith({
    String? maskedPhone,
    String? maskedEmail,
    bool? isPhonePublic,
    bool? isEmailPublic,
    String? instagramHandle,
    String? facebookHandle,
    String? tiktokHandle,
    String? twitterHandle,
    bool? isSocialMediaPublic,
  }) {
    return ContactInfo(
      maskedPhone: maskedPhone ?? this.maskedPhone,
      maskedEmail: maskedEmail ?? this.maskedEmail,
      isPhonePublic: isPhonePublic ?? this.isPhonePublic,
      isEmailPublic: isEmailPublic ?? this.isEmailPublic,
      instagramHandle: instagramHandle ?? this.instagramHandle,
      facebookHandle: facebookHandle ?? this.facebookHandle,
      tiktokHandle: tiktokHandle ?? this.tiktokHandle,
      twitterHandle: twitterHandle ?? this.twitterHandle,
      isSocialMediaPublic: isSocialMediaPublic ?? this.isSocialMediaPublic,
    );
  }
}

/// Farm information for sellers (Farm-specific data ONLY)
///
/// ✅ Single Source of Truth Implementation:
/// - Email: Use AuthUser.email ❌ NO farmEmail
/// - Phone: Use AuthUser.phoneNumber ❌ NO farmPhone
/// - Personal Bio: Use AuthUser.bio (canonical seller/store description too)
/// - Farm/Sender Address: Use AddressEntity with purpose=sender ❌ NO farmAddress
///
/// This class contains ONLY farm-specific data that is NOT in AuthUser or AddressEntity
class FarmInfo extends Equatable {
  final String farmName; // Farm/store name (different from personal name)
  final String?
  farmPhotoUrl; // Farm/brand logo (different from personal avatar)
  final String? farmWebsite; // Farm website
  final List<String>? specialties; // OPTIONAL legacy metadata
  final DateTime? establishedDate; // Farm established date

  const FarmInfo({
    required this.farmName,
    this.farmPhotoUrl,
    this.farmWebsite,
    this.specialties, // OPTIONAL now
    this.establishedDate,
  });

  @override
  List<Object?> get props => [
    farmName,
    farmPhotoUrl,
    farmWebsite,
    specialties,
    establishedDate,
  ];

  FarmInfo copyWith({
    String? farmName,
    String? farmPhotoUrl,
    String? farmWebsite,
    List<String>? specialties,
    DateTime? establishedDate,
  }) {
    return FarmInfo(
      farmName: farmName ?? this.farmName,
      farmPhotoUrl: farmPhotoUrl ?? this.farmPhotoUrl,
      farmWebsite: farmWebsite ?? this.farmWebsite,
      specialties: specialties ?? this.specialties,
      establishedDate: establishedDate ?? this.establishedDate,
    );
  }
}
