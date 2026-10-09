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
import 'package:labuda/domains/social/rating/rating.dart';
import 'package:intl/intl.dart';
import 'package:url_launcher/url_launcher.dart';

/// About tab content for profile screen - displays user bio, farm info, achievements, and contact
///
/// F5-local fit measure: whether a single-line title + badge pair fits the
/// incoming width. Local copy (no new shared authority).
bool _fitsTitleBadgeSingleLine({
  required BuildContext context,
  required double maxWidth,
  required String title,
  required String badge,
  required TextStyle? titleStyle,
  required TextStyle? badgeStyle,
  required double fixedExtrasWidth,
}) {
  if (!maxWidth.isFinite) {
    return false;
  }
  // NOTE: no explicit TextDirection type here — package:intl (imported by
  // this file) exports its own TextDirection; inference keeps dart:ui's.
  final direction = Directionality.of(context);
  final TextScaler scaler = MediaQuery.textScalerOf(context);

  double singleLineWidth(String text, TextStyle? style) {
    final TextPainter painter = TextPainter(
      text: TextSpan(text: text, style: style),
      textDirection: direction,
      textScaler: scaler,
      maxLines: 1,
    )..layout();
    return painter.width;
  }

  const double safetyMargin = 2;
  return singleLineWidth(title, titleStyle) +
          fixedExtrasWidth +
          singleLineWidth(badge, badgeStyle) +
          safetyMargin <=
      maxWidth;
}

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
    final scheme = Theme.of(context).colorScheme;

    final dataAsync = ref.watch(profileAboutDataProvider(userId));

    return dataAsync.when(
      data: (data) => _buildContent(context, data, scheme, ref),
      loading: () => _buildLoading(),
      error: (error, stack) => _buildError(context, error.toString(), scheme),
    );
  }

  Widget _buildContent(
    BuildContext context,
    ProfileAboutData data,
    ColorScheme scheme,
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
      return _buildDegradedPlaceholder(context, scheme, data.user.lifecycle);
    }

    // Get seller state for the current profile.
    final sellerState = isOwnProfile
        ? _currentAccountSellerState(ref)
        : SellerState.fromAuthUser(data.user);
    final showSellerSections = sellerState?.isSeller ?? false;

    return CustomScrollView(
      slivers: [
        SliverPadding(
          padding: const EdgeInsets.all(AppMetrics.p16),
          sliver: SliverList(
            delegate: SliverChildListDelegate([
              // Section 0: Seller Status Badge (for own profile or seller profiles)
              if (isOwnProfile && sellerState == null) ...[
                _buildPendingSellerStatusCard(context: context, scheme: scheme),
                const SizedBox(height: 16),
              ] else if (sellerState != null &&
                  (isOwnProfile || sellerState.isSeller)) ...[
                _SellerStatusBadge(sellerState: sellerState, scheme: scheme),
                const SizedBox(height: 16),
              ],

              // Section 1: About
              if (data.bio.isNotEmpty || data.location != null) ...[
                _ProfileSectionCard(
                  title: 'About',
                  icon: Icons.person_outline,
                  child: _buildAboutSection(context, data, scheme),
                ),
                const SizedBox(height: 16),
              ],

              // Section 2: Farm Info (seller only)
              if (showSellerSections && data.farmInfo != null) ...[
                _ProfileSectionCard(
                  title: 'Informasi Farm',
                  icon: Icons.store_outlined,
                  child: _buildFarmInfoSection(context, data, scheme),
                ),
                const SizedBox(height: 16),
              ],

              // Section 3: Verification Badges (REAL data only)
              // REMOVED: Achievements section - NO backend support, deleted in PROFILE PURGE
              if (_hasAnyVerificationBadges(data)) ...[
                _ProfileSectionCard(
                  title: 'Verification',
                  icon: Icons.verified_outlined,
                  child: _buildVerificationSection(data, scheme),
                ),
                const SizedBox(height: 16),
              ],

              // Section 4: Rating & Reviews (seller only - uses REAL data from rating module)
              // REMOVED: Fake metrics (Total Penjualan, Completion Rate) - NO backend support
              if (showSellerSections) ...[
                _ProfileSectionCard(
                  title: 'Rating & Reviews',
                  icon: Icons.star_outline,
                  child: _buildRatingSection(scheme, ref),
                ),
                const SizedBox(height: 16),
              ],

              // Section 5: Contact Information
              if (_shouldShowContact(data)) ...[
                _ProfileSectionCard(
                  title: 'Contact Information',
                  icon: Icons.contact_phone_outlined,
                  child: _buildContactSection(context, data, scheme),
                ),
              ],
            ]),
          ),
        ),
      ],
    );
  }

  /// Own-profile seller state — canonical RF-02 authority.
  ///
  /// Delegates entirely to `SellerState.fromAuthUser` (seller domain), which
  /// derives state from the hydrated backend snapshot's two canonical axes:
  /// IDENTITY (`hasSellerProfile`) and EXPIRY (`sellerSubscriptionStatus`).
  /// Only `sellerSubscriptionStatus == 'expired'` may produce
  /// `SellerState.expired()`; capability (`hasMarketAuthority`) false means
  /// "cannot sell right now", never "subscription ended". `null` here means
  /// the backend snapshot is not hydrated yet (pending seller-status UI).
  SellerState? _currentAccountSellerState(WidgetRef ref) {
    final user = ref.watch(authenticatedUserProvider);
    if (user == null) {
      return null;
    }
    return SellerState.fromAuthUser(user);
  }

  Widget _buildPendingSellerStatusCard({
    required BuildContext context,
    required ColorScheme scheme,
  }) {
    final background = scheme.surfaceContainerHighest;
    final border = scheme.outlineVariant;
    final textPrimary = scheme.onSurfaceVariant;

    return Container(
      margin: const EdgeInsets.only(bottom: AppMetrics.p16),
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: background,
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(color: border),
      ),
      child: Row(
        children: [
          Icon(
            Icons.hourglass_top_outlined,
            size: AppIconSize.action,
            color: scheme.secondary,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              'Checking seller status...',
              style: context.typeRoles.bodyDense.copyWith(
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
  Widget _buildAboutSection(
    BuildContext context,
    ProfileAboutData data,
    ColorScheme scheme,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Bio
        if (data.bio.isNotEmpty) ...[
          Text(
            data.bio,
            style: context.typeRoles.bodyDense.copyWith(
              height: 1.5,
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 16),
        ],

        // Location — compact Address/Location authority (one bounded line).
        if (data.location != null) ...[
          AddressLocationView(
            location: data.location!,
            mode: AddressLocationMode.compact,
            icon: Icons.location_on_outlined,
            iconSize: AppIconSize.inlineGlyph,
            style: context.typeRoles.bodyDense.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 8),
        ],

        // Join date — compact Metadata authority (one bounded line).
        MetadataView(
          text: _formatJoinDate(data.joinedAt),
          mode: MetadataMode.compact,
          icon: Icons.calendar_today_outlined,
          iconSize: AppIconSize.inlineGlyph,
          style: context.typeRoles.bodyDense.copyWith(
            color: scheme.onSurfaceVariant,
          ),
        ),

        // Last active — compact Metadata authority (one bounded line).
        if (data.lastActiveAt != null) ...[
          const SizedBox(height: 8),
          MetadataView(
            text: _formatLastActive(data.lastActiveAt!),
            mode: MetadataMode.compact,
            icon: Icons.access_time,
            iconSize: AppIconSize.inlineGlyph,
            style: context.typeRoles.bodyDense.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ],
    );
  }

  // Section 2: Farm Info
  Widget _buildFarmInfoSection(
    BuildContext context,
    ProfileAboutData data,
    ColorScheme scheme,
  ) {
    final farmInfo = data.farmInfo!;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (farmInfo.farmName.isNotEmpty)
          _ProfileInfoRow(label: 'Farm Name', value: farmInfo.farmName),

        // Canonical seller description comes from AuthUser.bio
        if (data.bio.isNotEmpty) ...[
          const SizedBox(height: 8),
          Text(
            'Description',
            style: context.typeRoles.bodyDense.copyWith(
              fontWeight: FontWeight.w600,
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            data.bio,
            style: context.typeRoles.bodyDense.copyWith(
              height: 1.5,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ],
    );
  }

  // Section 3: Verification Badges (REAL data only)
  // REMOVED: Achievements section - NO backend support, deleted in PROFILE PURGE
  Widget _buildVerificationSection(ProfileAboutData data, ColorScheme scheme) {
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
  Widget _buildRatingSection(ColorScheme scheme, WidgetRef ref) {
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
                scheme: scheme,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: _StatCard(
                label: 'Total Reviews',
                value: totalReviews.toString(),
                icon: Icons.rate_review_outlined,
                scheme: scheme,
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
              scheme: scheme,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: _StatCard(
              label: 'Total Reviews',
              value: '...',
              icon: Icons.rate_review_outlined,
              scheme: scheme,
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
              scheme: scheme,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: _StatCard(
              label: 'Total Reviews',
              value: '0',
              icon: Icons.rate_review_outlined,
              scheme: scheme,
            ),
          ),
        ],
      ),
    );
  }

  // Section 5: Contact Information
  Widget _buildContactSection(
    BuildContext context,
    ProfileAboutData data,
    ColorScheme scheme,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Email — shown only on own profile (private contact).
        if (isOwnProfile) ...[
          if (data.maskedEmail != null) ...[
            Row(
              children: [
                Icon(
                  Icons.email_outlined,
                  size: AppIconSize.action,
                  color: scheme.onSurfaceVariant,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    data.maskedEmail!,
                    style: context.typeRoles.bodyDense.copyWith(
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
          ],
        ],

        // Phone — shown only on own profile (private contact).
        if (isOwnProfile) ...[
          if (data.maskedPhone != null) ...[
            Row(
              children: [
                Icon(
                  Icons.phone_outlined,
                  size: AppIconSize.action,
                  color: scheme.onSurfaceVariant,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    data.maskedPhone!,
                    style: context.typeRoles.bodyDense.copyWith(
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
          ],
        ],

        // Social Media — presence of a handle is the visibility authority.
        if (data.hasSocialMedia) ...[
          Divider(color: scheme.outlineVariant),
          const SizedBox(height: 8),
          Text(
            'Social Media',
            style: context.typeRoles.bodyDense.copyWith(
              fontWeight: FontWeight.w600,
              color: scheme.onSurfaceVariant,
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
                  scheme: scheme,
                ),
              if (data.facebookHandle != null)
                _SocialMediaChip(
                  icon: Icons.facebook,
                  label: data.facebookHandle!,
                  url: _getFacebookUrl(data.facebookHandle!),
                  scheme: scheme,
                ),
              if (data.tiktokHandle != null)
                _SocialMediaChip(
                  icon: Icons.play_circle_outline,
                  label: data.tiktokHandle!,
                  url: _getTiktokUrl(data.tiktokHandle!),
                  scheme: scheme,
                ),
              if (data.twitterHandle != null)
                _SocialMediaChip(
                  icon: Icons.chat_bubble_outline,
                  label: data.twitterHandle!,
                  url: _getTwitterUrl(data.twitterHandle!),
                  scheme: scheme,
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
  Widget _buildDegradedPlaceholder(
    BuildContext context,
    ColorScheme scheme,
    ContentLifecycle lifecycle,
  ) {
    final label = lifecycle.publicRedactionLabel;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.lock_outline,
              size: AppIconSize.display,
              color: scheme.onSurfaceVariant,
            ),
            const SizedBox(height: 16),
            Text(
              label,
              style: context.typeRoles.titleCompact.copyWith(
                fontWeight: FontWeight.w600,
                color: scheme.onSurfaceVariant,
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
        padding: EdgeInsets.all(AppMetrics.p32),
        child: CircularProgressIndicator(),
      ),
    );
  }

  // Error state
  Widget _buildError(BuildContext context, String error, ColorScheme scheme) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.error_outline,
              size: AppIconSize.display,
              color: scheme.onSurfaceVariant,
            ),
            const SizedBox(height: 16),
            Text(
              'Failed to load profile',
              style: context.typeRoles.titleCompact.copyWith(
                fontWeight: FontWeight.w600,
                color: scheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              error,
              style: context.typeRoles.bodyDense.copyWith(
                color: scheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }

  // === Helper Methods ===

  // Contact info: own profile always; others only when a public handle exists.
  // Presence of a social handle is the visibility authority (no toggle).
  bool _shouldShowContact(ProfileAboutData data) {
    if (isOwnProfile) return true;
    return data.hasSocialMedia;
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
    final scheme = Theme.of(context).colorScheme;

    return Card(
      margin: EdgeInsets.zero,
      color: scheme.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(icon, size: AppIconSize.action, color: scheme.primary),
                const SizedBox(width: 8),
                Text(
                  title,
                  style: context.typeRoles.titleCompact.copyWith(
                    fontWeight: FontWeight.w600,
                    color: scheme.onSurface,
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
    final scheme = Theme.of(context).colorScheme;

    return Padding(
      padding: const EdgeInsets.only(bottom: AppMetrics.p8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: AppContentSize.termLabel,
            child: Text(
              label,
              style: context.typeRoles.bodyDense.copyWith(
                color: scheme.onSurfaceVariant,
              ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: context.typeRoles.bodyDense.copyWith(
                fontWeight: FontWeight.w500,
                color: scheme.onSurfaceVariant,
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
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p12,
        vertical: AppMetrics.p8,
      ),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Text(
        text,
        style: context.typeRoles.labelMicro.copyWith(
          fontWeight: FontWeight.w500,
          color: scheme.onSurfaceVariant,
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
  final ColorScheme scheme;

  const _StatCard({
    required this.label,
    required this.value,
    required this.icon,
    required this.scheme,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: AppIconSize.action, color: scheme.primary),
          const SizedBox(height: 8),
          Text(
            value,
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            label,
            style: context.typeRoles.labelMicro.copyWith(
              color: scheme.onSurfaceVariant,
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
  final ColorScheme scheme;

  const _SocialMediaChip({
    required this.icon,
    required this.label,
    required this.url,
    required this.scheme,
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
            AppSnackBar.showError(context, 'Tidak dapat membuka tautan');
          }
        }
      },
      borderRadius: BorderRadius.circular(AppShape.r8),
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: AppMetrics.p12,
          vertical: AppMetrics.p8,
        ),
        decoration: BoxDecoration(
          color: scheme.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(AppShape.r8),
          border: Border.all(color: scheme.outlineVariant),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: AppIconSize.inlineGlyph, color: scheme.primary),
            const SizedBox(width: 8),
            Text(
              label,
              style: context.typeRoles.bodyDense.copyWith(
                color: scheme.onSurfaceVariant,
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
/// **SELLER UX ALIGNMENT (RF-02):**
/// - Displays the canonical `SellerState` produced by `SellerState.fromAuthUser`
/// - 4 states: NOT_SELLER, PENDING_ACTIVATION ('none'), ACTIVE, EXPIRED
/// - Expiry/renewal copy renders ONLY for EXPIRED (`sellerSubscriptionStatus
///   == 'expired'`); capability `hasMarketAuthority == false` is never expiry
class _SellerStatusBadge extends ConsumerWidget {
  final SellerState sellerState;
  final ColorScheme scheme;

  const _SellerStatusBadge({required this.sellerState, required this.scheme});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(
          color: _getStatusColor().withValues(alpha: 0.3),
          width: 1,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // F5 header (adaptive): title + seller-state badge share one row
          // when the single-line pair fits, else the title stacks over the
          // fully-readable badge.
          LayoutBuilder(
            builder: (BuildContext context, BoxConstraints constraints) {
              final TextStyle titleStyle = context.typeRoles.bodyDense
                  .copyWith(
                    fontWeight: FontWeight.w500,
                    color: scheme.onSurfaceVariant,
                  );
              final TextStyle badgeStyle = context.typeRoles.labelMicro
                  .copyWith(fontWeight: FontWeight.w600);
              final bool fits = _fitsTitleBadgeSingleLine(
                context: context,
                maxWidth: constraints.maxWidth,
                title: 'Status Penjual',
                badge: sellerState.displayLabel,
                titleStyle: titleStyle,
                badgeStyle: badgeStyle,
                fixedExtrasWidth:
                    AppIconSize.action + 8 + AppMetrics.p12 * 2,
              );
              if (fits) {
                return Row(
                  children: [
                    Icon(
                      _getStatusIcon(),
                      color: _getStatusColor(),
                      size: AppIconSize.action,
                    ),
                    const SizedBox(width: 8),
                    Text(
                      'Status Penjual',
                      style: context.typeRoles.bodyDense.copyWith(
                        fontWeight: FontWeight.w500,
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                    const Spacer(),
                    _buildStatusBadge(context),
                  ],
                );
              }
              return Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        _getStatusIcon(),
                        color: _getStatusColor(),
                        size: AppIconSize.action,
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          'Status Penjual',
                          style: context.typeRoles.bodyDense.copyWith(
                            fontWeight: FontWeight.w500,
                            color: scheme.onSurfaceVariant,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 4),
                  _buildStatusBadge(context),
                ],
              );
            },
          ),
          if (sellerState.isExpired && sellerState.bannerMessage != null) ...[
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: context.statusColors.error.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(AppShape.r8),
                border: Border.all(
                  color: context.statusColors.error.withValues(alpha: 0.3),
                ),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.warning_amber_rounded,
                    color: context.statusColors.error,
                    size: AppIconSize.action,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      sellerState.bannerMessage!,
                      style: context.typeRoles.bodyDense.copyWith(
                        color: scheme.onSurfaceVariant,
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

  Widget _buildStatusBadge(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p12,
        vertical: AppMetrics.p4,
      ),
      decoration: BoxDecoration(
        color: _getStatusColor().withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      child: Text(
        sellerState.displayLabel,
        style: context.typeRoles.labelMicro.copyWith(
          fontWeight: FontWeight.w600,
          color: _getStatusColor(),
        ),
        softWrap: true,
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
      case SellerStateType.pendingActivation:
        return Icons.hourglass_top_outlined;
      case SellerStateType.active:
        return Icons.store_outlined;
      case SellerStateType.expired:
        return Icons.error_outline;
    }
  }
}
