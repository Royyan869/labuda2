import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Reusable App Logo component dengan styling konsisten
///
/// Features:
/// - Hero animation support
/// - Customizable size
/// - Gradient background dengan shadow
/// - Consistent branding
class AppLogo extends StatelessWidget {
  final double size;
  final String? heroTag;
  final bool showShadow;

  const AppLogo({
    super.key,
    this.size = 80,
    this.heroTag,
    this.showShadow = true,
  });

  /// Small logo untuk navbar
  const AppLogo.small({super.key, this.heroTag, this.showShadow = false})
    : size = 40;

  /// Large logo untuk splash/onboarding
  const AppLogo.large({super.key, this.heroTag, this.showShadow = true})
    : size = 120;

  @override
  Widget build(BuildContext context) {
    final logo = Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        gradient: AppColors.primaryGradient,
        borderRadius: BorderRadius.circular(size * 0.2), // 20% of size
        boxShadow: showShadow
            ? [
                BoxShadow(
                  color: AppColors.primaryRed.withValues(alpha: 0.3),
                  blurRadius: size * 0.2,
                  offset: Offset(0, size * 0.1),
                ),
              ]
            : null,
      ),
      child: Icon(
        Icons.water_drop, // LABUDA icon
        size: size * 0.5, // 50% of container size
        color: AppColors.neutralWhite,
      ),
    );

    // Wrap dengan Hero jika ada heroTag
    if (heroTag != null) {
      return Hero(tag: heroTag!, child: logo);
    }

    return logo;
  }
}
