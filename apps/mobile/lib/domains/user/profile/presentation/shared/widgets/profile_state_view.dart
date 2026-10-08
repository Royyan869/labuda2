import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Profile state view for conditional rendering
///
/// Replaces scattered conditional rendering (if/else blocks) in profile screens.
/// Provides smooth transitions between loading, content, success, and error states.
///
/// Example usage:
/// ```dart
/// ProfileStateView(
///   isLoading: controller.isLoading,
///   error: controller.errorMessage,
///   success: controller.successMessage,
///   content: FormContent(...),
///   onErrorDismiss: controller.clearError,
/// )
/// ```
///
/// @see REFACTOR_UI.md section 11 for non-auth form guidelines
class ProfileStateView extends StatelessWidget {
  final bool isLoading;
  final String? error;
  final String? success;
  final Widget content;
  final Widget? loadingWidget;
  final Widget? errorWidget;
  final Widget? successWidget;
  final VoidCallback? onErrorDismiss;
  final VoidCallback? onSuccessDismiss;

  const ProfileStateView({
    super.key,
    required this.content,
    this.isLoading = false,
    this.error,
    this.success,
    this.loadingWidget,
    this.errorWidget,
    this.successWidget,
    this.onErrorDismiss,
    this.onSuccessDismiss,
  });

  @override
  Widget build(BuildContext context) {
    // Show success state if present
    if (success != null) {
      return successWidget ?? _buildDefaultSuccess(context);
    }

    // Show error state if error exists and not loading
    if (error != null && !isLoading) {
      return errorWidget ?? _buildDefaultError(context);
    }

    // Show loading state
    if (isLoading) {
      return loadingWidget ?? _buildDefaultLoading(context);
    }

    // Show content
    return AnimatedSwitcher(
      duration: AppMotion.fast,
      child: content,
    );
  }

  Widget _buildDefaultLoading(BuildContext context) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          CircularProgressIndicator(
            valueColor: AlwaysStoppedAnimation<Color>(
              Theme.of(context).colorScheme.primary,
            ),
          ),
          const SizedBox(height: 16),
          Text(
            'Please wait...',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildDefaultError(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              width: 64,
              height: 64,
              decoration: BoxDecoration(
                color: context.statusColors.error.withValues(alpha: 0.1),
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.error_outline,
                size: AppIconSize.emphasis,
                color: context.statusColors.error,
              ),
            ),
            const SizedBox(height: 16),
            Text(
              error!,
              style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
            if (onErrorDismiss != null) ...[
              const SizedBox(height: 24),
              // Background colour comes from AppTheme.elevatedButtonTheme.
              ElevatedButton(
                onPressed: onErrorDismiss,
                child: const Text('Try Again'),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildDefaultSuccess(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              width: 80,
              height: 80,
              decoration: BoxDecoration(
                color: context.statusColors.success.withValues(alpha: 0.1),
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.check_circle,
                size: AppIconSize.display,
                color: context.statusColors.success,
              ),
            ),
            const SizedBox(height: 24),
            Text(
              success!,
              style: Theme.of(context).textTheme.titleLarge?.copyWith(
                color: Theme.of(context).colorScheme.onSurface,
                fontWeight: FontWeight.bold,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}

/// Banner widget for showing error/success messages at top of screen
///
/// Use this for non-blocking messages that don't require full screen state change.
///
/// Example usage:
/// ```dart
/// ProfileStateBanner(
///   error: controller.errorMessage,
///   success: controller.successMessage,
///   onDismiss: controller.clearMessages,
/// )
/// ```
class ProfileStateBanner extends StatelessWidget {
  final String? error;
  final String? success;
  final VoidCallback? onDismiss;

  const ProfileStateBanner({
    super.key,
    this.error,
    this.success,
    this.onDismiss,
  });

  @override
  Widget build(BuildContext context) {
    if (error == null && success == null) {
      return const SizedBox.shrink();
    }

    final isError = error != null;
    final message = isError ? error! : success!;
    final backgroundColor = isError ? context.statusColors.error : context.statusColors.success;
    final icon = isError ? Icons.error_outline : Icons.check_circle;

    return AnimatedContainer(
      duration: AppMotion.settled,
      margin: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p8, AppMetrics.p16, AppMetrics.p0),
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p12),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      child: Row(
        children: [
          Icon(icon, color: Theme.of(context).colorScheme.onPrimary),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              message,
              style: TextStyle(
                color: Theme.of(context).colorScheme.onPrimary,
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
          if (onDismiss != null)
            IconButton(
              onPressed: onDismiss,
              icon: Icon(Icons.close, color: Theme.of(context).colorScheme.onPrimary, semanticLabel: 'Tutup'),
              padding: EdgeInsets.zero,
              constraints: const BoxConstraints(),
            ),
        ],
      ),
    );
  }
}
