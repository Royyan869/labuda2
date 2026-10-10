import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/shared.dart';

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
    return Opacity(
      opacity: opacity,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          // Name row dengan verification badge
          _buildNameRow(context),

          // Farm name badge (seller only)
          if (farmName != null && farmName!.isNotEmpty) ...[
            const SizedBox(height: 4),
            _buildFarmNameBadge(context),
          ],

          // Username
          const SizedBox(height: 2),
          _buildUsername(context),

          // Location
          if (location != null && location!.isNotEmpty) ...[
            const SizedBox(height: 4),
            _buildLocation(context),
          ],

          // Bio (optional, biasanya di bawah avatar section)
          if (showBio && bio != null && bio!.trim().isNotEmpty) ...[
            const SizedBox(height: 12),
            _buildBio(context),
          ],
        ],
      ),
    );
  }

  Widget _buildNameRow(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Flexible(
          child: Text(
            name,
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
              color: scheme.onSurface,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        if (isVerified) ...[
          const SizedBox(width: 4),
          Icon(
            Icons.verified,
            size: AppIconSize.action,
            color: context.statusColors.info,
          ),
        ],
      ],
    );
  }

  Widget _buildFarmNameBadge(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p8,
        vertical: AppMetrics.p4,
      ),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHigh,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.storefront,
            size: AppIconSize.inlineGlyph,
            color: scheme.onSurfaceVariant,
          ),
          const SizedBox(width: 4),
          Flexible(
            child: Text(
              farmName!,
              style: context.typeRoles.labelMicro.copyWith(
                fontWeight: FontWeight.w500,
                color: scheme.onSurface,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildUsername(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Text(
      username.startsWith('@') ? username : '@$username',
      style: context.typeRoles.bodyDense.copyWith(
        color: scheme.onSurfaceVariant,
      ),
    );
  }

  Widget _buildLocation(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return AddressLocationView(
      location: location!,
      mode: AddressLocationMode.compact,
      icon: Icons.location_on_outlined,
      iconSize: AppIconSize.inlineGlyph,
      spacing: 4,
      mainAxisSize: MainAxisSize.min,
      style: context.typeRoles.bodyDense.copyWith(
        color: scheme.onSurfaceVariant,
      ),
    );
  }

  Widget _buildBio(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Text(
      bio!,
      style: context.typeRoles.bodyDense.copyWith(
        color: scheme.onSurface.withValues(alpha: 0.9),
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
    final scheme = Theme.of(context).colorScheme;

    return Opacity(
      opacity: opacity,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            name,
            style: context.typeRoles.titleCompact.copyWith(
              fontWeight: FontWeight.w600,
              color: scheme.onSurface,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
          Text(
            username.startsWith('@') ? username : '@$username',
            style: context.typeRoles.labelMicro.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}
