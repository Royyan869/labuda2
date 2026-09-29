import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/domains/social/share/domain/entities/share_target.dart';
import 'package:labuda/domains/social/share/domain/entities/share_destination.dart';
import 'package:labuda/domains/social/share/presentation/providers/share_notifier.dart';
import 'package:labuda/domains/social/share/presentation/providers/share_state.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
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
    final scheme = Theme.of(context).colorScheme;
    final backgroundColor = scheme.surfaceContainerLow;

    return Container(
      constraints: BoxConstraints(
        maxHeight: MediaQuery.of(context).size.height * 0.9,
      ),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(AppShape.r20)),
      ),
      child: SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Drag handle
            Container(
              margin: const EdgeInsets.only(top: AppMetrics.p12),
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: scheme.outlineVariant,
                borderRadius: BorderRadius.circular(AppShape.r2),
              ),
            ),

            // Scrollable content
            Flexible(
              child: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // Preview card - more compact
                    SharePreviewCard(target: target),

                    const SizedBox(height: 8),

                    // All share options in grid
                    Padding(
                      padding: const EdgeInsets.only(bottom: AppMetrics.p24),
                      child: ShareButtonGrid(
                        destinations: _getShareDestinations(),
                        onTap: (destination) =>
                            _handleDestinationTap(context, ref, destination),
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
        AppSnackBar.showSuccess(
          context,
          'Successfully shared to ${destination.label}',
          duration: const Duration(seconds: 3),
        );
      } else if (shareState is ShareError) {
        AppSnackBar.showError(
          context,
          shareState.message,
          duration: const Duration(seconds: 4),
        );
      } else {
        AppSnackBar.showError(
          context,
          'Failed to share content',
          duration: const Duration(seconds: 4),
        );
      }
    }
  }
}
