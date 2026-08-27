import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';

/// Mapper to convert between API models and domain entities
class UserApiMapper {
  /// Convert UserApiResponse to AuthUser domain entity
  static AuthUser toAuthUser(UserApiResponse response) {
    final profile = response.profile;

    return AuthUser(
      id: response.id,
      createdAt: response.createdAt,
      updatedAt: response.updatedAt,
      email: response.email,
      username: profile?.username ?? '',
      avatarUrl: profile?.avatarUrl,
      bio: profile?.bio,
      isEmailVerified: response.isEmailVerified ?? false, // From backend
      phoneNumber: response.phoneNumber,
      isPhoneVerified: response
          .isPhoneVerified, // From backend (aligned with other verification flags)
      phoneVerifiedAt: response.phoneVerifiedAt,
      dateOfBirth: profile?.dateOfBirth,
      // Backend authority fields
      accountStatus: response.accountStatus.isNotEmpty
          ? AccountStatus.fromApiValue(response.accountStatus)
          : null,
      sellerTier: response.sellerTier != null
          ? SellerTier.fromApiValue(response.sellerTier)
          : null,
      // S2: Seller state fields from backend
      hasSellerProfile: response.hasSellerProfile,
      sellerSubscriptionStatus: response.sellerSubscriptionStatus,
      hasMarketAuthority: response.hasMarketAuthority,
      totalPenaltyPoints: response.totalPenaltyPoints,
      activePenaltyPoints: response.activePenaltyPoints,
      isIdVerified: response.isIdVerified,
      isFarmVerified: response.isFarmVerified,
      idVerifiedAt: response.idVerifiedAt,
      farmVerifiedAt: response.farmVerifiedAt,
      roles: _mapBackendRoles(
        response.roles,
      ), // Roles from PostgreSQL (Single Source of Truth)
      provider: ShonaAuthProvider.email, // Default, could be enhanced
    );
  }

  /// Convert UserApiResponse to ProfileEntity
  static ProfileEntity toProfileEntity(UserApiResponse response) {
    final profile = response.profile;

    return ProfileEntity(
      id: profile?.id ?? response.id,
      userId: response.id,
      location:
          _normalizeLocation(response.location) ??
          _normalizeLocation(profile?.location),
      coverPhotoUrl: profile?.coverPhotoUrl,
      joinedAt: response.createdAt,
      lastActiveAt: profile?.lastActiveAt,
      stats: ProfileStats(
        followersCount: profile?.followersCount ?? 0,
        followingCount: profile?.followingCount ?? 0,
      ),
      verification: _createVerificationInfo(response),
      contactInfo: _mapContactInfo(response),
      farmInfo: null, // TODO: Fetch from separate endpoint
    );
  }

  /// Convert profile update parameters to UpdateProfileApiRequest
  static UpdateProfileApiRequest toUpdateProfileRequest({
    String? bio,
    String? avatarUrl,
    String? coverPhotoUrl,
    DateTime? dateOfBirth,
    String? gender,
    String? location,
    String? instagramHandle,
    String? facebookHandle,
    String? twitterHandle,
    String? tiktokHandle,
    String? youtubeHandle,
    String? websiteUrl,
    String? visibility,
    bool? showPhoneNumber,
    bool? showEmail,
    bool? showLocation,
    String? allowMessagesFrom,
    bool? allowTagging,
    bool? showActivityStatus,
    bool? showTransactionCount,
  }) {
    return UpdateProfileApiRequest(
      bio: bio,
      avatarUrl: avatarUrl,
      coverPhotoUrl: coverPhotoUrl,
      dateOfBirth: dateOfBirth,
      gender: gender,
      location: location,
      instagramHandle: instagramHandle,
      facebookHandle: facebookHandle,
      twitterHandle: twitterHandle,
      tiktokHandle: tiktokHandle,
      youtubeHandle: youtubeHandle,
      websiteUrl: websiteUrl,
      visibility: visibility,
      showPhoneNumber: showPhoneNumber,
      showEmail: showEmail,
      showLocation: showLocation,
      allowMessagesFrom: allowMessagesFrom,
      allowTagging: allowTagging,
      showActivityStatus: showActivityStatus,
      showTransactionCount: showTransactionCount,
    );
  }

  // ============ Private Helpers ============

  /// Map backend roles (from PostgreSQL) to UserRole enums
  /// Backend returns: ["buyer", "seller", "admin", "super_admin"]
  /// Canonical: ["user", "seller", "admin"]
  static List<UserRole> _mapBackendRoles(List<String> backendRoles) {
    if (backendRoles.isEmpty) {
      return [UserRole.user]; // Default role (canonical: user replaces buyer)
    }

    final roles = <UserRole>[];
    for (final roleStr in backendRoles) {
      final role = _mapStringToRole(roleStr);
      if (role != null) {
        roles.add(role);
      }
    }

    // Always ensure at least user role (canonical: user replaces buyer)
    if (roles.isEmpty) {
      return [UserRole.user];
    }

    return roles;
  }

  /// Map single role string to UserRole enum
  ///
  /// **CANONICAL MAPPING:** Uses UserRoleParser.fromApiValue() from core/src/auth/app_role.dart
  /// - "buyer" → UserRole.user (legacy "buyer" becomes canonical "user")
  /// - "user" → UserRole.user
  /// - "admin", "super_admin", "support_admin" → UserRole.admin
  /// - "moderator" → UserRole.user (ghost role, no longer valid)
  /// - "verifier" → null (ghost role, filtered out)
  static UserRole? _mapStringToRole(String roleStr) {
    // Filter out ghost roles explicitly
    if (roleStr.toLowerCase() == 'verifier') {
      return null;
    }

    return UserRoleParser.fromApiValue(roleStr);
  }

  /// Create UserVerificationInfo from backend response
  ///
  /// **IMPORTANT: This derives from the same backend source as AuthUser.**
  /// AuthUser is the canonical owner of verification flags.
  /// This method creates a display model for ProfileEntity convenience.
  /// Use UserVerificationInfo.fromAuthUser() when AuthUser is available.
  static UserVerificationInfo _createVerificationInfo(
    UserApiResponse response,
  ) {
    final badges = <ProfileBadge>[];

    // Add verification badges based on backend response
    // (Same source as AuthUser verification flags)
    if (response.isPhoneVerified == true) {
      badges.add(ProfileBadge.phoneVerified);
    }
    if (response.isEmailVerified == true) {
      badges.add(ProfileBadge.emailVerified);
    }
    if (response.isIdVerified == true) {
      badges.add(ProfileBadge.idVerified);
    }
    if (response.isFarmVerified == true) {
      badges.add(ProfileBadge.farmVerified);
    }

    return UserVerificationInfo(
      isPhoneVerified: response.isPhoneVerified ?? false,
      isEmailVerified: response.isEmailVerified ?? false, // From backend
      isIdVerified: response.isIdVerified ?? false,
      isFarmVerified: response.isFarmVerified ?? false,
      badges: badges,
    );
  }

  static ContactInfo? _mapContactInfo(UserApiResponse response) {
    final profile = response.profile;
    final socialMedia = profile?.socialMedia;
    final privacy = profile?.privacy;

    if (socialMedia == null && privacy == null) return null;

    return ContactInfo(
      maskedPhone: response.phoneNumber != null
          ? _maskPhone(response.phoneNumber!)
          : null,
      maskedEmail: _maskEmail(response.email),
      isPhonePublic: privacy?.showPhoneNumber ?? false,
      isEmailPublic: privacy?.showEmail ?? false,
      instagramHandle: socialMedia?.instagramHandle,
      facebookHandle: socialMedia?.facebookHandle,
      tiktokHandle: socialMedia?.tiktokHandle,
      twitterHandle: socialMedia?.twitterHandle,
      isSocialMediaPublic: true,
    );
  }

  static String _maskPhone(String phone) {
    if (phone.length < 6) return '***';
    return '${phone.substring(0, 3)}***${phone.substring(phone.length - 3)}';
  }

  static String _maskEmail(String email) {
    final parts = email.split('@');
    if (parts.length != 2) return '***@***';

    final name = parts[0];
    final domain = parts[1];

    if (name.length <= 2) return '***@$domain';
    return '${name.substring(0, 2)}***@$domain';
  }

  static String? _normalizeLocation(String? location) {
    final normalized = location?.trim();
    if (normalized == null || normalized.isEmpty) return null;
    return normalized;
  }
}
