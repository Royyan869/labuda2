import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/user/profile/presentation/utils/profile_lifecycle_redaction.dart';
import 'package:labuda/domains/user/profile/profile.dart'
    show ProfileAboutData, profileAboutDataProvider;
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';
import 'package:labuda/domains/social/rating/rating.dart';
import 'package:intl/intl.dart';
import 'package:url_launcher/url_launcher.dart';

/// About tab content for profile screen - displays user bio, farm info, achievements, and contact
class ProfileAboutTab extends ConsumerWidget {
  final String userId;
  final bool isOwnProfile;

  const ProfileAboutTab({
    super.key,
    required this.userId,
    this.isOwnProfile = false,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final dataAsync = ref.watch(profileAboutDataProvider(userId));

    return dataAsync.when(
      data: (data) => _buildContent(context, data, isDark, ref),
      loading: () => _buildLoading(),
      error: (error, stack) => _buildError(context, error.toString(), isDark),
    );
  }

  Widget _buildContent(
    BuildContext context,
    ProfileAboutData data,
    bool isDark,
    WidgetRef ref,
  ) {
    // E5.3 — When the target's identity lifecycle is degraded
    // (unavailable / removed), suppress every sensitive About-tab section
    // (bio, location, farm info, verification badges, rating widget,
    // contact info, social media). Profile-detail fail-OPENs on the header
    // (E5.2 renders a redacted identity card), but secondary sections MUST
    // NOT leak the underlying profile data while the identity itself is
    // tombstoned. Own-profile is never degraded from the viewer's POV.
    if (!isOwnProfile &&
        profileLifecycleSuppressesSensitiveSections(data.user.lifecycle)) {
      return _buildDegradedPlaceholder(isDark, data.user.lifecycle);
    }

    // Get seller state for the current profile.
    final sellerState = isOwnProfile
        ? _currentAccountSellerState(ref)
        : SellerState.fromAuthUser(data.user);
    final showSellerSections = sellerState?.isSeller ?? false;

    return CustomScrollView(
      slivers: [
        SliverPadding(
          padding: const EdgeInsets.all(16),
          sliver: SliverList(
            delegate: SliverChildListDelegate([
              // Section 0: Seller Status Badge (for own profile or seller profiles)
              if (isOwnProfile && sellerState == null) ...[
                _buildPendingSellerStatusCard(isDark: isDark),
                const SizedBox(height: 16),
              ] else if (sellerState != null &&
                  (isOwnProfile || sellerState.isSeller)) ...[
                _SellerStatusBadge(sellerState: sellerState, isDark: isDark),
                const SizedBox(height: 16),
              ],

              // Section 1: About
              if (data.bio.isNotEmpty || data.location != null) ...[
                _ProfileSectionCard(
                  title: 'About',
                  icon: Icons.person_outline,
                  child: _buildAboutSection(data, isDark),
                ),
                const SizedBox(height: 16),
              ],

              // Section 2: Farm Info (seller only)
              if (showSellerSections && data.farmInfo != null) ...[
                _ProfileSectionCard(
                  title: 'Informasi Farm',
                  icon: Icons.store_outlined,
                  child: _buildFarmInfoSection(data, isDark),
                ),
                const SizedBox(height: 16),
              ],

              // Section 3: Verification Badges (REAL data only)
              // REMOVED: Achievements section - NO backend support, deleted in PROFILE PURGE
              if (_hasAnyVerificationBadges(data)) ...[
                _ProfileSectionCard(
                  title: 'Verification',
                  icon: Icons.verified_outlined,
                  child: _buildVerificationSection(data, isDark),
                ),
                const SizedBox(height: 16),
              ],

              // Section 4: Rating & Reviews (seller only - uses REAL data from rating module)
              // REMOVED: Fake metrics (Total Penjualan, Completion Rate) - NO backend support
              if (showSellerSections) ...[
                _ProfileSectionCard(
                  title: 'Rating & Reviews',
                  icon: Icons.star_outline,
                  child: _buildRatingSection(isDark, ref),
                ),
                const SizedBox(height: 16),
              ],

              // Section 5: Contact Information
              if (_shouldShowContact(data)) ...[
                _ProfileSectionCard(
                  title: 'Contact Information',
                  icon: Icons.contact_phone_outlined,
                  child: _buildContactSection(data, isDark),
                ),
              ],
            ]),
          ),
        ),
      ],
    );
  }

  SellerState? _currentAccountSellerState(WidgetRef ref) {
    final identityStatus = ref.watch(sellerIdentityStatusProvider);
    final capabilityStatus = ref.watch(sellerCapabilityStatusProvider);

    if (identityStatus == SellerIdentityStatus.unknown ||
        capabilityStatus == SellerCapabilityStatus.unknown) {
      return null;
    }

    if (identityStatus == SellerIdentityStatus.seller) {
      return capabilityStatus == SellerCapabilityStatus.active
          ? const SellerState.active()
          : const SellerState.expired();
    }

    return const SellerState.notSeller();
  }

  Widget _buildPendingSellerStatusCard({required bool isDark}) {
    final background = isDark ? AppColors.darkGray700 : AppColors.neutralGray50;
    final border = isDark ? AppColors.neutralGray700 : AppColors.neutralGray200;
    final textPrimary = isDark
        ? AppColors.neutralGray200
        : AppColors.neutralGray700;

    return Container(
      margin: const EdgeInsets.only(bottom: 16),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: background,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: border),
      ),
      child: Row(
        children: [
          Icon(
            Icons.hourglass_top_outlined,
            size: 20,
            color: AppColors.primaryBlue,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              'Checking seller status...',
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w600,
                color: textPrimary,
              ),
            ),
          ),
        ],
      ),
    );
  }

  // Section 1: About
  Widget _buildAboutSection(ProfileAboutData data, bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Bio
        if (data.bio.isNotEmpty) ...[
          Text(
            data.bio,
            style: TextStyle(
              fontSize: 14,
              height: 1.5,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 16),
        ],

        // Location
        if (data.location != null) ...[
          Row(
            children: [
              Icon(
                Icons.location_on_outlined,
                size: 16,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray500,
              ),
              const SizedBox(width: 8),
              Text(
                data.location!,
                style: TextStyle(
                  fontSize: 14,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray500,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
        ],

        // Join date
        Row(
          children: [
            Icon(
              Icons.calendar_today_outlined,
              size: 16,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
            const SizedBox(width: 8),
            Text(
              _formatJoinDate(data.joinedAt),
              style: TextStyle(
                fontSize: 14,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray500,
              ),
            ),
          ],
        ),

        // Last active
        if (data.lastActiveAt != null) ...[
          const SizedBox(height: 8),
          Row(
            children: [
              Icon(
                Icons.access_time,
                size: 16,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray500,
              ),
              const SizedBox(width: 8),
              Text(
                _formatLastActive(data.lastActiveAt!),
                style: TextStyle(
                  fontSize: 14,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray500,
                ),
              ),
            ],
          ),
        ],
      ],
    );
  }

  // Section 2: Farm Info
  Widget _buildFarmInfoSection(ProfileAboutData data, bool isDark) {
    final farmInfo = data.farmInfo!;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (farmInfo.farmName.isNotEmpty)
          _ProfileInfoRow(label: 'Farm Name', value: farmInfo.farmName),

        if (farmInfo.establishedDate != null)
          _ProfileInfoRow(
            label: 'Established Since',
            value: DateFormat('yyyy').format(farmInfo.establishedDate!),
          ),

        if (farmInfo.specialties != null && farmInfo.specialties!.isNotEmpty)
          _ProfileInfoRow(
            label: 'Specialties',
            value: farmInfo.specialties!.join(', '),
          ),

        // Canonical seller description comes from AuthUser.bio
        if (data.bio.isNotEmpty) ...[
          const SizedBox(height: 8),
          Text(
            'Description',
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            data.bio,
            style: TextStyle(
              fontSize: 13,
              height: 1.5,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
        ],

        // Website
        if (farmInfo.farmWebsite != null &&
            farmInfo.farmWebsite!.isNotEmpty) ...[
          const SizedBox(height: 12),
          InkWell(
            onTap: () => _launchUrl(farmInfo.farmWebsite!),
            child: Row(
              children: [
                const Icon(Icons.language, size: 16, color: AppColors.primary),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    farmInfo.farmWebsite!,
                    style: const TextStyle(
                      fontSize: 14,
                      color: AppColors.primary,
                      decoration: TextDecoration.underline,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ],
    );
  }

  // Section 3: Verification Badges (REAL data only)
  // REMOVED: Achievements section - NO backend support, deleted in PROFILE PURGE
  Widget _buildVerificationSection(ProfileAboutData data, bool isDark) {
    final badges = <Widget>[];

    final verification = data.verification;
    if (verification != null) {
      if (verification.isPhoneVerified) {
        badges.add(const _VerificationBadge(text: '✅ Phone Verified'));
      }
      if (verification.isEmailVerified) {
        badges.add(const _VerificationBadge(text: '✅ Email Verified'));
      }
      if (verification.isIdVerified) {
        badges.add(const _VerificationBadge(text: '🆔 ID Verified'));
      }
      if (verification.isFarmVerified) {
        badges.add(const _VerificationBadge(text: '🏪 Farm Verified'));
      }

      // Verification badges (only real ones from backend)
      for (final badge in verification.badges) {
        badges.add(_VerificationBadge(text: _getVerificationBadgeText(badge)));
      }
    }

    return Wrap(spacing: 8, runSpacing: 8, children: badges);
  }

  // Section 4: Rating & Reviews (uses REAL data from rating module)
  // REMOVED: Fake metrics (Total Penjualan, Completion Rate) - NO backend support
  Widget _buildRatingSection(bool isDark, WidgetRef ref) {
    // Fetch real rating data from rating module
    final ratingSummaryAsync = ref.watch(
      getUserRatingSummaryProvider(userId: userId),
    );

    return ratingSummaryAsync.when(
      data: (result) {
        // Extract real rating data or use defaults
        final averageRating = result.isSuccess && result.data != null
            ? result.data!.averageRating
            : 0.0;
        final totalReviews = result.isSuccess && result.data != null
            ? result.data!.totalRatings
            : 0;

        return Row(
          children: [
            Expanded(
              child: _StatCard(
                label: 'Rating',
                value: averageRating > 0
                    ? averageRating.toStringAsFixed(1)
                    : '0.0',
                icon: Icons.star,
                isDark: isDark,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: _StatCard(
                label: 'Total Reviews',
                value: totalReviews.toString(),
                icon: Icons.rate_review_outlined,
                isDark: isDark,
              ),
            ),
          ],
        );
      },
      loading: () => Row(
        children: [
          Expanded(
            child: _StatCard(
              label: 'Rating',
              value: '...',
              icon: Icons.star,
              isDark: isDark,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: _StatCard(
              label: 'Total Reviews',
              value: '...',
              icon: Icons.rate_review_outlined,
              isDark: isDark,
            ),
          ),
        ],
      ),
      error: (error, stackTrace) => Row(
        children: [
          Expanded(
            child: _StatCard(
              label: 'Rating',
              value: '0.0',
              icon: Icons.star,
              isDark: isDark,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: _StatCard(
              label: 'Total Reviews',
              value: '0',
              icon: Icons.rate_review_outlined,
              isDark: isDark,
            ),
          ),
        ],
      ),
    );
  }

  // Section 5: Contact Information
  Widget _buildContactSection(ProfileAboutData data, bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Email
        if (data.isEmailPublic || isOwnProfile) ...[
          if (data.maskedEmail != null) ...[
            Row(
              children: [
                Icon(
                  Icons.email_outlined,
                  size: 18,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    data.maskedEmail!,
                    style: TextStyle(
                      fontSize: 14,
                      color: isDark
                          ? AppColors.neutralGray300
                          : AppColors.neutralGray700,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
          ],
        ],

        // Phone
        if (data.isPhonePublic || isOwnProfile) ...[
          if (data.maskedPhone != null) ...[
            Row(
              children: [
                Icon(
                  Icons.phone_outlined,
                  size: 18,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    data.maskedPhone!,
                    style: TextStyle(
                      fontSize: 14,
                      color: isDark
                          ? AppColors.neutralGray300
                          : AppColors.neutralGray700,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
          ],
        ],

        // Social Media
        if ((data.isSocialMediaPublic || isOwnProfile) &&
            data.hasSocialMedia) ...[
          Divider(
            color: isDark ? AppColors.neutralGray600 : AppColors.neutralGray300,
          ),
          const SizedBox(height: 8),
          Text(
            'Social Media',
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
          const SizedBox(height: 12),
          Wrap(
            spacing: 12,
            runSpacing: 12,
            children: [
              if (data.instagramHandle != null)
                _SocialMediaChip(
                  icon: Icons.camera_alt,
                  label: data.instagramHandle!,
                  url: _getInstagramUrl(data.instagramHandle!),
                  isDark: isDark,
                ),
              if (data.facebookHandle != null)
                _SocialMediaChip(
                  icon: Icons.facebook,
                  label: data.facebookHandle!,
                  url: _getFacebookUrl(data.facebookHandle!),
                  isDark: isDark,
                ),
              if (data.tiktokHandle != null)
                _SocialMediaChip(
                  icon: Icons.play_circle_outline,
                  label: data.tiktokHandle!,
                  url: _getTiktokUrl(data.tiktokHandle!),
                  isDark: isDark,
                ),
              if (data.twitterHandle != null)
                _SocialMediaChip(
                  icon: Icons.chat_bubble_outline,
                  label: data.twitterHandle!,
                  url: _getTwitterUrl(data.twitterHandle!),
                  isDark: isDark,
                ),
            ],
          ),
        ],
      ],
    );
  }

  // E5.3 — Degraded-lifecycle tombstone for the About tab. Branches per
  // canonical 2-string vocabulary (removed/unavailable) via
  // ContentLifecycleParse.publicRedactionLabel. Never reached for own profile.
  Widget _buildDegradedPlaceholder(bool isDark, ContentLifecycle lifecycle) {
    final label = lifecycle.publicRedactionLabel;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.lock_outline,
              size: 48,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
            const SizedBox(height: 16),
            Text(
              label,
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w600,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }

  // Loading state
  Widget _buildLoading() {
    return const Center(
      child: Padding(
        padding: EdgeInsets.all(32),
        child: CircularProgressIndicator(),
      ),
    );
  }

  // Error state
  Widget _buildError(BuildContext context, String error, bool isDark) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.error_outline,
              size: 48,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
            const SizedBox(height: 16),
            Text(
              'Failed to load profile',
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w600,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              error,
              style: TextStyle(
                fontSize: 14,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }

  // === Helper Methods ===

  // Privacy check: Should show contact info
  bool _shouldShowContact(ProfileAboutData data) {
    if (isOwnProfile) return true;

    // Show if any contact info is public
    return (data.isEmailPublic && data.maskedEmail != null) ||
        (data.isPhonePublic && data.maskedPhone != null) ||
        (data.isSocialMediaPublic && data.hasSocialMedia);
  }

  // Check if has any verification badges to display
  bool _hasAnyVerificationBadges(ProfileAboutData data) {
    if (data.verification != null) {
      final v = data.verification!;
      if (v.isPhoneVerified ||
          v.isEmailVerified ||
          v.isIdVerified ||
          v.isFarmVerified ||
          v.badges.isNotEmpty) {
        return true;
      }
    }
    return false;
  }

  // Format join date
  String _formatJoinDate(DateTime date) {
    return 'Joined ${DateFormat('MMMM yyyy').format(date)}';
  }

  // Format last active
  String _formatLastActive(DateTime date) {
    final now = DateTime.now();
    final diff = now.difference(date);

    if (diff.inMinutes < 1) return 'Last active just now';
    if (diff.inMinutes < 60) return 'Last active ${diff.inMinutes} minutes ago';
    if (diff.inHours < 24) return 'Last active ${diff.inHours} hours ago';
    if (diff.inDays < 7) return 'Last active ${diff.inDays} days ago';

    return 'Last active on ${DateFormat('MMM d, yyyy').format(date)}';
  }

  // REMOVED: _calculateCompletionRate - NO backend support, deleted in PROFILE PURGE
  // REMOVED: _getCompletionRateDisplay - NO backend support, deleted in PROFILE PURGE
  // REMOVED: _getAchievementEmoji - NO backend support, deleted in PROFILE PURGE

  // Get verification badge text from ProfileBadge enum (REAL values only)
  String _getVerificationBadgeText(ProfileBadge badge) {
    switch (badge) {
      case ProfileBadge.phoneVerified:
        return '✅ Phone Verified';
      case ProfileBadge.emailVerified:
        return '✅ Email Verified';
      case ProfileBadge.idVerified:
        return '🆔 ID Verified';
      case ProfileBadge.farmVerified:
        return '🏪 Farm Verified';
      // REMOVED: fake badges (topRatedSeller, proMember, fastResponse, communityModerator)
      // - NO backend support, deleted in PROFILE PURGE
    }
  }

  // Social media URL constructors
  String _getInstagramUrl(String handle) {
    final cleanHandle = handle.replaceAll('@', '');
    return 'https://instagram.com/$cleanHandle';
  }

  String _getFacebookUrl(String handle) {
    return 'https://facebook.com/$handle';
  }

  String _getTiktokUrl(String handle) {
    final cleanHandle = handle.replaceAll('@', '');
    return 'https://tiktok.com/@$cleanHandle';
  }

  String _getTwitterUrl(String handle) {
    final cleanHandle = handle.replaceAll('@', '');
    return 'https://twitter.com/$cleanHandle';
  }

  // Launch URL helper
  Future<void> _launchUrl(String urlString) async {
    final url = Uri.parse(
      urlString.startsWith('http') ? urlString : 'https://$urlString',
    );
    if (await canLaunchUrl(url)) {
      await launchUrl(url, mode: LaunchMode.externalApplication);
    }
  }
}

/// Section card widget for profile about tab
class _ProfileSectionCard extends StatelessWidget {
  final String title;
  final IconData icon;
  final Widget child;

  const _ProfileSectionCard({
    required this.title,
    required this.icon,
    required this.child,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Card(
      margin: EdgeInsets.zero,
      color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(icon, size: 20, color: AppColors.primaryRed),
                const SizedBox(width: 8),
                Text(
                  title,
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            child,
          ],
        ),
      ),
    );
  }
}

/// Info row widget for profile about tab
class _ProfileInfoRow extends StatelessWidget {
  final String label;
  final String value;

  const _ProfileInfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 100,
            child: Text(
              label,
              style: TextStyle(
                fontSize: 14,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray500,
              ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w500,
                color: isDark
                    ? AppColors.neutralGray200
                    : AppColors.neutralGray800,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Verification badge widget (REAL data only)
// REMOVED: Achievement badge - NO backend support, deleted in PROFILE PURGE
class _VerificationBadge extends StatelessWidget {
  final String text;

  const _VerificationBadge({required this.text});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      decoration: BoxDecoration(
        color: isDark
            ? AppColors.darkGray600.withValues(alpha: 0.5)
            : AppColors.neutralGray100,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: isDark ? AppColors.darkGray500 : AppColors.neutralGray200,
        ),
      ),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 12,
          fontWeight: FontWeight.w500,
          color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray700,
        ),
      ),
    );
  }
}

/// Stat card widget for performance metrics
class _StatCard extends StatelessWidget {
  final String label;
  final String value;
  final IconData icon;
  final bool isDark;

  const _StatCard({
    required this.label,
    required this.value,
    required this.icon,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isDark
            ? AppColors.darkGray600.withValues(alpha: 0.3)
            : AppColors.neutralGray50,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: isDark
              ? AppColors.darkGray500.withValues(alpha: 0.5)
              : AppColors.neutralGray200,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 20, color: AppColors.primary),
          const SizedBox(height: 8),
          Text(
            value,
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            label,
            style: TextStyle(
              fontSize: 11,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
          ),
        ],
      ),
    );
  }
}

/// Social media chip widget
class _SocialMediaChip extends StatelessWidget {
  final IconData icon;
  final String label;
  final String url;
  final bool isDark;

  const _SocialMediaChip({
    required this.icon,
    required this.label,
    required this.url,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: () async {
        final uri = Uri.parse(url);
        if (await canLaunchUrl(uri)) {
          await launchUrl(uri, mode: LaunchMode.externalApplication);
        } else {
          if (context.mounted) {
            AppSnackBar.showError(context, 'Cannot open link');
          }
        }
      },
      borderRadius: BorderRadius.circular(8),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        decoration: BoxDecoration(
          color: isDark
              ? AppColors.darkGray600.withValues(alpha: 0.3)
              : AppColors.neutralGray50,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: isDark
                ? AppColors.darkGray500.withValues(alpha: 0.5)
                : AppColors.neutralGray200,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 16, color: AppColors.primary),
            const SizedBox(width: 8),
            Text(
              label,
              style: TextStyle(
                fontSize: 13,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Seller Status Badge Widget
///
/// **OWNER:** Profile Domain
/// **SELLER UX ALIGNMENT:**
/// - Displays honest seller state from backend
/// - Shows one of 3 states: NOT_SELLER, ACTIVE, EXPIRED
/// - For expired sellers, shows renewal CTA
class _SellerStatusBadge extends ConsumerWidget {
  final SellerState sellerState;
  final bool isDark;

  const _SellerStatusBadge({required this.sellerState, required this.isDark});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: _getStatusColor().withValues(alpha: 0.3),
          width: 1,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(_getStatusIcon(), color: _getStatusColor(), size: 20),
              const SizedBox(width: 8),
              Text(
                'Status Penjual',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w500,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
              ),
              const Spacer(),
              _buildStatusBadge(),
            ],
          ),
          if (sellerState.isExpired && sellerState.bannerMessage != null) ...[
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: AppColors.statusError.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color: AppColors.statusError.withValues(alpha: 0.3),
                ),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.warning_amber_rounded,
                    color: AppColors.statusError,
                    size: 18,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      sellerState.bannerMessage!,
                      style: TextStyle(
                        fontSize: 13,
                        color: isDark
                            ? AppColors.neutralGray300
                            : AppColors.neutralGray800,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildStatusBadge() {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      decoration: BoxDecoration(
        color: _getStatusColor().withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        sellerState.displayLabel,
        style: TextStyle(
          fontSize: 12,
          fontWeight: FontWeight.w600,
          color: _getStatusColor(),
        ),
      ),
    );
  }

  Color _getStatusColor() {
    return Color(sellerState.badgeColorValue);
  }

  IconData _getStatusIcon() {
    switch (sellerState.type) {
      case SellerStateType.notSeller:
        return Icons.person_outline;
      case SellerStateType.active:
        return Icons.store_outlined;
      case SellerStateType.expired:
        return Icons.error_outline;
    }
  }
}
