import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/shared/widgets/carousel_video_player.dart';

class ChatResourceProjectionCard extends StatelessWidget {
  final ResourceProjection resourceProjection;

  /// CTA "Beli Sekarang" intent, delegated to the owning screen.
  ///
  /// Checkout navigation resolves `product_id` + the seller trust gate in the
  /// chat screen (Commerce stays the transaction authority — the card is a
  /// display layer and never carries a price or a preview). When no owner is
  /// wired, the button falls back to the canonical resource detail page.
  final VoidCallback? onBuy;

  const ChatResourceProjectionCard({
    super.key,
    required this.resourceProjection,
    this.onBuy,
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
      footer: _buildFooter(context),
      badges: _buildBadges(context),
      contentPadding: const EdgeInsets.all(12),
    );
  }

  Widget _buildMedia(BuildContext context) {
    final payload = resourceProjection.payload;
    switch (payload) {
      case ProfileLivePayload():
        return CommerceMarketplaceCardMedia(
          imageUrl: payload.avatarUrl,
          fallback: _placeholderMedia(context, Icons.person_outline_rounded),
          aspectRatio: 1,
          borderRadius: const BorderRadius.only(
            topLeft: Radius.circular(16),
            topRight: Radius.circular(16),
          ),
        );
      case ContentLivePayload():
        return _buildContentMedia(context, payload);
      case ForSaleLivePayload():
        return CommerceMarketplaceCardMedia(
          imageUrl: resourceProjection.primaryImageUrl,
          fallback: _placeholderMedia(context, Icons.storefront_outlined),
          aspectRatio: 4 / 3,
          borderRadius: const BorderRadius.only(
            topLeft: Radius.circular(16),
            topRight: Radius.circular(16),
          ),
        );
      case AuctionLivePayload():
        return CommerceMarketplaceCardMedia(
          imageUrl: resourceProjection.primaryImageUrl,
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
    }
  }

  /// Canonical Content media frame for the chat resource projection card.
  ///
  /// [ResourceMediaRef.mediaKind] — the transported persisted
  /// `content_media.media_type` — is the render authority:
  /// - image — [CommerceMarketplaceCardMedia], i.e. [StableNetworkImage] / the
  ///   shared network-media path (`resolveNetworkImageUrl`).
  /// - video — [CarouselVideoPlayer], the shared video primitive. A video
  ///   reference must never reach the image decoder.
  Widget _buildContentMedia(
    BuildContext context,
    ContentLivePayload payload,
  ) {
    const borderRadius = BorderRadius.only(
      topLeft: Radius.circular(16),
      topRight: Radius.circular(16),
    );

    if (payload.media.isEmpty) {
      return CommerceMarketplaceCardMedia(
        imageUrl: null,
        fallback: _placeholderMedia(context, Icons.article_outlined),
        aspectRatio: 4 / 3,
        borderRadius: borderRadius,
      );
    }

    final media = payload.media.first;
    if (media.mediaKind == ResourceMediaKind.video) {
      return ClipRRect(
        borderRadius: borderRadius,
        child: AspectRatio(
          aspectRatio: 4 / 3,
          child: LayoutBuilder(
            builder: (context, constraints) => CarouselVideoPlayer(
              videoUrl: media.url,
              width: constraints.maxWidth,
              height: constraints.maxHeight,
              fit: BoxFit.cover,
            ),
          ),
        ),
      );
    }

    return CommerceMarketplaceCardMedia(
      imageUrl: media.url,
      fallback: _placeholderMedia(context, Icons.article_outlined),
      aspectRatio: 4 / 3,
      borderRadius: borderRadius,
    );
  }

  /// Navigation-only CTA row (owner contract: chat = display layer).
  ///
  /// - for-sale + `canBuy` → "Beli Sekarang" → checkout (delegated via
  ///   [onBuy]; Commerce resolves product id, preview and trust gates).
  /// - auction + `canBid` → "Bid" → the canonical auction detail, which is
  ///   the bidding surface.
  Widget? _buildFooter(BuildContext context) {
    if (!resourceProjection.isLive) return null;
    final actions = resourceProjection.commerceActions;
    if (actions == null) return null;

    final buttons = <Widget>[];
    if (actions.canBuy) {
      final onPressed = onBuy ?? _canonicalPushAction(context);
      if (onPressed != null) {
        buttons.add(
          _ctaButton(
            context,
            label: 'Beli Sekarang',
            icon: Icons.shopping_cart_outlined,
            emphasis: true,
            onPressed: onPressed,
          ),
        );
      }
    }
    if (actions.canBid) {
      final onPressed = _canonicalPushAction(context);
      if (onPressed != null) {
        buttons.add(
          _ctaButton(
            context,
            label: 'Bid',
            icon: Icons.gavel_rounded,
            emphasis: false,
            onPressed: onPressed,
          ),
        );
      }
    }
    if (buttons.isEmpty) return null;

    return Padding(
      padding: const EdgeInsets.only(top: 10),
      child: Wrap(spacing: 8, runSpacing: 8, children: buttons),
    );
  }

  VoidCallback? _canonicalPushAction(BuildContext context) {
    final url = resourceProjection.canonicalUrl;
    if (url == null || url.isEmpty) return null;
    return () => context.push(url);
  }

  Widget _ctaButton(
    BuildContext context, {
    required String label,
    required IconData icon,
    required bool emphasis,
    required VoidCallback onPressed,
  }) {
    final scheme = Theme.of(context).colorScheme;
    return FilledButton.icon(
      onPressed: onPressed,
      icon: Icon(icon, size: 16),
      label: Text(label),
      style: FilledButton.styleFrom(
        visualDensity: VisualDensity.compact,
        minimumSize: const Size(0, 34),
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
        backgroundColor: emphasis
            ? scheme.primary
            : scheme.surfaceContainerHighest,
        foregroundColor: emphasis ? scheme.onPrimary : scheme.onSurface,
        textStyle: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700),
      ),
    );
  }

  Widget _buildValue(BuildContext context) {
    final text = _valueText();
    return CommerceMarketplaceCardValue(
      value: text?.isNotEmpty == true
          ? text!
          : resourceProjection.canonicalDisplayLabel,
      caption: _captionText(),
      compact: true,
    );
  }

  /// PRICE IS RENDERED ON EVERY SURFACE (owner decision, 2026-09-27).
  ///
  /// The envelope carries the canonical money on LIVE and chat shows the very
  /// same string as discovery — money formatting is owned by the envelope
  /// (`formattedPrice` / `formattedAmount`), never re-derived here.
  String? _valueText() {
    if (!resourceProjection.isLive) {
      return 'Tidak dapat ditampilkan';
    }
    return switch (resourceProjection.payload) {
      ProfileLivePayload p =>
        p.storeName?.trim().isNotEmpty == true
            ? p.storeName!.trim()
            : (p.isSeller ? 'Penjual' : 'Profil'),
      ContentLivePayload p => '@${p.author.username}',
      ForSaleLivePayload p => p.formattedPrice,
      AuctionLivePayload p => p.formattedAmount,
      null => null,
    };
  }

  /// Availability/lifecycle stays visible as the caption, so rendering the
  /// price never costs the honest status.
  String _captionText() {
    if (!resourceProjection.isLive) {
      return 'Diblokir';
    }
    return switch (resourceProjection.payload) {
      ForSaleLivePayload p => _forSaleStatusText(p.status),
      AuctionLivePayload p =>
        p.lifecycle == 'active' ? 'Berlangsung' : p.lifecycle,
      _ => 'Status',
    };
  }

  /// Indonesian availability label for a for-sale projection status wire value.
  String _forSaleStatusText(String status) {
    switch (status) {
      case 'available':
      case 'active':
        return 'Tersedia';
      case 'reserved':
        return 'Dipesan';
      case 'sold':
        return 'Terjual';
      default:
        return 'Tidak tersedia';
    }
  }

  Widget? _buildMetadata(BuildContext context) {
    final parts = <String>[];
    parts.add(resourceProjection.resourceType.displayLabel);
    parts.add(resourceProjection.isLive ? 'LIVE' : 'TOMBSTONE');
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
      // NOTE (CTA contract): "Beli" / "Bid" capability chips are intentionally
      // absent — the footer renders them as real navigation buttons instead.
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

  String _actionSummary(CommerceActionCapabilities actions) {
    final labels = <String>[];
    if (actions.canChat) labels.add('chat');
    if (actions.canNegotiate) labels.add('nego');
    if (actions.canBuy) labels.add('beli');
    if (actions.canBid) labels.add('bid');
    if (actions.canManage) labels.add('kelola');
    return labels.join(' - ');
  }
}
