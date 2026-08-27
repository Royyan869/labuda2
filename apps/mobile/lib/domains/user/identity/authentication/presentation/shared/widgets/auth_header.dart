import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Authentication screen header
///
/// Provides consistent header styling with logo, title, and subtitle.
/// Supports optional animations.
///
/// Example usage:
/// ```dart
/// AuthHeader(
///   title: 'Welcome Back',
///   subtitle: 'Sign in to your LABUDA account',
/// )
///
/// // With animations
/// AuthHeader.animated(
///   title: 'Create Account',
///   subtitle: 'Join LABUDA today',
///   fadeAnimation: _fadeAnimation,
///   slideAnimation: _slideAnimation,
/// )
/// ```
class AuthHeader extends StatelessWidget {
  final String title;
  final String? subtitle;
  final bool showLogo;
  final String? logoPath;
  final Animation<double>? fadeAnimation;
  final Animation<Offset>? slideAnimation;
  final EdgeInsetsGeometry? margin;

  const AuthHeader({
    super.key,
    required this.title,
    this.subtitle,
    this.showLogo = true,
    this.logoPath,
    this.fadeAnimation,
    this.slideAnimation,
    this.margin,
  });

  /// Header with animations
  const AuthHeader.animated({
    super.key,
    required this.title,
    this.subtitle,
    this.showLogo = true,
    this.logoPath,
    required this.fadeAnimation,
    required this.slideAnimation,
    this.margin,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    final headerContent = Column(
      children: [
        if (showLogo) ...[
          const SizedBox(height: 40),
          _buildLogo(context, isDark),
        ],
        const SizedBox(height: 32),
        Text(
          title,
          style: Theme.of(context).textTheme.headlineMedium?.copyWith(
            fontWeight: FontWeight.bold,
            color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
          ),
          textAlign: TextAlign.center,
        ),
        if (subtitle != null) ...[
          const SizedBox(height: 8),
          Text(
            subtitle!,
            style: Theme.of(context).textTheme.bodyLarge?.copyWith(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
            textAlign: TextAlign.center,
          ),
        ],
        const SizedBox(height: 48),
      ],
    );

    final wrappedContent = Padding(
      padding: margin ?? EdgeInsets.zero,
      child: headerContent,
    );

    // Apply animations if provided
    if (fadeAnimation != null && slideAnimation != null) {
      return FadeTransition(
        opacity: fadeAnimation!,
        child: SlideTransition(
          position: slideAnimation!,
          child: wrappedContent,
        ),
      );
    }

    if (fadeAnimation != null) {
      return FadeTransition(opacity: fadeAnimation!, child: wrappedContent);
    }

    return wrappedContent;
  }

  Widget _buildLogo(BuildContext context, bool isDark) {
    return Center(
      child: Container(
        width: 80,
        height: 80,
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(16),
          boxShadow: [
            BoxShadow(
              color: isDark
                  ? Colors.black.withValues(alpha: 0.3)
                  : Colors.grey.withValues(alpha: 0.2),
              blurRadius: 12,
              offset: const Offset(0, 4),
            ),
          ],
        ),
        child: ClipRRect(
          borderRadius: BorderRadius.circular(16),
          child: Image.asset(
            logoPath ?? 'assets/images/app_logo.png',
            width: 80,
            height: 80,
            fit: BoxFit.cover,
            errorBuilder: (context, error, stackTrace) {
              // Fallback if logo image not found
              return Container(
                width: 80,
                height: 80,
                decoration: BoxDecoration(
                  color: AppColors.primaryRed,
                  borderRadius: BorderRadius.circular(16),
                ),
                child: const Icon(
                  Icons.lock_person,
                  size: 40,
                  color: AppColors.neutralWhite,
                ),
              );
            },
          ),
        ),
      ),
    );
  }
}
