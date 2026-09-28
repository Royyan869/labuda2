import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter_blurhash/flutter_blurhash.dart';
import 'package:shimmer/shimmer.dart';
import 'package:labuda/core/services/blurhash_cache_service.dart';

/// Image quality options for loading different image sizes
enum MediaQuality { thumbnail, medium, high, webp }

/// Canonical network-media render authority.
///
/// Contract: [imageUrl] is the backend-resolved CloudFront URL, used as-is.
/// Mobile never rebuilds, mutates, or strips media URLs. Loading (shimmer /
/// blurhash) and error states are always distinct.
class AppImage extends StatelessWidget {
  final String? imageUrl;
  final String? blurhash;
  final double? width;
  final double? height;
  final BoxFit fit;
  final BorderRadius? borderRadius;
  final bool isCircle;
  final Widget? placeholder;
  final Widget? errorWidget;
  final Color? backgroundColor;
  final VoidCallback? onTap;
  final MediaQuality? quality;

  const AppImage({
    super.key,
    this.imageUrl,
    this.blurhash,
    this.width,
    this.height,
    this.fit = BoxFit.cover,
    this.borderRadius,
    this.isCircle = false,
    this.placeholder,
    this.errorWidget,
    this.backgroundColor,
    this.onTap,
    this.quality,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    Widget imageWidget = Container(
      width: width,
      height: height,
      decoration: BoxDecoration(
        color: backgroundColor ?? scheme.surfaceContainerHighest,
        borderRadius: isCircle ? null : borderRadius,
        shape: isCircle ? BoxShape.circle : BoxShape.rectangle,
      ),
      child: ClipRRect(
        borderRadius: isCircle
            ? BorderRadius.circular((width ?? height ?? 100) / 2)
            : (borderRadius ?? BorderRadius.zero),
        child: _buildImageContent(context),
      ),
    );

    if (onTap != null) {
      imageWidget = GestureDetector(onTap: onTap, child: imageWidget);
    }

    return imageWidget;
  }

  Widget _buildImageContent(BuildContext context) {
    if (imageUrl == null || imageUrl!.isEmpty) {
      return _buildPlaceholder(context);
    }

    if (kIsWeb) {
      return _buildWebImage(context);
    } else {
      return _buildMobileImage(context);
    }
  }

  Widget _buildWebImage(BuildContext context) {
    return CachedNetworkImage(
      imageUrl: imageUrl!,
      width: width,
      height: height,
      fit: fit,
      placeholder: (context, url) => _buildBlurhashPlaceholder(context),
      errorWidget: (context, url, error) {
        return _buildErrorState(context, error);
      },
      imageBuilder: (context, imageProvider) {
        return Container(
          width: width,
          height: height,
          decoration: BoxDecoration(
            borderRadius: isCircle ? null : borderRadius,
            shape: isCircle ? BoxShape.circle : BoxShape.rectangle,
            image: DecorationImage(image: imageProvider, fit: fit),
          ),
        );
      },
      // Basic Settings (cache optimization disabled temporarily)
      fadeInDuration: const Duration(milliseconds: 200),
      fadeOutDuration: const Duration(milliseconds: 100),
    );
  }

  Widget _buildMobileImage(BuildContext context) {
    return CachedNetworkImage(
      imageUrl: imageUrl!,
      width: width,
      height: height,
      fit: fit,
      placeholder: (context, url) => _buildBlurhashPlaceholder(context),
      errorWidget: (context, url, error) {
        return _buildErrorState(context, error);
      },
      imageBuilder: (context, imageProvider) {
        return Container(
          width: width,
          height: height,
          decoration: BoxDecoration(
            borderRadius: isCircle ? null : borderRadius,
            shape: isCircle ? BoxShape.circle : BoxShape.rectangle,
            image: DecorationImage(image: imageProvider, fit: fit),
          ),
        );
      },
      // Basic Settings (cache optimization disabled temporarily)
      fadeInDuration: const Duration(milliseconds: 200),
      fadeOutDuration: const Duration(milliseconds: 100),
    );
  }

  Widget _buildPlaceholder(BuildContext context) {
    if (placeholder != null) return placeholder!;
    final scheme = Theme.of(context).colorScheme;

    return Container(
      width: width,
      height: height,
      color: scheme.surfaceContainerHighest,
      child: Icon(
        Icons.image_outlined,
        size: (width != null && height != null)
            ? (width! < height! ? width! : height!) * 0.4
            : 32,
        color: scheme.onSurfaceVariant,
      ),
    );
  }

  Widget _buildBlurhashPlaceholder(BuildContext context) {
    // Check for explicit blurhash parameter first
    if (blurhash != null && blurhash!.isNotEmpty && imageUrl != null) {
      return _buildBlurhashWidget(blurhash!, context);
    }

    // Try to get from cache asynchronously
    if (imageUrl != null) {
      return FutureBuilder<String?>(
        future: BlurhashCacheService.instance.getBlurhash(imageUrl!),
        builder: (context, snapshot) {
          if (snapshot.hasData &&
              snapshot.data != null &&
              snapshot.data!.isNotEmpty) {
            return _buildBlurhashWidget(snapshot.data!, context);
          }

          // Fallback to shimmer loading state
          return _buildShimmerPlaceholder(context);
        },
      );
    }

    // Default shimmer placeholder
    return _buildShimmerPlaceholder(context);
  }

  Widget _buildBlurhashWidget(String hash, BuildContext context) {
    return ClipRRect(
      borderRadius: isCircle
          ? BorderRadius.circular((width ?? height ?? 100) / 2)
          : (borderRadius ?? BorderRadius.zero),
      child: BlurHash(
        hash: hash,
        image: imageUrl!,
        imageFit: fit,
        duration: const Duration(milliseconds: 300),
        curve: Curves.easeInOut,
      ),
    );
  }

  Widget _buildShimmerPlaceholder(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Shimmer.fromColors(
      baseColor: scheme.surfaceContainerLow,
      highlightColor: scheme.surfaceContainerHighest,
      child: Container(
        width: width,
        height: height,
        decoration: BoxDecoration(
          color: scheme.surfaceContainerLow,
          borderRadius: isCircle
              ? BorderRadius.circular((width ?? height ?? 100) / 2)
              : (borderRadius ?? BorderRadius.zero),
        ),
      ),
    );
  }

  Widget _buildErrorState(BuildContext context, [Object? error]) {
    if (errorWidget != null) return errorWidget!;
    final scheme = Theme.of(context).colorScheme;

    return Container(
      width: width,
      height: height,
      color: scheme.surfaceContainerHighest,
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.broken_image_outlined,
            size: (width != null && height != null)
                ? (width! < height! ? width! : height!) * 0.4
                : 32,
            color: scheme.onSurfaceVariant,
          ),
          if (error != null && (width == null || width! > 100)) ...[
            const SizedBox(height: 4),
            Text(
              'Image Error',
              style: TextStyle(
                fontSize: 10,
                color: scheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ],
      ),
    );
  }

  // Named constructors untuk common use cases
  static AppImage avatar({
    required String? imageUrl,
    String? blurhash,
    required double size,
    VoidCallback? onTap,
    Widget? placeholder,
  }) {
    return AppImage(
      imageUrl: imageUrl,
      blurhash: blurhash,
      width: size,
      height: size,
      isCircle: true,
      onTap: onTap,
      placeholder: placeholder,
      quality: size <= 150 ? MediaQuality.thumbnail : MediaQuality.medium,
    );
  }

  static AppImage cover({
    required String? imageUrl,
    String? blurhash,
    required double width,
    required double height,
    BorderRadius? borderRadius,
    VoidCallback? onTap,
    MediaQuality? quality,
  }) {
    return AppImage(
      imageUrl: imageUrl,
      blurhash: blurhash,
      width: width,
      height: height,
      fit: BoxFit.cover,
      borderRadius: borderRadius ?? BorderRadius.circular(8),
      onTap: onTap,
      quality: quality,
    );
  }

  static AppImage thumbnail({
    required String? imageUrl,
    String? blurhash,
    double size = 60,
    BorderRadius? borderRadius,
    VoidCallback? onTap,
  }) {
    return AppImage(
      imageUrl: imageUrl,
      blurhash: blurhash,
      width: size,
      height: size,
      fit: BoxFit.cover,
      borderRadius: borderRadius ?? BorderRadius.circular(8),
      onTap: onTap,
      quality: MediaQuality.thumbnail, // Force thumbnail quality
    );
  }
}
