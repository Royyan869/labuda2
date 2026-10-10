import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';
import 'package:hishumi/shared/widgets/carousel_video_player.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

class ChatResourceProjectionCard extends StatelessWidget {
  final ResourceProjection resourceProjection;

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
      contentPadding: const EdgeInsets.all(AppMetrics.p12),
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
            topLeft: Radius.circular(AppShape.r16),
            topRight: Radius.circular(AppShape.r16),
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
            topLeft: Radius.circular(AppShape.r16),
            topRight: Radius.circular(AppShape.r16),
          ),
        );
      case AuctionLivePayload():
        return CommerceMarketplaceCardMedia(
          imageUrl: resourceProjection.primaryImageUrl,
          fallback: _placeholderMedia(context, Icons.gavel_rounded),
          aspectRatio: 4 / 3,
          borderRadius: const BorderRadius.only(
            topLeft: Radius.circular(AppShape.r16),
            topRight: Radius.circular(AppShape.r16),
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
  /// - image — [CommerceMarketplaceCardMedia], i.e. [AppImage] (CloudFront
  ///   URL as-is, cached).
  /// - video — [CarouselVideoPlayer], the shared video primitive. A video
  ///   reference must never reach the image decoder.
  Widget _buildContentMedia(
    BuildContext context,
    ContentLivePayload payload,
  ) {
    const borderRadius = BorderRadius.only(
      topLeft: Radius.circular(AppShape.r16),
      topRight: Radius.circular(AppShape.r16),
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

  /// Availability/lifecycle stays visible as the caption. Delegates to the
  /// shared Commerce presentation mapping — the card never calculates
  /// lifecycle itself.
  String _captionText() {
    if (!resourceProjection.isLive) {
      return 'Diblokir';
    }
    return commerceLifecycleLabel(resourceProjection.payload) ?? 'Status';
  }

  Widget? _buildMetadata(BuildContext context) {
    final parts = <String>[];
    parts.add(resourceProjection.resourceType.displayLabel);
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

  /// Informational product attribute only.
  ///
  /// The generic product reference card must not leak viewer-scoped Commerce
  /// capability: the former `Chat` (`canChat`) and `Kelola` (`canManage`)
  /// badges were removed. `Nego` is rendered from the canonical PRODUCT-LEVEL
  /// attribute `ForSaleLivePayload.negotiationEnabled` and is never actionable.
  /// Commerce actions (buy / bid / ongkir) live on the canonical Commerce
  /// detail surface.
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
}
