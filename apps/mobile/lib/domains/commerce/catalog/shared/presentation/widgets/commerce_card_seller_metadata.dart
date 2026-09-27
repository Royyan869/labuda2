import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_inactive_badge.dart';
import 'package:labuda/shared/utils/commerce_seller_identity.dart';

/// Canonical seller metadata block for commerce DISCOVERY cards.
///
/// ONE AUTHORITY for both sale channels: [ForSaleCard] and [AuctionCard]
/// render their seller block through this widget, so identity order,
/// redaction vocabulary, typography and the seller-trust badge cannot drift
/// between channels.
///
/// AXIS BOUNDARY:
///   - user-identity axis (banned/suspended/removed) → full redaction label,
///     handle + store are never rendered, no fabricated fallback identity.
///   - seller-trust axis (subscription expired) → [SellerInactiveBadge] below
///     the identity, content stays visible.
class CommerceCardSellerMetadata extends StatelessWidget {
  final String? username;
  final String? storeName;
  final ContentLifecycle sellerUserLifecycle;
  final ContentLifecycle sellerTrustLifecycle;

  const CommerceCardSellerMetadata({
    super.key,
    required this.username,
    required this.storeName,
    required this.sellerUserLifecycle,
    required this.sellerTrustLifecycle,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final degraded = sellerUserLifecycle.isDegraded;

    final identity = degraded
        ? null
        : buildCommerceSellerIdentity(username: username, storeName: storeName);

    // Hide rather than fabricate: no identity truth → nothing renders.
    if (!degraded && identity == null) return const SizedBox.shrink();

    final showInactiveBadge = shouldShowSellerInactiveBadge(
      sellerTrustLifecycle: sellerTrustLifecycle,
      sellerUserLifecycle: sellerUserLifecycle,
    );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          degraded ? sellerUserLifecycle.publicRedactionLabel : identity!.line1,
          style: AppTypography.bodySmall.copyWith(
            color: scheme.onSurfaceVariant,
            fontStyle: degraded ? FontStyle.italic : FontStyle.normal,
          ),
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
        ),
        if (!degraded && identity!.line2 != null) ...[
          const SizedBox(height: 2),
          Text(
            identity.line2!,
            style: AppTypography.bodySmall.copyWith(
              color: scheme.onSurfaceVariant,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ],
        if (showInactiveBadge) ...[
          const SizedBox(height: 6),
          const SellerInactiveBadge(),
        ],
      ],
    );
  }
}
