import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:flutter/services.dart';

/// Banner Action Model
///
/// Represents an action button in the notification banner
/// Semantic tint of a banner action. Resolved from the theme where the
/// button is rendered, so a service building actions never needs a
/// BuildContext (and cannot pick a colour that ignores light/dark).
enum BannerTone { accent, success, warning }

class BannerAction {
  final String label;
  final VoidCallback onTap;
  final BannerTone tone;
  final IconData? icon;

  const BannerAction({
    required this.label,
    required this.onTap,
    this.tone = BannerTone.accent,
    this.icon,
  });
}

/// In-App Notification Banner Widget
///
/// Displays notification banner at top of screen with:
/// - Slide-in animation from top
/// - Tap to navigate to notification detail
/// - Swipe up to dismiss
/// - Close button (X) for manual dismiss
/// - Auto-dismiss after 4 seconds (handled by service)
/// - Optional action buttons for quick actions
///
/// Size: < 300 lines (per GUIDELINES)
class InAppNotificationBanner extends StatefulWidget {
  final String title;
  final String body;
  final String? avatarUrl;
  final VoidCallback? onTap;
  final VoidCallback onDismiss;
  final List<BannerAction>? actions;

  const InAppNotificationBanner({
    super.key,
    required this.title,
    required this.body,
    this.avatarUrl,
    this.onTap,
    required this.onDismiss,
    this.actions,
  });

  @override
  State<InAppNotificationBanner> createState() =>
      _InAppNotificationBannerState();
}

class _InAppNotificationBannerState extends State<InAppNotificationBanner>
    with SingleTickerProviderStateMixin {
  late AnimationController _controller;
  late Animation<Offset> _slideAnimation;
  late Animation<double> _fadeAnimation;

  @override
  void initState() {
    super.initState();

    // Animation controller for slide-in effect
    _controller = AnimationController(
      duration: AppMotion.relaxed,
      vsync: this,
    );

    // Slide from top
    _slideAnimation = Tween<Offset>(
      begin: const Offset(0, -1),
      end: Offset.zero,
    ).animate(CurvedAnimation(parent: _controller, curve: Curves.easeOutCubic));

    // Fade in
    _fadeAnimation = Tween<double>(
      begin: 0.0,
      end: 1.0,
    ).animate(CurvedAnimation(parent: _controller, curve: Curves.easeIn));

    // Start animation with haptic feedback
    _controller.forward();

    // Light haptic feedback when banner appears
    HapticFeedback.lightImpact();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  /// Dismiss with animation
  Future<void> _dismissWithAnimation() async {
    // Haptic feedback on dismiss
    HapticFeedback.selectionClick();
    await _controller.reverse();
    widget.onDismiss();
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Positioned(
      top: 0,
      left: 0,
      right: 0,
      child: SlideTransition(
        position: _slideAnimation,
        child: FadeTransition(
          opacity: _fadeAnimation,
          child: SafeArea(
            child: Padding(
              padding: const EdgeInsets.all(AppMetrics.p12),
              child: GestureDetector(
                onTap: widget.onTap != null
                    ? () {
                        _dismissWithAnimation();
                        widget.onTap!();
                      }
                    : null,
                onVerticalDragEnd: (details) {
                  // Swipe up to dismiss
                  if (details.primaryVelocity != null &&
                      details.primaryVelocity! < -500) {
                    _dismissWithAnimation();
                  }
                },
                child: Material(
                  elevation: AppElevation.overlay,
                  shadowColor: colorScheme.shadow.withValues(alpha: 0.3),
                  borderRadius: BorderRadius.circular(AppShape.r12),
                  color: colorScheme.surface,
                  child: Container(
                    padding: const EdgeInsets.all(AppMetrics.p12),
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(AppShape.r12),
                      border: Border.all(
                        color: colorScheme.outlineVariant,
                        width: 1,
                      ),
                    ),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        // Avatar
                        if (widget.avatarUrl != null)
                          ClipOval(
                            child: AppImage(
                              imageUrl: widget.avatarUrl,
                              width: 40,
                              height: 40,
                              fit: BoxFit.cover,
                              isCircle: true,
                              errorWidget: _buildDefaultAvatar(),
                            ),
                          )
                        else
                          _buildDefaultAvatar(),

                        const SizedBox(width: 12),

                        // Content
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              // Title
                              Text(
                                widget.title,
                                style: TextStyle(
                                  fontSize: AppType.s14,
                                  fontWeight: FontWeight.w600,
                                  color: colorScheme.onSurface,
                                ),
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                              ),
                              const SizedBox(height: 4),

                              // Body
                              Text(
                                widget.body,
                                style: TextStyle(
                                  fontSize: AppType.s13,
                                  fontWeight: FontWeight.w400,
                                  color: colorScheme.onSurfaceVariant,
                                  height: 1.3,
                                ),
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                              ),

                              // Action buttons (if any)
                              if (widget.actions != null &&
                                  widget.actions!.isNotEmpty) ...[
                                const SizedBox(height: 8),
                                Row(
                                  children: widget.actions!.map((action) {
                                    return Padding(
                                      padding: const EdgeInsets.only(right: AppMetrics.p8),
                                      child: _buildActionButton(action),
                                    );
                                  }).toList(),
                                ),
                              ],
                            ],
                          ),
                        ),

                        const SizedBox(width: 8),

                        // Close button
                        GestureDetector(
                          onTap: _dismissWithAnimation,
                          child: Container(
                            padding: const EdgeInsets.all(AppMetrics.p4),
                            decoration: BoxDecoration(
                              color: colorScheme.surfaceContainerHighest,
                              shape: BoxShape.circle,
                            ),
                            child: Icon(
                              Icons.close,
                              size: 16,
                              color: colorScheme.onSurfaceVariant,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildDefaultAvatar() {
    final colorScheme = Theme.of(context).colorScheme;

    return Container(
      width: 40,
      height: 40,
      decoration: BoxDecoration(
        color: colorScheme.surfaceContainerHighest,
        shape: BoxShape.circle,
      ),
      child: Icon(
        Icons.notifications_outlined,
        size: 20,
        color: colorScheme.onSurfaceVariant,
      ),
    );
  }

  /// Build action button
  Widget _buildActionButton(BannerAction action) {
    // One authority: the tone resolves through the theme extension at
    // render time (light/dark aware), never a colour carried in data.
    final buttonColor = switch (action.tone) {
      BannerTone.accent => Theme.of(context).colorScheme.primary,
      BannerTone.success => context.statusColors.success,
      BannerTone.warning => context.statusColors.warning,
    };

    return GestureDetector(
      onTap: () {
        // Haptic feedback on button tap
        HapticFeedback.mediumImpact();
        _dismissWithAnimation();
        action.onTap();
      },
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p6),
        decoration: BoxDecoration(
          color: buttonColor.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(AppShape.r6),
          border: Border.all(
            color: buttonColor.withValues(alpha: 0.3),
            width: 1,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (action.icon != null) ...[
              Icon(action.icon, size: 14, color: buttonColor),
              const SizedBox(width: 4),
            ],
            Text(
              action.label,
              style: TextStyle(
                fontSize: AppType.s12,
                fontWeight: FontWeight.w600,
                color: buttonColor,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
