import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';

class ContentResourceProjectionCard extends StatelessWidget {
  final ContentResourceProjection resourceProjection;
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
        ? onTap ?? () => context.push(resourceProjection.canonicalPath)
        : null;

    return CommerceMarketplaceCardShell(
      onTap: resolvedTap,
      compact: compact,
      semanticLabel: resourceProjection.titleText,
      media: _buildMedia(context),
      title: resourceProjection.titleText,
      value: CommerceMarketplaceCardValue(
        value: resourceProjection.valueText?.isNotEmpty == true
            ? resourceProjection.valueText!
            : resourceProjection.typeLabel,
        caption: resourceProjection.statusText,
        compact: compact,
      ),
      badges: _buildBadges(context),
      metadata: _buildMetadata(context),
      contentPadding: EdgeInsets.all(compact ? 10 : 12),
    );
  }

  Widget _buildMedia(BuildContext context) {
    final isProfile =
        resourceProjection.resourceType ==
        ContentResourceProjectionType.profile;
    final imageUrl = resourceProjection.imageUrl;
    return CommerceMarketplaceCardMedia(
      imageUrl: imageUrl,
      aspectRatio: isProfile ? 1 : 4 / 3,
      showVideoBadge: false,
      borderRadius: const BorderRadius.only(
        topLeft: Radius.circular(16),
        topRight: Radius.circular(16),
      ),
      fallback: _placeholderMedia(
        context,
        _iconForType(resourceProjection.resourceType),
      ),
    );
  }

  Widget? _buildMetadata(BuildContext context) {
    final parts = <String>[resourceProjection.typeLabel];
    if (resourceProjection.nestedResourceLabel != null) {
      parts.add(resourceProjection.nestedResourceLabel!);
    }
    if (resourceProjection.isLive) {
      parts.add('LIVE');
    } else {
      parts.add('TOMBSTONE');
    }
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

    if (resourceProjection.nestedResourceLabel != null) {
      badges.add(
        CommerceMarketplaceCardBadge(
          label: resourceProjection.nestedResourceLabel!,
          compact: true,
        ),
      );
    }

    return badges;
  }

  Widget _placeholderMedia(BuildContext context, IconData icon) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      color: scheme.surfaceContainerHighest,
      child: Center(
        child: Icon(icon, color: scheme.onSurfaceVariant, size: 36),
      ),
    );
  }

  IconData _iconForType(ContentResourceProjectionType type) {
    switch (type) {
      case ContentResourceProjectionType.profile:
        return Icons.person_outline_rounded;
      case ContentResourceProjectionType.content:
        return Icons.article_outlined;
      case ContentResourceProjectionType.fixedPriceSale:
        return Icons.storefront_outlined;
      case ContentResourceProjectionType.auction:
        return Icons.gavel_rounded;
    }
  }
}
