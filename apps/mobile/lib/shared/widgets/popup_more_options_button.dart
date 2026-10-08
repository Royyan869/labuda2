import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Content type for more options menu
enum PopupMoreOptionsContentType { content, profile, forSale, auction }

/// Reusable Popup More Options Button (3 dots menu using PopupMenuButton)
///
/// Features:
/// - Consistent styling across cards and detail screens
/// - PopupMenuButton instead of modal bottomsheet for better UX
/// - Configurable options for different content types
/// - Standard behavior for report, hide, delete
/// - Request-specific status toggle for creators
/// - Creator vs non-creator options
class PopupMoreOptionsButton extends StatelessWidget {
  final VoidCallback? onReport;
  final VoidCallback? onHide;
  final VoidCallback? onDelete;
  final VoidCallback? onEdit;
  final VoidCallback? onCancel; // For auction cancellation
  final VoidCallback? onShare; // For profile share
  final VoidCallback? onBlock; // For profile block
  final bool isDeleting;
  final bool isCreator;
  final PopupMoreOptionsContentType contentType;
  final Color? iconColor;
  final double iconSize;

  const PopupMoreOptionsButton({
    super.key,
    this.onReport,
    this.onHide,
    this.onDelete,
    this.onEdit,
    this.onCancel,
    this.onShare,
    this.onBlock,
    this.isDeleting = false,
    this.isCreator = false,
    this.contentType = PopupMoreOptionsContentType.content,
    this.iconColor,
    this.iconSize = AppIconSize.action,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return PopupMenuButton<String>(
      icon: Icon(
        Icons.more_vert,
        size: iconSize,
        color: iconColor ?? scheme.onSurfaceVariant,
      ),
      enabled: !isDeleting,
      onSelected: (value) => _handleMenuSelection(context, value),
      itemBuilder: (context) => _buildMenuItems(context),
      offset: const Offset(0, 8), // Offset popup slightly below icon
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppShape.r8)),
      elevation: AppElevation.overlay,
      shadowColor: scheme.shadow.withValues(alpha: 0.3),
    );
  }

  List<PopupMenuEntry<String>> _buildMenuItems(BuildContext context) {
    final items = <PopupMenuEntry<String>>[];

    // Profile-specific options first
    if (contentType == PopupMoreOptionsContentType.profile) {
      // Share profile option
      if (onShare != null) {
        items.add(
          PopupMenuItem<String>(
            value: 'share',
            child: Row(
              children: [
                const Icon(Icons.share_outlined, size: AppIconSize.action),
                const SizedBox(width: 12),
                const Text('Share Profile'),
              ],
            ),
          ),
        );
      }

      // Block user option
      if (onBlock != null) {
        items.add(
          PopupMenuItem<String>(
            value: 'block',
            child: Row(
              children: [
                const Icon(Icons.block_outlined, size: AppIconSize.action),
                const SizedBox(width: 12),
                const Text('Block User'),
              ],
            ),
          ),
        );
      }

      // Report user option — only when the action is actually provided; a
      // visible-but-dead item that can only toast "coming soon" is a bug.
      if (onReport != null) {
        items.add(
          PopupMenuItem<String>(
            value: 'report',
            child: Row(
              children: [
                const Icon(Icons.report_outlined, size: AppIconSize.action),
                const SizedBox(width: 12),
                const Text('Report User'),
              ],
            ),
          ),
        );
      }

      return items; // Return early for profile, no need for other options
    }

    // Report option (only for non-creators, and only when provided).
    if (!isCreator && onReport != null) {
      items.add(
        PopupMenuItem<String>(
          value: 'report',
          child: Row(
            children: [
              const Icon(Icons.report_outlined, size: AppIconSize.action),
              const SizedBox(width: 12),
              const Text('Report'),
            ],
          ),
        ),
      );
    }

    // Edit option (for creator)
    // For Content: Controlled by onEdit callback
    // For ForSale: Always allow edit (both private and for sale)
    // For Auction: Controlled by onEdit callback (only draft/scheduled)
    if (isCreator && onEdit != null) {
      final canEdit =
          contentType == PopupMoreOptionsContentType.content ||
          contentType == PopupMoreOptionsContentType.forSale ||
          contentType == PopupMoreOptionsContentType.auction;

      if (canEdit) {
        items.add(
          PopupMenuItem<String>(
            value: 'edit',
            child: Row(
              children: [
                const Icon(Icons.edit_outlined, size: AppIconSize.action),
                const SizedBox(width: 12),
                const Text('Edit'),
              ],
            ),
          ),
        );
      }
    }

    // Delete option (for creator) — canonical rule: an optional action must
    // not render when its callback is null (mirrors the onEdit guard above;
    // a visible-but-dead Delete is a bug, not a disabled affordance).
    if (isCreator && onDelete != null) {
      items.add(
        PopupMenuItem<String>(
          value: 'delete',
          enabled: !isDeleting,
          child: Row(
            children: [
              isDeleting
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Icon(
                      Icons.delete_outline,
                      size: AppIconSize.action,
                      color: Theme.of(context).colorScheme.error,
                    ),
              const SizedBox(width: 12),
              Text(
                isDeleting ? 'Deleting...' : 'Delete',
                style: TextStyle(
                  color: Theme.of(context).colorScheme.error,
                ),
              ),
            ],
          ),
        ),
      );
    }

    // Cancel option (for auction creator - cancels active auction)
    if (isCreator &&
        contentType == PopupMoreOptionsContentType.auction &&
        onCancel != null) {
      items.add(
        PopupMenuItem<String>(
          value: 'cancel',
          child: Row(
            children: [
              const Icon(
                Icons.cancel_outlined,
                size: AppIconSize.action,
                color: AppColors.koiOrange,
              ),
              const SizedBox(width: 12),
              const Text(
                'Cancel Auction',
                style: TextStyle(color: AppColors.koiOrange),
              ),
            ],
          ),
        ),
      );
    }

    return items;
  }

  void _handleMenuSelection(BuildContext context, String value) {
    // Every item is only rendered when its callback exists, so selection is a
    // direct dispatch — there is no "action unavailable" fallback to toast.
    switch (value) {
      case 'share':
        onShare?.call();
        break;
      case 'block':
        onBlock?.call();
        break;
      case 'report':
        onReport?.call();
        break;
      case 'edit':
        onEdit?.call();
        break;
      case 'delete':
        onDelete?.call();
        break;
      case 'cancel':
        onCancel?.call();
        break;
    }
  }
}
