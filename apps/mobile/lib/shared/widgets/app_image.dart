import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter_blurhash/flutter_blurhash.dart';
import 'package:shimmer/shimmer.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/services/blurhash_cache_service.dart';

/// Image quality options for loading different image sizes
enum MediaQuality { thumbnail, medium, high, webp }

/// Reusable Image Widget dengan fallback dan loading states
///
/// Features:
/// - Network image dengan fallback
/// - Loading state dengan shimmer effect
/// - Error state dengan icon
/// - Customizable placeholder
/// - Caching support
/// - Different shapes (circle, rounded, rectangle)
/// - Multiple resolution variants (thumbnail, medium, large, webp)
/// - Automatic quality selection based on size
class AppImage extends StatelessWidget {
  final String? imageUrl;
  final String? blurhash; // Blurhash for smooth placeholder
  final double? width;
  final double? height;
  final BoxFit fit;
  final BorderRadius? borderRadius;
  final bool isCircle;
  final Widget? placeholder;
  final Widget? errorWidget;
  final Color? backgroundColor;
  final VoidCallback? onTap;
  final MediaQuality? quality; // Force specific quality

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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    Widget imageWidget = Container(
      width: width,
      height: height,
      decoration: BoxDecoration(
        color:
            backgroundColor ??
            (isDark ? AppColors.darkGray600 : AppColors.neutralGray100),
        borderRadius: isCircle ? null : borderRadius,
        shape: isCircle ? BoxShape.circle : BoxShape.rectangle,
      ),
      child: ClipRRect(
        borderRadius: isCircle
            ? BorderRadius.circular((width ?? height ?? 100) / 2)
            : (borderRadius ?? BorderRadius.zero),
        child: _buildImageContent(context, isDark),
      ),
    );

    if (onTap != null) {
      imageWidget = GestureDetector(onTap: onTap, child: imageWidget);
    }

    return imageWidget;
  }

  Widget _buildImageContent(BuildContext context, bool isDark) {
    if (imageUrl == null || imageUrl!.isEmpty) {
      return _buildPlaceholder(context, isDark);
    }

    if (kIsWeb) {
      return _buildWebImage(context, isDark);
    } else {
      return _buildMobileImage(context, isDark);
    }
  }

  Widget _buildWebImage(BuildContext context, bool isDark) {
    final optimizedUrl = _getOptimizedImageUrl(imageUrl!);

    return CachedNetworkImage(
      imageUrl: optimizedUrl,
      width: width,
      height: height,
      fit: fit,
      placeholder: (context, url) => _buildBlurhashPlaceholder(context, isDark),
      errorWidget: (context, url, error) {
        if (url.contains('firebasestorage.googleapis.com')) {
          return _buildFirebaseImageFallback(context, isDark);
        }
        if (url.contains('s3.ap-southeast-1.amazonaws.com') ||
            url.contains('cloudfront.net')) {
          return _buildAwsS3ImageFallback(context, isDark, url);
        }
        return _buildErrorState(context, isDark, error);
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

  Widget _buildMobileImage(BuildContext context, bool isDark) {
    final optimizedUrl = _getOptimizedImageUrl(imageUrl!);

    return CachedNetworkImage(
      imageUrl: optimizedUrl,
      width: width,
      height: height,
      fit: fit,
      placeholder: (context, url) => _buildBlurhashPlaceholder(context, isDark),
      errorWidget: (context, url, error) {
        return _buildErrorState(context, isDark, error);
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

  Widget _buildFirebaseImageFallback(BuildContext context, bool isDark) {
    String fallbackUrl = imageUrl!;
    if (fallbackUrl.contains('?alt=media&token=')) {
      fallbackUrl = '${fallbackUrl.split('?alt=media&token=').first}?alt=media';
    }

    return Image.network(
      fallbackUrl,
      width: width,
      height: height,
      fit: fit,
      loadingBuilder: (context, child, loadingProgress) {
        if (loadingProgress == null) return child;
        return _buildLoadingState(context, isDark);
      },
      errorBuilder: (context, error, stackTrace) {
        return _buildErrorState(context, isDark, error);
      },
    );
  }

  Widget _buildPlaceholder(BuildContext context, bool isDark) {
    if (placeholder != null) return placeholder!;

    return Container(
      width: width,
      height: height,
      color: isDark ? AppColors.darkGray600 : AppColors.neutralGray100,
      child: Icon(
        Icons.image_outlined,
        size: (width != null && height != null)
            ? (width! < height! ? width! : height!) * 0.4
            : 32,
        color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
      ),
    );
  }

  Widget _buildBlurhashPlaceholder(BuildContext context, bool isDark) {
    // Check for explicit blurhash parameter first
    if (blurhash != null && blurhash!.isNotEmpty && imageUrl != null) {
      return _buildBlurhashWidget(blurhash!, context, isDark);
    }

    // Try to get from cache asynchronously
    if (imageUrl != null) {
      return FutureBuilder<String?>(
        future: BlurhashCacheService.instance.getBlurhash(imageUrl!),
        builder: (context, snapshot) {
          if (snapshot.hasData &&
              snapshot.data != null &&
              snapshot.data!.isNotEmpty) {
            return _buildBlurhashWidget(snapshot.data!, context, isDark);
          }

          // Fallback to shimmer loading state
          return _buildShimmerPlaceholder(context, isDark);
        },
      );
    }

    // Default shimmer placeholder
    return _buildShimmerPlaceholder(context, isDark);
  }

  Widget _buildBlurhashWidget(String hash, BuildContext context, bool isDark) {
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

  Widget _buildShimmerPlaceholder(BuildContext context, bool isDark) {
    return Shimmer.fromColors(
      baseColor: isDark ? AppColors.darkGray600 : AppColors.neutralGray100,
      highlightColor: isDark ? AppColors.darkGray500 : AppColors.neutralGray200,
      child: Container(
        width: width,
        height: height,
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray100,
          borderRadius: isCircle
              ? BorderRadius.circular((width ?? height ?? 100) / 2)
              : (borderRadius ?? BorderRadius.zero),
        ),
      ),
    );
  }

  /// Transform S3 URLs to CloudFront and select optimal quality
  String _getOptimizedImageUrl(String url) {
    // Get quality-specific URL
    String qualityUrl = _getQualitySpecificUrl(url);

    // CRITICAL FIX: Do NOT mutate Signed URLs provided by Backend
    if (qualityUrl.contains('X-Amz-Signature') || qualityUrl.contains('Expires=') || qualityUrl.contains('token=')) {
      return qualityUrl;
    }

    // Don't transform Firebase Storage URLs - keep them as is
    if (qualityUrl.contains('firebasestorage.googleapis.com')) {
      return qualityUrl;
    }

    // Transform static/public S3 URLs to CloudFront if enabled
    if (AppConstants.useCloudFront &&
        qualityUrl.contains('s3.ap-southeast-1.amazonaws.com')) {
      return qualityUrl.replaceFirst(
        'https://labuda-videos.s3.ap-southeast-1.amazonaws.com',
        AppConstants.cdnBaseUrl,
      );
    }

    // Return quality URL if no transformation needed
    return qualityUrl;
  }

  /// Select appropriate image quality based on display size
  /// Note: This is a simplified version that doesn't parse MediaEntity
  /// The actual quality selection should be done at the data layer
  String _getQualitySpecificUrl(String url) {
    // For now, just return the original URL
    // TODO: Implement proper quality selection based on URL patterns
    return url;
  }

  Widget _buildLoadingState(BuildContext context, bool isDark) {
    return _buildBlurhashPlaceholder(context, isDark);
  }

  Widget _buildErrorState(BuildContext context, bool isDark, [Object? error]) {
    if (errorWidget != null) return errorWidget!;

    return Container(
      width: width,
      height: height,
      color: isDark ? AppColors.darkGray600 : AppColors.neutralGray100,
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.broken_image_outlined,
            size: (width != null && height != null)
                ? (width! < height! ? width! : height!) * 0.4
                : 32,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
          ),
          if (error != null && (width == null || width! > 100)) ...[
            const SizedBox(height: 4),
            Text(
              'Image Error',
              style: TextStyle(
                fontSize: 10,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray500,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildAwsS3ImageFallback(
    BuildContext context,
    bool isDark,
    String url,
  ) {
    return Container(
      width: width,
      height: height,
      color: isDark ? AppColors.darkGray600 : AppColors.neutralGray100,
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.cloud_off_outlined,
            size: (width != null && height != null)
                ? (width! < height! ? width! : height!) * 0.3
                : 28,
            color: AppColors.warning,
          ),
          const SizedBox(height: 4),
          Text(
            'AWS S3 CORS',
            style: TextStyle(
              fontSize: 8,
              color: AppColors.warning,
              fontWeight: FontWeight.w500,
            ),
            textAlign: TextAlign.center,
          ),
          if (width != null && width! > 150) ...[
            const SizedBox(height: 2),
            Text(
              'Configure CORS\nin S3 bucket',
              style: TextStyle(
                fontSize: 7,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray500,
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
