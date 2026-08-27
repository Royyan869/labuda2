import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/share/domain/entities/share_target.dart';
import 'package:labuda/domains/social/share/domain/entities/share_destination.dart';
import 'package:labuda/domains/social/share/presentation/providers/share_notifier.dart';
import 'package:labuda/domains/social/share/presentation/providers/share_state.dart';
import 'share_preview_card.dart';
import 'share_button_grid.dart';
import 'share_as_post_dialog.dart';

/// Bottom sheet for sharing content
/// Shows preview card and destination options
class ShareBottomSheet extends ConsumerWidget {
  final ShareTarget target;
  final bool
  canSharePost; // True if target is a Post (enable "Share Post" option)

  const ShareBottomSheet({
    super.key,
    required this.target,
    this.canSharePost = false,
  });

  /// Show the share bottom sheet
  static Future<void> show({
    required BuildContext context,
    required ShareTarget target,
    bool canSharePost = false,
  }) {
    return showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) =>
          ShareBottomSheet(target: target, canSharePost: canSharePost),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final backgroundColor = isDark
        ? AppColors.darkGray800
        : AppColors.neutralWhite;

    return Container(
      constraints: BoxConstraints(
        maxHeight: MediaQuery.of(context).size.height * 0.9,
      ),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
      ),
      child: SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Drag handle
            Container(
              margin: const EdgeInsets.only(top: 12),
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: isDark
                    ? AppColors.darkGray500
                    : AppColors.neutralGray300,
                borderRadius: BorderRadius.circular(2),
              ),
            ),

            // Scrollable content
            Flexible(
              child: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // Preview card - more compact
                    SharePreviewCard(target: target, isDark: isDark),

                    const SizedBox(height: 8),

                    // All share options in grid
                    Padding(
                      padding: const EdgeInsets.only(bottom: 24),
                      child: ShareButtonGrid(
                        destinations: _getShareDestinations(),
                        onTap: (destination) =>
                            _handleDestinationTap(context, ref, destination),
                        isDark: isDark,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  /// Get all share destinations based on content type
  List<ShareDestination> _getShareDestinations() {
    final destinations = <ShareDestination>[];

    // For Posts: add "Share Post" option
    if (canSharePost) {
      destinations.add(
        const ShareDestination(
          type: ShareDestinationType.shareToFeed,
          label: 'Share Post',
          iconName: 'repeat',
          colorHex: '#1976D2', // primaryBlue
          isInternal: true,
        ),
      );
    }

    // Share as new Post (all content types)
    destinations.add(
      const ShareDestination(
        type: ShareDestinationType.shareToFeed,
        label: 'To Feed',
        iconName: 'home',
        colorHex: '#D32F2F', // primaryRed
        isInternal: true,
      ),
    );

    // Add external destinations
    destinations.addAll(ShareDestination.externalDestinations);

    return destinations;
  }

  /// Handle tap on any share destination
  void _handleDestinationTap(
    BuildContext context,
    WidgetRef ref,
    ShareDestination destination,
  ) {
    // Check if it's internal sharing
    if (destination.isInternal) {
      // Check if it's repost (for Posts only)
      final isRepost = canSharePost && destination.iconName == 'repeat';

      if (isRepost) {
        _handleSharePost(context, ref);
      } else {
        _handleShareAsPost(context, ref);
      }
    } else {
      // External sharing
      _handleExternalShare(context, ref, destination);
    }
  }

  Future<void> _handleShareAsPost(BuildContext context, WidgetRef ref) async {
    // Close bottom sheet
    Navigator.pop(context);

    // Show dialog for caption input
    await ShareAsPostDialog.show(context: context, target: target);
  }

  Future<void> _handleSharePost(BuildContext context, WidgetRef ref) async {
    // Close bottom sheet
    Navigator.pop(context);

    // For re-sharing posts, use the same dialog but with different default caption
    await ShareAsPostDialog.show(
      context: context,
      target: target,
      isRepost: true,
    );
  }

  Future<void> _handleExternalShare(
    BuildContext context,
    WidgetRef ref,
    ShareDestination destination,
  ) async {
    // Show loading
    if (context.mounted) {
      showDialog(
        context: context,
        barrierDismissible: false,
        builder: (context) => const Center(child: CircularProgressIndicator()),
      );
    }

    // Execute share
    final notifier = ref.read(shareNotifierProvider.notifier);
    await notifier.shareViaExternal(
      target: target,
      destination: destination.type,
    );

    // Close loading
    if (context.mounted) {
      Navigator.pop(context);
    }

    // Close bottom sheet
    if (context.mounted) {
      Navigator.pop(context);
    }

    // Show result
    if (context.mounted) {
      final shareState = ref.read(shareNotifierProvider);

      if (shareState is ShareSuccess) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Successfully shared to ${destination.label}'),
            backgroundColor: AppColors.statusSuccess,
            duration: const Duration(seconds: 3),
          ),
        );
      } else if (shareState is ShareError) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(shareState.failure.message),
            backgroundColor: AppColors.statusError,
            duration: const Duration(seconds: 4),
          ),
        );
      } else {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Failed to share content'),
            backgroundColor: AppColors.statusError,
            duration: Duration(seconds: 4),
          ),
        );
      }
    }
  }
}
