import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Professional Avatar Component
///
/// Features:
/// - Cached network image loading dengan fallback
/// - Initials generation otomatis dari userId atau custom
/// - Multiple size variants (small, medium, large, extraLarge)
/// - Professional styling dengan shadow dan border
/// - Theme-aware colors (dark/light mode)
/// - Tap gesture support untuk navigation
/// - Edit icon overlay untuk profile editing
/// - Specialized constructors untuk use cases berbeda
class ProfileAvatar extends StatelessWidget {
  final String userId;
  final double size;
  final String? imageUrl;
  final String? initials;
  final bool isVerified;
  final VoidCallback? onTap;
  final bool showShadow;
  final bool showEditIcon;
  final VoidCallback? onEditTap;

  const ProfileAvatar({
    super.key,
    required this.userId,
    required this.size,
    this.imageUrl,
    this.initials,
    this.isVerified = false,
    this.onTap,
    this.showShadow = true,
    this.showEditIcon = false,
    this.onEditTap,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return GestureDetector(
      onTap: onTap,
      child: SizedBox(
        width: size,
        height: size,
        child: Stack(
          alignment: Alignment.center,
          children: [
            // Avatar container
            Container(
              width: size,
              height: size,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                boxShadow: showShadow
                    ? [
                        BoxShadow(
                          color: AppColors.dark.withValues(
                            alpha: isDark ? 0.3 : 0.1,
                          ),
                          blurRadius: 6,
                          offset: const Offset(0, 2),
                        ),
                      ]
                    : null,
              ),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(size / 2),
                child: _buildAvatarContent(context, isDark),
              ),
            ),

            // Edit icon only (no verification badge)
            if (showEditIcon)
              Positioned(bottom: 2, right: 2, child: _buildEditIcon(context)),
          ],
        ),
      ),
    );
  }

  Widget _buildAvatarContent(BuildContext context, bool isDark) {
    if (imageUrl != null && imageUrl!.isNotEmpty) {
      // Use imageUrl as-is if it already has cache busting parameter
      final finalImageUrl = imageUrl!;

      // CORS Workaround: Use Image.network instead of CachedNetworkImage for development
      if (kIsWeb && finalImageUrl.contains('firebasestorage.googleapis.com')) {
        return ClipRRect(
          borderRadius: BorderRadius.circular(size / 2),
          child: Image.network(
            finalImageUrl,
            width: size,
            height: size,
            fit: BoxFit.cover,
            loadingBuilder: (context, child, loadingProgress) {
              if (loadingProgress == null) return child;
              // NO CircularProgressIndicator - just show empty circle while loading
              return Container(
                width: size,
                height: size,
                decoration: BoxDecoration(
                  color: isDark
                      ? AppColors.darkGray700
                      : AppColors.neutralGray200,
                  shape: BoxShape.circle,
                ),
              );
            },
            errorBuilder: (context, error, stackTrace) {
              return _buildInitialsAvatar(isDark);
            },
          ),
        );
      }

      // Fallback to CachedNetworkImage for other cases
      return CachedNetworkImage(
        imageUrl: finalImageUrl,
        fit: BoxFit.cover,
        width: size,
        height: size,
        cacheKey: finalImageUrl.split(
          '?',
        )[0], // Use base URL without timestamp as cache key
        memCacheWidth: (size * 2).toInt().clamp(
          50,
          400,
        ), // Cache at 2x for retina displays
        memCacheHeight: (size * 2).toInt().clamp(50, 400),
        placeholder: (context, url) => Container(
          width: size,
          height: size,
          decoration: BoxDecoration(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
            shape: BoxShape.circle,
          ),
          // NO CircularProgressIndicator - just show empty circle while loading
        ),
        errorWidget: (context, url, error) {
          return _buildInitialsAvatar(isDark);
        },
      );
    }

    return _buildInitialsAvatar(isDark);
  }

  Widget _buildInitialsAvatar(bool isDark) {
    final displayInitials = initials ?? _generateInitials(userId);

    return Container(
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            AppColors.primaryRed.withValues(alpha: 0.8),
            AppColors.primaryRed,
          ],
        ),
      ),
      child: Center(
        child: Text(
          displayInitials,
          style: TextStyle(
            color: AppColors.neutralWhite,
            fontSize: _getFontSize(),
            fontWeight: FontWeight.bold,
          ),
        ),
      ),
    );
  }

  Widget _buildEditIcon(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final iconSize = (size * 0.3).clamp(16.0, 28.0);

    return GestureDetector(
      onTap: onEditTap,
      child: Container(
        width: iconSize,
        height: iconSize,
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray700,
          shape: BoxShape.circle,
          border: Border.all(color: AppColors.neutralWhite, width: 1.5),
        ),
        child: Icon(
          Icons.camera_alt,
          color: AppColors.neutralWhite,
          size: iconSize * 0.6,
        ),
      ),
    );
  }

  String _generateInitials(String userId) {
    // Use centralized UserInitialsHelper for consistency
    return UserInitialsHelper.fromUserId(userId);
  }

  double _getFontSize() {
    // Responsive font size based on avatar size
    if (size <= 24) return size * 0.3;
    if (size <= 40) return size * 0.35;
    if (size <= 60) return size * 0.4;
    return size * 0.4;
  }

  // Named constructors untuk common use cases

  /// Small avatar untuk list items, comments
  static ProfileAvatar small({
    required String userId,
    String? imageUrl,
    String? initials,
    VoidCallback? onTap,
  }) {
    return ProfileAvatar(
      userId: userId,
      size: 24,
      imageUrl: imageUrl,
      initials: initials,
      onTap: onTap,
      showShadow: false,
    );
  }

  /// Medium avatar untuk posts, cards
  static ProfileAvatar medium({
    required String userId,
    String? imageUrl,
    String? initials,
    VoidCallback? onTap,
  }) {
    return ProfileAvatar(
      userId: userId,
      size: 40,
      imageUrl: imageUrl,
      initials: initials,
      onTap: onTap,
    );
  }

  /// Large avatar untuk profiles, headers
  static ProfileAvatar large({
    required String userId,
    String? imageUrl,
    String? initials,
    VoidCallback? onTap,
  }) {
    return ProfileAvatar(
      userId: userId,
      size: 80,
      imageUrl: imageUrl,
      initials: initials,
      onTap: onTap,
    );
  }

  /// Extra large avatar untuk profile pages
  static ProfileAvatar extraLarge({
    required String userId,
    String? imageUrl,
    String? initials,
    VoidCallback? onTap,
  }) {
    return ProfileAvatar(
      userId: userId,
      size: 120,
      imageUrl: imageUrl,
      initials: initials,
      onTap: onTap,
    );
  }

  /// Comment avatar - minimal untuk comment threads
  static ProfileAvatar comment({
    required String userId,
    String? imageUrl,
    String? initials,
    VoidCallback? onTap,
  }) {
    return ProfileAvatar(
      userId: userId,
      size: 28,
      imageUrl: imageUrl,
      initials: initials,
      onTap: onTap,
      showShadow: false,
    );
  }

  /// Post header avatar - untuk social media posts
  static ProfileAvatar postHeader({
    required String userId,
    String? imageUrl,
    String? initials,
    VoidCallback? onTap,
  }) {
    return ProfileAvatar(
      userId: userId,
      size: 36,
      imageUrl: imageUrl,
      initials: initials,
      onTap: onTap,
    );
  }
}
