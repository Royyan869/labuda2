import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Dialog(
      backgroundColor: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: Container(
        width: 340,
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Icon warning
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: AppColors.primaryRed.withValues(alpha: 0.1),
                shape: BoxShape.circle,
              ),
              child: const Icon(
                Icons.block,
                color: AppColors.primaryRed,
                size: 28,
              ),
            ),
            const SizedBox(height: 16),

            // Title
            Text(
              'Block $targetDisplayName?',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.w600,
                color: isDark
                    ? AppColors.neutralWhite
                    : AppColors.neutralGray900,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),

            // Consequences list
            _buildConsequenceItem(
              icon: Icons.visibility_off_outlined,
              text: 'You will not see content from this user',
              isDark: isDark,
            ),
            const SizedBox(height: 8),
            _buildConsequenceItem(
              icon: Icons.person_off_outlined,
              text: 'This user will not be able to see your content',
              isDark: isDark,
            ),
            const SizedBox(height: 8),
            _buildConsequenceItem(
              icon: Icons.chat_bubble_outline,
              text: 'Chat with this user will be hidden',
              isDark: isDark,
            ),
            const SizedBox(height: 8),
            _buildConsequenceItem(
              icon: Icons.people_outline,
              text: 'Follow relationship will be removed',
              isDark: isDark,
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
                      foregroundColor: isDark
                          ? AppColors.neutralGray300
                          : AppColors.neutralGray700,
                      side: BorderSide(
                        color: isDark
                            ? AppColors.darkGray600
                            : AppColors.neutralGray300,
                      ),
                      padding: const EdgeInsets.symmetric(vertical: 12),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(8),
                      ),
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
                      backgroundColor: AppColors.primaryRed,
                      foregroundColor: AppColors.neutralWhite,
                      padding: const EdgeInsets.symmetric(vertical: 12),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(8),
                      ),
                    ),
                    child: isLoading
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: AppColors.neutralWhite,
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

  Widget _buildConsequenceItem({
    required IconData icon,
    required String text,
    required bool isDark,
  }) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(
          icon,
          size: 18,
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Text(
            text,
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray600,
              height: 1.3,
            ),
          ),
        ),
      ],
    );
  }
}
