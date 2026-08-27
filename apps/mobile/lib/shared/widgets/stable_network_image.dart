import 'package:flutter/material.dart';
import 'package:labuda/core/src/utils/constants/app_constants.dart';

String? resolveNetworkImageUrl(String? value) {
  final trimmed = value?.trim();
  if (trimmed == null || trimmed.isEmpty) return null;

  final uri = Uri.tryParse(trimmed);
  if (uri != null && (uri.hasScheme || uri.hasAuthority)) {
    return trimmed;
  }

  final normalizedPath = _normalizeRelativePath(uri?.path ?? trimmed);
  if (normalizedPath.isEmpty) {
    return trimmed;
  }

  final query = uri != null && uri.query.isNotEmpty ? '?${uri.query}' : '';
  final baseUrl = AppConstants.useCloudFront
      ? AppConstants.cdnBaseUrl
      : AppConstants.awsS3BaseUrl;
  return '$baseUrl/$normalizedPath$query';
}

/// Network image that keeps the last successful frame visible while a new URL
/// is loading.
///
/// This avoids placeholder flicker when signed URLs rotate or when a fresh URL
/// is published before the next fetch completes.
class StableNetworkImage extends StatefulWidget {
  final String? imageUrl;
  final String? logicalCacheKey;
  final String? reloadToken;
  final Widget fallback;
  final BoxFit fit;
  final Alignment alignment;
  final BorderRadius? borderRadius;

  const StableNetworkImage({
    super.key,
    required this.imageUrl,
    this.logicalCacheKey,
    this.reloadToken,
    required this.fallback,
    this.fit = BoxFit.cover,
    this.alignment = Alignment.center,
    this.borderRadius,
  });

  @override
  State<StableNetworkImage> createState() => _StableNetworkImageState();
}

class _StableNetworkImageState extends State<StableNetworkImage> {
  String? _displayedUrl;
  String? _displayedLogicalCacheKey;
  String? _displayedReloadToken;
  bool _hasSuccessfulFrame = false;

  @override
  void initState() {
    super.initState();
    _displayedUrl = resolveNetworkImageUrl(widget.imageUrl);
    _displayedLogicalCacheKey = _normalize(widget.logicalCacheKey);
    _displayedReloadToken = _normalize(widget.reloadToken);
    _hasSuccessfulFrame = false;
  }

  @override
  void didUpdateWidget(covariant StableNetworkImage oldWidget) {
    super.didUpdateWidget(oldWidget);

    final nextUrl = resolveNetworkImageUrl(widget.imageUrl);
    final nextLogicalKey = _normalize(widget.logicalCacheKey);
    final nextReloadToken = _normalize(widget.reloadToken);
    if (nextUrl == _displayedUrl && nextReloadToken == _displayedReloadToken) {
      _displayedLogicalCacheKey = nextLogicalKey;
      return;
    }

    if (nextUrl == null) {
      _displayedUrl = null;
      _displayedLogicalCacheKey = null;
      _displayedReloadToken = null;
      _hasSuccessfulFrame = false;
      return;
    }

    _displayedUrl ??= nextUrl;
    if (_displayedUrl == nextUrl) {
      _displayedLogicalCacheKey = nextLogicalKey;
    }
  }

  @override
  Widget build(BuildContext context) {
    final nextUrl = resolveNetworkImageUrl(widget.imageUrl);
    final currentUrl = _displayedUrl;
    final nextLogicalKey = _normalize(widget.logicalCacheKey);
    final nextReloadToken = _normalize(widget.reloadToken);
    final hasReloadRequest =
        currentUrl != null &&
        currentUrl == nextUrl &&
        _displayedReloadToken != nextReloadToken;

    if (nextUrl == null) {
      return widget.fallback;
    }

    final nextImage = Image.network(
      nextUrl,
      key: ValueKey('stable-image:$nextUrl|${nextReloadToken ?? ''}'),
      fit: widget.fit,
      alignment: widget.alignment,
      gaplessPlayback: true,
      loadingBuilder: (context, child, loadingProgress) {
        if (loadingProgress == null) {
          if (_displayedUrl != nextUrl ||
              _displayedReloadToken != nextReloadToken) {
            WidgetsBinding.instance.addPostFrameCallback((_) {
              if (!mounted) return;
              setState(() {
                _displayedUrl = nextUrl;
                _displayedLogicalCacheKey = nextLogicalKey;
                _displayedReloadToken = nextReloadToken;
                _hasSuccessfulFrame = true;
              });
            });
          } else if (_displayedLogicalCacheKey != nextLogicalKey) {
            WidgetsBinding.instance.addPostFrameCallback((_) {
              if (!mounted) return;
              setState(() => _displayedLogicalCacheKey = nextLogicalKey);
            });
          }
          return child;
        }

        if (currentUrl == null || currentUrl == nextUrl) {
          return widget.fallback;
        }

        return const SizedBox.shrink();
      },
      errorBuilder: (context, error, stackTrace) {
        if (_hasSuccessfulFrame) {
          return const SizedBox.shrink();
        }
        return widget.fallback;
      },
    );

    if (currentUrl == null || (currentUrl == nextUrl && !hasReloadRequest)) {
      return _wrapBorder(nextImage);
    }

    return _wrapBorder(
      Stack(
        fit: StackFit.expand,
        children: [_buildImage(currentUrl), nextImage],
      ),
    );
  }

  Widget _buildImage(String url) {
    final image = Image.network(
      url,
      key: ValueKey('stable-image:$url|${_displayedReloadToken ?? ''}'),
      fit: widget.fit,
      alignment: widget.alignment,
      gaplessPlayback: true,
      errorBuilder: (context, error, stackTrace) => widget.fallback,
    );

    if (widget.borderRadius == null) {
      return image;
    }

    return ClipRRect(borderRadius: widget.borderRadius!, child: image);
  }

  Widget _wrapBorder(Widget child) {
    if (widget.borderRadius == null) {
      return child;
    }
    return ClipRRect(borderRadius: widget.borderRadius!, child: child);
  }

  String? _normalize(String? value) {
    final trimmed = value?.trim();
    if (trimmed == null || trimmed.isEmpty) return null;
    return trimmed;
  }
}

String _normalizeRelativePath(String value) {
  var out = value.trim().replaceAll('\\', '/');
  out = out.replaceFirst(RegExp(r'^/+'), '');
  out = out.replaceAll(RegExp(r'/+'), '/');
  return out;
}
