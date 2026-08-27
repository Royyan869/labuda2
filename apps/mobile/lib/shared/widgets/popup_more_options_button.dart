import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Content type for more options menu
enum PopupMoreOptionsContentType { content, profile, listing, auction }

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
  final Function(String, String?)?
  onStatusChanged; // Legacy content status change hook
  final VoidCallback?
  onToggleStatus; // Deprecated - kept for backward compatibility
  final VoidCallback? onCancel; // For auction cancellation
  final VoidCallback? onShare; // For profile share
  final VoidCallback? onBlock; // For profile block
  final bool isDeleting;
  final bool isCreator;
  final PopupMoreOptionsContentType contentType;
  final String? currentStatus; // For content status display
  final Color? iconColor;
  final double iconSize;

  const PopupMoreOptionsButton({
    super.key,
    this.onReport,
    this.onHide,
    this.onDelete,
    this.onEdit,
    this.onStatusChanged,
    this.onToggleStatus,
    this.onCancel,
    this.onShare,
    this.onBlock,
    this.isDeleting = false,
    this.isCreator = false,
    this.contentType = PopupMoreOptionsContentType.content,
    this.currentStatus,
    this.iconColor,
    this.iconSize = 20,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return PopupMenuButton<String>(
      icon: Icon(
        Icons.more_vert,
        size: iconSize,
        color:
            iconColor ??
            (isDark ? AppColors.neutralGray400 : AppColors.neutralGray600),
      ),
      enabled: !isDeleting,
      onSelected: (value) => _handleMenuSelection(context, value),
      itemBuilder: (context) => _buildMenuItems(context),
      offset: const Offset(0, 8), // Offset popup slightly below icon
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      elevation: 8,
      shadowColor: isDark ? Colors.black54 : Colors.black26,
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
                const Icon(Icons.share_outlined, size: 20),
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
                const Icon(Icons.block_outlined, size: 20),
                const SizedBox(width: 12),
                const Text('Block User'),
              ],
            ),
          ),
        );
      }

      // Report user option
      items.add(
        PopupMenuItem<String>(
          value: 'report',
          child: Row(
            children: [
              const Icon(Icons.report_outlined, size: 20),
              const SizedBox(width: 12),
              const Text('Report User'),
            ],
          ),
        ),
      );

      return items; // Return early for profile, no need for other options
    }

    // Report option (only for non-creators)
    if (!isCreator) {
      items.add(
        PopupMenuItem<String>(
          value: 'report',
          child: Row(
            children: [
              const Icon(Icons.report_outlined, size: 20),
              const SizedBox(width: 12),
              const Text('Report'),
            ],
          ),
        ),
      );
    }

    // Edit option (for creator)
    // For Content: Controlled by onEdit callback
    // For Listing: Always allow edit (both private and for sale)
    // For Auction: Controlled by onEdit callback (only draft/scheduled)
    if (isCreator && onEdit != null) {
      final canEdit =
          contentType == PopupMoreOptionsContentType.content ||
          contentType == PopupMoreOptionsContentType.listing ||
          contentType == PopupMoreOptionsContentType.auction;

      if (canEdit) {
        items.add(
          PopupMenuItem<String>(
            value: 'edit',
            child: Row(
              children: [
                const Icon(Icons.edit_outlined, size: 20),
                const SizedBox(width: 12),
                const Text('Edit'),
              ],
            ),
          ),
        );
      }
    }

    // Delete option (for creator)
    if (isCreator) {
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
                  : const Icon(
                      Icons.delete_outline,
                      size: 20,
                      color: AppColors.error,
                    ),
              const SizedBox(width: 12),
              Text(
                isDeleting ? 'Deleting...' : 'Delete',
                style: const TextStyle(color: AppColors.error),
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
                size: 20,
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
    switch (value) {
      case 'share':
        if (onShare != null) {
          onShare!();
        } else {
          AppSnackBar.showInfo(context, 'Share not available for this content');
        }
        break;
      case 'block':
        if (onBlock != null) {
          onBlock!();
        } else {
          AppSnackBar.showWarning(context, 'Block feature not available');
        }
        break;
      case 'report':
        if (onReport != null) {
          onReport!();
        } else {
          AppSnackBar.showWarning(context, 'Fitur laporan segera hadir');
        }
        break;
      case 'edit':
        if (onEdit != null) {
          onEdit!();
        } else {
          AppSnackBar.showInfo(context, 'Fitur edit segera hadir');
        }
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
