import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Info section untuk Profile V2
///
/// Features:
/// - Nama dengan verification badge
/// - Farm name badge (seller only)
/// - Username (@handle)
/// - Location dengan icon
/// - Bio text
class ProfileInfo extends StatelessWidget {
  final String name;
  final String? farmName;
  final String username;
  final String? location;
  final String? bio;
  final bool isVerified;
  final bool showBio;
  final double opacity;

  const ProfileInfo({
    super.key,
    required this.name,
    this.farmName,
    required this.username,
    this.location,
    this.bio,
    this.isVerified = false,
    this.showBio = true,
    this.opacity = 1.0,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Opacity(
      opacity: opacity,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          // Name row dengan verification badge
          _buildNameRow(isDark),

          // Farm name badge (seller only)
          if (farmName != null && farmName!.isNotEmpty) ...[
            const SizedBox(height: 4),
            _buildFarmNameBadge(isDark),
          ],

          // Username
          const SizedBox(height: 2),
          _buildUsername(isDark),

          // Location
          if (location != null && location!.isNotEmpty) ...[
            const SizedBox(height: 4),
            _buildLocation(isDark),
          ],

          // Bio (optional, biasanya di bawah avatar section)
          if (showBio && bio != null && bio!.trim().isNotEmpty) ...[
            const SizedBox(height: 12),
            _buildBio(isDark),
          ],
        ],
      ),
    );
  }

  Widget _buildNameRow(bool isDark) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Flexible(
          child: Text(
            name,
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        if (isVerified) ...[
          const SizedBox(width: 4),
          Icon(Icons.verified, size: 18, color: AppColors.statusInfo),
        ],
      ],
    );
  }

  Widget _buildFarmNameBadge(bool isDark) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.storefront,
            size: 14,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
          const SizedBox(width: 4),
          Flexible(
            child: Text(
              farmName!,
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w500,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildUsername(bool isDark) {
    return Text(
      username.startsWith('@') ? username : '@$username',
      style: TextStyle(
        fontSize: 14,
        color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
      ),
    );
  }

  Widget _buildLocation(bool isDark) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          Icons.location_on_outlined,
          size: 14,
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
        ),
        const SizedBox(width: 4),
        Flexible(
          child: Text(
            location!,
            style: TextStyle(
              fontSize: 13,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );
  }

  Widget _buildBio(bool isDark) {
    return Text(
      bio!,
      style: TextStyle(
        fontSize: 14,
        color: isDark
            ? AppColors.neutralWhite.withValues(alpha: 0.9)
            : AppColors.neutralGray700,
        height: 1.4,
      ),
      maxLines: 3,
      overflow: TextOverflow.ellipsis,
    );
  }
}

/// Compact version untuk AppBar (saat collapsed)
class ProfileInfoCompact extends StatelessWidget {
  final String name;
  final String username;
  final double opacity;

  const ProfileInfoCompact({
    super.key,
    required this.name,
    required this.username,
    this.opacity = 1.0,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Opacity(
      opacity: opacity,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            name,
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.w600,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
          Text(
            username.startsWith('@') ? username : '@$username',
            style: TextStyle(
              fontSize: 12,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
          ),
        ],
      ),
    );
  }
}
