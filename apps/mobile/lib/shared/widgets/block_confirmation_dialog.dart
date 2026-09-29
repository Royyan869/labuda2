import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Dialog konfirmasi sebelum memblokir user
///
/// Menampilkan informasi user yang akan diblokir dan konsekuensi blocking.
/// Returns true jika user mengkonfirmasi block, false jika cancel.
class BlockConfirmationDialog extends StatelessWidget {
  final String targetUserId;
  final String targetDisplayName;
  final String? targetAvatarUrl;
  final bool isLoading;

  const BlockConfirmationDialog({
    super.key,
    required this.targetUserId,
    required this.targetDisplayName,
    this.targetAvatarUrl,
    this.isLoading = false,
  });

  /// Show the block confirmation dialog
  ///
  /// Returns true if user confirms block, false otherwise.
  static Future<bool?> show(
    BuildContext context, {
    required String targetUserId,
    required String targetDisplayName,
    String? targetAvatarUrl,
  }) {
    return showDialog<bool>(
      context: context,
      barrierDismissible: true,
      builder: (context) => BlockConfirmationDialog(
        targetUserId: targetUserId,
        targetDisplayName: targetDisplayName,
        targetAvatarUrl: targetAvatarUrl,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Dialog(
      backgroundColor: scheme.surfaceContainerHigh,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppShape.r16)),
      child: Container(
        width: 340,
        padding: const EdgeInsets.all(AppMetrics.p20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Icon warning
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: scheme.error.withValues(alpha: 0.1),
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.block,
                color: scheme.error,
                size: 28,
              ),
            ),
            const SizedBox(height: 16),

            // Title
            Text(
              'Block $targetDisplayName?',
              style: TextStyle(
                fontSize: AppType.s18,
                fontWeight: FontWeight.w600,
                color: scheme.onSurface,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),

            // Consequences list
            _buildConsequenceItem(
              context,
              icon: Icons.visibility_off_outlined,
              text: 'You will not see content from this user',
            ),
            const SizedBox(height: 8),
            _buildConsequenceItem(
              context,
              icon: Icons.person_off_outlined,
              text: 'This user will not be able to see your content',
            ),
            const SizedBox(height: 8),
            _buildConsequenceItem(
              context,
              icon: Icons.chat_bubble_outline,
              text: 'Chat with this user will be hidden',
            ),
            const SizedBox(height: 8),
            _buildConsequenceItem(
              context,
              icon: Icons.people_outline,
              text: 'Follow relationship will be removed',
            ),
            const SizedBox(height: 24),

            // Buttons
            Row(
              children: [
                Expanded(
                  child: OutlinedButton(
                    onPressed: isLoading
                        ? null
                        : () => Navigator.pop(context, false),
                    style: OutlinedButton.styleFrom(
                      padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12),
                    ),
                    child: const Text('Cancel'),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: ElevatedButton(
                    onPressed: isLoading
                        ? null
                        : () => Navigator.pop(context, true),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: scheme.error,
                      foregroundColor: scheme.onError,
                      padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12),
                    ),
                    child: isLoading
                        ? SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: scheme.onError,
                            ),
                          )
                        : const Text('Block'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildConsequenceItem(
    BuildContext context, {
    required IconData icon,
    required String text,
  }) {
    final scheme = Theme.of(context).colorScheme;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(
          icon,
          size: 18,
          color: scheme.onSurfaceVariant,
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Text(
            text,
            style: TextStyle(
              fontSize: AppType.s14,
              color: scheme.onSurface,
              height: 1.3,
            ),
          ),
        ),
      ],
    );
  }
}
