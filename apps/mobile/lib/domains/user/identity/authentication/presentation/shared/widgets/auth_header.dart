import 'package:flutter/material.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

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
    final scheme = Theme.of(context).colorScheme;

    final headerContent = Column(
      children: [
        if (showLogo) ...[
          const SizedBox(height: 40),
          _buildLogo(context),
        ],
        const SizedBox(height: 32),
        Text(
          title,
          style: Theme.of(context).textTheme.headlineMedium?.copyWith(
            fontWeight: FontWeight.bold,
            color: scheme.onSurface,
          ),
          textAlign: TextAlign.center,
        ),
        if (subtitle != null) ...[
          const SizedBox(height: 8),
          Text(
            subtitle!,
            style: Theme.of(context).textTheme.bodyLarge?.copyWith(
              color: scheme.onSurfaceVariant,
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

  Widget _buildLogo(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Container(
        width: 80,
        height: 80,
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(AppShape.r16),
          boxShadow: [
            BoxShadow(
              color: scheme.shadow.withValues(alpha: 0.2),
              blurRadius: 12,
              offset: const Offset(0, 4),
            ),
          ],
        ),
        child: ClipRRect(
          borderRadius: BorderRadius.circular(AppShape.r16),
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
                  color: scheme.primary,
                  borderRadius: BorderRadius.circular(AppShape.r16),
                ),
                child: Icon(
                  Icons.lock_person,
                  size: AppIconSize.display,
                  color: scheme.onPrimary,
                ),
              );
            },
          ),
        ),
      ),
    );
  }
}
