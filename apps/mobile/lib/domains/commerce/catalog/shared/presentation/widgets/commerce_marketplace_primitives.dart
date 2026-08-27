import 'package:flutter/material.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

const Set<String> _transientQueryKeys = {
  'expires',
  'policy',
  'signature',
  'token',
  'x-amz-algorithm',
  'x-amz-credential',
  'x-amz-date',
  'x-amz-expires',
  'x-amz-signature',
  'x-amz-security-token',
  'x-amz-signedheaders',
  'x-amz-token',
};

String normalizeMarketplaceMediaReference(String reference) {
  final trimmed = reference.trim();
  if (trimmed.isEmpty) return '';

  final uri = Uri.tryParse(trimmed);
  if (uri == null) {
    return _normalizeRawReference(trimmed);
  }

  final normalizedPath = _normalizePath(
    uri.path.isNotEmpty ? uri.path : trimmed,
  );
  final query = _stableQuerySuffix(uri);

  if (normalizedPath.isNotEmpty) {
    return query.isEmpty ? normalizedPath : '$normalizedPath?$query';
  }

  if (uri.hasScheme || uri.hasAuthority) {
    return query.isEmpty ? trimmed : '$trimmed?$query';
  }

  return query.isEmpty ? trimmed : '$trimmed?$query';
}

String marketplaceMediaLogicalKey({
  required String entityId,
  required String mediaReference,
  required int position,
  String entityType = 'marketplace',
}) {
  return '$entityType|$entityId|$position|'
      '${normalizeMarketplaceMediaReference(mediaReference)}';
}

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
  final bool includeSafeAreaBottom;

  const CommerceMarketplaceGrid({
    super.key,
    required this.itemCount,
    required this.itemBuilder,
    this.padding = const EdgeInsets.fromLTRB(8, 8, 8, 16),
    this.crossAxisSpacing = 12,
    this.mainAxisSpacing = 12,
    this.childAspectRatio = 0.53,
    this.isLoading = false,
    this.error,
    this.errorStackTrace,
    this.loadingBuilder = _defaultLoadingBuilder,
    this.emptyBuilder = _defaultEmptyBuilder,
    this.errorBuilder = _defaultErrorBuilder,
    this.itemKeyBuilder,
    this.includeSafeAreaBottom = true,
  });

  @override
  Widget build(BuildContext context) {
    final bottomPadding = includeSafeAreaBottom
        ? MediaQuery.paddingOf(context).bottom
        : 0.0;
    final resolvedPadding = padding.add(EdgeInsets.only(bottom: bottomPadding));

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
      padding: resolvedPadding,
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

  static Widget _defaultLoadingBuilder(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: CircularProgressIndicator(color: theme.colorScheme.primary),
    );
  }

  static Widget _defaultEmptyBuilder(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Text(
          'Belum ada item untuk ditampilkan',
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
          textAlign: TextAlign.center,
        ),
      ),
    );
  }

  static Widget _defaultErrorBuilder(
    BuildContext context,
    Object error,
    StackTrace? stackTrace,
  ) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, color: theme.colorScheme.error, size: 48),
            const SizedBox(height: 12),
            Text(
              'Data belum bisa dimuat.',
              style: theme.textTheme.titleMedium?.copyWith(
                color: theme.colorScheme.onSurface,
                fontWeight: FontWeight.w700,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 8),
            Text(
              error.toString(),
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}

class CommerceMarketplaceCardShell extends StatelessWidget {
  final Widget media;
  final String title;
  final Widget value;
  final Widget? metadata;
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
    this.badges = const [],
    this.onTap,
    this.padding = EdgeInsets.zero,
    this.contentPadding = const EdgeInsets.all(12),
    this.borderRadius = const BorderRadius.all(Radius.circular(16)),
    this.compact = false,
    this.titleMaxLines = 2,
    this.semanticLabel,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final resolvedPadding = compact ? const EdgeInsets.all(10) : contentPadding;
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
                        Wrap(spacing: 6, runSpacing: 6, children: badges),
                        const SizedBox(height: 8),
                      ],
                      Text(
                        title,
                        style: titleStyle,
                        maxLines: titleMaxLines,
                        overflow: TextOverflow.ellipsis,
                      ),
                      const SizedBox(height: 8),
                      value,
                      if (metadata != null) ...[
                        const SizedBox(height: 8),
                        metadata!,
                      ],
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
  final String? logicalCacheKey;
  final String? reloadToken;
  final Widget fallback;
  final double aspectRatio;
  final BoxFit fit;
  final Alignment alignment;
  final BorderRadius? borderRadius;
  final MediaType? mediaType;
  final bool showVideoBadge;
  final String videoBadgeLabel;

  const CommerceMarketplaceCardMedia({
    super.key,
    required this.imageUrl,
    required this.fallback,
    this.logicalCacheKey,
    this.reloadToken,
    this.aspectRatio = 4 / 5,
    this.fit = BoxFit.cover,
    this.alignment = Alignment.center,
    this.borderRadius,
    this.mediaType,
    this.showVideoBadge = false,
    this.videoBadgeLabel = 'Video',
  });

  @override
  Widget build(BuildContext context) {
    final media = Stack(
      fit: StackFit.expand,
      children: [
        StableNetworkImage(
          imageUrl: imageUrl,
          logicalCacheKey: logicalCacheKey,
          reloadToken: reloadToken,
          fit: fit,
          alignment: alignment,
          fallback: fallback,
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
      ],
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
    this.padding = const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
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
          ? const EdgeInsets.symmetric(horizontal: 6, vertical: 3)
          : padding,
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(999),
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
                Icon(icon, size: compact ? 12 : 14, color: fg),
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

String _normalizeRawReference(String value) {
  final path = _normalizePath(value);
  return path.isEmpty ? value.trim() : path;
}

String _normalizePath(String value) {
  var out = value.trim();
  if (out.isEmpty) return '';
  if (out.contains('://')) {
    final uri = Uri.tryParse(out);
    if (uri != null) {
      out = uri.path;
    }
  }
  out = out.replaceAll('\\', '/');
  out = out.replaceFirst(RegExp(r'^/+'), '');
  out = out.replaceAll(RegExp(r'/+'), '/');
  return out;
}

String _stableQuerySuffix(Uri uri) {
  final entries = <MapEntry<String, String>>[];
  uri.queryParametersAll.forEach((rawKey, values) {
    final key = rawKey.trim();
    if (key.isEmpty) return;
    if (_transientQueryKeys.contains(key.toLowerCase())) return;

    final cleanedValues =
        values
            .map((value) => value.trim())
            .where((value) => value.isNotEmpty)
            .toList()
          ..sort();

    for (final value in cleanedValues) {
      entries.add(MapEntry(key, value));
    }
  });

  if (entries.isEmpty) return '';

  entries.sort((a, b) {
    final keyCompare = a.key.compareTo(b.key);
    if (keyCompare != 0) return keyCompare;
    return a.value.compareTo(b.value);
  });

  final buffer = StringBuffer();
  for (var i = 0; i < entries.length; i++) {
    final entry = entries[i];
    if (i > 0) buffer.write('&');
    buffer
      ..write(Uri.encodeQueryComponent(entry.key))
      ..write('=')
      ..write(Uri.encodeQueryComponent(entry.value));
  }
  return buffer.toString();
}
