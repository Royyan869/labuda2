import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_resource_projection.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';

class ChatResourceProjectionCard extends StatelessWidget {
  final ChatResourceProjection resourceProjection;

  const ChatResourceProjectionCard({
    super.key,
    required this.resourceProjection,
  });

  @override
  Widget build(BuildContext context) {
    final onTap =
        resourceProjection.isLive &&
            (resourceProjection.canonicalUrl?.isNotEmpty ?? false)
        ? () => context.push(resourceProjection.canonicalUrl!)
        : null;

    return CommerceMarketplaceCardShell(
      onTap: onTap,
      compact: true,
      title: resourceProjection.titleText,
      semanticLabel: resourceProjection.titleText,
      media: _buildMedia(context),
      value: _buildValue(context),
      metadata: _buildMetadata(context),
      badges: _buildBadges(context),
      contentPadding: const EdgeInsets.all(12),
    );
  }

  Widget _buildMedia(BuildContext context) {
    final payload = resourceProjection.payload;
    switch (payload) {
      case ChatResourceProfileLivePayload():
        return CommerceMarketplaceCardMedia(
          imageUrl: payload.avatarUrl,
          fallback: _placeholderMedia(context, Icons.person_outline_rounded),
          aspectRatio: 1,
          borderRadius: const BorderRadius.only(
            topLeft: Radius.circular(16),
            topRight: Radius.circular(16),
          ),
        );
      case ChatResourceContentLivePayload():
        final mediaUrl = payload.media.isNotEmpty
            ? payload.media.first.url
            : null;
        return CommerceMarketplaceCardMedia(
          imageUrl: mediaUrl,
          fallback: _placeholderMedia(context, Icons.article_outlined),
          aspectRatio: 4 / 3,
          borderRadius: const BorderRadius.only(
            topLeft: Radius.circular(16),
            topRight: Radius.circular(16),
          ),
        );
      case ChatResourceForSaleLivePayload():
        return CommerceMarketplaceCardMedia(
          imageUrl: payload.imageUrl,
          fallback: _placeholderMedia(context, Icons.storefront_outlined),
          aspectRatio: 4 / 3,
          borderRadius: const BorderRadius.only(
            topLeft: Radius.circular(16),
            topRight: Radius.circular(16),
          ),
        );
      case ChatResourceAuctionLivePayload():
        return CommerceMarketplaceCardMedia(
          imageUrl: payload.thumbnailUrl,
          fallback: _placeholderMedia(context, Icons.gavel_rounded),
          aspectRatio: 4 / 3,
          borderRadius: const BorderRadius.only(
            topLeft: Radius.circular(16),
            topRight: Radius.circular(16),
          ),
          showVideoBadge: false,
        );
      case null:
        return _placeholderMedia(context, Icons.block_outlined);
      default:
        return _placeholderMedia(context, Icons.block_outlined);
    }
  }

  Widget _buildValue(BuildContext context) {
    final text = resourceProjection.valueText;
    return CommerceMarketplaceCardValue(
      value: text?.isNotEmpty == true
          ? text!
          : resourceProjection.canonicalDisplayLabel,
      caption: resourceProjection.state == ChatResourceProjectionState.live
          ? 'Status'
          : 'Diblokir',
      compact: true,
    );
  }

  Widget? _buildMetadata(BuildContext context) {
    final parts = <String>[];
    parts.add(resourceProjection.resourceType.displayLabel);
    parts.add(
      resourceProjection.state == ChatResourceProjectionState.live
          ? 'LIVE'
          : 'TOMBSTONE',
    );
    final actions = resourceProjection.commerceActions;
    if (actions != null && actions.hasAnyAction) {
      parts.add(_actionSummary(actions));
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
        label: resourceProjection.resourceType.displayLabel,
        compact: true,
      ),
      CommerceMarketplaceCardBadge(
        label: resourceProjection.isLive ? 'LIVE' : 'TOMBSTONE',
        compact: true,
      ),
    ];

    final actions = resourceProjection.commerceActions;
    if (actions != null) {
      if (actions.canChat) {
        badges.add(
          const CommerceMarketplaceCardBadge(label: 'Chat', compact: true),
        );
      }
      if (actions.canNegotiate) {
        badges.add(
          const CommerceMarketplaceCardBadge(label: 'Nego', compact: true),
        );
      }
      if (actions.canBuy) {
        badges.add(
          const CommerceMarketplaceCardBadge(label: 'Beli', compact: true),
        );
      }
      if (actions.canBid) {
        badges.add(
          const CommerceMarketplaceCardBadge(label: 'Bid', compact: true),
        );
      }
      if (actions.canManage) {
        badges.add(
          const CommerceMarketplaceCardBadge(label: 'Kelola', compact: true),
        );
      }
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

  String _actionSummary(ChatCommerceActionCapabilities actions) {
    final labels = <String>[];
    if (actions.canChat) labels.add('chat');
    if (actions.canNegotiate) labels.add('nego');
    if (actions.canBuy) labels.add('beli');
    if (actions.canBid) labels.add('bid');
    if (actions.canManage) labels.add('kelola');
    return labels.join(' - ');
  }
}
