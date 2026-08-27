import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Back button widget for profile AppBar
class ProfileBackButton extends StatelessWidget {
  final double collapseProgress;
  final VoidCallback onPressed;

  const ProfileBackButton({
    super.key,
    required this.collapseProgress,
    required this.onPressed,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return IconButton(
      icon: Container(
        padding: const EdgeInsets.all(6),
        decoration: BoxDecoration(
          color: collapseProgress < 0.5
              ? AppColors.neutralBlack.withValues(alpha: 0.3)
              : Colors.transparent,
          shape: BoxShape.circle,
        ),
        child: Icon(
          Icons.arrow_back,
          color: collapseProgress < 0.5
              ? AppColors.neutralWhite
              : (isDark ? AppColors.neutralWhite : AppColors.neutralGray900),
          size: 20,
        ),
      ),
      onPressed: onPressed,
    );
  }
}

/// AppBar actions for own profile (Edit, Share, Settings)
class OwnProfileActions extends StatelessWidget {
  final double collapseProgress;
  final VoidCallback onEdit;
  final VoidCallback onShare;
  final VoidCallback onSettings;

  const OwnProfileActions({
    super.key,
    required this.collapseProgress,
    required this.onEdit,
    required this.onShare,
    required this.onSettings,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final iconColor = collapseProgress < 0.5
        ? AppColors.neutralWhite
        : (isDark ? AppColors.neutralWhite : AppColors.neutralGray900);
    final bgColor = collapseProgress < 0.5
        ? AppColors.neutralBlack.withValues(alpha: 0.3)
        : Colors.transparent;

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        _buildActionButton(
          Icons.edit_outlined,
          iconColor,
          bgColor,
          onEdit,
          'Edit Profile',
        ),
        _buildActionButton(
          Icons.share_outlined,
          iconColor,
          bgColor,
          onShare,
          'Share',
        ),
        _buildActionButton(
          Icons.settings_outlined,
          iconColor,
          bgColor,
          onSettings,
          'Settings',
        ),
      ],
    );
  }

  Widget _buildActionButton(
    IconData icon,
    Color iconColor,
    Color bgColor,
    VoidCallback onPressed,
    String tooltip,
  ) {
    return IconButton(
      icon: Container(
        padding: const EdgeInsets.all(6),
        decoration: BoxDecoration(color: bgColor, shape: BoxShape.circle),
        child: Icon(icon, color: iconColor, size: 20),
      ),
      onPressed: onPressed,
      tooltip: tooltip,
    );
  }
}

/// AppBar actions for other user's profile (Follow, Message, More)
class OtherProfileActions extends StatelessWidget {
  final double collapseProgress;
  final VoidCallback onFollow;
  final VoidCallback onMessage;
  final VoidCallback onShare;
  final VoidCallback onBlock;
  final VoidCallback onReport;

  const OtherProfileActions({
    super.key,
    required this.collapseProgress,
    required this.onFollow,
    required this.onMessage,
    required this.onShare,
    required this.onBlock,
    required this.onReport,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final iconColor = collapseProgress < 0.5
        ? AppColors.neutralWhite
        : (isDark ? AppColors.neutralWhite : AppColors.neutralGray900);
    final bgColor = collapseProgress < 0.5
        ? AppColors.neutralBlack.withValues(alpha: 0.3)
        : Colors.transparent;

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        IconButton(
          icon: Container(
            padding: const EdgeInsets.all(6),
            decoration: BoxDecoration(color: bgColor, shape: BoxShape.circle),
            child: Icon(Icons.person_add_outlined, color: iconColor, size: 20),
          ),
          onPressed: onFollow,
          tooltip: 'Follow',
        ),
        IconButton(
          icon: Container(
            padding: const EdgeInsets.all(6),
            decoration: BoxDecoration(color: bgColor, shape: BoxShape.circle),
            child: Icon(Icons.chat_bubble_outline, color: iconColor, size: 20),
          ),
          onPressed: onMessage,
          tooltip: 'Message',
        ),
        PopupMoreOptionsButton(
          contentType: PopupMoreOptionsContentType.profile,
          isCreator: false,
          isDeleting: false,
          onShare: onShare,
          onBlock: onBlock,
          onReport: onReport,
          iconColor: iconColor,
        ),
      ],
    );
  }
}
