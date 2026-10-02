import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Authentication state view for conditional rendering
///
/// Replaces scattered conditional rendering (if/else blocks) in auth screens.
/// Provides smooth transitions between loading, content, success, and error states.
///
/// Example usage:
/// ```dart
/// AuthStateView(
///   isLoading: controller.isLoading,
///   error: controller.errorMessage,
///   success: controller.successMessage,
///   content: FormContent(...),
///   onErrorDismiss: controller.clearError,
/// )
/// ```
class AuthStateView extends StatelessWidget {
  final bool isLoading;
  final String? error;
  final String? success;
  final Widget content;
  final Widget? loadingWidget;
  final Widget? errorWidget;
  final Widget? successWidget;
  final VoidCallback? onErrorDismiss;
  final VoidCallback? onSuccessDismiss;

  const AuthStateView({
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
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          CircularProgressIndicator(
            valueColor: AlwaysStoppedAnimation<Color>(scheme.primary),
          ),
          const SizedBox(height: 16),
          Text(
            'Please wait...',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildDefaultError(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

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
                color: scheme.error.withValues(alpha: 0.1),
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.error_outline,
                size: AppIconSize.emphasis,
                color: scheme.error,
              ),
            ),
            const SizedBox(height: 16),
            Text(
              error!,
              style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                color: scheme.onSurface,
              ),
              textAlign: TextAlign.center,
            ),
            if (onErrorDismiss != null) ...[
              const SizedBox(height: 24),
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
    final scheme = Theme.of(context).colorScheme;

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
                color: scheme.onSurface,
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
/// AuthStateBanner(
///   error: controller.errorMessage,
///   success: controller.successMessage,
///   onDismiss: controller.clearMessages,
/// )
/// ```
class AuthStateBanner extends StatelessWidget {
  final String? error;
  final String? success;
  final VoidCallback? onDismiss;

  const AuthStateBanner({super.key, this.error, this.success, this.onDismiss});

  @override
  Widget build(BuildContext context) {
    if (error == null && success == null) {
      return const SizedBox.shrink();
    }

    final scheme = Theme.of(context).colorScheme;
    final isError = error != null;
    final message = isError ? error! : success!;
    final backgroundColor = isError ? scheme.error : context.statusColors.success;
    final foregroundColor = isError ? scheme.onError : scheme.onPrimary;
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
          Icon(icon, color: foregroundColor),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              message,
              style: TextStyle(
                color: foregroundColor,
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
          if (onDismiss != null)
            IconButton(
              onPressed: onDismiss,
              icon: Icon(Icons.close, color: foregroundColor),
              padding: EdgeInsets.zero,
              constraints: const BoxConstraints(),
            ),
        ],
      ),
    );
  }
}
