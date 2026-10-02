/// SellerTierBadge — shared widget for the public seller reputation badge.
/// Renders a compact "Pro Seller" or "Elite Seller" pill on commerce-trust
/// surfaces: profile header, forSale detail, auction detail.
///
/// VISIBILITY RULES (backend-enforced + mobile lifecycle gate):
///   - Only "pro" and "elite" tiers are displayed. Basic tier = no badge.
///   - Null/unknown tier = no badge (graceful hide).
///   - Suspended/banned/deleted sellers never receive tier from backend.
///   - Expired-subscription sellers never receive tier from backend.
///   - Mobile additionally suppresses the badge when sellerTrustLifecycle
///     is not active — see CommerceDetailSellerCard (render authority for
///     both ForSale and Auction detail).
///
/// The backend controls visibility via ENABLE_PUBLIC_SELLER_TIER_PROFILE
/// feature flag + lifecycle gates. Mobile simply renders what the wire
/// provides; if seller_tier is null/absent, the widget returns SizedBox.shrink().
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Renders a seller tier badge pill, or nothing if [tier] is null/basic/unknown.
///
/// [tier] is the raw wire value from the backend: "pro", "elite", or null.
/// Unknown values are treated as null (no badge shown).
class SellerTierBadge extends StatelessWidget {
  final String? tier;

  const SellerTierBadge({super.key, required this.tier});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final l10n = AppLocalizations.of(context);
    final config = _tierConfig(tier, l10n: l10n);
    if (config == null) return const SizedBox.shrink();

    // Tier hues: pro is a palette brand (no scheme role); elite reuses the
    // scheme secondary (same canonical blue). Tints derive from alpha over
    // the scheme surface so they adapt; the label is scheme ink.
    final brandColor = config.isPro
        ? AppColors.primaryYellow
        : scheme.secondary;
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p4),
      decoration: BoxDecoration(
        color: brandColor.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(
          color: brandColor.withValues(alpha: 0.45),
          width: 1,
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(config.icon, size: AppIconSize.inlineGlyph, color: brandColor),
          const SizedBox(width: 4),
          Text(
            config.label,
            style: theme.textTheme.labelSmall?.copyWith(
              color: scheme.onSurface,
              fontWeight: FontWeight.w600,
              letterSpacing: 0.2,
            ),
          ),
        ],
      ),
    );
  }

  /// Returns display config for known tiers, or null for basic/unknown/null.
  ///
  /// Single canonical definition per tier (no per-mode variants, no raw
  /// hex). Labels use [l10n] when available, falling back to English
  /// strings for contexts without a locale (e.g. plain widget tests).
  static _TierDisplayConfig? _tierConfig(
    String? tier, {
    AppLocalizations? l10n,
  }) {
    switch (tier) {
      case 'pro':
        return _TierDisplayConfig(
          label: l10n?.sellerTierPro ?? 'Pro Seller',
          icon: Icons.star_rounded,
          isPro: true,
        );
      case 'elite':
        return _TierDisplayConfig(
          label: l10n?.sellerTierElite ?? 'Elite Seller',
          icon: Icons.workspace_premium_rounded,
          isPro: false,
        );
      default:
        return null; // basic, null, unknown → no badge
    }
  }
}

class _TierDisplayConfig {
  final String label;
  final IconData icon;
  final bool isPro;

  const _TierDisplayConfig({
    required this.label,
    required this.icon,
    required this.isPro,
  });
}
