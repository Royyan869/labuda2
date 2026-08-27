/// SellerTierBadge — shared widget for the public seller reputation badge.
/// Renders a compact "Pro Seller" or "Elite Seller" pill on commerce-trust
/// surfaces: profile header, listing detail, auction detail.
///
/// VISIBILITY RULES (backend-enforced + mobile lifecycle gate):
///   - Only "pro" and "elite" tiers are displayed. Basic tier = no badge.
///   - Null/unknown tier = no badge (graceful hide).
///   - Suspended/banned/deleted sellers never receive tier from backend.
///   - Expired-subscription sellers never receive tier from backend.
///   - Mobile additionally suppresses the badge when sellerTrustLifecycle
///     is not active — see _ListingSellerCard and AuctionSellerCard.
///
/// The backend controls visibility via ENABLE_PUBLIC_SELLER_TIER_PROFILE
/// feature flag + lifecycle gates. Mobile simply renders what the wire
/// provides; if seller_tier is null/absent, the widget returns SizedBox.shrink().
library;

import 'package:flutter/material.dart';
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
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final l10n = AppLocalizations.of(context);
    final config = _tierConfig(tier, isDark: isDark, l10n: l10n);
    if (config == null) return const SizedBox.shrink();

    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: config.backgroundColor,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: config.borderColor, width: 1),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(config.icon, size: 14, color: config.foregroundColor),
          const SizedBox(width: 4),
          Text(
            config.label,
            style: theme.textTheme.labelSmall?.copyWith(
              color: config.foregroundColor,
              fontWeight: FontWeight.w600,
              letterSpacing: 0.2,
            ),
          ),
        ],
      ),
    );
  }

  /// Returns display config for known tiers, or null for basic/unknown/null.
  /// Colors adapt to [isDark]; labels use [l10n] when available, falling back
  /// to English strings for contexts without a locale (e.g. plain widget tests).
  static _TierDisplayConfig? _tierConfig(
    String? tier, {
    required bool isDark,
    AppLocalizations? l10n,
  }) {
    switch (tier) {
      case 'pro':
        return _TierDisplayConfig(
          label: l10n?.sellerTierPro ?? 'Pro Seller',
          icon: Icons.star_rounded,
          backgroundColor: isDark
              ? const Color(0xFF2C1E00) // dark amber tint
              : const Color(0xFFFFF8E1), // amber-50
          borderColor: const Color(0xFFFFD54F), // amber-300 (both modes)
          foregroundColor: isDark
              ? const Color(0xFFFFE082) // amber-200
              : const Color(0xFFF57F17), // amber-900
        );
      case 'elite':
        return _TierDisplayConfig(
          label: l10n?.sellerTierElite ?? 'Elite Seller',
          icon: Icons.workspace_premium_rounded,
          backgroundColor: isDark
              ? const Color(0xFF1A1A3A) // dark indigo tint
              : const Color(0xFFE8EAF6), // indigo-50
          borderColor: const Color(0xFF7986CB), // indigo-300 (both modes)
          foregroundColor: isDark
              ? const Color(0xFF9FA8DA) // indigo-200
              : const Color(0xFF283593), // indigo-800
        );
      default:
        return null; // basic, null, unknown → no badge
    }
  }
}

class _TierDisplayConfig {
  final String label;
  final IconData icon;
  final Color backgroundColor;
  final Color borderColor;
  final Color foregroundColor;

  const _TierDisplayConfig({
    required this.label,
    required this.icon,
    required this.backgroundColor,
    required this.borderColor,
    required this.foregroundColor,
  });
}
