/// SellerInactiveBadge — shared widget for the expired-seller visibility
/// convergence. Renders a compact "Penjual tidak aktif" pill anywhere the
/// seller-trust lifecycle is degraded so buyers see clearly that the seller
/// cannot transact, while listing/auction content stays visible.
///
/// AXIS BOUNDARY: this badge is for the SELLER-TRUST axis only (subscription
/// expired/lapsed). The user-identity axis (banned/suspended/deleted) is
/// handled separately by the per-domain redaction widgets, which fully
/// remove or tombstone the surface — those callers SHOULD NOT also render
/// this badge.
library;

import 'package:flutter/material.dart';

import 'content_lifecycle.dart';

/// Returns true when the surface should render [SellerInactiveBadge] for
/// the given seller-trust lifecycle. False for active or when the surface
/// is already showing a user-axis redaction (which fully tombstones).
bool shouldShowSellerInactiveBadge({
  required ContentLifecycle sellerTrustLifecycle,
  required ContentLifecycle sellerUserLifecycle,
}) {
  if (sellerUserLifecycle != ContentLifecycle.active) {
    // User-axis already handles the surface (tombstone/redaction).
    return false;
  }
  return sellerTrustLifecycle != ContentLifecycle.active;
}

/// Compact inactive-seller pill. Default copy is Indonesian
/// "Penjual tidak aktif"; callers can pass [label] for context-specific
/// strings (e.g. "Langganan penjual berakhir" for the seller-detail card).
class SellerInactiveBadge extends StatelessWidget {
  final String label;
  final EdgeInsetsGeometry padding;

  const SellerInactiveBadge({
    super.key,
    this.label = 'Penjual tidak aktif',
    this.padding = const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: padding,
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.pause_circle_outline,
            size: 14,
            color: theme.colorScheme.onSurfaceVariant,
          ),
          const SizedBox(width: 4),
          Text(
            label,
            style: theme.textTheme.labelSmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w500,
            ),
          ),
        ],
      ),
    );
  }
}
