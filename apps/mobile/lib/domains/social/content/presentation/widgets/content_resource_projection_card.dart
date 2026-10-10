import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

/// Discovery-surface card for the canonical resource projection.
///
/// PRICE IS RENDERED ON EVERY SURFACE (owner decision, 2026-09-27): this card
/// and the chat card both render the money the envelope carries on LIVE, using
/// the same canonical strings (`formattedPrice` / `formattedAmount`).
class ContentResourceProjectionCard extends StatelessWidget {
  final ResourceProjection resourceProjection;
  final VoidCallback? onTap;
  final bool compact;

  const ContentResourceProjectionCard({
    super.key,
    required this.resourceProjection,
    this.onTap,
    this.compact = false,
  });

  @override
  Widget build(BuildContext context) {
    final resolvedTap = resourceProjection.isLive
        ? onTap ?? () => context.push(resourceProjection.resolvedPath)
        : null;

    final value = _valueText();

    return CommerceMarketplaceCardShell(
      onTap: resolvedTap,
      compact: compact,
      semanticLabel: resourceProjection.titleText,
      media: _buildMedia(context),
      title: resourceProjection.titleText,
      value: CommerceMarketplaceCardValue(
        value: value?.isNotEmpty == true ? value! : resourceProjection.typeLabel,
        caption: _captionLabel(),
        compact: compact,
      ),
      badges: _buildBadges(context),
      metadata: _buildMetadata(context),
      contentPadding: EdgeInsets.all(
        compact ? AppMetrics.p12 : AppMetrics.p12,
      ),
    );
  }

  /// Lifecycle caption. Commerce payloads use the shared presentation mapping
  /// (the same one the chat card uses); non-commerce payloads keep the raw
  /// projected lifecycle; tombstone stays 'TOMBSTONE' or the raw status.
  String _captionLabel() {
    if (resourceProjection.isLive) {
      final commerce = commerceLifecycleLabel(resourceProjection.payload);
      if (commerce != null) return commerce;
    }
    return resourceProjection.statusLabel ?? 'TOMBSTONE';
  }

  /// Value: identity for profile/content, canonical money for commerce — the
  /// same strings the chat card renders (one formatting authority).
  String? _valueText() {
    final p = resourceProjection.payload;
    return switch (p) {
      ProfileLivePayload(:final username) => username,
      ContentLivePayload(:final author) => author.username,
      ForSaleLivePayload sale => sale.formattedPrice,
      AuctionLivePayload auction => auction.formattedAmount,
      null => null,
    };
  }

  Widget _buildMedia(BuildContext context) {
    final isProfile =
        resourceProjection.resourceType == ResourceProjectionType.profile;
    return CommerceMarketplaceCardMedia(
      imageUrl: resourceProjection.primaryImageUrl,
      aspectRatio: isProfile ? 1 : 4 / 3,
      showVideoBadge: false,
      borderRadius: const BorderRadius.only(
        topLeft: Radius.circular(AppShape.r16),
        topRight: Radius.circular(AppShape.r16),
      ),
      fallback: _placeholderMedia(
        context,
        _iconForType(resourceProjection.resourceType),
      ),
    );
  }

  Widget? _buildMetadata(BuildContext context) {
    final parts = <String>[resourceProjection.typeLabel];
    final nested = resourceProjection.nestedResourceLabel;
    if (nested != null) {
      parts.add(nested);
    }
    parts.add(resourceProjection.isLive ? 'LIVE' : 'TOMBSTONE');
    return Text(
      parts.join(' - '),
      style: Theme.of(context).textTheme.bodySmall?.copyWith(
        color: Theme.of(context).colorScheme.onSurfaceVariant,
      ),
      maxLines: 2,
      overflow: TextOverflow.ellipsis,
    );
  }

  List<Widget> _buildBadges(BuildContext context) {
    final badges = <Widget>[
      CommerceMarketplaceCardBadge(
        label: resourceProjection.typeLabel,
        compact: true,
      ),
      CommerceMarketplaceCardBadge(
        label: resourceProjection.isLive ? 'LIVE' : 'TOMBSTONE',
        compact: true,
      ),
    ];

    final nested = resourceProjection.nestedResourceLabel;
    if (nested != null) {
      badges.add(CommerceMarketplaceCardBadge(label: nested, compact: true));
    }

    // Product attribute (informational only): a for-sale listing that is
    // negotiable, active and in stock shows "Nego". It is never actionable.
    final payload = resourceProjection.payload;
    if (payload is ForSaleLivePayload && payload.negotiationEnabled) {
      badges.add(
        const CommerceMarketplaceCardBadge(label: 'Nego', compact: true),
      );
    }

    return badges;
  }

  Widget _placeholderMedia(BuildContext context, IconData icon) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      color: scheme.surfaceContainerHighest,
      child: Center(
        child: Icon(icon, color: scheme.onSurfaceVariant, size: AppIconSize.emphasis),
      ),
    );
  }

  IconData _iconForType(ResourceProjectionType type) {
    switch (type) {
      case ResourceProjectionType.profile:
        return Icons.person_outline_rounded;
      case ResourceProjectionType.content:
        return Icons.article_outlined;
      case ResourceProjectionType.fixedPriceSale:
        return Icons.storefront_outlined;
      case ResourceProjectionType.auction:
        return Icons.gavel_rounded;
    }
  }
}
