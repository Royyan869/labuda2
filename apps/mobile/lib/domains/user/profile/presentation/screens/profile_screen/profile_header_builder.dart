import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart' hide ProfileAvatar;
import 'package:labuda/shared/governance/seller_tier_badge.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/profile_cover.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/profile_avatar.dart';

/// Constants for profile header animations
class ProfileHeaderConstants {
  static const double headerCollapsedHeight = 60.0;
  static const double coverPhotoHeight = 160.0;
  static const double avatarSize = 96.0;
  static const double avatarCollapsedSize = 40.0;
  static const double profileInfoBaseHeight = 56.0;
}

/// Builder for the expanded header with flying avatar animation
class ProfileExpandedHeaderBuilder extends StatelessWidget {
  final String userId;
  final bool isDark;
  final bool isSeller;
  final bool isOwnProfile;
  final Map<String, dynamic> profileData;
  final double headerExpandedHeight;
  final double collapseProgress;

  const ProfileExpandedHeaderBuilder({
    super.key,
    required this.userId,
    required this.isDark,
    required this.isSeller,
    required this.isOwnProfile,
    required this.profileData,
    required this.headerExpandedHeight,
    required this.collapseProgress,
  });

  @override
  Widget build(BuildContext context) {
    final statusBarHeight = MediaQuery.of(context).padding.top;
    final coverHeight =
        ProfileHeaderConstants.coverPhotoHeight + statusBarHeight;

    // Content fades out as we scroll
    final contentOpacity = (1 - collapseProgress * 1.5).clamp(0.0, 1.0);

    return Stack(
      fit: StackFit.expand,
      clipBehavior: Clip.hardEdge,
      children: [
        // Cover photo
        Positioned(
          top: 0,
          left: 0,
          right: 0,
          height: coverHeight,
          child: ProfileCover(
            coverPhotoUrl: profileData['coverPhotoUrl'],
            height: coverHeight,
            isOwnProfile: isOwnProfile,
          ),
        ),

        // Content area
        Positioned(
          top: coverHeight,
          left: 0,
          right: 0,
          height: headerExpandedHeight - coverHeight,
          child: Opacity(
            opacity: contentOpacity,
            child: Container(
              color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
              padding: const EdgeInsets.only(left: 16, right: 16, top: 8),
              child: _ProfileInfoContent(
                isDark: isDark,
                profileData: profileData,
              ),
            ),
          ),
        ),

        // Flying avatar with name
        _FlyingAvatarWithInfo(
          isDark: isDark,
          profileData: profileData,
          isSeller: isSeller,
          userId: userId,
          coverHeight: coverHeight,
          collapseProgress: collapseProgress,
        ),
      ],
    );
  }
}

/// Profile info content (location + bio)
class _ProfileInfoContent extends StatelessWidget {
  final bool isDark;
  final Map<String, dynamic> profileData;

  const _ProfileInfoContent({required this.isDark, required this.profileData});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        // Spacer for avatar overlap area
        const SizedBox(height: 44),

        // Seller tier badge (pro/elite only; null/basic = hidden)
        if (profileData['sellerTier'] != null) ...[
          const SizedBox(height: 8),
          SellerTierBadge(tier: profileData['sellerTier'] as String?),
        ],

        // Location
        if (profileData['location'] != null &&
            profileData['location'].toString().isNotEmpty) ...[
          const SizedBox(height: 8),
          Row(
            children: [
              Icon(
                Icons.location_on_outlined,
                size: 14,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray500,
              ),
              const SizedBox(width: 4),
              Expanded(
                child: Text(
                  profileData['location'],
                  style: TextStyle(
                    fontSize: 12,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray500,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
        ],

        // Bio
        if (profileData['bio'] != null &&
            profileData['bio'].toString().trim().isNotEmpty) ...[
          const SizedBox(height: 6),
          Text(
            profileData['bio'],
            style: TextStyle(
              fontSize: 13,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
              height: 1.3,
            ),
          ),
        ],
      ],
    );
  }
}

/// Flying avatar with name animation
class _FlyingAvatarWithInfo extends StatelessWidget {
  final bool isDark;
  final Map<String, dynamic> profileData;
  final bool isSeller;
  final String userId;
  final double coverHeight;
  final double collapseProgress;

  const _FlyingAvatarWithInfo({
    required this.isDark,
    required this.profileData,
    required this.isSeller,
    required this.userId,
    required this.coverHeight,
    required this.collapseProgress,
  });

  double _lerp(double start, double end, double progress) {
    return start + (end - start) * progress;
  }

  @override
  Widget build(BuildContext context) {
    final statusBarHeight = MediaQuery.of(context).padding.top;

    // Avatar animation positions
    final avatarStartTop =
        coverHeight - (ProfileHeaderConstants.avatarSize / 2);
    const avatarStartLeft = 16.0;
    final avatarStartSize = ProfileHeaderConstants.avatarSize;

    final avatarEndTop =
        statusBarHeight +
        (kToolbarHeight - ProfileHeaderConstants.avatarCollapsedSize) / 2;
    const avatarEndLeft = 56.0;
    final avatarEndSize = ProfileHeaderConstants.avatarCollapsedSize;

    final currentAvatarTop = _lerp(
      avatarStartTop,
      avatarEndTop,
      collapseProgress,
    );
    final currentAvatarLeft = _lerp(
      avatarStartLeft,
      avatarEndLeft,
      collapseProgress,
    );
    final currentAvatarSize = _lerp(
      avatarStartSize,
      avatarEndSize,
      collapseProgress,
    );

    // Identity animation positions
    final textStartTop = coverHeight + 8;
    final textStartLeft = 16 + ProfileHeaderConstants.avatarSize + 12.0;
    const nameStartSize = 18.0;
    const farmStartSize = 13.0;

    final textEndTop = statusBarHeight + (kToolbarHeight - 36) / 2;
    final textEndLeft = 56.0 + ProfileHeaderConstants.avatarCollapsedSize + 8.0;
    const nameEndSize = 14.0;
    const farmEndSize = 11.0;

    const textStartRight = 16.0;
    const textEndRight = 150.0;

    final currentTextTop = _lerp(textStartTop, textEndTop, collapseProgress);
    final currentTextLeft = _lerp(textStartLeft, textEndLeft, collapseProgress);
    final currentTextRight = _lerp(
      textStartRight,
      textEndRight,
      collapseProgress,
    );
    final currentNameSize = _lerp(nameStartSize, nameEndSize, collapseProgress);
    final currentFarmSize = _lerp(farmStartSize, farmEndSize, collapseProgress);

    // Colors
    final nameColor = isDark
        ? AppColors.neutralWhite
        : AppColors.neutralGray900;
    final usernameExpandedColor = isDark
        ? AppColors.neutralGray400
        : AppColors.neutralGray500;
    final usernameColor = Color.lerp(
      usernameExpandedColor,
      nameColor,
      collapseProgress,
    )!;

    return Stack(
      children: [
        // Flying Avatar
        Positioned(
          top: currentAvatarTop,
          left: currentAvatarLeft,
          child: ProfileAvatar(
            userId: userId,
            avatarUrl: profileData['avatar'],
            farmPhotoUrl: profileData['farmPhotoUrl'],
            initials: UserInitialsHelper.fromName(profileData['name']),
            isSeller: isSeller,
            size: currentAvatarSize,
            showOnlineStatus: collapseProgress < 0.5,
          ),
        ),

        // Flying identity lines
        Positioned(
          top: currentTextTop,
          left: currentTextLeft,
          right: currentTextRight,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                profileData['name'],
                style: TextStyle(
                  fontSize: currentNameSize,
                  fontWeight: FontWeight.bold,
                  color: nameColor,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              if (profileData['farmName'] != null &&
                  profileData['farmName'].toString().isNotEmpty) ...[
                Text(
                  profileData['farmName'],
                  style: TextStyle(
                    fontSize: currentFarmSize,
                    color: usernameColor,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ],
          ),
        ),
      ],
    );
  }
}

/// Helper class for calculating header heights
class ProfileHeaderCalculator {
  /// Calculate dynamic header height based on profile content
  static double calculateExpandedHeight({
    required double statusBarHeight,
    required bool hasLocation,
    required String? bio,
    required double availableWidth,
  }) {
    double height =
        ProfileHeaderConstants.coverPhotoHeight +
        statusBarHeight +
        ProfileHeaderConstants.profileInfoBaseHeight;

    if (hasLocation) {
      height += 26.0;
    }

    if (bio != null && bio.trim().isNotEmpty) {
      final bioHeight = _measureTextHeight(
        text: bio,
        style: const TextStyle(fontSize: 13, height: 1.3),
        maxWidth: availableWidth,
      );
      height += 6.0 + bioHeight;
    }

    height += 12.0; // Buffer

    return height;
  }

  static double _measureTextHeight({
    required String text,
    required TextStyle style,
    required double maxWidth,
  }) {
    final textPainter = TextPainter(
      text: TextSpan(text: text, style: style),
      textDirection: TextDirection.ltr,
      maxLines: null,
    )..layout(maxWidth: maxWidth);

    return textPainter.height;
  }

  /// Calculate collapse progress (0.0 = expanded, 1.0 = collapsed)
  static double calculateCollapseProgress(double scrollOffset) {
    const collapseStart =
        ProfileHeaderConstants.coverPhotoHeight -
        ProfileHeaderConstants.headerCollapsedHeight;
    if (scrollOffset <= 0) return 0.0;
    if (scrollOffset >= collapseStart) return 1.0;
    return scrollOffset / collapseStart;
  }
}
