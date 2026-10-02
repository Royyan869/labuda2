import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart'; // For AuthUser and extensions (isSeller, etc.)
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/user_data_provider.dart'
    show userDataProvider;
import 'package:labuda/domains/user/profile/presentation/providers/profile_stream_provider.dart'
    show profileStreamProvider;

/// Combined data class for Profile About tab
/// Merges AuthUser (core identity) + ProfileEntity (extended social features)
class ProfileAboutData {
  final AuthUser user;
  final ProfileEntity? profile;
  final String? location;

  const ProfileAboutData({required this.user, this.profile, this.location});

  // Convenience getters for About tab sections

  // Section 1: About
  String get bio => user.bio ?? '';
  DateTime get joinedAt => profile?.joinedAt ?? user.createdAt;
  DateTime? get lastActiveAt => profile?.lastActiveAt;

  // Section 2: Farm Info (seller only)
  FarmInfo? get farmInfo => profile?.farmInfo;
  String? get farmName => farmInfo?.farmName;
  String? get farmWebsite => farmInfo?.farmWebsite;
  List<String>? get specialties => farmInfo?.specialties;
  DateTime? get establishedDate => farmInfo?.establishedDate;

  // Section 3: Verification Badges (REAL data only)
  // REMOVED: Achievements section - NO backend support, deleted in PROFILE PURGE
  UserVerificationInfo? get verification => profile?.verification;

  // Section 4: Performance (seller only)
  ProfileStats? get stats => profile?.stats;

  // Section 5: Contact Information
  ContactInfo? get contactInfo => profile?.contactInfo;
  String? get maskedEmail => contactInfo?.maskedEmail;
  String? get maskedPhone => contactInfo?.maskedPhone;
  bool get isEmailPublic => contactInfo?.isEmailPublic ?? false;
  bool get isPhonePublic => contactInfo?.isPhonePublic ?? false;

  // Social Media
  String? get instagramHandle => contactInfo?.instagramHandle;
  String? get facebookHandle => contactInfo?.facebookHandle;
  String? get tiktokHandle => contactInfo?.tiktokHandle;
  String? get twitterHandle => contactInfo?.twitterHandle;
  bool get isSocialMediaPublic => contactInfo?.isSocialMediaPublic ?? true;

  // Helper: Check if user is seller
  bool get isSeller => user.hasCreatedSellerProfile;

  // Helper: Check if any social media is set
  bool get hasSocialMedia =>
      instagramHandle != null ||
      facebookHandle != null ||
      tiktokHandle != null ||
      twitterHandle != null;
}

/// Provider for Profile About tab data
/// Combines AuthUser and ProfileEntity for complete profile view
final profileAboutDataProvider =
    FutureProvider.family<ProfileAboutData, String>((ref, userId) async {
      // Fetch AuthUser (required)
      final userResult = await ref.watch(userDataProvider(userId).future);

      if (userResult == null) {
        throw Exception('User not found');
      }

      // Fetch ProfileEntity (optional - may not exist for new users)
      final profile = await ref.watch(profileStreamProvider(userId).future);

      // ONE location rule for every profile surface: the backend's public
      // origin line (city, province of the sender address), falling back to
      // the user's own location field. The client-side composition from raw
      // address records is PURGED — the server owns that rule, so the About
      // tab and the profile header can never disagree.
      return ProfileAboutData(
        user: userResult,
        profile: profile,
        location: normalizeProfileLocation(profile?.publicOriginLine) ??
            normalizeProfileLocation(profile?.location),
      );
    });

String? normalizeProfileLocation(String? location) {
  final normalized = location?.trim();
  if (normalized == null || normalized.isEmpty) return null;
  return normalized;
}
