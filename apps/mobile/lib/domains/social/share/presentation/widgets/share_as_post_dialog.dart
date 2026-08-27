import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/share/domain/entities/share_target.dart';
import 'package:labuda/domains/social/share/presentation/providers/share_notifier.dart';
import 'package:labuda/domains/social/share/presentation/providers/share_state.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/blocked_action_gate.dart';
import 'package:labuda/features/home/home.dart';
import 'share_preview_card.dart';

/// Dialog for sharing content as a new Post with optional caption
class ShareAsPostDialog extends ConsumerStatefulWidget {
  final ShareTarget target;
  final bool isRepost; // True if sharing a Post as a repost

  const ShareAsPostDialog({
    super.key,
    required this.target,
    this.isRepost = false,
  });

  /// Show the dialog
  static Future<void> show({
    required BuildContext context,
    required ShareTarget target,
    bool isRepost = false,
  }) {
    return showDialog(
      context: context,
      builder: (context) =>
          ShareAsPostDialog(target: target, isRepost: isRepost),
    );
  }

  @override
  ConsumerState<ShareAsPostDialog> createState() => _ShareAsPostDialogState();
}

class _ShareAsPostDialogState extends ConsumerState<ShareAsPostDialog> {
  final _captionController = TextEditingController();
  bool _isLoading = false;

  @override
  void dispose() {
    _captionController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final backgroundColor = isDark
        ? AppColors.darkGray800
        : AppColors.neutralWhite;
    final textColor = isDark
        ? AppColors.neutralGray100
        : AppColors.neutralGray900;
    final borderColor = isDark
        ? AppColors.darkGray600
        : AppColors.neutralGray300;
    final dividerColor = isDark
        ? AppColors.darkGray600
        : AppColors.neutralGray200;

    return Dialog(
      backgroundColor: backgroundColor,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: Container(
        constraints: const BoxConstraints(maxWidth: 500),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Header
            Padding(
              padding: const EdgeInsets.all(20),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      widget.isRepost ? 'Share This Post' : 'Share to Feed',
                      style: AppTypography.h5.copyWith(
                        fontWeight: FontWeight.w600,
                        color: textColor,
                      ),
                    ),
                  ),
                  IconButton(
                    icon: Icon(Icons.close, color: textColor),
                    onPressed: () => Navigator.pop(context),
                    padding: EdgeInsets.zero,
                    constraints: const BoxConstraints(),
                  ),
                ],
              ),
            ),

            Divider(height: 1, color: dividerColor),

            // Scrollable content
            Flexible(
              child: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // Preview card
                    SharePreviewCard(target: widget.target, isDark: isDark),

                    Divider(height: 1, color: dividerColor),

                    // Caption input
                    Padding(
                      padding: const EdgeInsets.all(20),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            widget.isRepost
                                ? 'Add a comment (optional)'
                                : 'Write a caption (optional)',
                            style: AppTypography.bodyMedium.copyWith(
                              fontWeight: FontWeight.w500,
                              color: textColor,
                            ),
                          ),
                          const SizedBox(height: 12),
                          TextField(
                            controller: _captionController,
                            maxLines: 4,
                            maxLength: 500,
                            style: AppTypography.bodyMedium.copyWith(
                              color: textColor,
                            ),
                            decoration: InputDecoration(
                              hintText: widget.isRepost
                                  ? 'Add your thoughts...'
                                  : 'Write something about this...',
                              hintStyle: AppTypography.bodyMedium.copyWith(
                                color: isDark
                                    ? AppColors.neutralGray500
                                    : AppColors.neutralGray400,
                              ),
                              border: OutlineInputBorder(
                                borderRadius: BorderRadius.circular(12),
                                borderSide: BorderSide(color: borderColor),
                              ),
                              enabledBorder: OutlineInputBorder(
                                borderRadius: BorderRadius.circular(12),
                                borderSide: BorderSide(color: borderColor),
                              ),
                              focusedBorder: OutlineInputBorder(
                                borderRadius: BorderRadius.circular(12),
                                borderSide: const BorderSide(
                                  color: AppColors.primaryRed,
                                  width: 2,
                                ),
                              ),
                              contentPadding: const EdgeInsets.all(16),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),

            Divider(height: 1, color: dividerColor),

            // Action buttons
            Padding(
              padding: const EdgeInsets.all(20),
              child: Row(
                children: [
                  Expanded(
                    child: OutlinedButton(
                      onPressed: _isLoading
                          ? null
                          : () => Navigator.pop(context),
                      style: OutlinedButton.styleFrom(
                        padding: const EdgeInsets.symmetric(vertical: 14),
                        side: BorderSide(color: borderColor),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(12),
                        ),
                      ),
                      child: Text(
                        'Cancel',
                        style: AppTypography.button.copyWith(color: textColor),
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: ElevatedButton(
                      onPressed: _isLoading ? null : _handlePost,
                      style: ElevatedButton.styleFrom(
                        backgroundColor: AppColors.primaryRed,
                        padding: const EdgeInsets.symmetric(vertical: 14),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(12),
                        ),
                      ),
                      child: _isLoading
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                valueColor: AlwaysStoppedAnimation<Color>(
                                  AppColors.neutralWhite,
                                ),
                              ),
                            )
                          : Text(
                              widget.isRepost ? 'Share' : 'Post',
                              style: AppTypography.button.copyWith(
                                color: AppColors.neutralWhite,
                              ),
                            ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _handlePost() async {
    // Get current user ID
    final authState = ref.read(authControllerProvider);
    final userId = switch (authState) {
      AuthStateAuthenticated(:final user) => user.id,
      _ => null,
    };

    if (userId == null) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('You must login first'),
            backgroundColor: AppColors.statusError,
            duration: Duration(seconds: 4),
          ),
        );
      }
      return;
    }

    // Pre-flight gate: share-as-post creates a new Post → BLOCKED for
    // unverified users per email-gating-matrix ("Create post").
    if (authState is AuthStateAuthenticated && !authState.emailVerified) {
      await showBlockedActionGate(
        context,
        actionDescription: 'membagikan ke feed',
      );
      return;
    }

    setState(() => _isLoading = true);

    final notifier = ref.read(shareNotifierProvider.notifier);
    final caption = _captionController.text.trim();

    final postId = await notifier.shareAsPost(
      target: widget.target,
      authorId: userId,
      caption: caption.isEmpty ? null : caption,
    );

    if (mounted) {
      setState(() => _isLoading = false);

      if (postId != null) {
        // Close dialog
        Navigator.pop(context);

        // Refresh feed to show the new shared post
        refreshFeedGlobally();

        // Show success message
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Successfully shared to feed'),
            backgroundColor: AppColors.statusSuccess,
            duration: Duration(seconds: 3),
          ),
        );
      } else {
        // Show error
        final shareState = ref.read(shareNotifierProvider);
        if (shareState is ShareError) {
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
              content: Text('Failed to share to feed'),
              backgroundColor: AppColors.statusError,
              duration: Duration(seconds: 4),
            ),
          );
        }
      }
    }
  }
}
