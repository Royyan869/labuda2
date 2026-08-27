import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:labuda/core/core.dart';

/// Banner Action Model
///
/// Represents an action button in the notification banner
class BannerAction {
  final String label;
  final VoidCallback onTap;
  final Color? color;
  final IconData? icon;

  const BannerAction({
    required this.label,
    required this.onTap,
    this.color,
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
      duration: const Duration(milliseconds: 400),
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

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
              padding: const EdgeInsets.all(12.0),
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
                  elevation: 8,
                  shadowColor: Colors.black.withValues(alpha: 0.3),
                  borderRadius: BorderRadius.circular(12),
                  color: isDark
                      ? AppColors.darkGray800
                      : AppColors.neutralWhite,
                  child: Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(12),
                      border: Border.all(
                        color: isDark
                            ? AppColors.darkGray700
                            : AppColors.neutralGray200,
                        width: 1,
                      ),
                    ),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        // Avatar
                        if (widget.avatarUrl != null)
                          ClipOval(
                            child: Image.network(
                              widget.avatarUrl!,
                              width: 40,
                              height: 40,
                              fit: BoxFit.cover,
                              errorBuilder: (context, error, stackTrace) {
                                return _buildDefaultAvatar();
                              },
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
                                  fontSize: 14,
                                  fontWeight: FontWeight.w600,
                                  color: isDark
                                      ? AppColors.neutralWhite
                                      : AppColors.neutralGray900,
                                ),
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                              ),
                              const SizedBox(height: 4),

                              // Body
                              Text(
                                widget.body,
                                style: TextStyle(
                                  fontSize: 13,
                                  fontWeight: FontWeight.w400,
                                  color: isDark
                                      ? AppColors.neutralGray300
                                      : AppColors.neutralGray600,
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
                                      padding: const EdgeInsets.only(right: 8),
                                      child: _buildActionButton(action, isDark),
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
                            padding: const EdgeInsets.all(4),
                            decoration: BoxDecoration(
                              color: isDark
                                  ? AppColors.darkGray700
                                  : AppColors.neutralGray100,
                              shape: BoxShape.circle,
                            ),
                            child: Icon(
                              Icons.close,
                              size: 16,
                              color: isDark
                                  ? AppColors.neutralGray400
                                  : AppColors.neutralGray600,
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      width: 40,
      height: 40,
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
        shape: BoxShape.circle,
      ),
      child: Icon(
        Icons.notifications_outlined,
        size: 20,
        color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
      ),
    );
  }

  /// Build action button
  Widget _buildActionButton(BannerAction action, bool isDark) {
    final buttonColor = action.color ?? AppColors.primaryBlue;

    return GestureDetector(
      onTap: () {
        // Haptic feedback on button tap
        HapticFeedback.mediumImpact();
        _dismissWithAnimation();
        action.onTap();
      },
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: buttonColor.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(6),
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
                fontSize: 12,
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
