import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Banner yang ditampilkan saat melihat profile/content dari blocked user
///
/// Menampilkan warning dan opsi untuk unblock.
class BlockedUserBanner extends StatelessWidget {
  final String? displayName;
  final VoidCallback? onUnblock;
  final bool isLoading;

  const BlockedUserBanner({
    super.key,
    this.displayName,
    this.onUnblock,
    this.isLoading = false,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      decoration: BoxDecoration(
        color: isDark
            ? AppColors.statusWarning.withValues(alpha: 0.15)
            : AppColors.statusWarning.withValues(alpha: 0.1),
        border: Border(
          bottom: BorderSide(
            color: isDark
                ? AppColors.statusWarning.withValues(alpha: 0.3)
                : AppColors.statusWarning.withValues(alpha: 0.2),
          ),
        ),
      ),
      child: Row(
        children: [
          Icon(Icons.block, size: 20, color: AppColors.statusWarning),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              displayName != null
                  ? 'Kamu telah memblokir $displayName'
                  : 'Kamu telah memblokir user ini',
              style: TextStyle(
                fontSize: 14,
                color: isDark
                    ? AppColors.neutralGray200
                    : AppColors.neutralGray700,
              ),
            ),
          ),
          if (onUnblock != null) ...[
            const SizedBox(width: 8),
            TextButton(
              onPressed: isLoading ? null : onUnblock,
              style: TextButton.styleFrom(
                foregroundColor: AppColors.primaryRed,
                padding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 6,
                ),
                minimumSize: Size.zero,
                tapTargetSize: MaterialTapTargetSize.shrinkWrap,
              ),
              child: isLoading
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: AppColors.primaryRed,
                      ),
                    )
                  : const Text(
                      'Unblock',
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
            ),
          ],
        ],
      ),
    );
  }
}
