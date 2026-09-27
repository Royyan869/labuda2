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
    final scheme = Theme.of(context).colorScheme;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      decoration: BoxDecoration(
        color: AppColors.statusWarning.withValues(alpha: 0.12),
        border: Border(
          bottom: BorderSide(
            color: AppColors.statusWarning.withValues(alpha: 0.3),
          ),
        ),
      ),
      child: Row(
        children: [
          const Icon(Icons.block, size: 20, color: AppColors.statusWarning),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              displayName != null
                  ? 'Kamu telah memblokir $displayName'
                  : 'Kamu telah memblokir user ini',
              style: TextStyle(fontSize: 14, color: scheme.onSurface),
            ),
          ),
          if (onUnblock != null) ...[
            const SizedBox(width: 8),
            TextButton(
              onPressed: isLoading ? null : onUnblock,
              style: TextButton.styleFrom(
                foregroundColor: scheme.primary,
                padding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 6,
                ),
                minimumSize: Size.zero,
                tapTargetSize: MaterialTapTargetSize.shrinkWrap,
              ),
              child: isLoading
                  ? SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: scheme.primary,
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
