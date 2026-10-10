import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_metrics.dart';
import 'package:hishumi/domains/social/content/domain/entities/content.dart';
import 'package:hishumi/shared/widgets/app_image.dart';
import 'package:hishumi/shared/widgets/empty_state.dart';
import 'package:hishumi/shared/widgets/loading_indicator.dart';
import 'package:hishumi/shared/widgets/page_error_state.dart';

/// CANONICAL public commerce grid (2 columns): every marketplace surface
/// renders through this widget with the shared empty/error vocabulary.
///
/// INSET AUTHORITY (SAFE-AREA-02/03): this grid owns only content/layout
/// spacing — the token [CommerceMarketplaceMetrics.gridBottomPadding] under
/// the last row. The bottom SYSTEM inset belongs to the parent screen
/// boundary (a `Scaffold.bottomNavigationBar` or a body-level `SafeArea`),
/// never to the grid; the grid therefore reads no inset out of the ambient
/// window metrics at all. Mounting it in a bar-less, SafeArea-less body is a
/// caller bug, not a fallback this widget silently compensates for.
class CommerceMarketplaceGrid extends StatelessWidget {
  final int itemCount;
  final IndexedWidgetBuilder itemBuilder;
  final EdgeInsetsGeometry padding;
  final double crossAxisSpacing;
  final double mainAxisSpacing;
  final double childAspectRatio;
  final bool isLoading;
  final Object? error;
  final StackTrace? errorStackTrace;
  final WidgetBuilder loadingBuilder;
  final WidgetBuilder emptyBuilder;
  final Widget Function(
    BuildContext context,
    Object error,
    StackTrace? stackTrace,
  )
  errorBuilder;
  final Key Function(int index)? itemKeyBuilder;

  const CommerceMarketplaceGrid({
    super.key,
    required this.itemCount,
    required this.itemBuilder,
    this.padding = const EdgeInsets.fromLTRB(
      CommerceMarketplaceMetrics.gridEdgePadding,
      CommerceMarketplaceMetrics.gridEdgePadding,
      CommerceMarketplaceMetrics.gridEdgePadding,
      CommerceMarketplaceMetrics.gridBottomPadding,
    ),
    this.crossAxisSpacing = CommerceMarketplaceMetrics.gridGap,
    this.mainAxisSpacing = CommerceMarketplaceMetrics.gridGap,
    this.childAspectRatio = CommerceMarketplaceMetrics.childAspectRatio,
    this.isLoading = false,
    this.error,
    this.errorStackTrace,
    this.loadingBuilder = _defaultLoadingBuilder,
    this.emptyBuilder = _defaultEmptyBuilder,
    this.errorBuilder = _defaultErrorBuilder,
    this.itemKeyBuilder,
  });

  @override
  Widget build(BuildContext context) {
    if (isLoading && itemCount == 0) {
      return SliverFillRemaining(
        hasScrollBody: false,
        child: loadingBuilder(context),
      );
    }

    if (error != null && itemCount == 0) {
      return SliverFillRemaining(
        hasScrollBody: false,
        child: errorBuilder(context, error!, errorStackTrace),
      );
    }

    if (itemCount == 0) {
      return SliverFillRemaining(
        hasScrollBody: false,
        child: emptyBuilder(context),
      );
    }

    return SliverPadding(
      padding: padding,
      sliver: SliverLayoutBuilder(
        builder: (context, _) {
          return SliverGrid(
            gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: _crossAxisCountForWidth(),
              crossAxisSpacing: crossAxisSpacing,
              mainAxisSpacing: mainAxisSpacing,
              childAspectRatio: childAspectRatio,
            ),
            delegate: SliverChildBuilderDelegate((context, index) {
              final child = itemBuilder(context, index);
              final key = itemKeyBuilder?.call(index);
              return key == null ? child : KeyedSubtree(key: key, child: child);
            }, childCount: itemCount),
          );
        },
      ),
    );
  }

  static int _crossAxisCountForWidth() {
    return 2;
  }

  /// CANONICAL first-load loading fallback: the shared [LoadingIndicator].
  /// Only rendered when there are no items yet (see [build]); refresh keeps
  /// existing items visible and never swaps to this state.
  static Widget _defaultLoadingBuilder(BuildContext context) {
    return const Center(child: LoadingIndicator());
  }

  /// CANONICAL collection-empty fallback: the shared [EmptyState] with
  /// localized copy. A surface that owns domain wording supplies its own
  /// emptyBuilder.
  static Widget _defaultEmptyBuilder(BuildContext context) {
    final l10n = context.l10n;
    return EmptyState(
      title: l10n.emptyCollectionTitle,
      subtitle: l10n.emptyCollectionMessage,
    );
  }

  /// CANONICAL page-level load error fallback (PageErrorState): safe
  /// localized copy only — the raw [error] object never reaches the screen.
  /// A caller that owns a retry signal supplies its own errorBuilder.
  static Widget _defaultErrorBuilder(
    BuildContext context,
    Object error,
    StackTrace? stackTrace,
  ) {
    return const PageErrorState();
  }
}

/// CANONICAL card frame for every public commerce grid (owner-locked).
///
/// Structure: media (4:5, contain — chips such as video, auction countdown or
/// `Dipromosikan` are OVERLAYS on the media) → title (one line) → value (one
/// line). Geometry comes from [CommerceMarketplaceMetrics], never literals.
///
/// For Sale, Auction and the promotion grid that reuses both must render an
/// IDENTICAL frame; a channel only fills slot data.
class CommerceMarketplaceCardShell extends StatelessWidget {
  final Widget media;
  final String title;
  final Widget value;
  final Widget? metadata;

  /// Optional action row rendered under [metadata], inside the card frame.
  /// Reserved for navigation CTAs (the owning domain keeps transaction
  /// authority; the card itself carries no business logic).
  final Widget? footer;
  final List<Widget> badges;
  final VoidCallback? onTap;
  final EdgeInsetsGeometry padding;
  final EdgeInsetsGeometry contentPadding;
  final BorderRadiusGeometry borderRadius;
  final bool compact;
  final int titleMaxLines;
  final String? semanticLabel;

  const CommerceMarketplaceCardShell({
    super.key,
    required this.media,
    required this.title,
    required this.value,
    this.metadata,
    this.footer,
    this.badges = const [],
    this.onTap,
    this.padding = EdgeInsets.zero,
    this.contentPadding = const EdgeInsets.all(
      CommerceMarketplaceMetrics.contentPadding,
    ),
    this.borderRadius = const BorderRadius.all(
      Radius.circular(CommerceMarketplaceMetrics.cardRadius),
    ),
    this.compact = false,
    this.titleMaxLines = CommerceMarketplaceMetrics.titleMaxLines,
    this.semanticLabel,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final resolvedPadding = contentPadding;
    final titleStyle =
        (compact ? theme.textTheme.titleSmall : theme.textTheme.titleMedium)
            ?.copyWith(
              color: scheme.onSurface,
              fontWeight: FontWeight.w700,
              height: 1.15,
            );

    return Semantics(
      container: true,
      button: onTap != null,
      label: semanticLabel ?? title,
      child: Padding(
        padding: padding,
        child: Material(
          color: scheme.surface,
          shape: RoundedRectangleBorder(
            borderRadius: borderRadius,
            side: BorderSide(color: scheme.outlineVariant),
          ),
          clipBehavior: Clip.antiAlias,
          child: InkWell(
            onTap: onTap,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                media,
                Padding(
                  padding: resolvedPadding,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (badges.isNotEmpty) ...[
                        Wrap(
                          spacing: CommerceMarketplaceMetrics.contentGap,
                          runSpacing: CommerceMarketplaceMetrics.contentGap,
                          children: badges,
                        ),
                        const SizedBox(
                          height: CommerceMarketplaceMetrics.contentGap,
                        ),
                      ],
                      Text(
                        title,
                        style: titleStyle,
                        maxLines: titleMaxLines,
                        overflow: TextOverflow.ellipsis,
                      ),
                      const SizedBox(
                        height: CommerceMarketplaceMetrics.contentGap,
                      ),
                      value,
                      if (metadata != null) ...[
                        const SizedBox(
                          height: CommerceMarketplaceMetrics.contentGap,
                        ),
                        metadata!,
                      ],
                      if (footer != null) ...[footer!],
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class CommerceMarketplaceCardMedia extends StatelessWidget {
  final String? imageUrl;
  final String? reloadToken;
  final Widget fallback;
  final double aspectRatio;
  final BoxFit fit;
  final Alignment alignment;
  final BorderRadius? borderRadius;
  final MediaType? mediaType;
  final bool showVideoBadge;
  final String videoBadgeLabel;

  /// Chip rendered ON the media (bottom-left), e.g. the auction countdown or
  /// the `Dipromosikan` disclosure. Overlay chips live here — never in a row
  /// under the media — so every channel keeps the same card rhythm.
  final Widget? overlay;

  const CommerceMarketplaceCardMedia({
    super.key,
    required this.imageUrl,
    required this.fallback,
    this.reloadToken,
    this.aspectRatio = CommerceMarketplaceMetrics.mediaAspectRatio,
    this.fit = BoxFit.contain,
    this.alignment = Alignment.center,
    this.borderRadius,
    this.mediaType,
    this.showVideoBadge = false,
    this.videoBadgeLabel = 'Video',
    this.overlay,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final trimmed = imageUrl?.trim() ?? '';

    // `fit: contain` (token-locked) keeps portrait AND landscape koi photos
    // fully visible; the tinted mat absorbs the letterbox bars so the 4:5
    // frame stays uniform across cards. Backend CloudFront URL is used as-is.
    final media = ColoredBox(
      color: scheme.surfaceContainerHighest,
      child: Stack(
        fit: StackFit.expand,
        children: [
          if (trimmed.isEmpty)
            fallback
          else
            AppImage(
              key: ValueKey('marketplace-media:$trimmed|${reloadToken ?? ''}'),
              imageUrl: trimmed,
              fit: fit,
              cacheWidth: 600,
              backgroundColor: scheme.surfaceContainerHighest,
              errorWidget: fallback,
            ),
          if (showVideoBadge || mediaType == MediaType.video)
            Positioned(
              top: 8,
              right: 8,
              child: CommerceMarketplaceCardBadge(
                label: videoBadgeLabel,
                icon: Icons.videocam_outlined,
                compact: true,
              ),
            ),
          if (overlay != null) Positioned(left: 8, bottom: 8, child: overlay!),
        ],
      ),
    );

    final clipped = borderRadius == null
        ? media
        : ClipRRect(borderRadius: borderRadius!, child: media);

    return AspectRatio(aspectRatio: aspectRatio, child: clipped);
  }
}

class CommerceMarketplaceCardValue extends StatelessWidget {
  final String value;
  final String? caption;
  final TextAlign textAlign;
  final bool compact;
  final Color? valueColor;
  final Color? captionColor;

  const CommerceMarketplaceCardValue({
    super.key,
    required this.value,
    this.caption,
    this.textAlign = TextAlign.start,
    this.compact = false,
    this.valueColor,
    this.captionColor,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final valueStyle =
        (compact ? theme.textTheme.titleSmall : theme.textTheme.titleMedium)
            ?.copyWith(
              color: valueColor ?? scheme.primary,
              fontWeight: FontWeight.w800,
              height: 1.1,
            );
    final captionStyle = theme.textTheme.bodySmall?.copyWith(
      color: captionColor ?? scheme.onSurfaceVariant,
      height: 1.1,
    );

    return Column(
      crossAxisAlignment: textAlign == TextAlign.end
          ? CrossAxisAlignment.end
          : CrossAxisAlignment.start,
      children: [
        if (caption != null) ...[
          Text(
            caption!,
            style: captionStyle,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            textAlign: textAlign,
          ),
          const SizedBox(height: 2),
        ],
        Text(
          value,
          style: valueStyle,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          textAlign: textAlign,
        ),
      ],
    );
  }
}

class CommerceMarketplaceCardBadge extends StatelessWidget {
  final String label;
  final IconData? icon;
  final Color? backgroundColor;
  final Color? foregroundColor;
  final EdgeInsetsGeometry padding;
  final bool compact;

  const CommerceMarketplaceCardBadge({
    super.key,
    required this.label,
    this.icon,
    this.backgroundColor,
    this.foregroundColor,
    this.padding = const EdgeInsets.symmetric(
      horizontal: AppMetrics.p8,
      vertical: AppMetrics.p4,
    ),
    this.compact = false,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final bg = backgroundColor ?? scheme.surfaceContainerHighest;
    final fg = foregroundColor ?? scheme.onSurfaceVariant;

    return Container(
      padding: compact
          ? const EdgeInsets.symmetric(
              horizontal: AppMetrics.p8,
              vertical: AppMetrics.p4,
            )
          : padding,
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(AppShape.pill),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: icon == null
          ? Text(
              label,
              style:
                  (compact
                          ? theme.textTheme.labelSmall
                          : theme.textTheme.labelMedium)
                      ?.copyWith(color: fg, fontWeight: FontWeight.w600),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              softWrap: false,
            )
          : Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  icon,
                  size: compact
                      ? AppIconSize.inlineGlyph
                      : AppIconSize.inlineGlyph,
                  color: fg,
                ),
                const SizedBox(width: 4),
                Flexible(
                  child: Text(
                    label,
                    style:
                        (compact
                                ? theme.textTheme.labelSmall
                                : theme.textTheme.labelMedium)
                            ?.copyWith(color: fg, fontWeight: FontWeight.w600),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    softWrap: false,
                  ),
                ),
              ],
            ),
    );
  }
}
