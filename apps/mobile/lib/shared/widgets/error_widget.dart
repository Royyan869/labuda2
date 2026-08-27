import 'package:flutter/material.dart';

import 'package:labuda/core/core.dart';
import 'app_snackbar.dart';
import 'app_button.dart';

class ErrorDisplayWidget extends StatelessWidget {
  final String message;
  final String? title;
  final VoidCallback? onRetry;
  final IconData? icon;
  final bool showRetryButton;
  final VoidCallback? onContactSupport;

  const ErrorDisplayWidget({
    super.key,
    required this.message,
    this.title,
    this.onRetry,
    this.icon,
    this.showRetryButton = true,
    this.onContactSupport,
  });

  const ErrorDisplayWidget.network({
    super.key,
    this.message = 'Koneksi bermasalah. Periksa internet kamu lalu coba lagi.',
    this.title = 'Tidak Ada Koneksi',
    this.onRetry,
    this.showRetryButton = true,
    this.onContactSupport,
  }) : icon = Icons.wifi_off_outlined;

  const ErrorDisplayWidget.server({
    super.key,
    this.message = 'Coba lagi beberapa saat.',
    this.title = 'Data belum bisa dimuat',
    this.onRetry,
    this.showRetryButton = true,
    this.onContactSupport,
  }) : icon = Icons.error_outline;

  const ErrorDisplayWidget.notFound({
    super.key,
    this.message = 'Data tidak ditemukan atau sudah tidak tersedia.',
    this.title = 'Tidak Ditemukan',
    this.onRetry,
    this.showRetryButton = false,
    this.onContactSupport,
  }) : icon = Icons.search_off_outlined;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              icon ?? Icons.error_outline,
              size: 64,
              color: context.colorScheme.error.withValues(alpha: 0.7),
            ),
            const SizedBox(height: 16),
            if (title != null) ...[
              Text(
                title!,
                style: AppTypography.h5.copyWith(
                  color: context.colorScheme.onSurface,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 8),
            ],
            Text(
              message,
              style: AppTypography.bodyMedium.copyWith(
                color: context.colorScheme.onSurface.withValues(alpha: 0.7),
              ),
              textAlign: TextAlign.center,
            ),
            if (showRetryButton && onRetry != null) ...[
              const SizedBox(height: 24),
              AppButton.secondary(
                text: 'Coba Lagi',
                onPressed: onRetry,
                icon: Icons.refresh,
              ),
            ],
            // CONTEXTUAL SUPPORT BRIDGE (Phase 2 Hardening)
            // Offer help when retry is available or for server errors
            if ((showRetryButton && onRetry != null) ||
                icon == Icons.error_outline) ...[
              const SizedBox(height: 12),
              TextButton.icon(
                onPressed: onContactSupport,
                icon: const Icon(Icons.support_agent, size: 16),
                label: const Text('Butuh bantuan?'),
                style: TextButton.styleFrom(
                  foregroundColor: context.colorScheme.onSurface.withValues(
                    alpha: 0.6,
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

/// Error SnackBar utility using AppSnackBar
///
/// @deprecated Use AppSnackBar.showError() directly instead
class ErrorSnackBar {
  static void show(
    BuildContext context,
    String message, {
    VoidCallback? onRetry,
  }) {
    AppSnackBar.showError(context, message);
    // Note: onRetry functionality is not directly supported by AppSnackBar
    // Consider using a dialog or other UI pattern for retry actions
  }
}

/// Success SnackBar utility using AppSnackBar
///
/// @deprecated Use AppSnackBar.showSuccess() directly instead
class SuccessSnackBar {
  static void show(BuildContext context, String message) {
    AppSnackBar.showSuccess(context, message);
  }
}
